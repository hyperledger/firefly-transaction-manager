// Copyright © 2023 Kaleido, Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package simple

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"sync"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/ffresty"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-common/pkg/retry"
	"github.com/hyperledger/firefly-transaction-manager/internal/persistence"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/txhandler"
)

// UpdateType informs FFTM whether the transaction needs an update to be persisted after this execution of the policy engine
type UpdateType int

const (
	UpdateNo     UpdateType = iota // Instructs that no update is necessary
	UpdateYes                      // Instructs that the transaction should be updated in persistence
	UpdateDelete                   // Instructs that the transaction should be removed completely from persistence - generally only returned when TX status is TxStatusDeleteRequested
)

type TransactionHandlerFactory struct{}

func (f *TransactionHandlerFactory) Name() string {
	return "simple"
}

// simpleTransactionHandler is a base transaction handler forming an example for extension:
// - It uses a public gas estimation
// - It submits the transaction once
// - It logs errors transactions breach certain configured thresholds of staleness
func (f *TransactionHandlerFactory) NewTransactionHandler(ctx context.Context, conf config.Section) (txhandler.TransactionHandler, error) {
	gasOracleConfig := conf.SubSection(GasOracleConfig)
	sth := &simpleTransactionHandler{
		resubmitInterval: conf.GetDuration(ResubmitInterval),
		fixedGasPrice:    fftypes.JSONAnyPtr(conf.GetString(FixedGasPrice)),

		gasOracleMethod:        gasOracleConfig.GetString(GasOracleMethod),
		gasOracleQueryInterval: gasOracleConfig.GetDuration(GasOracleQueryInterval),
		gasOracleMode:          gasOracleConfig.GetString(GasOracleMode),

		lockedNonces:      make(map[string]*lockedNonce),
		nonceStateTimeout: config.GetDuration(tmconfig.TransactionsNonceStateTimeout),
		maxInFlight:       config.GetInt(tmconfig.TransactionsMaxInFlight),
		inflightStale:     make(chan bool, 1),
		inflightUpdate:    make(chan bool, 1),

		policyLoopInterval: config.GetDuration(tmconfig.PolicyLoopInterval),
		retry: &retry.Retry{
			InitialDelay: config.GetDuration(tmconfig.PolicyLoopRetryInitDelay),
			MaximumDelay: config.GetDuration(tmconfig.PolicyLoopRetryMaxDelay),
			Factor:       config.GetFloat64(tmconfig.PolicyLoopRetryFactor),
		},
	}
	switch sth.gasOracleMode {
	case GasOracleModeConnector:
		// No initialization required
	case GasOracleModeRESTAPI:
		sth.gasOracleClient = ffresty.New(ctx, gasOracleConfig)
		templateString := gasOracleConfig.GetString(GasOracleTemplate)
		if templateString == "" {
			return nil, i18n.NewError(ctx, tmmsgs.MsgMissingGOTemplate)
		}
		template, err := template.New("").Funcs(sprig.FuncMap()).Parse(templateString)
		if err != nil {
			return nil, i18n.NewError(ctx, tmmsgs.MsgBadGOTemplate, err)
		}
		sth.gasOracleTemplate = template
	default:
		if sth.fixedGasPrice.IsNil() {
			return nil, i18n.NewError(ctx, tmmsgs.MsgNoGasConfigSetForTransactionHandler)
		}
	}
	return sth, nil
}

type simpleTransactionHandler struct {
	ctx              context.Context
	tkAPI            *txhandler.ToolkitAPI
	fixedGasPrice    *fftypes.JSONAny
	resubmitInterval time.Duration

	gasOracleMode          string
	gasOracleClient        *resty.Client
	gasOracleMethod        string
	gasOracleTemplate      *template.Template
	gasOracleQueryInterval time.Duration
	gasOracleQueryValue    *fftypes.JSONAny
	gasOracleLastQueryTime *fftypes.FFTime

	lockedNonces            map[string]*lockedNonce
	policyLoopInterval      time.Duration
	policyLoopDone          chan struct{}
	nonceStateTimeout       time.Duration
	inflightStale           chan bool
	inflightUpdate          chan bool
	mux                     sync.Mutex
	inflight                []*pendingState
	policyEngineAPIRequests []*policyEngineAPIRequest
	maxInFlight             int
	retry                   *retry.Retry
}
type pendingState struct {
	mtx             *apitypes.ManagedTX
	lastPolicyCycle time.Time
	confirmed       bool
	remove          bool
}

