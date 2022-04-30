// Copyright © 2022 Kaleido, Inc.
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
	"context"
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly-transaction-manager/pkg/fftm"
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengine"
	"github.com/hyperledger/firefly/pkg/config"
	"github.com/hyperledger/firefly/pkg/ffresty"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/hyperledger/firefly/pkg/i18n"
	"github.com/hyperledger/firefly/pkg/log"
	"github.com/tidwall/gjson"
)

type PolicyEngineFactory struct{}

func (f *PolicyEngineFactory) Name() string {
	return "simple"
}

// simplePolicyEngine is a base policy engine forming an example for extension:
// - It uses a public gas estimation
// - It submits the transaction once
// - It logs errors transactions breach certain configured thresholds of staleness
func (f *PolicyEngineFactory) NewPolicyEngine(ctx context.Context, prefix config.Prefix) (pe policyengine.PolicyEngine, err error) {
	gasOraclePrefix := prefix.SubPrefix(GasOraclePrefix)
	p := &simplePolicyEngine{
		warnInterval:  prefix.GetDuration(WarnInterval),
		fixedGasPrice: fftypes.JSONAnyPtr(prefix.GetString(FixedGasPrice)),

		gasOracleMethod:        gasOraclePrefix.GetString(GasOracleMethod),
		gasOracleGJSON:         gasOraclePrefix.GetString(GasOracleGJSON),
		gasOracleQueryInterval: gasOraclePrefix.GetDuration(GasOracleQueryInterval),
		gasOracleMode:          gasOraclePrefix.GetString(GasOracleMode),
	}
	switch p.gasOracleMode {
	case GasOracleModeConnector:
		// No initialization required
	case GasOracleModeRESTAPI:
		p.gasOracleClient = ffresty.New(ctx, gasOraclePrefix)
	default:
		if p.fixedGasPrice.IsNil() {
			return nil, i18n.NewError(ctx, tmmsgs.MsgNoGasConfigSetForPolicyEngine)
		}
	}
	return p, nil
}

type simplePolicyEngine struct {
	fixedGasPrice *fftypes.JSONAny
	warnInterval  time.Duration

	gasOracleMode          string
	gasOracleClient        *resty.Client
	gasOracleMethod        string
	gasOracleGJSON         string
	gasOracleQueryInterval time.Duration
	gasOracleQueryValue    *fftypes.JSONAny
	gasOracleLastQueryTime *fftypes.FFTime
}

type simplePolicyInfo struct {
	LastWarnTime *fftypes.FFTime `json:"lastWarnTime"`
}

// withPolicyInfo is a convenience helper to run some logic that accesses/updates our policy section
func (p *simplePolicyEngine) withPolicyInfo(ctx context.Context, mtx *fftm.ManagedTXOutput, fn func(info *simplePolicyInfo) (updated bool, reason ffcapi.ErrorReason, err error)) (updated bool, reason ffcapi.ErrorReason, err error) {
	var info simplePolicyInfo
	infoBytes := []byte(mtx.PolicyInfo.String())
	if len(infoBytes) > 0 {
		err := json.Unmarshal(infoBytes, &info)
		if err != nil {
			log.L(ctx).Warnf("Failed to parse existing info `%s`: %s", infoBytes, err)
		}
	}
	updated, reason, err = fn(&info)
	if updated {
		infoBytes, _ = json.Marshal(&info)
		mtx.PolicyInfo = fftypes.JSONAnyPtrBytes(infoBytes)
	}
	return updated, reason, err
}

