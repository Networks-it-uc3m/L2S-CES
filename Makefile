# Image URL to use for building/pushing
IMG ?= alexdecb/l2sces:0.4
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.29.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

include config.mk

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

REPOSITORY=l2sc-es
## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross
## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)
KIND ?= kind
DOCKER ?= docker 

WORKER_CLUSTER_NUM ?= 2
## Tool Versions
KUSTOMIZE_VERSION ?= v5.7.1
CONTROLLER_TOOLS_VERSION ?= v0.19.0

ENVTEST_VERSION ?= latest
GOLANGCI_LINT_VERSION ?= v1.54.2
##@ Build and Push


L2SMMD_NAMESPACE ?= default

.PHONY: docker-build
docker-build: ## Build docker image with the server.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image to the repository.
	$(CONTAINER_TOOL) push ${IMG}

##@ Help

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	"$(CONTROLLER_GEN)" rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	"$(CONTROLLER_GEN)" object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -


.PHONY: generate-proto
export PATH := $(PATH):$(LOCALBIN)
generate-proto: install-tools ## Generate gRPC code from .proto file.
	protoc -I=api/v1 --go_out=paths=source_relative:./api/v1/l2sces --go-grpc_out=paths=source_relative:./api/v1/l2sces api/v1/l2sces.proto

.PHONY: run-server
include .env
export $(shell sed 's/=.*//' .env)
run-server: 
	go run ./cmd/server

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

.PHONY: build
build: manifests generate fmt vet
	go build -o $(LOCALBIN)/server ./cmd/server/
	go build -o $(LOCALBIN)/apply-cert ./cmd/apply-cert/
	go build -o bin/manager cmd/main.go


.PHONY: build-nemo
build-nemo: kustomize ## Generate a consolidated YAML with CRDs and deployment.
	echo "" > deployments/nemo-deployment.yaml
	echo "---" >> deployments/nemo-deployment.yaml  # Add a document separator before appending
	cd config/manager && $(KUSTOMIZE) edit set image manager=${IMG}
	$(KUSTOMIZE) build config/nemo >> deployments/nemo-deployment.yaml

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	echo "" > deployments/l2sces-deployment.yaml
	echo "---" >> deployments/l2sces-deployment.yaml  # Add a document separator before appending
	cd config/default && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default >> deployments/l2sces-deployment.yaml

.PHONY: build-codeco
build-codeco: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	echo "" > deployments/codeco-deployment.yaml
	echo "---" >> deployments/codeco-deployment.yaml  # Add a document separator before appending
	cd config/codeco && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/codeco >> deployments/codeco-deployment.yaml

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...


.PHONY: deploy
deploy: kustomize ## Deploy server to the K8s cluster specified in ~/.kube/config.
	cd config/server && $(KUSTOMIZE) edit set image server=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f - 

.PHONY: undeploy
undeploy: kustomize ## Undeploy server from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=true -f -

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))
	
.PHONY: setup
setup: create-control-plane create-workers add-cni install-l2sm
.PHONY: setup-dev
setup-dev: create-control-plane create-workers add-cni install-l2sm deploy-dev
	@echo "Development environment successfully set up."

.PHONY: create-control-plane
create-control-plane:
	if $(KIND) get clusters | grep -q "$(HUB)"; then \
		echo "Cluster $(HUB) already exists. Skipping creation."; \
	else \
		$(KIND) create cluster --config ./examples/quickstart/control-plane-cluster.yaml --name $(HUB);  \
	fi

.PHONY: create-workers
create-workers:
	for number in $(shell seq 1 ${WORKER_CLUSTER_NUM}); do \
		if $(KIND) get clusters | grep -q "$(WORKER)$$number"; then \
			echo "Cluster 'cluster$$number' already exists. Skipping creation."; \
		else \
			$(KIND) create cluster --config ./examples/quickstart/worker-cluster.yaml --name $(WORKER)$$number; \
		fi; \
		$(KUBECTL) config view -o jsonpath='{.clusters[?(@.name == "kind-$(WORKER)'$$number'")].cluster.certificate-authority-data}' --raw | base64 -d > config/certs/$(WORKER)$$number.key; \
	done


.PHONY: deploy-dev
deploy-dev: apply-cert kustomize
	$(KUBECTL) config use-context kind-$(HUB)
	$(KUSTOMIZE) build config/dev | $(KUBECTL) apply -f - 
	