type simplePolicyInfo struct {
	LastWarnTime *fftypes.FFTime `json:"lastWarnTime"`
}

// withPolicyInfo is a convenience helper to run some logic that accesses/updates our policy section
func (sth *simpleTransactionHandler) withPolicyInfo(ctx context.Context, mtx *apitypes.ManagedTX, fn func(info *simplePolicyInfo) (update UpdateType, reason ffcapi.ErrorReason, err error)) (update UpdateType, reason ffcapi.ErrorReason, err error) {
	var info simplePolicyInfo
	infoBytes := []byte(mtx.PolicyInfo.String())
	if len(infoBytes) > 0 {
		err := json.Unmarshal(infoBytes, &info)
		if err != nil {
			log.L(ctx).Warnf("Failed to parse existing info `%s`: %s", infoBytes, err)
		}
	}
	update, reason, err = fn(&info)
	if update != UpdateNo {
		infoBytes, _ = json.Marshal(&info)
		mtx.PolicyInfo = fftypes.JSONAnyPtrBytes(infoBytes)
	}
	return update, reason, err
}

func (sth *simpleTransactionHandler) Init(ctx context.Context, tkAPI *txhandler.ToolkitAPI) {
	sth.tkAPI = tkAPI
}

func (sth *simpleTransactionHandler) Start(ctx context.Context) (done <-chan struct{}, err error) {
	if sth.ctx == nil { // only start once
		sth.ctx = ctx // set the context for policy loop
		sth.policyLoopDone = make(chan struct{})
		sth.markInflightStale()
		go sth.policyLoop()
	}
	return sth.policyLoopDone, nil
}
func (sth *simpleTransactionHandler) RegisterNewTransaction(ctx context.Context, txReq *apitypes.TransactionRequest) (mtx *apitypes.ManagedTX, err error) {

	// Prepare the transaction, which will mean we have a transaction that should be submittable.
	// If we fail at this stage, we don't need to write any state as we are sure we haven't submitted
	// anything to the blockchain itself.
	prepared, _, err := sth.tkAPI.Connector.TransactionPrepare(ctx, &ffcapi.TransactionPrepareRequest{
		TransactionInput: txReq.TransactionInput,
	})
	if err != nil {
		return nil, err
	}

	return sth.createManagedTx(ctx, txReq.Headers.ID, &txReq.TransactionHeaders, prepared.Gas, prepared.TransactionData)
}
func (sth *simpleTransactionHandler) RegisterNewContractDeployment(ctx context.Context, txReq *apitypes.ContractDeployRequest) (mtx *apitypes.ManagedTX, err error) {

	// Prepare the transaction, which will mean we have a transaction that should be submittable.
	// If we fail at this stage, we don't need to write any state as we are sure we haven't submitted
	// anything to the blockchain itself.
	prepared, _, err := sth.tkAPI.Connector.DeployContractPrepare(ctx, &txReq.ContractDeployPrepareRequest)
	if err != nil {
		return nil, err
	}

	return sth.createManagedTx(ctx, txReq.Headers.ID, &txReq.TransactionHeaders, prepared.Gas, prepared.TransactionData)
}
func (sth *simpleTransactionHandler) CancelTransaction(ctx context.Context, txID string) (mtx *apitypes.ManagedTX, err error) {
	res := sth.policyEngineAPIRequest(ctx, &policyEngineAPIRequest{
		requestType: policyEngineAPIRequestTypeDelete,
		txID:        txID,
	})
	return res.tx, nil
}