func (p *simplePolicyEngine) Execute(ctx context.Context, cAPI ffcapi.API, mtx *fftm.ManagedTXOutput) (updated bool, reason ffcapi.ErrorReason, err error) {
	// Simple policy engine only submits once.
	if mtx.FirstSubmit == nil {

		mtx.GasPrice, err = p.getGasPrice(ctx, cAPI)
		if err != nil {
			return false, "", err
		}
		sendTX := &ffcapi.SendTransactionRequest{
			TransactionHeaders: mtx.Request.TransactionHeaders,
			GasPrice:           mtx.GasPrice,
			TransactionData:    mtx.TransactionData,
		}
		sendTX.TransactionHeaders.Nonce = (*fftypes.FFBigInt)(mtx.Nonce.Int())
		sendTX.TransactionHeaders.Gas = (*fftypes.FFBigInt)(mtx.Gas.Int())
		log.L(ctx).Infof("Sending transaction: %+v", sendTX)
		res, reason, err := cAPI.SendTransaction(ctx, sendTX)
		if err != nil {
			// A more sophisticated policy engine would consider the reason here, and potentially adjust the transaction for future attempts
			return false, reason, err
		}
		log.L(ctx).Infof("Transaction hash=%s", res.TransactionHash)
		mtx.TransactionHash = res.TransactionHash
		mtx.FirstSubmit = fftypes.Now()
		mtx.LastSubmit = mtx.FirstSubmit
		return true, "", nil

	} else if mtx.Receipt == nil {

		// A more sophisticated policy engine would look at the reason for the lack of a receipt, and consider taking progressive
		// action such as increasing the gas cost slowly over time. This simple example shows how the policy engine
		// can use the FireFly core operation as a store for its historical state/decisions (in this case the last time we warned).
		return p.withPolicyInfo(ctx, mtx, func(info *simplePolicyInfo) (updated bool, reason ffcapi.ErrorReason, err error) {
			lastWarnTime := info.LastWarnTime
			if lastWarnTime == nil {
				lastWarnTime = mtx.FirstSubmit
			}
			now := fftypes.Now()
			if now.Time().Sub(*lastWarnTime.Time()) > p.warnInterval {
				secsSinceSubmit := float64(now.Time().Sub(*mtx.FirstSubmit.Time())) / float64(time.Second)
				log.L(ctx).Warnf("Transaction %s (op=%s) has not been mined after %.2fs", mtx.TransactionHash, mtx.ID, secsSinceSubmit)
				info.LastWarnTime = now
				return true, "", nil
			}
			return false, "", nil
		})

	}
	// No action in the case we have a receipt
	return false, "", nil
}

// getGasPrice either uses a fixed gas price, or invokes a gas station API
func (p *simplePolicyEngine) getGasPrice(ctx context.Context, cAPI ffcapi.API) (gasPrice *fftypes.JSONAny, err error) {
	if p.gasOracleQueryValue != nil && p.gasOracleLastQueryTime != nil &&
		time.Since(*p.gasOracleLastQueryTime.Time()) < p.gasOracleQueryInterval {
		return p.gasOracleQueryValue, nil
	}
	switch p.gasOracleMode {
	case GasOracleModeRESTAPI:
		// Make a REST call against an endpoint, and extract a value/structure to pass to the connector
		gasPrice, err := p.getGasPriceAPI(ctx)
		if err != nil {
			return nil, err
		}
		p.gasOracleQueryValue = gasPrice
		p.gasOracleLastQueryTime = fftypes.Now()
		return p.gasOracleQueryValue, nil
	case GasOracleModeConnector:
		// Call the connector
		res, _, err := cAPI.GetGasPrice(ctx, &ffcapi.GetGasPriceRequest{})
		if err != nil {
			return nil, err
		}
		p.gasOracleQueryValue = res.GasPrice
		p.gasOracleLastQueryTime = fftypes.Now()
		return p.gasOracleQueryValue, nil
	default:
		// Disabled - rust a fixed value
		return p.fixedGasPrice, nil
	}
}

func (p *simplePolicyEngine) getGasPriceAPI(ctx context.Context) (gasPrice *fftypes.JSONAny, err error) {
	res, err := p.gasOracleClient.R().
		SetDoNotParseResponse(true).
		Execute(p.gasOracleMethod, "")
	var rawResponse []byte
	if err == nil {
		rawResponse, err = ioutil.ReadAll(res.RawBody())
	}
	if err != nil {
		return nil, i18n.WrapError(ctx, err, tmmsgs.MsgErrorQueryingGasOracleAPI, -1, rawResponse)
	}
	if res.IsError() {
		return nil, i18n.WrapError(ctx, err, tmmsgs.MsgErrorQueryingGasOracleAPI, res.StatusCode(), rawResponse)
	}
	return fftypes.JSONAnyPtr(gjson.Get(string(rawResponse), p.gasOracleGJSON).Raw), nil
}
