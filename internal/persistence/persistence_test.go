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

package persistence

import (
	"testing"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/stretchr/testify/assert"
)

func TestJSONOrStringNull(t *testing.T) {
	assert.Nil(t, JSONOrString(nil))
}

func TestJSONOrStringGoodJSON(t *testing.T) {
	assert.Equal(t, `{ "good":   "json"}`, (string)(*JSONOrString(fftypes.JSONAnyPtr(`{ "good":   "json"}`))))
}

func TestJSONOrStringBadJSON(t *testing.T) {
	assert.Equal(t, "\"\\\"bad\\\":   \\\"json\\\"}\"", (string)(*JSONOrString(fftypes.JSONAnyPtr(`"bad":   "json"}`))))
}