func (sth *simpleTransactionHandler) GetTransactionByID(ctx context.Context, txID string) (transaction *apitypes.ManagedTX, err error) {
	tx, err := sth.tkAPI.Persistence.GetTransactionByID(ctx, txID)
	if err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, i18n.NewError(ctx, tmmsgs.MsgTransactionNotFound, txID)
	}
	return tx, nil
}
func (sth *simpleTransactionHandler) GetTransactions(ctx context.Context, afterStr, signer string, pending bool, limit int, direction persistence.SortDirection) (transactions []*apitypes.ManagedTX, err error) {
	var afterTx *apitypes.ManagedTX
	if afterStr != "" {
		// Get the transaction, as we need this to exist to pick the right field depending on the index that's been chosen
		afterTx, err = sth.tkAPI.Persistence.GetTransactionByID(ctx, afterStr)
		if err != nil {
			return nil, err
		}
		if afterTx == nil {
			return nil, i18n.NewError(ctx, tmmsgs.MsgPaginationErrTxNotFound, afterStr)
		}
	}
	switch {
	case signer != "" && pending:
		return nil, i18n.NewError(ctx, tmmsgs.MsgTXConflictSignerPending)
	case signer != "":
		var afterNonce *fftypes.FFBigInt
		if afterTx != nil {
			afterNonce = afterTx.Nonce
		}
		return sth.tkAPI.Persistence.ListTransactionsByNonce(ctx, signer, afterNonce, limit, direction)
	case pending:
		var afterSequence *fftypes.UUID
		if afterTx != nil {
			afterSequence = afterTx.SequenceID
		}
		return sth.tkAPI.Persistence.ListTransactionsPending(ctx, afterSequence, limit, direction)
	default:
		return sth.tkAPI.Persistence.ListTransactionsByCreateTime(ctx, afterTx, limit, direction)
	}
}

func (sth *simpleTransactionHandler) createManagedTx(ctx context.Context, txID string, txHeaders *ffcapi.TransactionHeaders, gas *fftypes.FFBigInt, transactionData string) (*apitypes.ManagedTX, error) {

	// The request ID is the primary ID, and should be supplied by the user for idempotence
	if txID == "" {
		txID = fftypes.NewUUID().String()
	}

	// First job is to assign the next nonce to this request.
	// We block any further sends on this nonce until we've got this one successfully into the node, or
	// fail deterministically in a way that allows us to return it.
	lockedNonce, err := sth.assignAndLockNonce(ctx, txID, txHeaders.From)
	if err != nil {
		return nil, err
	}
	// We will call markSpent() once we reach the point the nonce has been used
	defer lockedNonce.complete(ctx)

	// Sequencing ID is always generated by us - so we have a deterministic order of transactions
	// Note: We must allocate this within the nonce lock, to ensure that the nonce sequence and the
	//       global transaction sequence line up.
	seqID := apitypes.NewULID()

	// Next we update FireFly core with the pre-submitted record pending record, with the allocated nonce.
	// From this point on, we will guide this transaction through to submission.
	// We return an "ack" at this point, and dispatch the work of getting the transaction submitted
	// to the background worker.
	now := fftypes.Now()
	mtx := &apitypes.ManagedTX{
		ID:                 txID, // on input the request ID must be the namespaced operation ID
		Created:            now,
		Updated:            now,
		SequenceID:         seqID,
		Nonce:              fftypes.NewFFBigInt(int64(lockedNonce.nonce)),
		Gas:                gas,
		TransactionHeaders: *txHeaders,
		TransactionData:    transactionData,
		Status:             apitypes.TxStatusPending,
	}

	sth.tkAPI.TXHistory.SetSubStatus(ctx, mtx, apitypes.TxSubStatusReceived)
	sth.tkAPI.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionAssignNonce, fftypes.JSONAnyPtr(`{"nonce":"`+mtx.Nonce.String()+`"}`), nil)

	if err = sth.tkAPI.Persistence.WriteTransaction(ctx, mtx, true); err != nil {
		return nil, err
	}
	log.L(ctx).Infof("Tracking transaction %s at nonce %s / %d", mtx.ID, mtx.TransactionHeaders.From, mtx.Nonce.Int64())
	sth.markInflightStale()

	// Ok - we've spent it. The rest of the processing will be triggered off of lockedNonce
	// completion adding this transaction to the pool (and/or the change event that comes in from
	// FireFly core from the update to the transaction)
	lockedNonce.spent = mtx
	return mtx, nil
}

