VGO=go
GOFILES := $(shell find internal pkg -name '*.go' -print)
GOBIN := $(shell $(VGO) env GOPATH)/bin
LINT := $(GOBIN)/golangci-lint
MOCKERY := $(GOBIN)/mockery

# Expect that FireFly compiles with CGO disabled
CGO_ENABLED=0
GOGC=30

.DELETE_ON_ERROR:

all: build test go-mod-tidy
test: deps lint
		$(VGO) test ./internal/... ./pkg/... -cover -coverprofile=coverage.txt -covermode=atomic -timeout=30s ${TEST_FLAGS}
coverage.html:
		$(VGO) tool cover -html=coverage.txt
coverage: test coverage.html
lint: ${LINT}
		GOGC=20 $(LINT) run -v --timeout 5m

${MOCKERY}:
		$(VGO) install github.com/vektra/mockery/cmd/mockery@latest
${LINT}:
		$(VGO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.47.0


define makemock
mocks: mocks-$(strip $(1))-$(strip $(2))
mocks-$(strip $(1))-$(strip $(2)): ${MOCKERY}
	${MOCKERY} --case underscore --dir $(1) --name $(2) --outpkg $(3) --output mocks/$(strip $(3))
endef

$(eval $(call makemock, pkg/ffcapi,             API,                    ffcapimocks))
$(eval $(call makemock, pkg/policyengine,       PolicyEngine,           policyenginemocks))
$(eval $(call makemock, pkg/txhistory,          Manager,                txhistorymocks))
$(eval $(call makemock, internal/confirmations, Manager,                confirmationsmocks))
$(eval $(call makemock, internal/persistence,   Persistence,            persistencemocks))
$(eval $(call makemock, internal/ws,            WebSocketChannels,      wsmocks))
$(eval $(call makemock, internal/events,        Stream,                 eventsmocks))

go-mod-tidy: .ALWAYS
		$(VGO) mod tidy
build: test
.ALWAYS: ;
clean:
		$(VGO) clean
deps:
		$(VGO) get ./internal/... ./pkg/...
		$(VGO) get -t ./internal/... ./pkg/...
reference:
		$(VGO) test ./pkg/fftm -timeout=10s -tags docs
