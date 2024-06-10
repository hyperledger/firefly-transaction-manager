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

package apitypes

import "encoding/json"

// BaseRequest is the common headers to all requests, and captures the full input payload for later decoding to a specific type
type BaseRequest struct {
	headerDecoder
	fullPayload []byte
}

type headerDecoder struct {
	Headers RequestHeaders `json:"headers"`
}

func (br *BaseRequest) UnmarshalJSON(data []byte) error {
	br.fullPayload = data
	return json.Unmarshal(data, &br.headerDecoder)
}

func (br *BaseRequest) UnmarshalTo(o interface{}) error {
	return json.Unmarshal(br.fullPayload, &o)
}

type RequestHeaders struct {
	ID   string      `ffstruct:"fftmrequest" json:"id"`
	Type RequestType `json:"type"`
}

type RequestType string

const (
	RequestTypeSendTransaction    RequestType = "SendTransaction"
	RequestTypeQuery              RequestType = "Query"
	RequestTypeDeploy             RequestType = "DeployContract"
	RequestTypeTransactionReceipt RequestType = "TransactionReceipt"
)
