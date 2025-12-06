.PHONY: help kind-create kind-delete hub-install hub-uninstall hub-forward hub-status test-hub clean build

# Configuration
KIND_CLUSTER_NAME ?= koncur-test
KONVEYOR_NAMESPACE ?= my-konveyor-operator
OLM_VERSION ?= v0.38.0
KUBECTL ?= kubectl

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

##@ Cluster Management

kind-create: ## Create a Kind cluster for testing
	@echo "Creating Kind cluster: $(KIND_CLUSTER_NAME)..."
	@mkdir -p .koncur/config
	@printf 'kind: Cluster\n' > .koncur/config/kind-config.yaml
	@printf 'apiVersion: kind.x-k8s.io/v1alpha4\n' >> .koncur/config/kind-config.yaml
	@printf 'nodes:\n' >> .koncur/config/kind-config.yaml
	@printf -- '- role: control-plane\n' >> .koncur/config/kind-config.yaml
	@printf '  extraPortMappings:\n' >> .koncur/config/kind-config.yaml
	@printf '  - containerPort: 30080\n' >> .koncur/config/kind-config.yaml
	@printf '    hostPort: 8081\n' >> .koncur/config/kind-config.yaml
	@printf '    protocol: TCP\n' >> .koncur/config/kind-config.yaml
	@kind create cluster --name $(KIND_CLUSTER_NAME) --config .koncur/config/kind-config.yaml
	@echo "Cluster created successfully"

kind-delete: ## Delete the Kind cluster
	@echo "Deleting Kind cluster: $(KIND_CLUSTER_NAME)..."
	@kind delete cluster --name $(KIND_CLUSTER_NAME)
	@echo "Cluster deleted"

##@ Tackle Hub Installation

hub-install: ## Install Tackle Hub on the Kind cluster
	@echo "Installing OLM..."
	@curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/$(OLM_VERSION)/install.sh | bash -s $(OLM_VERSION)
	@echo "Waiting for OLM to be ready..."
	@$(KUBECTL) wait --for=condition=ready pod -l app=olm-operator -n olm --timeout=300s
	@$(KUBECTL) wait --for=condition=ready pod -l app=catalog-operator -n olm --timeout=300s
	@echo "Installing Konveyor operator..."
	@$(KUBECTL) create -f https://operatorhub.io/install/konveyor-operator.yaml
	@echo "Waiting for operator pod to be created..."
	@sleep 15
	@echo "Waiting for operator CRD to be available..."
	@$(KUBECTL) wait --for condition=established --timeout=300s crd/tackles.tackle.konveyor.io || true
	@echo "Waiting for operator to be ready..."
	@$(KUBECTL) wait --for=condition=ready pod -l control-plane=operator -n my-konveyor-operator --timeout=300s || true
	@echo "Creating Tackle instance..."
	@mkdir -p .koncur/config
	@printf 'kind: Tackle\n' > .koncur/config/tackle-cr.yaml
	@printf 'apiVersion: tackle.konveyor.io/v1alpha1\n' >> .koncur/config/tackle-cr.yaml
	@printf 'metadata:\n' >> .koncur/config/tackle-cr.yaml
	@printf '  name: tackle\n' >> .koncur/config/tackle-cr.yaml
	@printf '  namespace: my-konveyor-operator\n' >> .koncur/config/tackle-cr.yaml
	@printf 'spec:\n' >> .koncur/config/tackle-cr.yaml
	@printf '  feature_auth_required: false\n' >> .koncur/config/tackle-cr.yaml
	@$(KUBECTL) apply -f .koncur/config/tackle-cr.yaml
	@echo "Waiting for Tackle Hub to be ready (this may take a few minutes)..."
	@echo "Monitoring deployment progress..."
	@$(KUBECTL) wait --for=condition=ready pod -l app.kubernetes.io/name=tackle-hub -n $(KONVEYOR_NAMESPACE) --timeout=600s || true
	@echo ""
	@echo "Tackle Hub installation complete!"
	@echo "Run 'make hub-status' to check the status"
	@echo "Run 'make hub-forward' to access the UI"

