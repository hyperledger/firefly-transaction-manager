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

package ffcapi

type SubmissionError struct {
	Error string `json:"error"`
	// When submissionRejected: true, the failure is considered final and the operation should transition directly to failed.
	// This should be returned in all cases where the FFCAPI connector or underlying blockchain node has evaluated the transaction
	// during the prepare phase, and determined it will fail if it were submitted.
	// The idempotencyKey is "spent" in these scenarios in the FF Core layer, but the transaction is never recorded into the FFTM
	// database, so the nonce is not spent, and the transaction is never submitted.
	SubmissionRejected bool `json:"submissionRejected,omitempty"`
}

func MapSubmissionRejected(reason ErrorReason) bool {
	switch reason {
	case ErrorReasonInvalidInputs,
		ErrorReasonTransactionReverted,
		ErrorReasonInsufficientFunds:
		// These reason codes are considered as rejections of the transaction - see ffcapi.SubmissionError
		return true
	default:
		// Everything else is eligible for idempotent retry of submission
		return false
	}
}
