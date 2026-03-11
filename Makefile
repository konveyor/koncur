.PHONY: help kind-create kind-delete hub-install hub-install-auth _hub-install hub-uninstall hub-forward hub-status hub-logs test-hub clean build setup setup-auth teardown test-archive

# Configuration
KIND_CLUSTER_NAME ?= koncur-test
KONVEYOR_NAMESPACE ?= konveyor-tackle
KUBECTL ?= kubectl
HOST_PORT ?= 8080
HOST_PORT_TLS ?= 8443

# Image FQINs with defaults
HUB ?= quay.io/konveyor/tackle2-hub:latest
ANALYZER_ADDON ?= quay.io/konveyor/tackle2-addon-analyzer:latest
CSHARP_PROVIDER_IMG ?= quay.io/konveyor/c-sharp-provider:latest
GENERIC_PROVIDER_IMG ?= quay.io/konveyor/generic-external-provider:latest
JAVA_PROVIDER_IMG ?= quay.io/konveyor/java-external-provider:latest
RUNNER_IMG ?= quay.io/konveyor/kantra:latest
DISCOVERY_ADDON ?= quay.io/konveyor/tackle2-addon-discovery:latest
PLATFORM_ADDON ?= quay.io/konveyor/tackle2-addon-platform:latest

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

##@ Cluster Management

kind-create: ## Create a Kind cluster for testing with ingress support
	@echo "Creating Kind cluster: $(KIND_CLUSTER_NAME)..."
	@mkdir -p -m 777 cache .koncur/config
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
	@printf '    hostPort: $(HOST_PORT)\n' >> .koncur/config/kind-config.yaml
	@printf '    protocol: TCP\n' >> .koncur/config/kind-config.yaml
	@printf '    listenAddress: "0.0.0.0"\n' >> .koncur/config/kind-config.yaml
	@printf '  - containerPort: 443\n' >> .koncur/config/kind-config.yaml
	@printf '    hostPort: $(HOST_PORT_TLS)\n' >> .koncur/config/kind-config.yaml
	@printf '    protocol: TCP\n' >> .koncur/config/kind-config.yaml
	@printf '    listenAddress: "0.0.0.0"\n' >> .koncur/config/kind-config.yaml
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
	@$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/refs/tags/controller-v1.13.7/deploy/static/provider/kind/deploy.yaml 
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

AUTH_ENABLED ?= false
TACKLE_ADMIN_USER ?= admin
TACKLE_ADMIN_PASS ?= Passw0rd!

hub-install: ## Install Tackle Hub on the Kind cluster (auth disabled)
	@$(MAKE) _hub-install AUTH_ENABLED=false

hub-install-auth: ## Install Tackle Hub with authentication enabled
	@$(MAKE) _hub-install AUTH_ENABLED=true

