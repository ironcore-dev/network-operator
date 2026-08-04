# Image URL to use all building/pushing image targets
IMG ?= controller:latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

GOOS    := $(shell go env GOOS)
GOARCH  := $(shell go env GOARCH)

VERSION    ?= $(shell git describe --tags --always --abbrev=7)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

LDFLAGS = -ldflags="-X 'main.version=$(VERSION)' -X 'main.gitCommit=$(GIT_COMMIT)' -X 'main.buildDate=$(BUILD_DATE)'"

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker. However,
# you might want to replace it to use other tools. (i.e. podman, nerdctl)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
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

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

YEAR ?= $(shell date +%Y)

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt",year="$(YEAR)" paths="./..."
	$(CONTROLLER_GEN) applyconfiguration:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: goimports gofumpt ## Run goimports and gofumpt against code.
	@$(GOIMPORTS) -w -local github.com/ironcore-dev/network-operator $(shell git ls-files '*.go' | grep -E -v 'zz_generated.deepcopy.go')
	@$(GOFUMPT) -l -w $(shell git ls-files '*.go' | grep -E -v 'zz_generated.deepcopy.go')

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e | grep -v /lab) -coverprofile cover.out

.PHONY: coverage
coverage: test ## Run tests and generate coverage report.
	go tool cover -html=cover.out -o cover.html

KIND_CLUSTER ?= network-operator-test-e2e

.PHONY: setup-test-e2e
setup-test-e2e: kind ## Set up a Kind cluster for e2e tests if it does not exist
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac

# E2E test dependency versions
E2E_PROMETHEUS_OPERATOR_VERSION ?= v0.82.2
E2E_CERTMANAGER_VERSION ?= v1.17.2

.PHONY: test-e2e
test-e2e: setup-test-e2e manifests generate ## Run the e2e tests. Expected an isolated environment using Kind.
	KIND=$(KIND) KIND_CLUSTER=$(KIND_CLUSTER) E2E_PROMETHEUS_OPERATOR_VERSION=$(E2E_PROMETHEUS_OPERATOR_VERSION) E2E_CERTMANAGER_VERSION=$(E2E_CERTMANAGER_VERSION) go test ./test/e2e/ -v -ginkgo.v
	$(MAKE) cleanup-test-e2e

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

.PHONY: test-gnmi
test-gnmi: FORCE ## Run integration tests for gNMI.
	@printf "\e[1;33m>> gNMI integration tests not yet implemented\e[0m\n"

.PHONY: test-lab
test-lab: ## Run lab tests against a real network device.
	go test ./test/lab/ -v

.PHONY: lint
lint: golangci-lint-custom ## Run golangci-lint linter
	$(GOLANGCI_LINT_CUSTOM) run

.PHONY: lint-fix
lint-fix: golangci-lint-custom ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT_CUSTOM) run --fix

.PHONY: lint-config
lint-config: golangci-lint-custom ## Verify golangci-lint linter configuration
	$(GOLANGCI_LINT_CUSTOM) config verify

.PHONY: lint-sh
lint-sh: shellcheck ## Run shellcheck on shell scripts.
	find . -name '*.sh' -not -path './bin/*' -not -path '*/node_modules/*' -exec $(SHELLCHECK) {} +

.PHONY: spellcheck
spellcheck: typos ## Run typos spell checker.
	$(TYPOS)

.PHONY: add-license
add-license: addlicense ## Add license headers to all go files.
	$(ADDLICENSE) -ignore '**/*.yml' -ignore '**/*.yaml' -f hack/license-header.txt .

.PHONY: check-license
check-license: addlicense ## Check that every file has a license header present.
	$(ADDLICENSE) -ignore '**/*.yml' -ignore '**/*.yaml' -check -c 'IronCore contributors' .

.PHONY: check-dependency-license
check-dependency-license: go-licenses ## Check that dependencies don't have non-FOSS licenses.
	$(GO_LICENSES) check ./... --disallowed_types=forbidden,restricted,unknown

.PHONY: check
check: generate manifests fmt lint vet lint-sh spellcheck test check-license check-dependency-license # Generate manifests, code, lint, fmt, test

.PHONY: clean
clean: ## Remove all generated files (bin/, dist/, coverage files)
	rm -rf bin/
	rm -rf dist/
	rm -f cover.out

##@ Build

.PHONY: docs
docs: crd-ref-docs ## Generate API reference documentation.
	$(CRD_REF_DOCS) --source-path=./api --config=./hack/api-reference/config.yaml --renderer=markdown --output-path=./docs/api-reference/index.md
	@sed -i.bak \
		-e '/^SPDX-/d' \
	  -e 's/#networkingmetalironcoredevv1alpha1/#networking-metal-ironcore-dev-v1alpha1/g' \
	  -e 's/#poolnetworkingmetalironcoredevv1alpha1/#pool-networking-metal-ironcore-dev-v1alpha1/g' \
	  -e 's/#nxcisconetworkingmetalironcoredevv1alpha1/#nx-cisco-networking-metal-ironcore-dev-v1alpha1/g' \
	  -e 's/#xecisconetworkingmetalironcoredevv1alpha1/#xe-cisco-networking-metal-ironcore-dev-v1alpha1/g' \
	  -e 's/#xrcisconetworkingmetalironcoredevv1alpha1/#xr-cisco-networking-metal-ironcore-dev-v1alpha1/g' \
	  docs/api-reference/index.md
	@find . -type f -name "*.bak" -delete

