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
	"testing"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-transaction-manager/pkg/ffcapi"
	"github.com/stretchr/testify/assert"
)

func TestManagedTXUpdateMsgStringEqual(t *testing.T) {

	assert.Empty(t, ((*ManagedTXUpdate)(nil)).MsgString())
	mtu1 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          10,
		Info:           "things happened",
		Error:          "it was bad",
		MappedReason:   ffcapi.ErrorKnownTransaction,
	}
	mtu2 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          1,
		Info:           "things happened",
		Error:          "it was bad",
		MappedReason:   ffcapi.ErrorKnownTransaction,
	}
	assert.Equal(t, mtu1.MsgString(), mtu2.MsgString())
}

func TestManagedTXUpdateMsgStringNotMatchErr(t *testing.T) {

	mtu1 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          10,
		Info:           "things happened",
		Error:          "it was bad",
		MappedReason:   ffcapi.ErrorKnownTransaction,
	}
	mtu2 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          1,
		Info:           "things happened",
		Error:          "it was more bad",
		MappedReason:   ffcapi.ErrorKnownTransaction,
	}
	assert.NotEqual(t, mtu1.MsgString(), mtu2.MsgString())
}

func TestManagedTXUpdateMsgStringNotMatchReason(t *testing.T) {

	mtu1 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          10,
		Info:           "things happened",
		MappedReason:   ffcapi.ErrorKnownTransaction,
	}
	mtu2 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          1,
		Info:           "things happened",
		MappedReason:   "",
	}
	assert.NotEqual(t, mtu1.MsgString(), mtu2.MsgString())
}

func TestManagedTXUpdateMsgStringNotMatchInfo(t *testing.T) {

	mtu1 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          10,
		Info:           "things happened",
	}
	mtu2 := ManagedTXUpdate{
		Time:           fftypes.Now(),
		LastOccurrence: fftypes.Now(),
		Count:          1,
		Info:           "then we were done",
	}
	assert.NotEqual(t, mtu1.MsgString(), mtu2.MsgString())
}
