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
	"crypto/rand"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortEvents(t *testing.T) {

	events := make(Events, 1000)
	for i := 0; i < 1000; i++ {
		v, _ := rand.Int(rand.Reader, big.NewInt(100000000))
		events[i] = &Event{
			ProtocolID: fmt.Sprintf("%.9d", v.Int64()),
		}
	}
	sort.Sort(events)

	for i := 1; i < 1000; i++ {
		assert.Negative(t, strings.Compare(events[i-1].ProtocolID, events[i].ProtocolID))
	}
}
