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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"
	"github.com/hyperledger/firefly-common/pkg/config"
	"github.com/hyperledger/firefly-common/pkg/ffapi"
	"github.com/hyperledger/firefly-common/pkg/i18n"
	"github.com/hyperledger/firefly-common/pkg/log"
	"github.com/hyperledger/firefly-transaction-manager/internal/metrics"
	"github.com/hyperledger/firefly-transaction-manager/internal/tmconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (m *manager) router() *mux.Router {
	mux := mux.NewRouter()
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

	mux.HandleFunc("/ws", m.wsServer.Handler)

	mux.NotFoundHandler = hf.APIWrapper(func(res http.ResponseWriter, req *http.Request) (status int, err error) {
		return 404, i18n.NewError(req.Context(), i18n.Msg404NotFound)
	})
	return mux
}

func (m *manager) createMetricsMuxRouter() *mux.Router {
	r := mux.NewRouter()

	r.Path(config.GetString(tmconfig.MetricsPath)).Handler(promhttp.InstrumentMetricHandler(metrics.Registry(),
		promhttp.HandlerFor(metrics.Registry(), promhttp.HandlerOpts{})))

	return r
}

func (m *manager) runAPIServer() {
	m.apiServer.ServeHTTP(m.ctx)
}

func (m *manager) runMetricsServer() {
	m.metricsServer.ServeHTTP(m.ctx)
}

func (m *manager) runDebugServer() {
	debugPort := config.GetInt(tmconfig.DebugPort)
	defer func() {
		close(m.debugServerDone)
	}()

	if debugPort >= 0 {
		r := mux.NewRouter()
		r.PathPrefix("/debug/pprof/cmdline").HandlerFunc(pprof.Cmdline)
		r.PathPrefix("/debug/pprof/profile").HandlerFunc(pprof.Profile)
		r.PathPrefix("/debug/pprof/symbol").HandlerFunc(pprof.Symbol)
		r.PathPrefix("/debug/pprof/trace").HandlerFunc(pprof.Trace)
		r.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)
		m.debugServer = &http.Server{Addr: fmt.Sprintf("localhost:%d", debugPort), Handler: r, ReadHeaderTimeout: 30 * time.Second}
		log.L(m.ctx).Debugf("Debug HTTP endpoint listening on localhost:%d", debugPort)
		_ = m.debugServer.ListenAndServe()
	}
}