ROOT_DIR := $(shell pwd)
DOCS_IMG ?= ironcore-dev/network-operator-docs:latest

.PHONY: run-docs
run-docs:
	$(CONTAINER_TOOL) build -t $(DOCS_IMG) -f docs/Dockerfile docs --load
	$(CONTAINER_TOOL) run --rm --init -p 5173:5173 -v $(ROOT_DIR)/docs:/workspace -v /workspace/node_modules $(DOCS_IMG)

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

.PHONY: helm
helm: kubebuilder
	@mv charts/network-operator charts/chart
	$(KUBEBUILDER) edit --plugins=helm/v2-alpha --output-dir=charts
	@mv charts/chart charts/network-operator && rm -rf dist
	@# Fix cert-manager volumeMounts/volumes indentation (https://github.com/kubernetes-sigs/kubebuilder/issues/5677)
	@sed -i.bak \
	  -e '/certManager.enable/,/end/{s/^        - mountPath:/          - mountPath:/;s/^          name: webhook-certs/            name: webhook-certs/;s/^          readOnly: true/            readOnly: true/;s/^      - name: webhook-certs/        - name: webhook-certs/;s/^        secret:/          secret:/;s/^          secretName:/            secretName:/}' \
	  charts/network-operator/templates/manager/manager.yaml
	@find . -type f -name "*.bak" -delete

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build \
		--platform=linux/$(GOARCH) \
		--build-arg=BUILD_DATE=$(BUILD_DATE) \
	 	--build-arg=GIT_COMMIT=$(GIT_COMMIT) \
	 	--build-arg=VERSION=$(VERSION) \
	 	--tag $(IMG) .

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	@if ! grep -q "image: $(IMG)" config/manager/manager.yaml; then \
		cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG); \
	fi
	$(KUSTOMIZE) build config/default > dist/install.yaml

# TEST_LAB_IMG defines the image to used for packaging the lab tests.
TEST_LAB_IMG ?= ghcr.io/ironcore-dev/network-operator-lab-test:latest

.PHONY: docker-build-test-lab
docker-build-test-lab: FORCE
	@printf "\e[1;36m>> $(CONTAINER_TOOL) build --file=test/lab/Dockerfile --tag=$(TEST_LAB_IMG) .\e[0m\n"
	@$(CONTAINER_TOOL) build --platform=linux/$(GOARCH) --file=test/lab/Dockerfile --tag=$(TEST_LAB_IMG) .

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster.
	@if ! grep -q "image: $(IMG)" config/manager/manager.yaml; then \
		cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG); \
	fi
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

CURL_RETRIES=3

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= $(LOCALBIN)/kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
KUBEBUILDER ?= $(LOCALBIN)/kubebuilder
CRD_REF_DOCS ?= $(LOCALBIN)/crd-ref-docs
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_CUSTOM = $(LOCALBIN)/golangci-lint-custom
GOIMPORTS ?= $(LOCALBIN)/goimports
GOFUMPT ?= $(LOCALBIN)/gofumpt
ADDLICENSE ?= $(LOCALBIN)/addlicense
GO_LICENSES ?= $(LOCALBIN)/go-licenses
TYPOS ?= $(LOCALBIN)/typos
SHELLCHECK ?= $(LOCALBIN)/shellcheck
NETOP_PROVIDER ?= $(LOCALBIN)/netop-provider

## Tool Versions
KUSTOMIZE_VERSION ?= v5.8.1
CONTROLLER_TOOLS_VERSION ?= v0.21.0
#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell go list -m -f "{{ .Version }}" sigs.k8s.io/controller-runtime | awk -F'[v.]' '{printf "release-%d.%d", $$2, $$3}')
#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell go list -m -f "{{ .Version }}" k8s.io/api | awk -F'[v.]' '{printf "1.%d.%d",$$3, $$4}')
KUBEBUILDER_VERSION ?= v4.15.0
CRD_REF_DOCS_VERSION ?= v0.3.0
GOLANGCI_LINT_VERSION ?= v2.12.2
GOIMPORTS_VERSION ?= v0.48.0
GOFUMPT_VERSION ?= v0.10.0
ADDLICENSE_VERSION ?= v1.2.0
GO_LICENSES_VERSION ?= v2.0.1
TYPOS_VERSION ?= v1.48.0
SHELLCHECK_VERSION ?= v0.11.0
KIND_VERSION ?= v0.32.0

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: kubebuilder
kubebuilder: $(KUBEBUILDER) ## Download kubebuilder locally if necessary.
$(KUBEBUILDER): $(LOCALBIN)
	$(call go-install-tool,$(KUBEBUILDER),sigs.k8s.io/kubebuilder/v4,$(KUBEBUILDER_VERSION))

