# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.27.1

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

GIT_VERSION?=0.0.0
HYBRID_MANIFEST_URL?=https://do-not-delete-temp-hybrid-manifest.s3.amazonaws.com/manifest.yaml

.PHONY: all
all: crds generate fmt vet build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: clean
clean:
	rm -rf $(LOCALBIN)

.PHONY: crds
crds: controller-gen ## Generate CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd paths="./..." output:crd:artifacts:config=crds/

.PHONY: generate
generate: generate-code generate-doc ## Generate code and documentation.

.PHONY: generate-code
generate-code: controller-gen conversion-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object paths="./..."
	$(CONVERSION_GEN) --input-dirs="./internal/api/bridge" --output-file-base=zz_generated.conversion --output-base="./" --go-header-file=/dev/null -v0

.PHONY: generate-doc
generate-doc: crd-ref-docs
	$(CRD_REF_DOCS) --config=doc/config.yaml --source-path=api/ --renderer=markdown --templates-dir=doc/templates --output-path=doc/api.md
	cat -s doc/api.md > doc/api.md.tmp
	sed '$$ d' doc/api.md.tmp > doc/api.md
	rm doc/api.md.tmp

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: ## Run validate tests.
	go test ./... 

COVERAGEFILE = $(LOCALBIN)/coverage.out
.PHONY: coverage
coverage: test
	go test -coverprofile=$(COVERAGEFILE) ./...
	go tool cover -html=$(COVERAGEFILE)

.PHONY: test-e2e
test-e2e: build ## Run e2e tests.
	test/e2e/run.sh


##@ Build

## Build binary.
.PHONY: build
build: LINKER_FLAGS :=-X github.com/aws/eks-hybrid/cmd/nodeadm/version.GitVersion=$(GIT_VERSION) -X github.com/aws/eks-hybrid/internal/aws/eks.manifestUrl=$(HYBRID_MANIFEST_URL) -s -w -buildid='' -extldflags -static
build:
	go build -ldflags "$(LINKER_FLAGS)" -trimpath -o $(LOCALBIN)/nodeadm cmd/nodeadm/main.go

.PHONY: build-cross-platform
build-cross-platform: LINKER_FLAGS :=-X github.com/aws/eks-hybrid/cmd/nodeadm/version.GitVersion=$(GIT_VERSION) -X github.com/aws/eks-hybrid/internal/aws/eks.manifestUrl=$(HYBRID_MANIFEST_URL) -s -w -buildid='' -extldflags -static
build-cross-platform:
	GOARCH=amd64 go build -ldflags "$(LINKER_FLAGS)" -trimpath -o $(LOCALBIN)/amd64/nodeadm cmd/nodeadm/main.go
	GOARCH=arm64 go build -ldflags "$(LINKER_FLAGS)" -trimpath -o $(LOCALBIN)/arm64/nodeadm cmd/nodeadm/main.go

.PHONY: run
run: build ## Run binary
	go run cmd/nodeadm/main.go $(args)

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/_bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
CONVERSION_GEN ?= $(LOCALBIN)/conversion-gen
CRD_REF_DOCS ?= $(LOCALBIN)/crd-ref-docs
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT_CONFIG ?= .github/workflows/golangci-lint.yml
GOLANGCI_LINT := $(LOCALBIN)/golangci-lint

## Tool Versions
KUSTOMIZE_VERSION ?= v5.0.1
CONTROLLER_TOOLS_VERSION ?= v0.12.0
CODE_GENERATOR_VERSION ?= v0.28.1
CRD_REF_DOCS_VERSION ?= cf959ab94ea543cb8efd25dc35081880b7ca6a81

tools: kustomize controller-gen conversion-gen crd-ref-docs ## Install the toolchain.

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || \
	GOBIN=$(LOCALBIN) GO111MODULE=on go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: conversion-gen
conversion-gen: $(CONVERSION_GEN) ## Download conversion-gen locally if necessary.
$(CONVERSION_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/conversion-gen || \
	GOBIN=$(LOCALBIN) go install k8s.io/code-generator/cmd/conversion-gen@$(CODE_GENERATOR_VERSION)

.PHONY: crd-ref-docs
crd-ref-docs: $(CRD_REF_DOCS) ## Download crd-ref-docs locally if necessary.
$(CRD_REF_DOCS): $(LOCALBIN)
	test -s $(LOCALBIN)/crd-ref-docs || \
	GOBIN=$(LOCALBIN) go install github.com/elastic/crd-ref-docs@$(CRD_REF_DOCS_VERSION)

.PHONY: update-deps
update-deps:
	go get $(shell go list -f '{{if not (or .Main .Indirect)}}{{.Path}}{{end}}' -mod=mod -m all) && go mod tidy

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Run golangci-lint
	$(GOLANGCI_LINT) run --new-from-rev main

.PHONY: mocks
mocks: MOCKGEN := ${GOBIN}/mockgen --build_flags=--mod=mod
mocks: ## Generate mocks
	go install github.com/golang/mock/mockgen@v1.6.0
	${MOCKGEN} -destination=internal/validation/util/mocks/client.go -package=mocks -source "internal/validation/util/netclient.go" NetClient

$(GOLANGCI_LINT): $(LOCALBIN) $(GOLANGCI_LINT_CONFIG)
	$(eval GOLANGCI_LINT_VERSION?=$(shell cat .github/workflows/golangci-lint.yml | yq e '.jobs.golangci.steps[] | select(.name == "golangci-lint") .with.version' -))
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(LOCALBIN) $(GOLANGCI_LINT_VERSION)

.PHONY: e2e-tests-binary
e2e-tests-binary:
	GOMAXPROCS=10 go test ./test/e2e/validate -c -o "./_bin/e2e.test" -tags "e2e"