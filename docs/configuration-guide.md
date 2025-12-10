# Configuration Guide

This guide explains how to create and manage configuration files for Koncur using the `koncur config` command.

## Overview

Koncur uses two types of configuration files:

1. **Target Configuration** - Defines where and how to run analysis (Kantra CLI, Tackle Hub, VSCode, etc.)
2. **Test Configuration** - Defines what to analyze and what results to expect

## Quick Start

### Generate a Target Configuration

```bash
# Interactive mode - prompts for all options
koncur config target

# Specify target type directly
koncur config target --type tackle-hub

# Specify output location
koncur config target --type kantra --output my-kantra-config.yaml
```

### Generate a Test Configuration

```bash
# Interactive mode - prompts for test details
koncur config test

# Specify output location
koncur config test --output my-test.yaml
```

## Target Configuration

Target configurations specify which execution environment to use for running analysis.

### Supported Target Types

- **kantra** - Kantra CLI execution (local binary)
- **tackle-hub** - Tackle Hub API execution (requires Hub instance)
- **tackle-ui** - Tackle UI browser automation (not yet implemented)
- **kai-rpc** - Kai analyzer RPC (not yet implemented)
- **vscode** - VSCode extension execution (not yet implemented)

### Kantra Target

Runs analysis using the Kantra CLI binary.

#### Interactive Creation

```bash
koncur config target --type kantra
```

You'll be prompted for:
- **Kantra binary path** (optional) - Path to kantra binary, or press Enter to use PATH
- **Maven settings.xml path** (optional) - Path to Maven settings file for dependency resolution

#### Example Output

```yaml
type: kantra
kantra:
  binaryPath: /usr/local/bin/kantra
  mavenSettings: /home/user/.m2/settings.xml
```

#### Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Must be `"kantra"` |
| `kantra.binaryPath` | string | No | Path to kantra binary. If not specified, uses `kantra` from PATH |
| `kantra.mavenSettings` | string | No | Path to Maven settings.xml for dependency resolution |

### Tackle Hub Target

Runs analysis using the Tackle Hub API.

#### Interactive Creation

```bash
koncur config target --type tackle-hub
```

