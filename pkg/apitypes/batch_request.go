// Copyright © 2025 Kaleido, Inc.
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

// BatchRequest contains an array of requests to be processed in batch
type BatchRequest struct {
	Requests []*BaseRequest `json:"requests"`
}

// BatchResponseItem represents a single response in a batch operation
type BatchResponseItem struct {
	ID      string      `json:"id,omitempty"`     // ID from the request headers, if present
	Success bool        `json:"success"`          // Whether this request succeeded
	Output  interface{} `json:"output,omitempty"` // The successful response (ManagedTX for SendTransaction/Deploy)
	Error   interface{} `json:"error,omitempty"`  // Error (SubmissionError for SendTransaction/Deploy, string for others) if success is false
}

// BatchResponse contains the results of a batch operation
type BatchResponse struct {
	Responses []*BatchResponseItem `json:"responses"`
}
