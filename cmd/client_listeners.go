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

package cmd

import (
	"github.com/hyperledger/firefly-transaction-manager/internal/apiclient"
	"github.com/spf13/cobra"
)

var listenerID string

func clientListenersCommand(clientFactory func() (apiclient.FFTMClient, error)) *cobra.Command {
	clientListenersCmd := &cobra.Command{
		Use:   "listeners <subcommand>",
		Short: "Make API requests to an blockchain connector instance",
	}
	clientListenersCmd.PersistentFlags().StringVarP(&eventStreamID, "eventstream", "", "", "The event stream ID")
	clientListenersCmd.AddCommand(clientListenersListCommand(clientFactory))
	clientListenersCmd.AddCommand(clientListenersDeleteCommand(clientFactory))
	return clientListenersCmd
}
