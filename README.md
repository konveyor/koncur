# Koncur

> **Koncur** - A test harness for Konveyor tools that "concurs with your expected results!"

Koncur is a declarative test harness for running and validating end-to-end tests for Konveyor tools (Kantra, Tackle, Kai). Define **what** you want to analyze, not **how** to run commands.

## Features

- **Declarative test definitions** - Specify application, label selector, and analysis mode
- **Multiple execution targets** - Kantra CLI, Tackle Hub API, Tackle UI, Kai RPC, VSCode extension
- **Flexible target configuration** - Separate target config from test definitions
- **Exact match validation** - Compare actual output against expected RuleSets
- **Clear diff output** - See exactly what differs when tests fail
- **Multiple input formats** - Support for inline expected results or file references
- **Built on Konveyor types** - Uses analyzer-lsp RuleSet structures directly
- **Automatic output filtering** - Filters empty rulesets for cleaner expected outputs
- **Test output management** - Clean up old test runs with the `clean` command

## Installation

```bash
go build -o koncur ./cmd/koncur
```

## Quick Start

### 1. Create a test definition

```yaml
# my-test.yaml
name: "Sample Kantra Test"
description: "Test cloud-readiness analysis"

analysis:
  application: /path/to/application/source
  labelSelector: "konveyor.io/target=quarkus"
  analysisMode: source-only

expect:
  exitCode: 0
  output:
    result:
      - name: cloud-readiness
        violations:
          session-00001:
            description: "Avoid use of HttpSession"
            category: mandatory
            incidents:
              - uri: "file:///src/main/java/MyServlet.java"
                lineNumber: 42
```

### 2. Run the test

```bash
# Use default target (kantra)
koncur run my-test.yaml

# Specify target type
koncur run my-test.yaml --target kantra

# Use a target configuration file
koncur run my-test.yaml --target-config target-tackle-hub.yaml
```

## Test Definition Format

```yaml
name: "Test Name"
description: "Optional description"

analysis:
  # Application to analyze (file path or git URL)
  application: /path/to/source

  # Optional: Label selector expression
  labelSelector: "konveyor.io/target=quarkus"

  # Analysis mode: source-only | full
  analysisMode: source-only

# Optional: Execution timeout (default: 5m)
timeout: 10m

# Optional: Work directory (default: .koncur/output)
workDir: /tmp/my-tests

expect:
  exitCode: 0
  output:
    # Option 1: Inline expected RuleSets
    result:
      - name: ruleset-name
        violations: {...}

    # Option 2: Reference to external file
    file: /absolute/path/to/expected.yaml
```

## Target Configuration

Target configuration is separate from test definitions, allowing the same test to run against different targets/environments.

### Kantra (CLI)

```yaml
type: kantra
kantra:
  binaryPath: /usr/local/bin/kantra  # Optional
```

### Tackle Hub (API)

```yaml
type: tackle-hub
tackleHub:
  url: https://tackle-hub.example.com
  username: admin
  password: secret
  # Or use token:
  # token: your-api-token
```

### Tackle UI (Browser Automation)
**Not Implemented**

```yaml
type: tackle-ui
tackleUI:
  url: https://tackle.example.com
  username: admin
  password: secret
  browser: chrome  # chrome or firefox
  headless: true
```

### Kai RPC

**Not Implemented**

```yaml
type: kai-rpc
kaiRPC:
  host: localhost
  port: 8080
```

### VSCode Extension

** Not Implemented **

```yaml
type: vscode
vscode:
  binaryPath: /usr/local/bin/code  # Optional
  extensionId: konveyor.konveyor-analyzer
  workspaceDir: /path/to/workspace  # Optional
```

## Commands

### `koncur run <test-file>`

Execute a test and validate output against expected results.

```bash
koncur run testdata/examples/sample_test.yaml
```

### `koncur validate <test-file>`

Validate a test definition without running it.

```bash
koncur validate testdata/examples/sample_test.yaml
```

### `koncur generate`

Generate expected outputs by running tests and capturing their results. This command:
- Finds all `test.yaml` files in the specified directory
- Executes each test using the specified target
- Filters out empty rulesets (no violations, insights, or tags)
- Saves the filtered output as `expected-output.yaml` in each test directory
- Updates test definitions to use file-based expectations

```bash
# Generate expected outputs for all tests
koncur generate -d ./tests

# Generate for a specific test
koncur generate -d ./tests/my-test

# Filter by test name pattern
koncur generate -d ./tests --filter "tackle"

# Dry run (show what would be done)
koncur generate -d ./tests --dry-run

# Use a specific target
koncur generate -d ./tests --target kantra
```