func (sth *simpleTransactionHandler) submitTX(ctx context.Context, tk *txhandler.ToolkitAPI, mtx *apitypes.ManagedTX) (reason ffcapi.ErrorReason, err error) {
	sendTX := &ffcapi.TransactionSendRequest{
		TransactionHeaders: mtx.TransactionHeaders,
		GasPrice:           mtx.GasPrice,
		TransactionData:    mtx.TransactionData,
	}
	sendTX.TransactionHeaders.Nonce = (*fftypes.FFBigInt)(mtx.Nonce.Int())
	sendTX.TransactionHeaders.Gas = (*fftypes.FFBigInt)(mtx.Gas.Int())
	log.L(ctx).Debugf("Sending transaction %s at nonce %s / %d (lastSubmit=%s)", mtx.ID, mtx.TransactionHeaders.From, mtx.Nonce.Int64(), mtx.LastSubmit)
	res, reason, err := tk.Connector.TransactionSend(ctx, sendTX)
	if err == nil {
		tk.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionSubmitTransaction, fftypes.JSONAnyPtr(`{"reason":"`+string(reason)+`"}`), nil)
		mtx.TransactionHash = res.TransactionHash
		mtx.LastSubmit = fftypes.Now()
	} else {
		tk.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionSubmitTransaction, fftypes.JSONAnyPtr(`{"reason":"`+string(reason)+`"}`), fftypes.JSONAnyPtr(`{"error":"`+err.Error()+`"}`))
		// We have some simple rules for handling reasons from the connector, which could be enhanced by extending the connector.
		switch reason {
		case ffcapi.ErrorKnownTransaction, ffcapi.ErrorReasonNonceTooLow:
			// If we already have a transaction hash, this is fine - we just return as if we submitted it
			if mtx.TransactionHash != "" {
				log.L(ctx).Debugf("Transaction %s at nonce %s / %d known with hash: %s (%s)", mtx.ID, mtx.TransactionHeaders.From, mtx.Nonce.Int64(), mtx.TransactionHash, err)
				return "", nil
			}
			// Note: to cover the edge case where we had a timeout or other failure during the initial TransactionSend,
			//       a policy engine implementation would need to be able to re-calculate the hash that we would expect for the transaction.
			//       This would require a new FFCAPI API to calculate that hash, which requires the connector to perform the signing
			//       without submission to the node. For example using `eth_signTransaction` for EVM JSON/RPC.
			return reason, err
		default:
			return reason, err
		}
	}
	log.L(ctx).Infof("Transaction %s at nonce %s / %d submitted. Hash: %s", mtx.ID, mtx.TransactionHeaders.From, mtx.Nonce.Int64(), mtx.TransactionHash)
	tk.TXHistory.SetSubStatus(ctx, mtx, apitypes.TxSubStatusTracking)
	return "", nil
}

func (sth *simpleTransactionHandler) processTransaction(ctx context.Context, tk *txhandler.ToolkitAPI, mtx *apitypes.ManagedTX) (update UpdateType, reason ffcapi.ErrorReason, err error) {

	// Simply policy engine allows deletion of the transaction without additional checks ( ensuring the TX has not been submitted / gap filling the nonce etc. )
	if mtx.DeleteRequested != nil {
		return UpdateDelete, "", nil
	}

	// Simple policy engine only submits once.
	if mtx.FirstSubmit == nil {
		// Only calculate gas price here in the simple policy engine
		mtx.GasPrice, err = sth.getGasPrice(ctx, tk.Connector)
		if err != nil {
			tk.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionRetrieveGasPrice, nil, fftypes.JSONAnyPtr(`{"error":"`+err.Error()+`"}`))
			return UpdateNo, "", err
		}
		tk.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionRetrieveGasPrice, fftypes.JSONAnyPtr(`{"gasPrice":`+string(*mtx.GasPrice)+`}`), nil)
		// Submit the first time
		if reason, err := sth.submitTX(ctx, tk, mtx); err != nil {
			return UpdateYes, reason, err
		}
		mtx.FirstSubmit = mtx.LastSubmit
		return UpdateYes, "", nil

	} else if mtx.Receipt == nil {

		// A more sophisticated policy engine would look at the reason for the lack of a receipt, and consider taking progressive
		// action such as increasing the gas cost slowly over time. This simple example shows how the policy engine
		// can use the FireFly core operation as a store for its historical state/decisions (in this case the last time we warned).
		return sth.withPolicyInfo(ctx, mtx, func(info *simplePolicyInfo) (update UpdateType, reason ffcapi.ErrorReason, err error) {
			lastWarnTime := info.LastWarnTime
			if lastWarnTime == nil {
				lastWarnTime = mtx.FirstSubmit
			}
			now := fftypes.Now()
			if now.Time().Sub(*lastWarnTime.Time()) > sth.resubmitInterval {
				secsSinceSubmit := float64(now.Time().Sub(*mtx.FirstSubmit.Time())) / float64(time.Second)
				log.L(ctx).Infof("Transaction %s at nonce %s / %d has not been mined after %.2fs", mtx.ID, mtx.TransactionHeaders.From, mtx.Nonce.Int64(), secsSinceSubmit)
				info.LastWarnTime = now
				// We do a resubmit at this point - as it might no longer be in the TX pool
				tk.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionTimeout, nil, nil)
				tk.TXHistory.SetSubStatus(ctx, mtx, apitypes.TxSubStatusStale)
				mtx.GasPrice, err = sth.getGasPrice(ctx, tk.Connector)
				if err != nil {
					tk.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionRetrieveGasPrice, nil, fftypes.JSONAnyPtr(`{"error":"`+err.Error()+`"}`))
					return UpdateNo, "", err
				}
				tk.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionRetrieveGasPrice, fftypes.JSONAnyPtr(`{"gasPrice":`+string(*mtx.GasPrice)+`}`), nil)
				if reason, err := sth.submitTX(ctx, tk, mtx); err != nil {
					if reason != ffcapi.ErrorKnownTransaction {
						return UpdateYes, reason, err
					}
				}
				tk.TXHistory.SetSubStatus(ctx, mtx, apitypes.TxSubStatusTracking)
				return UpdateYes, "", nil
			}
			return UpdateNo, "", nil
		})

	}
	// No action in the case we have a receipt
	return UpdateNo, "", nil
}

