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
	"testing"
	"time"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/stretchr/testify/assert"
)

func TestCheckUpdateString(t *testing.T) {
	var val1 = "val1"
	var val2 = "val2"
	var pVal3 *string

	changed := CheckUpdateString(false, &pVal3, nil, nil, "defVal")
	assert.Equal(t, "defVal", *pVal3) // the default won
	assert.True(t, changed)

	changed = CheckUpdateString(false, &pVal3, &val1, &val2, "differentDefault")
	assert.Equal(t, "val2", *pVal3) // val2 won
	assert.True(t, changed)

	changed = CheckUpdateString(true, &pVal3, &val2, &val2, "differentDefault")
	assert.Equal(t, "val2", *pVal3)
	assert.True(t, changed) // because it was already changed

	changed = CheckUpdateString(false, &pVal3, &val2, &val2, "differentDefault")
	assert.Equal(t, "val2", *pVal3)
	assert.False(t, changed) // the value hasn't changed

	changed = CheckUpdateString(false, &pVal3, &val1, nil, "differentDefault")
	assert.Equal(t, "val1", *pVal3) // val1 won
	assert.False(t, changed)        // which was the current value
}

func TestCheckUpdateBool(t *testing.T) {
	var val1 = true
	var val2 = false
	var pVal3 *bool

	changed := CheckUpdateBool(false, &pVal3, nil, nil, true)
	assert.Equal(t, true, *pVal3) // the default won
	assert.True(t, changed)

	changed = CheckUpdateBool(false, &pVal3, &val1, &val2, false)
	assert.Equal(t, false, *pVal3) // val2 won
	assert.True(t, changed)

	changed = CheckUpdateBool(true, &pVal3, &val2, &val2, false)
	assert.Equal(t, false, *pVal3)
	assert.True(t, changed) // because it was already changed

	changed = CheckUpdateBool(false, &pVal3, &val2, &val2, false)
	assert.Equal(t, false, *pVal3)
	assert.False(t, changed) // the value hasn't changed

	changed = CheckUpdateBool(false, &pVal3, &val1, nil, false)
	assert.Equal(t, true, *pVal3) // val1 won
	assert.False(t, changed)      // which was the current value
}

func TestCheckUpdateInt64(t *testing.T) {
	var val1 uint64 = 1111
	var val2 uint64 = 2222
	var pVal3 *uint64

	changed := CheckUpdateUint64(false, &pVal3, nil, nil, 3333)
	assert.Equal(t, uint64(3333), *pVal3) // the default won
	assert.True(t, changed)

	changed = CheckUpdateUint64(false, &pVal3, &val1, &val2, 4444)
	assert.Equal(t, uint64(2222), *pVal3) // val2 won
	assert.True(t, changed)

	changed = CheckUpdateUint64(true, &pVal3, &val2, &val2, 4444)
	assert.Equal(t, uint64(2222), *pVal3)
	assert.True(t, changed) // because it was already changed

	changed = CheckUpdateUint64(false, &pVal3, &val2, &val2, 4444)
	assert.Equal(t, uint64(2222), *pVal3)
	assert.False(t, changed) // the value hasn't changed

	changed = CheckUpdateUint64(false, &pVal3, &val1, nil, 4444)
	assert.Equal(t, uint64(1111), *pVal3) // val1 won
	assert.False(t, changed)              // which was the current value
}

func TestCheckUpdateDuration(t *testing.T) {
	var val1 fftypes.FFDuration = fftypes.FFDuration(1111 * time.Second)
	var val2 fftypes.FFDuration = fftypes.FFDuration(2222 * time.Second)
	var pVal3 *fftypes.FFDuration

	changed := CheckUpdateDuration(false, &pVal3, nil, nil, fftypes.FFDuration(3333*time.Second))
	assert.Equal(t, fftypes.FFDuration(3333*time.Second), *pVal3) // the default won
	assert.True(t, changed)

	changed = CheckUpdateDuration(false, &pVal3, &val1, &val2, fftypes.FFDuration(4444*time.Second))
	assert.Equal(t, fftypes.FFDuration(2222*time.Second), *pVal3) // val2 won
	assert.True(t, changed)

	changed = CheckUpdateDuration(true, &pVal3, &val2, &val2, fftypes.FFDuration(4444*time.Second))
	assert.Equal(t, fftypes.FFDuration(2222*time.Second), *pVal3)
	assert.True(t, changed) // because it was already changed

	changed = CheckUpdateDuration(false, &pVal3, &val2, &val2, fftypes.FFDuration(4444*time.Second))
	assert.Equal(t, fftypes.FFDuration(2222*time.Second), *pVal3)
	assert.False(t, changed) // the value hasn't changed

	changed = CheckUpdateDuration(false, &pVal3, &val1, nil, fftypes.FFDuration(4444*time.Second))
	assert.Equal(t, fftypes.FFDuration(1111*time.Second), *pVal3) // val1 won
	assert.False(t, changed)                                      // which was the current value
}

func TestCheckUpdateEnum(t *testing.T) {
	var val1 fftypes.FFEnum = fftypes.FFEnum("val1")
	var val2 fftypes.FFEnum = fftypes.FFEnum("val2")
	var pVal3 *fftypes.FFEnum

	changed := CheckUpdateEnum(false, &pVal3, nil, nil, fftypes.FFEnum("def1"))
	assert.Equal(t, fftypes.FFEnum("def1"), *pVal3) // the default won
	assert.True(t, changed)

	changed = CheckUpdateEnum(false, &pVal3, &val1, &val2, fftypes.FFEnum("def2"))
	assert.Equal(t, fftypes.FFEnum("val2"), *pVal3) // val2 won
	assert.True(t, changed)

	changed = CheckUpdateEnum(true, &pVal3, &val2, &val2, fftypes.FFEnum("def2"))
	assert.Equal(t, fftypes.FFEnum("val2"), *pVal3)
	assert.True(t, changed) // because it was already changed

	changed = CheckUpdateEnum(false, &pVal3, &val2, &val2, fftypes.FFEnum("def2"))
	assert.Equal(t, fftypes.FFEnum("val2"), *pVal3)
	assert.False(t, changed) // the value hasn't changed

	changed = CheckUpdateEnum(false, &pVal3, &val1, nil, fftypes.FFEnum("def2"))
	assert.Equal(t, fftypes.FFEnum("val1"), *pVal3) // val1 won
	assert.False(t, changed)                        // which was the current value
}

func TestCheckUpdateStringMap(t *testing.T) {
	val1 := map[string]string{"key1": "val1"}
	val2 := map[string]string{"key2": "val2"}
	var pVal3 map[string]string

	changed := CheckUpdateStringMap(false, &pVal3, val1, val2)
	assert.Equal(t, map[string]string{"key2": "val2"}, pVal3) // val2 won
	assert.True(t, changed)

	changed = CheckUpdateStringMap(true, &pVal3, val2, val2)
	assert.Equal(t, map[string]string{"key2": "val2"}, pVal3)
	assert.True(t, changed) // because it was already changed

	changed = CheckUpdateStringMap(false, &pVal3, val2, val2)
	assert.Equal(t, map[string]string{"key2": "val2"}, pVal3)
	assert.False(t, changed) // the value hasn't changed

	changed = CheckUpdateStringMap(false, &pVal3, val1, nil)
	assert.Equal(t, map[string]string{"key1": "val1"}, pVal3) // val1 won
	assert.False(t, changed)                                  // which was the current value
}
