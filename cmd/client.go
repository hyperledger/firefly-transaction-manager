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
	"context"
	"fmt"

	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/httpserver"
	"github.com/hyperledger/firefly-transaction-manager/internal/apiclient"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/spf13/cobra"
)

var url string
var nameRegex string

func ClientCommand(fftmClientFactory ...func() apiclient.FFTMClient) *cobra.Command {
	clientCmd := &cobra.Command{
		Use:   "client <subcommand>",
		Short: "Make API requests to a blockchain connector instance",
	}
	defaultURL := fmt.Sprintf("http://%s:%s", tmconfig.APIConfig.GetString(httpserver.HTTPConfAddress), tmconfig.APIConfig.GetString(httpserver.HTTPConfPort))

	// Flags need to be set before initializing the client below, because they change some of its options
	clientCmd.PersistentFlags().StringVarP(&url, "url", "", defaultURL, "The URL of the blockchain connector")

	var clientFactory func() apiclient.FFTMClient
	if len(fftmClientFactory) > 0 && fftmClientFactory[0] != nil {
		clientFactory = fftmClientFactory[0]
	} else {
		clientFactory = createClient
	}

	clientCmd.AddCommand(clientEventStreamsCommand(clientFactory))
	clientCmd.AddCommand(clientListenersCommand(clientFactory))

	return clientCmd
}

func createClient() apiclient.FFTMClient {
	cfg := config.RootSection("fftm_client")
	apiclient.InitConfig(cfg)
	if url != "" {
		cfg.Set("url", url)
	}
	return apiclient.NewFFTMClient(context.Background(), cfg)
}

func init() {
	tmconfig.Reset()
}
