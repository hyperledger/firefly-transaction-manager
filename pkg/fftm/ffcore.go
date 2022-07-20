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
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengine"
	"github.com/hyperledger/firefly/pkg/core"
)

// opUpdate allows us to avoid JSONObject serialization to a map before we upload our managedTXOutput
type opUpdate struct {
	Status core.OpStatus                 `json:"status"`
	Output *policyengine.ManagedTXOutput `json:"output"`
	Error  string                        `json:"error"`
}

func (m *manager) writeManagedTX(ctx context.Context, mtx *policyengine.ManagedTXOutput, status core.OpStatus, errString string) error {
	log.L(ctx).Debugf("Updating operation %s status=%s", mtx.ID, status)
	var errorInfo fftypes.RESTError
	var op core.Operation
	res, err := m.ffCoreClient.R().
		SetResult(&op).
		SetError(&errorInfo).
		SetBody(&opUpdate{
			Output: mtx,
			Status: status,
			Error:  errString,
		}).
		SetContext(ctx).
		Patch(fmt.Sprintf("/spi/v1/operations/%s", mtx.ID))
	if err != nil {
		return err
	}
	if res.IsError() {
		return i18n.NewError(m.ctx, tmmsgs.MsgCoreError, res.StatusCode(), errorInfo.Error)
	}
	return nil
}

func (m *manager) queryAndAddPending(nsOpID string) {
	var errorInfo fftypes.RESTError
	var op *core.Operation
	res, err := m.ffCoreClient.R().
		SetResult(&op).
		SetError(&errorInfo).
		Get(fmt.Sprintf("/spi/v1/operations/%s", nsOpID))
	if err == nil {
		// Operations are not deleted, so we consider not found the same as any other error
		if res.IsError() {
			err = i18n.NewError(m.ctx, tmmsgs.MsgCoreError, res.StatusCode(), errorInfo.Error)
		}
	}
	if err != nil {
		// We logo the error, then schedule a full poll (rather than retrying here)
		log.L(m.ctx).Errorf("Scheduling full poll due to error from core: %s", err)
		m.requestFullScan()
		return
	}
	// If the operation has been marked as success (by us or otherwise), or failed, then
	// we can remove it. If we resolved it, then we would have cleared it up on the .
	switch op.Status {
	case core.OpStatusSucceeded, core.OpStatusFailed:
		m.markCancelledIfTracked(nsOpID)
	case core.OpStatusPending:
		m.trackIfManaged(op)
	}
}

func (m *manager) readOperationPage(ns string, lastOp *core.Operation) ([]*core.Operation, error) {
	var errorInfo fftypes.RESTError
	var ops []*core.Operation
	query := url.Values{
		"sort":   []string{"created"},
		"type":   m.opTypes,
		"status": []string{string(core.OpStatusPending)},
	}
	if lastOp != nil {
		// For all but the 1st page, we use the last operation as the reference point.
		// Extremely unlikely to get multiple ops withe same creation date, but not impossible
		// so >= check, and removal of the duplicate at the end of the function.
		query.Set("created", fmt.Sprintf(">=%d", lastOp.Created.UnixNano()))
		query.Set("limit", strconv.FormatInt(m.fullScanPageSize+1, 10))
	} else {
		query.Set("limit", strconv.FormatInt(m.fullScanPageSize, 10))
	}
	res, err := m.ffCoreClient.R().
		SetQueryParamsFromValues(query).
		SetResult(&ops).
		SetError(&errorInfo).
		Get(fmt.Sprintf("/spi/v1/namespaces/%s/operations", ns))
	if err != nil {
		return nil, i18n.WrapError(m.ctx, err, tmmsgs.MsgCoreError, -1, err)
	}
	if res.IsError() {
		return nil, i18n.NewError(m.ctx, tmmsgs.MsgCoreError, res.StatusCode(), errorInfo.Error)
	}
	if lastOp != nil && len(ops) > 0 && ops[0].ID.Equals(lastOp.ID) {
		ops = ops[1:]
	}
	return ops, nil
}
