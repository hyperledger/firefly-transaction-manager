// Copyright © 2021 Kaleido, Inc.
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
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const configDir = "../test/data/config"

func TestRunOK(t *testing.T) {

	rootCmd.SetArgs([]string{"-f", "../test/firefly.fftm.yaml"})
	defer rootCmd.SetArgs([]string{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		err := Execute()
		assert.NoError(t, err)
	}()

	time.Sleep(10 * time.Millisecond)
	sigs <- os.Kill

	<-done

}

func TestRunMissingConfig(t *testing.T) {

	rootCmd.SetArgs([]string{"-f", "../test/does-not-exist.fftm.yaml"})
	defer rootCmd.SetArgs([]string{})

	err := Execute()
	assert.Regexp(t, "FF00101", err)

}

func TestRunBadConfig(t *testing.T) {

	rootCmd.SetArgs([]string{"-f", "../test/empty-config.fftm.yaml"})
	defer rootCmd.SetArgs([]string{})

	err := Execute()
	assert.Regexp(t, "FF201018", err)

}

func TestRunFailStartup(t *testing.T) {

	rootCmd.SetArgs([]string{"-f", "../test/quick-fail.fftm.yaml"})
	defer rootCmd.SetArgs([]string{})

	err := Execute()
	assert.Regexp(t, "FF201017", err)

}