**Flags:**
- `-d, --test-dir` - Directory containing test definitions (default: `./tests`)
- `-f, --filter` - Filter tests by name pattern
- `--dry-run` - Show what would be done without executing
- `-t, --target` - Target type to use (default: `kantra`)

### `koncur clean`

Clean up old test run outputs from the `.koncur/output` directory.

By default, keeps the most recent run for each test and deletes older ones.

```bash
# Clean old test runs (keeps latest for each test)
koncur clean

# Preview what would be deleted
koncur clean --dry-run

# Remove all output directories
koncur clean --all

# Preview removing everything
koncur clean --all --dry-run
```

**Flags:**
- `--all` - Remove all output directories (not just old ones)
- `--dry-run` - Show what would be deleted without actually deleting

### Global Flags

- `-v, --verbose` - Enable verbose logging

## Examples

See `testdata/examples/` for sample test definitions.

## Architecture

- **`pkg/config/`** - Test definition types and loading
- **`pkg/targets/`** - Target executors (Kantra, Tackle, Kai)
- **`pkg/parser/`** - Output parsing (RuleSets)
- **`pkg/validator/`** - Exact match validation with diff
- **`pkg/cli/`** - CLI commands

## Development

```bash
# Build
go build -o koncur ./cmd/koncur

# Run tests
go test ./...

# Validate a test definition
./koncur validate testdata/examples/sample_test.yaml
```

## Testing Against Tackle Hub

Koncur includes a Makefile for quickly setting up and testing against a local Tackle Hub instance running in Kind (Kubernetes in Docker).

### Quick Setup

```bash
# Complete setup: create cluster, install hub, build binary
make setup

# This runs:
# 1. make kind-create  - Creates Kind cluster with ingress
# 2. make hub-install  - Installs Tackle Hub with OLM
# 3. make build        - Builds the koncur binary
```

### Accessing Tackle Hub

Once setup is complete, Tackle Hub is accessible via:

**Ingress (recommended):**
- Hub API: `http://localhost:8080/hub`
- Hub UI: `http://localhost:8080/hub`

**Port-forward (alternative):**
```bash
make hub-forward
# Hub will be available at http://localhost:8081
```

### Running Tests

```bash
# Run a test against Tackle Hub
make test-hub

# Or run manually with koncur
./koncur run tests/tackle-testapp-with-deps/test.yaml \
  --target-config .koncur/config/target-tackle-hub.yaml
```

### Makefile Targets

**Setup & Teardown:**
- `make setup` - Complete setup (cluster + hub + build)
- `make teardown` - Complete teardown (uninstall hub + delete cluster)

**Cluster Management:**
- `make kind-create` - Create Kind cluster with ingress-nginx
- `make kind-delete` - Delete the Kind cluster

**Tackle Hub:**
- `make hub-install` - Install Tackle Hub (OLM + operator + CR)
- `make hub-uninstall` - Uninstall Tackle Hub
- `make hub-status` - Check Tackle Hub status
- `make hub-forward` - Port-forward to access Hub at :8081

**Build & Test:**
- `make build` - Build koncur binary
- `make test-hub` - Run tackle-testapp test against Hub
- `make clean` - Clean build artifacts and test outputs

### Configuration

The Makefile uses these configurable variables:

```bash
KIND_CLUSTER_NAME ?= koncur-test
KONVEYOR_NAMESPACE ?= konveyor-tackle
KUBECTL ?= kubectl
```

### Target Configuration

The default Tackle Hub target config (`.koncur/config/target-tackle-hub.yaml`):

```yaml
type: tackle-hub
tackleHub:
  url: http://localhost:8080/hub
  token: ""
  mavenSettings: settings.xml
```

### Troubleshooting

**Ingress not working:**
- Ensure Kind cluster was created with ingress support: `kubectl get pods -n ingress-nginx`
- Verify ingress controller is running and ready
- Check ingress resource: `kubectl get ingress -n konveyor-tackle`

**Operator not ready:**
- Check operator logs: `kubectl logs -n konveyor-tackle -l name=tackle-operator`
- Verify CRD installed: `kubectl get crd tackles.tackle.konveyor.io`

**Hub pods not starting:**
- Check pod status: `make hub-status`
- View pod logs: `kubectl logs -n konveyor-tackle -l app.kubernetes.io/name=tackle-hub`

## License

Apache 2.0
