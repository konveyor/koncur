# Local Testing with Custom Images

This guide explains how to run Koncur tests against locally-built or custom container images for both Kantra CLI and Tackle Hub.

## Overview

When developing Konveyor components (analyzer, providers, hub, etc.), you often need to test against your own builds rather than the published images. Koncur supports this through:

- **Kantra CLI**: Point Koncur at a custom-built kantra binary (which itself uses a configurable container image)
- **Tackle Hub**: Override any component image when deploying Hub into a Kind cluster
- **Portable test archive**: Run the full test suite without cloning the repo using `--test-archive`

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) installed and running
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) installed
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed
- Go 1.25+ (for building Koncur from source)

## Using a Test Archive (Portable Testing)

You don't need to clone the Koncur repository to run the test suite. A portable test archive (`koncur-tests.tar.gz`) bundles all test definitions and expected outputs into a ~200KB file.

### Building the Archive

From the Koncur repository:

```bash
make test-archive
# Creates: koncur-tests.tar.gz (~200KB)
```

### Running Tests from an Archive

With just the `koncur` binary and `koncur-tests.tar.gz`:

```bash
# Run against Tackle Hub
koncur run --test-archive koncur-tests.tar.gz \
  -t tackle-hub --target-config target-tackle-hub.yaml

# Run against Kantra CLI
koncur run --test-archive koncur-tests.tar.gz \
  -t kantra --target-config target-kantra.yaml

# Filter to specific tests
koncur run --test-archive koncur-tests.tar.gz \
  -t tackle-hub --target-config target-tackle-hub.yaml \
  --filter "daytrader"

# Skip tests requiring Maven settings
koncur run --test-archive koncur-tests.tar.gz \
  -t tackle-hub --target-config target-tackle-hub.yaml \
  --skip-maven
```

The archive is extracted to a temporary directory that is automatically cleaned up after the run.

### What's in the Archive

The archive contains only `test.yaml` and `expected-output.yaml` files from the test suite. Application source code is not included -- tests that use git URLs will clone their application source at runtime. Tests that reference local binary files (`.war`, `.ear`) will be skipped since those files are not in the archive.

### End-to-End Example: Bring Your Own Hub

If you already have a Tackle Hub instance running (on any cluster, not just Kind):

```bash
# 1. Create a target config pointing to your Hub
cat > target-tackle-hub.yaml <<EOF
type: tackle-hub
tackleHub:
  url: http://your-hub-instance:8080/hub
  token: ""
EOF

# 2. Run the test suite
koncur run --test-archive koncur-tests.tar.gz \
  -t tackle-hub --target-config target-tackle-hub.yaml --skip-maven
```

This is the fastest way to validate a Hub deployment with custom images -- no repo clone, no Kind cluster setup, just the binary + archive + target config.

## Kantra CLI with Custom Images

Kantra runs analysis inside a container. You can use a custom-built kantra binary and/or a custom runner image.

### Using a Custom Kantra Binary

If you've built kantra from source:

```bash
# Build kantra from your local checkout
cd /path/to/kantra
go build -o kantra ./cmd/kantra

# Create a target config pointing to your binary
cat > .koncur/config/target-kantra.yaml <<EOF
type: kantra
kantra:
  binaryPath: /path/to/kantra/kantra
EOF

# Run tests with your custom binary
./koncur run tests -t kantra --target-config .koncur/config/target-kantra.yaml
```

Or set the binary path inline:

```bash
./koncur run tests/tackle-testapp-with-deps/test.yaml \
  --target-config .koncur/config/target-kantra.yaml
```

### Using Custom Provider Images with Kantra

When kantra runs in container mode (`--run-local=false`, which koncur uses by default), it pulls container images for each language provider. You can override these images directly in the kantra target config:

```yaml
type: kantra
kantra:
  binaryPath: /usr/local/bin/kantra
  runnerImage: my-kantra:dev
  javaProviderImage: my-java-provider:dev
  genericProviderImage: my-generic-provider:dev
  csharpProviderImage: my-csharp-provider:dev
```

Koncur sets the corresponding environment variables (`RUNNER_IMG`, `JAVA_PROVIDER_IMG`, `GENERIC_PROVIDER_IMG`, `CSHARP_PROVIDER_IMG`) before invoking kantra, so kantra picks up your custom images automatically.

#### Available Image Overrides

| Config Field | Kantra Env Variable | Default |
|---|---|---|
| `runnerImage` | `RUNNER_IMG` | `quay.io/konveyor/kantra` |
| `javaProviderImage` | `JAVA_PROVIDER_IMG` | `quay.io/konveyor/java-external-provider:latest` |
| `genericProviderImage` | `GENERIC_PROVIDER_IMG` | `quay.io/konveyor/generic-external-provider:latest` |
| `csharpProviderImage` | `CSHARP_PROVIDER_IMG` | `quay.io/konveyor/c-sharp-provider:latest` |

