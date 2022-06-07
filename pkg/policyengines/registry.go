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

package policyengines

import (
	"context"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengine"
)

var policyEngines = make(map[string]Factory)

func NewPolicyEngine(ctx context.Context, basePrefix config.Prefix, name string) (policyengine.PolicyEngine, error) {
	factory, ok := policyEngines[name]
	if !ok {
		return nil, i18n.NewError(ctx, tmmsgs.MsgPolicyEngineNotRegistered, name)
	}
	return factory.NewPolicyEngine(ctx, basePrefix.SubPrefix(name))
}

type Factory interface {
	Name() string
	InitPrefix(prefix config.Prefix)
	NewPolicyEngine(ctx context.Context, prefix config.Prefix) (policyengine.PolicyEngine, error)
}

func RegisterEngine(basePrefix config.Prefix, factory Factory) string {
	name := factory.Name()
	policyEngines[name] = factory
	factory.InitPrefix(basePrefix.SubPrefix(name))
	return name
}
