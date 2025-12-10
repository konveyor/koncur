.PHONY: help kind-create kind-delete hub-install hub-uninstall hub-forward hub-status test-hub clean build

# Configuration
KIND_CLUSTER_NAME ?= koncur-test
KONVEYOR_NAMESPACE ?= konveyor-tackle
KUBECTL ?= kubectl

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

##@ Cluster Management

kind-create: ## Create a Kind cluster for testing with ingress support
	@echo "Creating Kind cluster: $(KIND_CLUSTER_NAME)..."
	@mkdir -p cache .koncur/config
	@printf 'kind: Cluster\n' > .koncur/config/kind-config.yaml
	@printf 'apiVersion: kind.x-k8s.io/v1alpha4\n' >> .koncur/config/kind-config.yaml
	@printf 'nodes:\n' >> .koncur/config/kind-config.yaml
	@printf -- '- role: control-plane\n' >> .koncur/config/kind-config.yaml
	@printf '  kubeadmConfigPatches:\n' >> .koncur/config/kind-config.yaml
	@printf '  - |\n' >> .koncur/config/kind-config.yaml
	@printf '    kind: InitConfiguration\n' >> .koncur/config/kind-config.yaml
	@printf '    nodeRegistration:\n' >> .koncur/config/kind-config.yaml
	@printf '      kubeletExtraArgs:\n' >> .koncur/config/kind-config.yaml
	@printf '        node-labels: "ingress-ready=true"\n' >> .koncur/config/kind-config.yaml
	@printf '  extraPortMappings:\n' >> .koncur/config/kind-config.yaml
	@printf '  - containerPort: 80\n' >> .koncur/config/kind-config.yaml
	@printf '    hostPort: 8080\n' >> .koncur/config/kind-config.yaml
	@printf '    protocol: TCP\n' >> .koncur/config/kind-config.yaml
	@printf '  - containerPort: 443\n' >> .koncur/config/kind-config.yaml
	@printf '    hostPort: 8443\n' >> .koncur/config/kind-config.yaml
	@printf '    protocol: TCP\n' >> .koncur/config/kind-config.yaml
	@printf '  extraMounts:\n' >> .koncur/config/kind-config.yaml
	@printf '  - hostPath: ./cache\n' >> .koncur/config/kind-config.yaml
	@printf '    containerPath: /cache\n' >> .koncur/config/kind-config.yaml
	@kind create cluster --name $(KIND_CLUSTER_NAME) --config .koncur/config/kind-config.yaml
	@echo "Configuring local-path-storage to use /cache directory with RWX support..."
	@$(KUBECTL) patch configmap local-path-config -n local-path-storage --type merge -p '{"data":{"config.json":"{\n        \"nodePathMap\":[],\n        \"sharedFileSystemPath\":\"/cache\"\n}"}}'
	@echo "Restarting local-path-provisioner to apply configuration..."
	@$(KUBECTL) rollout restart deployment local-path-provisioner -n local-path-storage
	@$(KUBECTL) rollout status deployment local-path-provisioner -n local-path-storage --timeout=60s
	@echo "Installing ingress-nginx controller..."
	@$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
	@echo "Waiting for ingress-nginx namespace to be created..."
	@for i in $$(seq 1 30); do \
		$(KUBECTL) get namespace ingress-nginx >/dev/null 2>&1 && break || sleep 2; \
		if [ $$i -eq 30 ]; then echo "Timeout waiting for ingress-nginx namespace"; exit 1; fi; \
	done
	@echo "Waiting for ingress controller pod to be created and ready..."
	@for i in $$(seq 1 120); do \
		if $(KUBECTL) wait --namespace ingress-nginx --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=5s >/dev/null 2>&1; then \
			echo "Ingress controller is ready"; \
			break; \
		fi; \
		if [ $$i -eq 120 ]; then echo "Timeout waiting for ingress controller to be ready"; exit 1; fi; \
		sleep 3; \
	done
	@echo "Cluster created successfully with ingress support"

kind-delete: ## Delete the Kind cluster
	@echo "Deleting Kind cluster: $(KIND_CLUSTER_NAME)..."
	@kind delete cluster --name $(KIND_CLUSTER_NAME)
	@echo "Cluster deleted"

##@ Tackle Hub Installation