hub-uninstall: ## Uninstall Tackle Hub
	@echo "Uninstalling Tackle Hub..."
	@$(KUBECTL) delete tackle tackle -n $(KONVEYOR_NAMESPACE) --ignore-not-found=true
	@$(KUBECTL) delete namespace $(KONVEYOR_NAMESPACE) --ignore-not-found=true
	@echo "Tackle Hub uninstalled"

hub-status: ## Check Tackle Hub status
	@echo "Checking Tackle Hub status..."
	@echo ""
	@echo "Namespace:"
	@$(KUBECTL) get namespace $(KONVEYOR_NAMESPACE) 2>/dev/null || echo "Namespace not found"
	@echo ""
	@echo "Pods:"
	@$(KUBECTL) get pods -n $(KONVEYOR_NAMESPACE) 2>/dev/null || echo "No pods found"
	@echo ""
	@echo "Services:"
	@$(KUBECTL) get svc -n $(KONVEYOR_NAMESPACE) 2>/dev/null || echo "No services found"
	@echo ""
	@echo "Tackle CR:"
	@$(KUBECTL) get tackle -n $(KONVEYOR_NAMESPACE) 2>/dev/null || echo "No Tackle CR found"

hub-forward: ## Port-forward to access Tackle Hub UI and API
	@echo "Port-forwarding Tackle Hub..."
	@echo "Hub API will be available at: http://localhost:8081"
	@echo "Hub UI will be available at: http://localhost:8081/hub"
	@echo ""
	@echo "Press Ctrl+C to stop port-forwarding"
	@$(KUBECTL) port-forward -n $(KONVEYOR_NAMESPACE) svc/tackle-hub 8081:8080

hub-logs: ## Show Tackle Hub logs
	@echo "Showing Tackle Hub logs (press Ctrl+C to exit)..."
	@$(KUBECTL) logs -f -n $(KONVEYOR_NAMESPACE) -l app.kubernetes.io/name=tackle-hub

##@ Testing

test-hub: ## Test the Tackle Hub integration with koncur
	@echo "Testing Tackle Hub integration..."
	@echo "Using target config: testdata/examples/target-tackle-hub.yaml"
	@echo ""
	@echo "Creating target configuration..."
	@mkdir -p .koncur/config
	@printf 'type: tackle-hub\n' > .koncur/config/target-tackle-hub.yaml
	@printf 'tackleHub:\n' >> .koncur/config/target-tackle-hub.yaml
	@printf '  url: http://localhost:8081\n' >> .koncur/config/target-tackle-hub.yaml
	@printf '  token: ""\n' >> .koncur/config/target-tackle-hub.yaml
	@echo "Running test with Tackle Hub target..."
	@./koncur run tests/tackle-testapp-with-deps/test.yaml --target-config .koncur/config/target-tackle-hub.yaml

##@ Build

build: ## Build the koncur binary
	@echo "Building koncur..."
	@go build -o koncur ./cmd/koncur
	@echo "Build complete: ./koncur"

clean: ## Clean build artifacts and test outputs
	@echo "Cleaning build artifacts..."
	@rm -f koncur
	@rm -rf .koncur/output/*
	@echo "Clean complete"

##@ Quick Setup

setup: kind-create hub-install build ## Complete setup: create cluster, install hub, build binary
	@echo ""
	@echo "=========================================="
	@echo "Setup complete!"
	@echo "=========================================="
	@echo ""
	@echo "Next steps:"
	@echo "1. In one terminal, run: make hub-forward"
	@echo "2. In another terminal, run: make test-hub"
	@echo ""

teardown: hub-uninstall kind-delete ## Complete teardown: uninstall hub, delete cluster
	@echo "Teardown complete"