The generic provider image is used for Python, Node.js, and Go analysis.

#### Example: Testing a Custom Java Provider

```bash
# Build your custom Java provider image
cd /path/to/java-external-provider
docker build -t my-java-provider:dev .

# Create a target config with the override
cat > target-kantra.yaml <<EOF
type: kantra
kantra:
  javaProviderImage: my-java-provider:dev
EOF

# Run the test suite
koncur run tests -t kantra --target-config target-kantra.yaml

# Or with a test archive (no repo clone needed)
koncur run --test-archive koncur-tests.tar.gz \
  -t kantra --target-config target-kantra.yaml
```

#### Example: Testing a Custom Kantra Runner

```bash
# Build the kantra runner image
cd /path/to/kantra
docker build -t my-kantra:dev .

# Create a target config
cat > target-kantra.yaml <<EOF
type: kantra
kantra:
  runnerImage: my-kantra:dev
EOF

koncur run tests -t kantra --target-config target-kantra.yaml
```

You can also override images via environment variables directly, without modifying the target config:

```bash
JAVA_PROVIDER_IMG=my-java-provider:dev \
koncur run tests -t kantra --target-config target-kantra.yaml
```

When both the config file and environment variable are set, the config file values take precedence (they override the env var for the kantra subprocess).

## Tackle Hub with Custom Images

Tackle Hub runs as multiple components in Kubernetes. You can override any component image either through the target config (which patches the Tackle CR automatically) or via Makefile env vars at install time.

### Quick Start (Config-Driven)

The simplest approach -- put image overrides in the target config:

```bash
# 1. Build your custom image(s)
docker build -t my-analyzer:dev /path/to/analyzer

# 2. Set up the cluster (first time only)
make setup

# 3. Load your image into Kind
kind load docker-image my-analyzer:dev --name koncur-test

# 4. Create a target config with image overrides
cat > target-tackle-hub.yaml <<EOF
type: tackle-hub
tackleHub:
  url: http://localhost:8080/hub
  images:
    analyzer: my-analyzer:dev
EOF

# 5. Run tests -- koncur patches the Tackle CR and waits for readiness automatically
./koncur run tests -t tackle-hub --target-config target-tackle-hub.yaml
```

When koncur sees `images` in the target config, it:
1. Patches the Tackle CR on the cluster via `kubectl`
2. Waits for the Hub pod to become ready with the new images
3. Runs the tests

This means you can iterate by just changing the target config and re-running -- no need to `hub-uninstall` / `hub-install`.

### Quick Start (Makefile-Driven)

Alternatively, override images at install time via Makefile env vars:

```bash
# 1. Build your custom image(s)
docker build -t my-analyzer:dev /path/to/analyzer

# 2. Create the Kind cluster
make kind-create

# 3. Load your image into Kind
kind load docker-image my-analyzer:dev --name koncur-test

# 4. Install Hub with your custom image
ANALYZER_ADDON=my-analyzer:dev make hub-install

# 5. Build Koncur and run tests
make build
./koncur run tests -t tackle-hub --target-config .koncur/config/target-tackle-hub.yaml
```

### Step-by-Step Guide

#### 1. Build Your Custom Images

Build whichever component(s) you're working on:

```bash
# Example: build a custom analyzer addon
cd /path/to/tackle2-addon-analyzer
docker build -t my-analyzer:dev .

# Example: build a custom Java provider
cd /path/to/java-external-provider
docker build -t my-java-provider:dev .

# Example: build a custom hub
cd /path/to/tackle2-hub
docker build -t my-hub:dev .
```

#### 2. Create the Kind Cluster

```bash
make kind-create
```

This creates a Kind cluster named `koncur-test` with ingress support and a cache mount for faster subsequent runs.

#### 3. Load Images into Kind

Kind clusters run in Docker and don't have access to your local Docker image cache. You must explicitly load images into the cluster:

```bash
# Load a single image
kind load docker-image my-analyzer:dev --name koncur-test

# Load multiple images
kind load docker-image my-hub:dev --name koncur-test
kind load docker-image my-java-provider:dev --name koncur-test
kind load docker-image my-analyzer:dev --name koncur-test
```

**Important**: The image tag must not be `:latest` when loading local images into Kind. Kind treats `:latest` specially and will always attempt to pull from a remote registry. Use a tag like `:dev`, `:local`, or a specific version instead.

