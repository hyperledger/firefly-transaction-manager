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

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlattenStructHelper(t *testing.T) {

	type testType1 struct {
		Field1 string `json:"f1"`
		Field2 int64  `json:"f2"`
	}

	type testType2 struct {
		testType1
		Field3        string `json:"f3"`
		IgnoreMe      string `json:"-"`
		DefaultName   string
		internalField string
		EmptyOne      *string  `json:"e1,omitempty"`
		EmptyTwo      string   `json:"e2,omitempty"`
		EmptyThree    int64    `json:"e3,omitempty"`
		EmptyFour     uint64   `json:"e4,omitempty"`
		EmptyFive     float64  `json:"e5,omitempty"`
		EmptySix      bool     `json:"e6,omitempty"`
		EmptySeven    []string `json:"e7,omitempty"`
	}

	t1 := &testType2{
		testType1: testType1{
			Field1: "val1",
			Field2: 2222,
		},
		Field3:        "val3",
		DefaultName:   "def111",
		internalField: "internal",
		EmptySeven:    []string{},
	}

	m := make(map[string]interface{})
	addJSONFieldsToMap(reflect.ValueOf(t1), m)
	b, err := json.Marshal(m)
	assert.NoError(t, err)
	assert.Equal(t, `{"DefaultName":"def111","f1":"val1","f2":2222,"f3":"val3"}`, string(b))

}

func TestUnknownEmpty(t *testing.T) {
	assert.False(t, isEmptyValue(reflect.ValueOf(make(chan struct{}))))
}