_hub-install: ## Internal target for hub installation
	@echo "Installing OLM..."
	@curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.38.0/install.sh | bash -s v0.38.0 || true
	@echo "Waiting for OLM to be ready..."
	@$(KUBECTL) wait --for=condition=ready pod -l app=olm-operator -n olm --timeout=300s
	@$(KUBECTL) wait --for=condition=ready pod -l app=catalog-operator -n olm --timeout=300s
	@echo "Restarting operatorhubio-catalog to ensure gRPC connectivity..."
	@$(KUBECTL) delete pod -n olm -l olm.catalogSource=operatorhubio-catalog --ignore-not-found=true || true
	@sleep 5
	@$(KUBECTL) wait --for=condition=ready pod -l olm.catalogSource=operatorhubio-catalog -n olm --timeout=120s || true
	@echo "Installing Tackle operator from main branch..."
	@$(KUBECTL) apply -f https://raw.githubusercontent.com/konveyor/tackle2-operator/main/tackle-k8s.yaml
	@echo "Waiting for Tackle CRD to be available..."
	@for i in $$(seq 1 120); do \
		$(KUBECTL) get crd tackles.tackle.konveyor.io >/dev/null 2>&1 && break || sleep 5; \
		if [ $$i -eq 120 ]; then echo "Timeout waiting for CRD to be created"; exit 1; fi; \
	done
	@$(KUBECTL) wait --for condition=established --timeout=300s crd/tackles.tackle.konveyor.io
	@echo "Waiting for operator to be ready..."
	@for i in $$(seq 1 120); do \
		if $(KUBECTL) wait --namespace ${KONVEYOR_NAMESPACE} --for=condition=ready pod --selector=name=tackle-operator --timeout=5s >/dev/null 2>&1; then \
			echo "Tackle operator is ready"; \
			break; \
		fi; \
		if [ $$i -eq 120 ]; then echo "Timeout waiting for operator to be ready"; exit 1; fi; \
		sleep 3; \
	done
	@echo "Pre-creating cache PV with fixed path..."
	@mkdir -m 777 -p cache/hub-cache
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
	@echo "Creating Tackle CR with auth=$(AUTH_ENABLED)..."
	@mkdir -p .koncur/config
	@printf 'kind: Tackle\n' > .koncur/config/tackle-cr.yaml
	@printf 'apiVersion: tackle.konveyor.io/v1alpha1\n' >> .koncur/config/tackle-cr.yaml
	@printf 'metadata:\n' >> .koncur/config/tackle-cr.yaml
	@printf '  name: tackle\n' >> .koncur/config/tackle-cr.yaml
	@printf '  namespace: ${KONVEYOR_NAMESPACE}\n' >> .koncur/config/tackle-cr.yaml
	@printf 'spec:\n' >> .koncur/config/tackle-cr.yaml
	@printf '  cache_storage_class: "manual"\n' >> .koncur/config/tackle-cr.yaml
	@printf '  cache_data_volume_size: "10Gi"\n' >> .koncur/config/tackle-cr.yaml
	@printf '  rwx_supported: "true"\n' >> .koncur/config/tackle-cr.yaml
	@printf '  provider_java_container_requests_cpu: "100m"\n' >> .koncur/config/tackle-cr.yaml
	@printf '  analyzer_container_requests_cpu: "100m"\n' >> .koncur/config/tackle-cr.yaml
	@printf '  provider_python_container_requests_cpu: "100m"\n' >> .koncur/config/tackle-cr.yaml
	@printf '  hub_image_fqin: $(HUB)\n' >> .koncur/config/tackle-cr.yaml
	@printf '  analyzer_fqin: $(ANALYZER_ADDON)\n' >> .koncur/config/tackle-cr.yaml
	@printf '  provider_c_sharp_image_fqin: $(CSHARP_PROVIDER_IMG)\n' >> .koncur/config/tackle-cr.yaml
	@printf '  provider_python_image_fqin: $(GENERIC_PROVIDER_IMG)\n' >> .koncur/config/tackle-cr.yaml
	@printf '  provider_nodejs_image_fqin: $(GENERIC_PROVIDER_IMG)\n' >> .koncur/config/tackle-cr.yaml
	@printf '  provider_java_image_fqin: $(JAVA_PROVIDER_IMG)\n' >> .koncur/config/tackle-cr.yaml
	@printf '  kantra_fqin: $(RUNNER_IMG)\n' >> .koncur/config/tackle-cr.yaml
	@printf '  language_discovery_fqin: $(DISCOVERY_ADDON)\n' >> .koncur/config/tackle-cr.yaml
	@printf '  platform_fqin: $(PLATFORM_ADDON)\n' >> .koncur/config/tackle-cr.yaml
	@printf '  feature_auth_required: "$(AUTH_ENABLED)"\n' >> .koncur/config/tackle-cr.yaml
	@$(KUBECTL) apply -f .koncur/config/tackle-cr.yaml
	@echo "Waiting for Tackle Hub to be ready (this may take a few minutes)..."
	@sleep 30
	@$(KUBECTL) wait --for=condition=ready pod -l app.kubernetes.io/name=tackle-hub -n ${KONVEYOR_NAMESPACE} --timeout=600s || true
	@echo "Waiting for Tackle Hub to be ready (this may take a few minutes)..."
	@sleep 30
	@$(KUBECTL) wait --for=condition=ready pod -l app.kubernetes.io/name=tackle-hub -n ${KONVEYOR_NAMESPACE} --timeout=600s || true
	@if [ "$(AUTH_ENABLED)" = "true" ]; then \
		echo "Waiting for Keycloak deployment to be created..."; \
		for i in $$(seq 1 120); do \
			$(KUBECTL) get deployment tackle-keycloak-sso -n ${KONVEYOR_NAMESPACE} >/dev/null 2>&1 && break || sleep 5; \
			if [ $$i -eq 120 ]; then echo "Timeout waiting for Keycloak deployment"; exit 1; fi; \
		done; \
		echo "Waiting for Keycloak to be ready..."; \
		$(KUBECTL) wait --for=condition=ready pod -l app.kubernetes.io/name=tackle-keycloak-sso -n ${KONVEYOR_NAMESPACE} --timeout=600s; \
		echo "Waiting for tackle ingress to be created..."; \
		for i in $$(seq 1 60); do \
			$(KUBECTL) get ingress tackle -n ${KONVEYOR_NAMESPACE} >/dev/null 2>&1 && break || sleep 5; \
			if [ $$i -eq 60 ]; then echo "Timeout waiting for tackle ingress"; exit 1; fi; \
		done; \
		echo "Creating NetworkPolicy to allow ingress-nginx to reach Keycloak..."; \
		printf 'apiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata:\n  name: tackle-keycloak-ingress\n  namespace: ${KONVEYOR_NAMESPACE}\n  labels:\n    app: tackle\nspec:\n  podSelector:\n    matchLabels:\n      role: tackle-keycloak-sso\n  policyTypes:\n  - Ingress\n  ingress:\n  - from:\n    - namespaceSelector:\n        matchLabels:\n          kubernetes.io/metadata.name: ingress-nginx\n    ports:\n    - port: 8080\n      protocol: TCP\n    - port: 8443\n      protocol: TCP\n' | $(KUBECTL) apply -f -; \
		echo "Configuring Keycloak hostname for https://localhost:$(HOST_PORT_TLS)/auth..."; \
		$(KUBECTL) set env deployment/tackle-keycloak-sso -n ${KONVEYOR_NAMESPACE} \
			KC_HOSTNAME=https://localhost:$(HOST_PORT_TLS)/auth \
			KC_HOSTNAME_BACKCHANNEL_DYNAMIC=true; \
		$(KUBECTL) patch deployment tackle-keycloak-sso -n ${KONVEYOR_NAMESPACE} --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/args", "value": ["-Djgroups.dns.query=mta-kc-discovery.openshift-mta", "--verbose", "start", "--hostname=https://localhost:$(HOST_PORT_TLS)/auth", "--hostname-backchannel-dynamic=true"]}]'; \
		echo "Waiting for Keycloak to restart with new configuration..."; \
		$(KUBECTL) rollout status deployment/tackle-keycloak-sso -n ${KONVEYOR_NAMESPACE} --timeout=180s; \
		$(KUBECTL) set env deployment/tackle-hub -n ${KONVEYOR_NAMESPACE} KEYCLOAK_REQ_PASS_UPDATE=false; \
		$(KUBECTL) rollout status deployment/tackle-hub -n ${KONVEYOR_NAMESPACE} --timeout=120s; \
		KC_POD=$$($(KUBECTL) get pods -n ${KONVEYOR_NAMESPACE} -l app.kubernetes.io/name=tackle-keycloak-sso -o jsonpath='{.items[0].metadata.name}'); \
		KC_PASS=$$($(KUBECTL) get secret tackle-keycloak-sso -n ${KONVEYOR_NAMESPACE} -o jsonpath='{.data.password}' | base64 -d); \
		$(KUBECTL) exec -n ${KONVEYOR_NAMESPACE} $$KC_POD -- /opt/keycloak/bin/kcadm.sh config credentials --server http://localhost:8080/auth --realm master --user admin --password "$$KC_PASS"; \
		echo "Waiting for admin user to be created in Keycloak..."; \
		ADMIN_USER_ID=""; \
		for i in $$(seq 1 30); do \
			ADMIN_USER_ID=$$($(KUBECTL) exec -n ${KONVEYOR_NAMESPACE} $$KC_POD -- /opt/keycloak/bin/kcadm.sh get users -r tackle -q username=$(TACKLE_ADMIN_USER) --fields id 2>/dev/null | grep -o '"id" *: *"[^"]*"' | cut -d'"' -f4); \
			if [ -n "$$ADMIN_USER_ID" ]; then \
				break; \
			fi; \
			echo "  Admin user not yet created, waiting 5s..."; \
			sleep 5; \
		done; \
		if [ -n "$$ADMIN_USER_ID" ]; then \
			$(KUBECTL) exec -n ${KONVEYOR_NAMESPACE} $$KC_POD -- /opt/keycloak/bin/kcadm.sh update users/$$ADMIN_USER_ID -r tackle -s 'requiredActions=[]'; \
		else \
			echo "Error: Admin user '$(TACKLE_ADMIN_USER)' was not created in Keycloak after 150s."; \
			exit 1; \
		fi; \
	fi
	@echo ""
	@echo "Tackle Hub installation complete!"
	@echo ""
	@if $(KUBECTL) get pods -n ingress-nginx --no-headers 2>/dev/null | grep -q ingress-nginx-controller; then \
		if [ "$(AUTH_ENABLED)" = "true" ]; then \
			echo "Access Tackle Hub via ingress at: https://localhost:$(HOST_PORT_TLS)"; \
			echo "(Auth enabled - HTTPS with self-signed certificate)"; \
		else \
			echo "Access Tackle Hub via ingress at: http://localhost:$(HOST_PORT)"; \
		fi; \
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