#### 4. Install Tackle Hub with Custom Images

Override one or more images using environment variables:

```bash
# Override just the analyzer
ANALYZER_ADDON=my-analyzer:dev make hub-install

# Override multiple components
HUB=my-hub:dev \
ANALYZER_ADDON=my-analyzer:dev \
JAVA_PROVIDER_IMG=my-java-provider:dev \
make hub-install
```

#### 5. Verify the Deployment

```bash
# Check all pods are running
make hub-status

# Verify your custom image is being used
kubectl get pods -n konveyor-tackle -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{range .spec.containers[*]}{.image}{"\n"}{end}{end}'
```

#### 6. Run Tests

```bash
make build

# Run all tests
./koncur run tests -t tackle-hub --target-config .koncur/config/target-tackle-hub.yaml

# Run a specific test
./koncur run tests/tackle-testapp-with-deps/test.yaml \
  -t tackle-hub --target-config .koncur/config/target-tackle-hub.yaml

# Filter tests by name
./koncur run tests -t tackle-hub \
  --target-config .koncur/config/target-tackle-hub.yaml \
  --filter "tackle-testapp"
```

### Available Image Overrides

Images can be overridden in two ways: via the target config (`images` block) or via Makefile env vars.

| Target Config Field | Makefile Env Var | Tackle CR Field | Default |
|---|---|---|---|
| `images.hub` | `HUB` | `hub_image_fqin` | `quay.io/konveyor/tackle2-hub:latest` |
| `images.analyzer` | `ANALYZER_ADDON` | `analyzer_fqin` | `quay.io/konveyor/tackle2-addon-analyzer:latest` |
| `images.javaProvider` | `JAVA_PROVIDER_IMG` | `provider_java_image_fqin` | `quay.io/konveyor/java-external-provider:latest` |
| `images.genericProvider` | `GENERIC_PROVIDER_IMG` | `provider_python_image_fqin` + `provider_nodejs_image_fqin` | `quay.io/konveyor/generic-external-provider:latest` |
| `images.csharpProvider` | `CSHARP_PROVIDER_IMG` | `provider_c_sharp_image_fqin` | `quay.io/konveyor/c-sharp-provider:latest` |
| `images.runner` | `RUNNER_IMG` | `kantra_fqin` | `quay.io/konveyor/kantra:latest` |
| `images.discoveryAddon` | `DISCOVERY_ADDON` | `language_discovery_fqin` | `quay.io/konveyor/tackle2-addon-discovery:latest` |
| `images.platformAddon` | `PLATFORM_ADDON` | `platform_fqin` | `quay.io/konveyor/tackle2-addon-platform:latest` |

#### Kubernetes Configuration

If your Tackle CR is in a non-default namespace or has a non-default name, set these in the target config:

| Config Field | Default | Description |
|---|---|---|
| `tackleHub.namespace` | `konveyor-tackle` | Kubernetes namespace containing the Tackle CR |
| `tackleHub.crName` | `tackle` | Name of the Tackle Custom Resource |

Example:

```yaml
type: tackle-hub
tackleHub:
  url: http://localhost:8080/hub
  namespace: my-konveyor-namespace
  crName: my-tackle
  images:
    analyzer: my-analyzer:dev
```

### Updating Images on a Running Cluster

The easiest way to iterate on images is to use the target config -- koncur handles the CR patching and readiness wait for you:

```bash
# 1. Build and load the new image
docker build -t my-analyzer:dev /path/to/analyzer
kind load docker-image my-analyzer:dev --name koncur-test

# 2. Just run tests -- koncur patches the CR and waits automatically
./koncur run tests -t tackle-hub --target-config target-tackle-hub.yaml
```

Where `target-tackle-hub.yaml` contains:
```yaml
type: tackle-hub
tackleHub:
  url: http://localhost:8080/hub
  images:
    analyzer: my-analyzer:dev
```

You can also patch the CR manually with `kubectl`:

```bash
kubectl patch tackle tackle -n konveyor-tackle --type merge \
  -p '{"spec": {"analyzer_fqin": "my-analyzer:dev"}}'
```

### Full Reinstall with New Images

If patching the CR doesn't pick up changes cleanly, do a full reinstall:

```bash
# Uninstall and reinstall (preserves the Kind cluster and cache)
make hub-uninstall

ANALYZER_ADDON=my-analyzer:dev make hub-install
```

This is faster than a full `make teardown && make setup` because it keeps the Kind cluster, ingress, and cache intact.

## Complete Workflow Examples

### Testing a Java Provider Change