.PHONY: crd-ref-docs
crd-ref-docs: $(CRD_REF_DOCS) ## Download crd-ref-docs locally if necessary.
$(CRD_REF_DOCS): $(LOCALBIN)
	$(call go-install-tool,$(CRD_REF_DOCS),github.com/elastic/crd-ref-docs,$(CRD_REF_DOCS_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

.PHONY: golangci-lint-custom
golangci-lint-custom: $(GOLANGCI_LINT_CUSTOM) ## Build golangci-lint with custom plugins if .custom-gcl.yaml exists.
$(GOLANGCI_LINT_CUSTOM): $(GOLANGCI_LINT) .custom-gcl.yaml
	@$(GOLANGCI_LINT) custom --destination $(LOCALBIN) --name golangci-lint-custom

.PHONY: goimports
goimports: $(GOIMPORTS) ## Download goimports locally if necessary.
$(GOIMPORTS): $(LOCALBIN)
	$(call go-install-tool,$(GOIMPORTS),golang.org/x/tools/cmd/goimports,$(GOIMPORTS_VERSION))

.PHONY: gofumpt
gofumpt: $(GOFUMPT) ## Download gofumpt locally if necessary.
$(GOFUMPT): $(LOCALBIN)
	$(call go-install-tool,$(GOFUMPT),mvdan.cc/gofumpt,$(GOFUMPT_VERSION))

.PHONY: addlicense
addlicense: $(ADDLICENSE) ## Download addlicense locally if necessary.
$(ADDLICENSE): $(LOCALBIN)
	$(call go-install-tool,$(ADDLICENSE),github.com/google/addlicense,$(ADDLICENSE_VERSION))

.PHONY: go-licenses
go-licenses: $(GO_LICENSES) ## Download go-licenses locally if necessary.
$(GO_LICENSES): $(LOCALBIN)
	$(call go-install-tool,$(GO_LICENSES),github.com/google/go-licenses,$(GO_LICENSES_VERSION))

TYPOS_OS     := $(if $(filter darwin,$(GOOS)),apple-darwin,unknown-linux-musl)
RELEASE_ARCH := $(if $(filter amd64,$(GOARCH)),x86_64,$(if $(filter arm64,$(GOARCH)),aarch64,$(GOARCH)))

.PHONY: typos
typos: $(TYPOS) ## Download typos locally if necessary.
$(TYPOS): $(LOCALBIN)
	$(call download-tool,$(TYPOS),https://github.com/crate-ci/typos/releases/download/$(TYPOS_VERSION)/typos-$(TYPOS_VERSION)-$(RELEASE_ARCH)-$(TYPOS_OS).tar.gz,$(TYPOS_VERSION),-xzf - -C $(LOCALBIN) ./typos)

.PHONY: shellcheck
shellcheck: $(SHELLCHECK) ## Download shellcheck locally if necessary.
$(SHELLCHECK): $(LOCALBIN)
	$(call download-tool,$(SHELLCHECK),https://github.com/koalaman/shellcheck/releases/download/$(SHELLCHECK_VERSION)/shellcheck-$(SHELLCHECK_VERSION).$(GOOS).$(RELEASE_ARCH).tar.xz,$(SHELLCHECK_VERSION),-xJf - --strip-components=1 -C $(LOCALBIN) shellcheck-$(SHELLCHECK_VERSION)/shellcheck)

.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind,$(KIND_VERSION))

.PHONY: netop-provider
netop-provider: $(NETOP_PROVIDER) ## Install the network operator provider binary.
$(NETOP_PROVIDER): $(LOCALBIN)
	go build -o $(NETOP_PROVIDER) ./hack/provider/main.go

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef

# download-tool will download a release artifact from GitHub and extract it to LOCALBIN, if it doesn't exist
# $1 - target path with name of binary
# $2 - full download URL
# $3 - specific version
# $4 - tar flags and archive path (e.g. "-xzf - -C $(LOCALBIN) ./typos")
define download-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
echo "Downloading $(notdir $(1)) $(3)" ;\
curl -sSLf $(2) | tar $(4) ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef

## --------------------------------------
## Tilt / Kind
## --------------------------------------

KIND_CLUSTER_NAME ?= network-operator

.PHONY: kind-create
kind-create: kind ## Create the kind cluster if needed
	KIND_CLUSTER_NAME=$(KIND_CLUSTER_NAME) CONTAINER_TOOL=$(CONTAINER_TOOL) ./hack/kind-with-registry.sh

.PHONY: kind-delete
kind-delete: kind ## Destroys the kind cluster.
	KIND_EXPERIMENTAL_PROVIDER=$(CONTAINER_TOOL) $(KIND) delete cluster --name=$(KIND_CLUSTER_NAME)
	$(CONTAINER_TOOL) stop kind-registry && $(CONTAINER_TOOL) rm kind-registry

.PHONY: tilt-up
tilt-up: $(KUSTOMIZE) kind-create ## Start tilt and create the kind cluster if needed
	tilt up --context kind-$(KIND_CLUSTER_NAME)
