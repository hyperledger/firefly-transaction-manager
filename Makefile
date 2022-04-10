VGO=go
GOFILES := $(shell find cmd internal pkg -name '*.go' -print)
GOBIN := $(shell $(VGO) env GOPATH)/bin
LINT := $(GOBIN)/golangci-lint
MOCKERY := $(GOBIN)/mockery

# Expect that FireFly compiles with CGO disabled
CGO_ENABLED=0
GOGC=30

.DELETE_ON_ERROR:

all: build test go-mod-tidy
test: deps lint
		$(VGO) test ./internal/... ./cmd/... -cover -coverprofile=coverage.txt -covermode=atomic -timeout=30s
coverage.html:
		$(VGO) tool cover -html=coverage.txt
coverage: test coverage.html
lint: ${LINT}
		GOGC=20 $(LINT) run -v --timeout 5m
${MOCKERY}:
		$(VGO) install github.com/vektra/mockery/cmd/mockery@latest
${LINT}:
		$(VGO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest


define makemock
mocks: mocks-$(strip $(1))-$(strip $(2))
mocks-$(strip $(1))-$(strip $(2)): ${MOCKERY}
	${MOCKERY} --case underscore --dir $(1) --name $(2) --outpkg $(3) --output mocks/$(strip $(3))
endef

$(eval $(call makemock, pkg/ffcapi,                  API,                 ffcapimocks))
$(eval $(call makemock, internal/manager,            Manager,             managermocks))

firefly-transaction-manager: ${GOFILES}
		$(VGO) build -o ./firefly-transaction-manager -ldflags "-X main.buildDate=`date -u +\"%Y-%m-%dT%H:%M:%SZ\"` -X main.buildVersion=$(BUILD_VERSION)" -tags=prod -tags=prod -v ./fftm 
go-mod-tidy: .ALWAYS
		$(VGO) mod tidy
build: firefly-transaction-manager
.ALWAYS: ;
clean:
		$(VGO) clean
deps:
		$(VGO) get ./fftm
swagger:
		$(VGO) test ./internal/apiserver -timeout=10s -tags swagger
docker:
		docker build --build-arg BUILD_VERSION=${BUILD_VERSION} ${DOCKER_ARGS} -t hyperledger/firefly-transaction-manager .