```bash
# Build the provider
cd /path/to/java-external-provider
docker build -t my-java-provider:dev .
cd /path/to/koncur

# First time: set up cluster and install Hub with defaults
make setup

# Load your image into Kind
kind load docker-image my-java-provider:dev --name koncur-test

# Create target config with image override
cat > target-tackle-hub.yaml <<EOF
type: tackle-hub
tackleHub:
  url: http://localhost:8080/hub
  images:
    javaProvider: my-java-provider:dev
EOF

# Run tests -- koncur patches the CR and waits automatically
./koncur run tests -t tackle-hub --target-config target-tackle-hub.yaml

# --- Iterate ---

# Make changes, rebuild the provider image
cd /path/to/java-external-provider
docker build -t my-java-provider:dev .
cd /path/to/koncur

# Reload into Kind and re-run (koncur re-patches the CR each time)
kind load docker-image my-java-provider:dev --name koncur-test
./koncur run tests -t tackle-hub --target-config target-tackle-hub.yaml
```

### Testing an Analyzer Change

```bash
# Build the analyzer addon
cd /path/to/tackle2-addon-analyzer
docker build -t my-analyzer:dev .
cd /path/to/koncur

# One-shot setup + test
make setup
kind load docker-image my-analyzer:dev --name koncur-test

cat > target-tackle-hub.yaml <<EOF
type: tackle-hub
tackleHub:
  url: http://localhost:8080/hub
  images:
    analyzer: my-analyzer:dev
EOF

./koncur run tests -t tackle-hub --target-config target-tackle-hub.yaml
```

### Testing Multiple Custom Components Together

```bash
# Build all custom images
docker build -t my-hub:dev /path/to/tackle2-hub
docker build -t my-analyzer:dev /path/to/tackle2-addon-analyzer
docker build -t my-java-provider:dev /path/to/java-external-provider

# Set up cluster and load all images
make setup
kind load docker-image my-hub:dev --name koncur-test
kind load docker-image my-analyzer:dev --name koncur-test
kind load docker-image my-java-provider:dev --name koncur-test

# Create target config with all overrides
cat > target-tackle-hub.yaml <<EOF
type: tackle-hub
tackleHub:
  url: http://localhost:8080/hub
  images:
    hub: my-hub:dev
    analyzer: my-analyzer:dev
    javaProvider: my-java-provider:dev
EOF

# Run tests
./koncur run tests -t tackle-hub --target-config target-tackle-hub.yaml
```

## Troubleshooting

### Image Pull Errors (ErrImagePull / ImagePullBackOff)

If pods show `ErrImagePull` or `ImagePullBackOff`:

1. **Check the image was loaded into Kind**:
   ```bash
   docker exec koncur-test-control-plane crictl images | grep my-image
   ```

2. **Avoid the `:latest` tag** for local images. Kind always tries to pull `:latest` from a registry. Use `:dev` or any other non-latest tag.

3. **Reload the image** if you rebuilt it:
   ```bash
   kind load docker-image my-image:dev --name koncur-test
   ```

### Pods Not Picking Up New Image

If you reloaded an image with the same tag but pods still use the old version:

1. **Delete the pods** to force them to restart with the new image:
   ```bash
   kubectl delete pods -n konveyor-tackle --all
   ```

2. **Wait for readiness**:
   ```bash
   kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=tackle-hub \
     -n konveyor-tackle --timeout=300s
   ```

### Hub Not Ready After Image Override

Some components depend on others. If Hub isn't becoming ready:

1. **Check pod status**:
   ```bash
   make hub-status
   ```

2. **Check pod logs** for the failing component:
   ```bash
   kubectl logs -n konveyor-tackle -l app.kubernetes.io/name=tackle-hub
   kubectl logs -n konveyor-tackle -l app.kubernetes.io/component=analyzer
   ```

3. **Check events** for scheduling or resource issues:
   ```bash
   kubectl get events -n konveyor-tackle --sort-by='.lastTimestamp'
   ```

### Tests Fail After Image Change

If tests were passing with upstream images but fail with your custom images:

1. **Generate new expected output** if your change intentionally modifies analysis results:
   ```bash
   ./koncur generate -d tests -t tackle-hub \
     --target-config .koncur/config/target-tackle-hub.yaml
   ```

2. **Compare the diff** to understand what changed:
   ```bash
   git diff tests/
   ```

3. **Run a single test with verbose output** to see details:
   ```bash
   ./koncur run tests/tackle-testapp-with-deps/test.yaml \
     -t tackle-hub --target-config .koncur/config/target-tackle-hub.yaml -v
   ```

## See Also

- [Cache Configuration](cache-configuration.md) - How the cache system works for faster iteration
- [Configuration Guide](configuration-guide.md) - General Koncur configuration