hub-install: ## Install Tackle Hub on the Kind cluster (from main branch)
	@echo "Installing OLM..."
	@curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.38.0/install.sh | bash -s v0.38.0 || true
	@echo "Waiting for OLM to be ready..."
	@$(KUBECTL) wait --for=condition=ready pod -l app=olm-operator -n olm --timeout=300s
	@$(KUBECTL) wait --for=condition=ready pod -l app=catalog-operator -n olm --timeout=300s
	@echo "Installing Tackle operator from main branch..."
	@$(KUBECTL) apply -f https://raw.githubusercontent.com/konveyor/tackle2-operator/main/tackle-k8s.yaml
	@echo "Waiting for Tackle CRD to be available..."
	@for i in $$(seq 1 60); do \
		$(KUBECTL) get crd tackles.tackle.konveyor.io >/dev/null 2>&1 && break || sleep 5; \
		if [ $$i -eq 60 ]; then echo "Timeout waiting for CRD to be created"; exit 1; fi; \
	done
	@$(KUBECTL) wait --for condition=established --timeout=300s crd/tackles.tackle.konveyor.io
	@echo "Waiting for operator to be ready..."
	@for i in $$(seq 1 120); do \
		if $(KUBECTL) wait --namespace konveyor-tackle --for=condition=ready pod --selector=name=tackle-operator --timeout=5s >/dev/null 2>&1; then \
			echo "Tackle operator is ready"; \
			break; \
		fi; \
		if [ $$i -eq 120 ]; then echo "Timeout waiting for operator to be ready"; exit 1; fi; \
		sleep 3; \
	done
	@echo "Pre-creating cache PV with fixed path..."
	@mkdir -p cache/hub-cache
	@mkdir -p .koncur/config
	@printf 'apiVersion: v1\n' > .koncur/config/cache-pv.yaml
	@printf 'kind: PersistentVolume\n' >> .koncur/config/cache-pv.yaml
	@printf 'metadata:\n' >> .koncur/config/cache-pv.yaml
	@printf '  name: tackle-cache-pv\n' >> .koncur/config/cache-pv.yaml
	@printf '  labels:\n' >> .koncur/config/cache-pv.yaml
	@printf '    type: tackle-cache\n' >> .koncur/config/cache-pv.yaml
	@printf 'spec:\n' >> .koncur/config/cache-pv.yaml
	@printf '  capacity:\n' >> .koncur/config/cache-pv.yaml
	@printf '    storage: 10Gi\n' >> .koncur/config/cache-pv.yaml
	@printf '  accessModes:\n' >> .koncur/config/cache-pv.yaml
	@printf '  - ReadWriteMany\n' >> .koncur/config/cache-pv.yaml
	@printf '  persistentVolumeReclaimPolicy: Retain\n' >> .koncur/config/cache-pv.yaml
	@printf '  storageClassName: manual\n' >> .koncur/config/cache-pv.yaml
	@printf '  hostPath:\n' >> .koncur/config/cache-pv.yaml
	@printf '    path: /cache/hub-cache\n' >> .koncur/config/cache-pv.yaml
	@printf '    type: DirectoryOrCreate\n' >> .koncur/config/cache-pv.yaml
	@$(KUBECTL) apply -f .koncur/config/cache-pv.yaml
	@echo "Creating Tackle CR with auth disabled..."
	@mkdir -p .koncur/config
	@printf 'kind: Tackle\n' > .koncur/config/tackle-cr.yaml
	@printf 'apiVersion: tackle.konveyor.io/v1alpha1\n' >> .koncur/config/tackle-cr.yaml
	@printf 'metadata:\n' >> .koncur/config/tackle-cr.yaml
	@printf '  name: tackle\n' >> .koncur/config/tackle-cr.yaml
	@printf '  namespace: konveyor-tackle\n' >> .koncur/config/tackle-cr.yaml
	@printf 'spec:\n' >> .koncur/config/tackle-cr.yaml
	@printf '  feature_auth_required: "false"\n' >> .koncur/config/tackle-cr.yaml
	@printf '  cache_storage_class: "manual"\n' >> .koncur/config/tackle-cr.yaml
	@printf '  cache_data_volume_size: "10Gi"\n' >> .koncur/config/tackle-cr.yaml
	@printf '  rwx_supported: "true"\n' >> .koncur/config/tackle-cr.yaml
	@$(KUBECTL) apply -f .koncur/config/tackle-cr.yaml
	@echo "Waiting for Tackle Hub to be ready (this may take a few minutes)..."
	@sleep 30
	@$(KUBECTL) wait --for=condition=ready pod -l app.kubernetes.io/name=tackle-hub -n konveyor-tackle --timeout=600s || true
	@echo ""
	@echo "Tackle Hub installation complete!"
	@echo ""
	@if $(KUBECTL) get pods -n ingress-nginx --no-headers 2>/dev/null | grep -q ingress-nginx-controller; then \
		echo "Access Tackle Hub via ingress at: http://localhost:8080"; \
		echo "Hub UI: http://localhost:8080/hub"; \
		echo ""; \
	fi
	@echo "Or run 'make hub-forward' to access via port-forward at :8081"
	@echo "Run 'make hub-status' to check the status"

hub-uninstall: ## Uninstall Tackle Hub
	@echo "Uninstalling Tackle Hub..."
	@$(KUBECTL) delete tackle tackle -n $(KONVEYOR_NAMESPACE) --ignore-not-found=true || true
	@$(KUBECTL) delete namespace $(KONVEYOR_NAMESPACE) --ignore-not-found=true || true
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
