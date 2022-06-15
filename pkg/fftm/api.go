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
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"
	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/ffapi"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmmsgs"
	"github.com/hyperledger/firefly-transaction-manager/pkg/policyengine"
)

func (m *manager) router() *mux.Router {
	mux := mux.NewRouter()
	mux.Path("/").Methods(http.MethodPost).Handler(http.HandlerFunc(m.apiHandler))
	hf := ffapi.HandlerFactory{
		DefaultRequestTimeout: config.GetDuration(tmconfig.APIDefaultRequestTimeout),
		MaxTimeout:            config.GetDuration(tmconfig.APIMaxRequestTimeout),
	}
	routes := m.routes()
	for _, r := range routes {
		mux.Path(r.Path).Methods(r.Method).Handler(hf.RouteHandler(r))
	}
	mux.Path("/api").Methods(http.MethodGet).Handler(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		url := req.URL.String() + "/spec.yaml"
		handler := hf.APIWrapper(hf.SwaggerUIHandler(url))
		handler(res, req)
	}))
	mux.Path("/api/spec.yaml").Methods(http.MethodGet).Handler(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		u := req.URL
		u.Path = ""
		swaggerGen := ffapi.NewSwaggerGen(&ffapi.Options{
			BaseURL: u.String(),
		})
		doc := swaggerGen.Generate(req.Context(), routes)
		res.Header().Add("Content-Type", "application/x-yaml")
		b, _ := yaml.Marshal(&doc)
		_, _ = res.Write(b)
	}))
	mux.Path("/api/spec.json").Methods(http.MethodGet).Handler(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		u := req.URL
		u.Path = ""
		swaggerGen := ffapi.NewSwaggerGen(&ffapi.Options{
			BaseURL: u.String(),
		})
		doc := swaggerGen.Generate(req.Context(), routes)
		res.Header().Add("Content-Type", "application/json")
		b, _ := json.Marshal(&doc)
		_, _ = res.Write(b)
	}))
	return mux
}

func (m *manager) runAPIServer() {
	m.apiServer.ServeHTTP(m.ctx)
}

func (m *manager) validateRequest(ctx context.Context, tReq *policyengine.TransactionRequest) error {
	if tReq == nil || tReq.Headers.ID == "" || tReq.Headers.Type == "" {
		log.L(ctx).Warnf("Invalid request: %+v", tReq)
		return i18n.NewError(ctx, tmmsgs.MsgErrorInvalidRequest)
	}
	return nil
}

func (m *manager) apiHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var tReq *policyengine.TransactionRequest
	statusCode := 200
	err := json.NewDecoder(r.Body).Decode(&tReq)
	if err == nil {
		err = m.validateRequest(ctx, tReq)
	}
	var resBody interface{}
	if err != nil {
		statusCode = 400
	} else {
		ctx = log.WithLogField(ctx, "requestId", tReq.Headers.ID)
		switch tReq.Headers.Type {
		case policyengine.RequestTypeSendTransaction:
			resBody, err = m.sendManagedTransaction(ctx, tReq)
		default:
			err = i18n.NewError(ctx, tmmsgs.MsgUnsupportedRequestType, tReq.Headers.Type)
			statusCode = 400
		}
	}
	if err != nil {
		log.L(ctx).Errorf("Request failed: %s", err)
		resBody = &fftypes.RESTError{Error: err.Error()}
		if statusCode < 400 {
			statusCode = 500
		}
	}
	w.Header().Set("Content-Type", "application/json")
	resBytes, _ := json.Marshal(&resBody)
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(resBytes)), 10))
	w.WriteHeader(statusCode)
	_, _ = w.Write(resBytes)
}
