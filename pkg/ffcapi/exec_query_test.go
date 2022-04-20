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

package ffcapi

import (
	"context"
	"testing"

	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/stretchr/testify/assert"
)

func TestExecQueryOK(t *testing.T) {
	a, cancel := newTestClient(t, &ExecQueryResponse{
		Outputs: []*fftypes.JSONAny{fftypes.JSONAnyPtr("{}")},
	})
	defer cancel()
	res, reason, err := a.ExecQuery(context.Background(), &ExecQueryRequest{})
	assert.NoError(t, err)
	assert.Empty(t, reason)
	assert.Len(t, res.Outputs, 1)
}

func TestExecQueryFail(t *testing.T) {
	a, cancel := newTestClient(t, &ResponseBase{
		ErrorResponse: ErrorResponse{
			Error:  "pop",
			Reason: ErrorReasonInvalidInputs,
		},
	})
	defer cancel()
	_, reason, err := a.ExecQuery(context.Background(), &ExecQueryRequest{})
	assert.Equal(t, ErrorReasonInvalidInputs, reason)
	assert.Regexp(t, "FF201012.*pop", err)
}