You'll be prompted for:
- **Tackle Hub URL** (default: http://localhost:8081)
- **Authentication method**:
  - Token - API token authentication
  - Username/Password - Basic authentication
  - None - No authentication (for instances with auth disabled)
- **Maven settings.xml path** (optional)

#### Example Output (Token Auth)

```yaml
type: tackle-hub
tackleHub:
  url: http://localhost:8081
  token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
  mavenSettings: /home/user/.m2/settings.xml
```

#### Example Output (Username/Password Auth)

```yaml
type: tackle-hub
tackleHub:
  url: http://localhost:8081
  username: admin
  password: Passw0rd!
  mavenSettings: /home/user/.m2/settings.xml
```

#### Example Output (No Auth)

```yaml
type: tackle-hub
tackleHub:
  url: http://localhost:8081
  token: ""
  mavenSettings: /home/user/.m2/settings.xml
```

#### Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Must be `"tackle-hub"` |
| `tackleHub.url` | string | Yes | Base URL of Tackle Hub instance |
| `tackleHub.token` | string | No | API token for authentication |
| `tackleHub.username` | string | No | Username for basic auth (alternative to token) |
| `tackleHub.password` | string | No | Password for basic auth (requires username) |
| `tackleHub.mavenSettings` | string | No | Path to Maven settings.xml |

### Tackle UI Target

⚠️ **Not Yet Implemented**

Runs analysis using browser automation against the Tackle UI.

#### Interactive Creation

```bash
koncur config target --type tackle-ui
```

You'll be prompted for:
- **Tackle UI URL** (default: http://localhost:8080)
- **Username** (default: admin)
- **Password**
- **Browser** (chrome or firefox)
- **Headless mode** (true or false)

#### Example Output

```yaml
type: tackle-ui
tackleUI:
  url: http://localhost:8080
  username: admin
  password: Passw0rd!
  browser: chrome
  headless: true
```

### Kai RPC Target

⚠️ **Not Yet Implemented**

Runs analysis using the Kai analyzer RPC interface.

#### Interactive Creation

```bash
koncur config target --type kai-rpc
```

You'll be prompted for:
- **Kai RPC Host** (default: localhost)
- **Kai RPC Port** (default: 8080)

#### Example Output

```yaml
type: kai-rpc
kaiRPC:
  host: localhost
  port: 8080
```

### VSCode Target

⚠️ **Not Yet Implemented**

Runs analysis using the VSCode Konveyor extension.

#### Interactive Creation

```bash
koncur config target --type vscode
```

You'll be prompted for:
- **VSCode binary path** (optional)
- **Extension ID** (default: konveyor.konveyor-analyzer)
- **Workspace directory** (optional)

#### Example Output

```yaml
type: vscode
vscode:
  binaryPath: /usr/local/bin/code
  extensionID: konveyor.konveyor-analyzer
  workspaceDir: /home/user/workspace
```

## Test Configuration

Test configurations define what to analyze and what results to expect.

### Interactive Creation

```bash
koncur config test
```

You'll be prompted for:
- **Test name** - Descriptive name for the test
- **Test description** (optional)
- **Application path or git URL** - Local path or git repository URL
- **Label selector** (optional) - Label selector for rule filtering (e.g., `konveyor.io/target=cloud-readiness`)
- **Analysis mode** - `source-only` or `full`

### Example Output

```yaml
name: test-daytrader-app
description: Test analysis of DayTrader application
analysis:
  application: https://github.com/konveyor/example-applications#daytrader
  labelSelector: konveyor.io/target=cloud-readiness
  analysisMode: source-only
expect:
  exitCode: 0
  output:
    result: []
```

### Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Test name |
| `description` | string | No | Test description |
| `analysis.application` | string | Yes | Local path or git URL. Git URLs can include branch: `url#branch` |
| `analysis.labelSelector` | string | No | Label selector for filtering rules |
| `analysis.analysisMode` | string | Yes | `source-only` or `full` (with dependencies) |
| `expect.exitCode` | int | Yes | Expected exit code (typically 0) |
| `expect.output.result` | array | Yes | Expected rulesets (populated by `koncur generate`) |

### Git URL Format

Git URLs can include a branch reference:

```yaml
application: https://github.com/owner/repo#branch-name
```

Examples:
- `https://github.com/konveyor/example-app#main`
- `https://github.com/konveyor/example-app#v1.0.0`
- `https://github.com/konveyor/example-app` (uses default branch)

### Analysis Modes

#### source-only

Analyzes only the application source code, without resolving dependencies.

- Faster execution
- Suitable for rule testing
- May miss dependency-related issues

#### full

Analyzes application with full dependency resolution.

- Requires Maven/Gradle configuration
- More comprehensive analysis
- Takes longer to execute
- May require Maven settings for private repositories

## Generating Expected Output

After creating a test configuration, you need to populate the expected output:

```bash
# Run the test once to generate expected output
koncur generate test.yaml --target-config target-config.yaml

# This updates test.yaml with actual results
```

The `generate` command:
1. Runs the analysis
2. Captures the actual output
3. Updates `expect.output.result` in the test file
4. Creates a baseline for future validation

## Default Output Locations

If you don't specify an output path with `--output`:

- **Target configs**: `.koncur/config/target-<type>.yaml`
- **Test configs**: `./test.yaml`

## Common Workflows

### Setting up Tackle Hub Testing

```bash
# 1. Start Tackle Hub (or use make hub-forward)
make hub-forward

# 2. Generate target configuration
koncur config target --type tackle-hub --output .koncur/config/target-tackle-hub.yaml

# 3. Generate test configuration
koncur config test --output my-app/test.yaml

# 4. Generate expected output
koncur generate my-app/test.yaml --target-config .koncur/config/target-tackle-hub.yaml

# 5. Run the test
koncur run my-app/test.yaml --target-config .koncur/config/target-tackle-hub.yaml
```

### Setting up Kantra Testing

```bash
# 1. Ensure kantra is installed
which kantra

# 2. Generate target configuration
koncur config target --type kantra --output .koncur/config/target-kantra.yaml

# 3. Generate test configuration
koncur config test --output my-app/test.yaml

# 4. Generate expected output
koncur generate my-app/test.yaml --target-config .koncur/config/target-kantra.yaml

# 5. Run the test
koncur run my-app/test.yaml --target-config .koncur/config/target-kantra.yaml
```

### Testing with Maven Private Repositories

If your application requires access to private Maven repositories:

```bash
# 1. Create target config with Maven settings
koncur config target --type tackle-hub
# When prompted for Maven settings, provide: /home/user/.m2/settings.xml

# 2. Create test and mark it as requiring Maven settings
# Edit the test file and add:
requireMavenSettings: true

# 3. Run the test
koncur run test.yaml --target-config target-config.yaml
```

## Configuration Best Practices

### Version Control

**Commit**:
- Test configurations (`test.yaml`)
- Target configuration templates (without secrets)

**Don't Commit**:
- Target configurations with tokens/passwords
- Use environment variables or separate credential files

### Organizing Configurations

```
project/
├── .koncur/
│   └── config/
│       ├── target-kantra.yaml
│       ├── target-tackle-hub.yaml
│       └── target-tackle-hub.template.yaml  # Template without secrets
├── tests/
│   ├── app1/
│   │   └── test.yaml
│   ├── app2/
│   │   └── test.yaml
│   └── app3/
│       └── test.yaml
└── .gitignore  # Ignore configs with secrets
```

### Using Environment Variables

You can manually edit generated configs to use environment variables:

```yaml
type: tackle-hub
tackleHub:
  url: ${TACKLE_HUB_URL:-http://localhost:8081}
  token: ${TACKLE_HUB_TOKEN}
```

Note: Koncur doesn't currently expand environment variables automatically, but you can use tools like `envsubst` to preprocess configs.

## Validation

Validate your configurations before running tests:

```bash
# Validate test configuration
koncur validate test.yaml

# The validate command checks:
# - Required fields are present
# - Field types are correct
# - Git URLs are valid
# - Referenced files exist
```

## Troubleshooting

### "Unknown command 'config'"

Make sure you've built the latest version:

```bash
go build -o koncur ./cmd/koncur
./koncur config --help
```

### Authentication Failures

For Tackle Hub with auth enabled:
1. Verify URL is correct
2. Check token/credentials are valid
3. Try accessing the Hub UI manually

For Tackle Hub with auth disabled:
1. Select "None" for authentication method
2. Leave token empty

### Maven Settings Not Found

Ensure the path to settings.xml is absolute:

```yaml
mavenSettings: /home/user/.m2/settings.xml  # Good
mavenSettings: ~/.m2/settings.xml           # May not work
mavenSettings: ../settings.xml              # May not work
```

## See Also

- [Running Tests](running-tests.md) - How to execute tests
- [Writing Tests](writing-tests.md) - Advanced test configuration
- [Target Types](target-types.md) - Detailed target type documentation