.PHONY: undeploy-dev
undeploy-dev: kustomize ## Undeploy server from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/dev | $(KUBECTL) delete --ignore-not-found=true -f -
	$(KUBECTL) delete secrets --all

# Define file extensions for various formats
FILES := $(shell find . -type f \( -name "*.go" -o -name "*.json" -o -name "*.yaml" -o -name "*.yml" -o -name "*.md" \))

# Install the addlicense tool if not installed
.PHONY: install-tools
install-tools:
	GOBIN=$(LOCALBIN) go install github.com/google/addlicense@latest
	GOBIN=$(LOCALBIN) go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	GOBIN=$(LOCALBIN) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))


.PHONY: add-license
add-license: install-tools
	@for file in $(FILES); do \
		$(LOCALBIN)/addlicense -f ./hack/LICENSE.txt -l apache "$${file}"; \
	done


CERT_FILES := $(shell find ./config/certs/ -name "*.key")

.PHONY: apply-cert
apply-cert: build
	$(KUBECTL) config use-context kind-$(HUB)
	@if [ -n "$(CERT_FILES)" ]; then \
		for file in $(CERT_FILES); do \
			clustername=$$(basename $${file} .key); \
			$(LOCALBIN)/apply-cert --namespace $(L2SMMD_NAMESPACE) --clustername $${clustername} $${file}; \
		done; \
	else \
		echo "No certificate files to process."; \
	fi

.PHONY: all
all: build

.PHONY: install-l2sm
install-l2sm:
	for number in $(shell seq 1 ${WORKER_CLUSTER_NUM}); do \
		$(KUBECTL) apply --context kind-$(WORKER)$$number -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.1/cert-manager.yaml; \
		$(KUBECTL) apply --context kind-$(WORKER)$$number -f https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/master/deployments/multus-daemonset-thick.yml; \
	done; \
	for number in $(shell seq 1 ${WORKER_CLUSTER_NUM}); do \
		$(KUBECTL) --context kind-$(WORKER)$$number wait --for=condition=Ready pods --all -A --timeout=300s; \
	done; \
	for number in $(shell seq 1 ${WORKER_CLUSTER_NUM}); do \
		$(KUBECTL) --context kind-$(WORKER)$$number apply -f $(L2SM_DEPLOYMENT); \
	done

.PHONY: add-cni
add-cni:
	@if [ ! -d "plugins/bin" ] || [ -z "$$(ls -A plugins/bin)" ]; then \
		mkdir -p plugins/bin; \
		wget -q https://github.com/containernetworking/plugins/releases/download/v1.6.0/cni-plugins-linux-amd64-v1.6.0.tgz; \
		tar -xf cni-plugins-linux-amd64-v1.6.0.tgz -C plugins/bin; \
		rm cni-plugins-linux-amd64-v1.6.0.tgz; \
	fi
	@nodes="$$( $(KIND) get nodes -A)"; \
	if [ -z "$$nodes" ]; then \
		echo "No nodes found. Is Kind running?"; \
		exit 1; \
	fi; \
	for node in $$nodes; do \
		echo "Copying plugins to node: $$node"; \
		$(DOCKER) cp ./plugins/bin/. $$node:/opt/cni/bin; \
		if [ $$? -ne 0 ]; then \
			echo "Failed to copy plugins to $$node"; \
			exit 1; \
		fi; \
		$(DOCKER) exec $$node modprobe br_netfilter; \
		$(DOCKER) exec $$node sysctl -p /etc/sysctl.conf; \
	done; \
	clusters="$$( $(KIND) get clusters )"; \
	if [ -z "$$clusters" ]; then \
		echo "No clusters found. Is Kind running?"; \
		exit 1; \
	fi; \
	for cluster in $$clusters; do \
		$(KUBECTL) --context kind-$$cluster apply -f https://raw.githubusercontent.com/flannel-io/flannel/master/Documentation/kube-flannel.yml; \
		if [ $$? -ne 0 ]; then \
			echo "Failed to install flannel in $$cluster"; \
			exit 1; \
		fi; \
	done

.PHONY: connect-clusters
connect-clusters:
	

.PHONY: copy-to-container
copy-to-container:
	@if [ -z "$(container)" ]; then \
		echo "Error: Please specify a container name using 'make copy-to-container container=<container_name>'"; \
		exit 1; \
	fi
	docker cp ./plugins/bin/. $(container):/opt/cni/bin
# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef


.PHONY: clean
clean:
	$(KIND) delete clusters --all
