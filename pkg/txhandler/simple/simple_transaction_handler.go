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
	"github.com/hyperledger/firefly-common/pkg/metric"
	"github.com/hyperledger/firefly-common/pkg/retry"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig" // shouldn't need this if you are developing a customized transaction handler
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"   // replace with your own messages if you are developing a customized transaction handler
	"github.com/hyperledger/firefly-transaction-manager/pkg/apitypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/txhandler"
)

const metricsTransactionsInflightUsed = "tx_in_flight_used_total"
const metricsTransactionsInflightUsedDescription = "Number of transactions currently in flight"

const metricsTransactionsInflightFree = "tx_in_flight_free_total"
const metricsTransactionsInflightFreeDescription = "Number of transactions left in the in flight queue"

const metricsTransactionProcessActionsTotal = "tx_process_action_total"
const metricsTransactionProcessActionsTotalDescription = "Number of transaction process transited into a new stage grouped by stage name"

const metricsLabelNameAction = "action"

const metricsTransactionProcessActionDuration = "tx_process_duration_seconds"
const metricsTransactionProcessActionDurationDescription = "Duration of transaction process grouped by action name"

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
// - It offers three ways of calculating gas price: use a fixed number, use the built-in API of a ethereum connector, use a RESTful gas oracle
// - It resubmits the transaction based on a configured interval until it succeed or fail
func (f *TransactionHandlerFactory) NewTransactionHandler(ctx context.Context, conf config.Section) (txhandler.TransactionHandler, error) {
	gasOracleConfig := conf.SubSection(GasOracleConfig)
	sth := &simpleTransactionHandler{
		resubmitInterval: conf.GetDuration(ResubmitInterval),
		fixedGasPrice:    fftypes.JSONAnyPtr(conf.GetString(FixedGasPrice)),

		gasOracleMethod:        gasOracleConfig.GetString(GasOracleMethod),
		gasOracleQueryInterval: gasOracleConfig.GetDuration(GasOracleQueryInterval),
		gasOracleMode:          gasOracleConfig.GetString(GasOracleMode),

		lockedNonces:   make(map[string]*lockedNonce),
		inflightStale:  make(chan bool, 1),
		inflightUpdate: make(chan bool, 1),
	}

	// check whether a policy engine name is provided
	if config.GetString(tmconfig.DeprecatedPolicyEngineName) != "" {
		log.L(ctx).Warnf("Initializing transaction handler with deprecated configurations. Please use 'transactions.handler' instead")
		sth.nonceStateTimeout = config.GetDuration(tmconfig.DeprecatedTransactionsNonceStateTimeout)
		sth.maxInFlight = config.GetInt(tmconfig.DeprecatedTransactionsMaxInFlight)
		sth.policyLoopInterval = config.GetDuration(tmconfig.DeprecatedPolicyLoopInterval)
		sth.retry = &retry.Retry{
			InitialDelay: config.GetDuration(tmconfig.DeprecatedPolicyLoopRetryInitDelay),
			MaximumDelay: config.GetDuration(tmconfig.DeprecatedPolicyLoopRetryMaxDelay),
			Factor:       config.GetFloat64(tmconfig.DeprecatedPolicyLoopRetryFactor),
		}
	} else {
		// if not, use the new transaction handler configurations
		sth.nonceStateTimeout = conf.GetDuration(NonceStateTimeout)
		sth.maxInFlight = conf.GetInt(MaxInFlight)
		sth.policyLoopInterval = conf.GetDuration(Interval)
		sth.retry = &retry.Retry{
			InitialDelay: conf.GetDuration(RetryInitDelay),
			MaximumDelay: conf.GetDuration(RetryMaxDelay),
			Factor:       conf.GetFloat64(RetryFactor),
		}
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
	toolkit          *txhandler.Toolkit
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
	mtx                     *apitypes.ManagedTX
	trackingTransactionHash string
	lastPolicyCycle         time.Time
	confirmed               bool
	remove                  bool
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

func (sth *simpleTransactionHandler) Init(ctx context.Context, toolkit *txhandler.Toolkit) {
	sth.toolkit = toolkit

	// init metrics
	sth.toolkit.MetricsManager.InitTxHandlerCounterMetricWithLabels(ctx, metricsTransactionProcessActionsTotal, metricsTransactionProcessActionsTotalDescription, []string{metricsLabelNameAction}, true)
	sth.toolkit.MetricsManager.InitTxHandlerHistogramMetricWithLabels(ctx, metricsTransactionProcessActionDuration, metricsTransactionProcessActionDurationDescription, []float64{} /*fallback to default buckets*/, []string{metricsLabelNameAction}, true)
	sth.toolkit.MetricsManager.InitTxHandlerGaugeMetric(ctx, metricsTransactionsInflightUsed, metricsTransactionsInflightUsedDescription, false)
	sth.toolkit.MetricsManager.InitTxHandlerGaugeMetric(ctx, metricsTransactionsInflightFree, metricsTransactionsInflightFreeDescription, false)
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
func (sth *simpleTransactionHandler) HandleNewTransaction(ctx context.Context, txReq *apitypes.TransactionRequest) (mtx *apitypes.ManagedTX, err error) {

	// Prepare the transaction, which will mean we have a transaction that should be submittable.
	// If we fail at this stage, we don't need to write any state as we are sure we haven't submitted
	// anything to the blockchain itself.
	prepared, _, err := sth.toolkit.Connector.TransactionPrepare(ctx, &ffcapi.TransactionPrepareRequest{
		TransactionInput: txReq.TransactionInput,
	})
	if err != nil {
		return nil, err
	}

	return sth.createManagedTx(ctx, txReq.Headers.ID, &txReq.TransactionHeaders, prepared.Gas, prepared.TransactionData)
}
func (sth *simpleTransactionHandler) HandleNewContractDeployment(ctx context.Context, txReq *apitypes.ContractDeployRequest) (mtx *apitypes.ManagedTX, err error) {

	// Prepare the transaction, which will mean we have a transaction that should be submittable.
	// If we fail at this stage, we don't need to write any state as we are sure we haven't submitted
	// anything to the blockchain itself.
	prepared, _, err := sth.toolkit.Connector.DeployContractPrepare(ctx, &txReq.ContractDeployPrepareRequest)
	if err != nil {
		return nil, err
	}

	return sth.createManagedTx(ctx, txReq.Headers.ID, &txReq.TransactionHeaders, prepared.Gas, prepared.TransactionData)
}
func (sth *simpleTransactionHandler) HandleCancelTransaction(ctx context.Context, txID string) (mtx *apitypes.ManagedTX, err error) {
	res := sth.policyEngineAPIRequest(ctx, &policyEngineAPIRequest{
		requestType: policyEngineAPIRequestTypeDelete,
		txID:        txID,
	})
	return res.tx, nil
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

	// Next we update FireFly core with the pre-submitted record pending record, with the allocated nonce.
	// From this point on, we will guide this transaction through to submission.
	// We return an "ack" at this point, and dispatch the work of getting the transaction submitted
	// to the background worker.
	now := fftypes.Now()
	mtx := &apitypes.ManagedTX{
		ID:                 txID, // on input the request ID must be the namespaced operation ID
		Created:            now,
		Updated:            now,
		Nonce:              fftypes.NewFFBigInt(int64(lockedNonce.nonce)),
		Gas:                gas,
		TransactionHeaders: *txHeaders,
		TransactionData:    transactionData,
		Status:             apitypes.TxStatusPending,
	}

	sth.toolkit.TXHistory.SetSubStatus(ctx, mtx, apitypes.TxSubStatusReceived)
	sth.toolkit.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionAssignNonce, fftypes.JSONAnyPtr(`{"nonce":"`+mtx.Nonce.String()+`"}`), nil)

	// Sequencing ID will be added as part of persistence logic - so we have a deterministic order of transactions
	// Note: We must ensure persistence happens this within the nonce lock, to ensure that the nonce sequence and the
	//       global transaction sequence line up.
	if err = sth.toolkit.TXPersistence.WriteTransaction(ctx, mtx, true); err != nil {
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

func (sth *simpleTransactionHandler) submitTX(ctx context.Context, mtx *apitypes.ManagedTX) (reason ffcapi.ErrorReason, err error) {
	sendTX := &ffcapi.TransactionSendRequest{
		TransactionHeaders: mtx.TransactionHeaders,
		GasPrice:           mtx.GasPrice,
		TransactionData:    mtx.TransactionData,
	}
	sendTX.TransactionHeaders.Nonce = (*fftypes.FFBigInt)(mtx.Nonce.Int())
	sendTX.TransactionHeaders.Gas = (*fftypes.FFBigInt)(mtx.Gas.Int())
	log.L(ctx).Debugf("Sending transaction %s at nonce %s / %d (lastSubmit=%s)", mtx.ID, mtx.TransactionHeaders.From, mtx.Nonce.Int64(), mtx.LastSubmit)
	transactionSendStartTime := time.Now()
	res, reason, err := sth.toolkit.Connector.TransactionSend(ctx, sendTX)
	sth.toolkit.MetricsManager.IncTxHandlerCounterMetricWithLabels(ctx, metricsTransactionProcessActionsTotal, map[string]string{metricsLabelNameAction: "transaction_submission"}, &metric.FireflyDefaultLabels{Namespace: mtx.Namespace(ctx)})
	sth.toolkit.MetricsManager.ObserveTxHandlerHistogramMetricWithLabels(ctx, metricsTransactionProcessActionDuration, time.Since(transactionSendStartTime).Seconds(), map[string]string{metricsLabelNameAction: "transaction_submission"}, &metric.FireflyDefaultLabels{Namespace: mtx.Namespace(ctx)})
	if err == nil {
		sth.toolkit.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionSubmitTransaction, fftypes.JSONAnyPtr(`{"reason":"`+string(reason)+`"}`), nil)
		mtx.TransactionHash = res.TransactionHash
		mtx.LastSubmit = fftypes.Now()
	} else {
		sth.toolkit.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionSubmitTransaction, fftypes.JSONAnyPtr(`{"reason":"`+string(reason)+`"}`), fftypes.JSONAnyPtr(`{"error":"`+err.Error()+`"}`))
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
	sth.toolkit.TXHistory.SetSubStatus(ctx, mtx, apitypes.TxSubStatusTracking)
	return "", nil
}

func (sth *simpleTransactionHandler) processTransaction(ctx context.Context, mtx *apitypes.ManagedTX) (update UpdateType, reason ffcapi.ErrorReason, err error) {

	// Simply policy engine allows deletion of the transaction without additional checks ( ensuring the TX has not been submitted / gap filling the nonce etc. )
	if mtx.DeleteRequested != nil {
		return UpdateDelete, "", nil
	}

	if mtx.FirstSubmit == nil {
		// Only calculate gas price here in the simple policy engine
		mtx.GasPrice, err = sth.getGasPrice(ctx, sth.toolkit.Connector)
		if err != nil {
			sth.toolkit.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionRetrieveGasPrice, nil, fftypes.JSONAnyPtr(`{"error":"`+err.Error()+`"}`))
			return UpdateNo, "", err
		}
		sth.toolkit.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionRetrieveGasPrice, fftypes.JSONAnyPtr(`{"gasPrice":`+string(*mtx.GasPrice)+`}`), nil)
		// Submit the first time
		if reason, err := sth.submitTX(ctx, mtx); err != nil {
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
				sth.toolkit.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionTimeout, nil, nil)
				sth.toolkit.TXHistory.SetSubStatus(ctx, mtx, apitypes.TxSubStatusStale)
				mtx.GasPrice, err = sth.getGasPrice(ctx, sth.toolkit.Connector)
				if err != nil {
					sth.toolkit.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionRetrieveGasPrice, nil, fftypes.JSONAnyPtr(`{"error":"`+err.Error()+`"}`))
					return UpdateNo, "", err
				}
				sth.toolkit.TXHistory.AddSubStatusAction(ctx, mtx, apitypes.TxActionRetrieveGasPrice, fftypes.JSONAnyPtr(`{"gasPrice":`+string(*mtx.GasPrice)+`}`), nil)
				if reason, err := sth.submitTX(ctx, mtx); err != nil {
					if reason != ffcapi.ErrorKnownTransaction {
						return UpdateYes, reason, err
					}
				}
				sth.toolkit.TXHistory.SetSubStatus(ctx, mtx, apitypes.TxSubStatusTracking)
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
	res, err := sth.gasOracleClient.R().
		Execute(sth.gasOracleMethod, "")
	if err != nil {
		return nil, i18n.WrapError(ctx, err, tmmsgs.MsgErrorQueryingGasOracleAPI, -1, err.Error())
	}
	if res.IsError() {
		return nil, i18n.WrapError(ctx, err, tmmsgs.MsgErrorQueryingGasOracleAPI, res.StatusCode(), res.RawResponse)
	}
	// Parse the response body as JSON
	var data map[string]interface{}
	err = json.Unmarshal(res.Body(), &data)
	if err != nil {
		return nil, i18n.WrapError(ctx, err, tmmsgs.MsgInvalidJSONGasObject)
	}
	buff := new(bytes.Buffer)
	err = sth.gasOracleTemplate.Execute(buff, data)
	if err != nil {
		return nil, i18n.WrapError(ctx, err, tmmsgs.MsgGasOracleResultError)
	}
	return fftypes.JSONAnyPtr(buff.String()), nil
}