// getGasPrice either uses a fixed gas price, or invokes a gas station API
func (sth *simpleTransactionHandler) getGasPrice(ctx context.Context, cAPI ffcapi.API) (gasPrice *fftypes.JSONAny, err error) {
	if sth.gasOracleQueryValue != nil && sth.gasOracleLastQueryTime != nil &&
		time.Since(*sth.gasOracleLastQueryTime.Time()) < sth.gasOracleQueryInterval {
		return sth.gasOracleQueryValue, nil
	}
	switch sth.gasOracleMode {
	case GasOracleModeRESTAPI:
		// Make a REST call against an endpoint, and extract a value/structure to pass to the connector
		gasPrice, err := sth.getGasPriceAPI(ctx)
		if err != nil {
			return nil, err
		}
		sth.gasOracleQueryValue = gasPrice
		sth.gasOracleLastQueryTime = fftypes.Now()
		return sth.gasOracleQueryValue, nil
	case GasOracleModeConnector:
		// Call the connector
		res, _, err := cAPI.GasPriceEstimate(ctx, &ffcapi.GasPriceEstimateRequest{})
		if err != nil {
			return nil, err
		}
		sth.gasOracleQueryValue = res.GasPrice
		sth.gasOracleLastQueryTime = fftypes.Now()
		return sth.gasOracleQueryValue, nil
	default:
		// Disabled - just a fixed value - note that the fixed value can be any JSON structure,
		// as interpreted by the connector. For example EVMConnect support a simple value, or a
		// post EIP-1559 structure.
		return sth.fixedGasPrice, nil
	}
}

func (sth *simpleTransactionHandler) getGasPriceAPI(ctx context.Context) (gasPrice *fftypes.JSONAny, err error) {
	var jsonResponse map[string]interface{}
	res, err := sth.gasOracleClient.R().
		SetResult(&jsonResponse).
		Execute(sth.gasOracleMethod, "")
	if err != nil {
		return nil, i18n.WrapError(ctx, err, tmmsgs.MsgErrorQueryingGasOracleAPI, -1, err.Error())
	}
	if res.IsError() {
		return nil, i18n.WrapError(ctx, err, tmmsgs.MsgErrorQueryingGasOracleAPI, res.StatusCode(), res.RawResponse)
	}
	buff := new(bytes.Buffer)
	err = sth.gasOracleTemplate.Execute(buff, jsonResponse)
	if err != nil {
		return nil, i18n.WrapError(ctx, err, tmmsgs.MsgGasOracleResultError)
	}
	return fftypes.JSONAnyPtr(buff.String()), nil
}