test-hub: build ## Test the Tackle Hub integration with koncur (matches CI)
	@echo "Testing Tackle Hub integration..."
	@echo ""
	@echo "Creating target configuration..."
	@mkdir -p .koncur/config
	@printf 'type: tackle-hub\n' > .koncur/config/target-tackle-hub.yaml
	@printf 'tackleHub:\n' >> .koncur/config/target-tackle-hub.yaml
	@printf '  url: http://localhost:8081\n' >> .koncur/config/target-tackle-hub.yaml
	@echo "Running all tests with Tackle Hub target..."
	./koncur run tests -t tackle-hub --target-config .koncur/config/target-tackle-hub.yaml -o yaml --output-file test-hub.yaml

##@ Build

build: ## Build the koncur binary
	@echo "Building koncur..."
	@go build -o koncur ./cmd/koncur
	@echo "Build complete: ./koncur"

test-archive: ## Build a portable test archive (koncur-tests.tar.gz)
	@echo "Building test archive..."
	@files=$$(cd tests && find . -type f \( -name 'test.yaml' -o -name 'expected-output.yaml' \) | sed 's|^\./||' | sort); \
	if [ -z "$$files" ]; then \
		echo "Error: no test files found in tests/"; \
		exit 1; \
	fi; \
	tar czf koncur-tests.tar.gz -C tests $$files
	@echo "Archive created: koncur-tests.tar.gz"
	@listing=$$(tar tzf koncur-tests.tar.gz); \
	echo "Contents:"; \
	echo "$$listing" | head -30; \
	echo "..."; \
	echo "Total files: $$(echo "$$listing" | wc -l | tr -d ' ')"
	@ls -lh koncur-tests.tar.gz | awk '{print "Size: " $$5}'

clean: ## Clean build artifacts and test outputs
	@echo "Cleaning build artifacts..."
	@rm -f koncur
	@rm -f koncur-tests.tar.gz
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

setup-auth: kind-create hub-install-auth build ## Complete setup with auth: create cluster, install hub with Keycloak, build binary
	@echo ""
	@echo "=========================================="
	@echo "Auth setup complete!"
	@echo "=========================================="
	@echo ""
	@echo "Access: https://localhost:$(HOST_PORT_TLS)"
	@echo "  User: $(TACKLE_ADMIN_USER)"
	@echo "  Pass: Passw0rd!
	@echo ""
	@echo "Note: Uses a self-signed certificate. Accept the browser warning to proceed."
	@echo "      Password is from the tackle-keycloak-sso secret in $(KONVEYOR_NAMESPACE)."
	@echo ""

teardown: hub-uninstall kind-delete ## Complete teardown: uninstall hub, delete cluster
	@echo "Teardown complete"
