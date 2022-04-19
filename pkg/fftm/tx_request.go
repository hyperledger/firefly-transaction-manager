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

package fftm

import (
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/hyperledger/firefly/pkg/fftypes"
)

// TransactionRequest is the external interface into sending transactions to the front-side of Transaction Manager
// Note this is a deliberate match for the EthConnect subset that is supported by FireFly core
type TransactionRequest struct {
	Headers struct {
		ID   *fftypes.UUID `json:"id"`
		Type RequestType   `json:"type"`
	} `json:"headers"`
	ffcapi.TransactionInput
}

type RequestType string

const (
	RequestTypeSendTransaction = "SendTransaction"
	RequestTypeQuery           = "Query"
)
