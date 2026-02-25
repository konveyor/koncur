Automated build from `main` branch.

**Commit:** ${SHA}

## Downloads

| Platform | Binary |
|----------|--------|
| Linux (amd64) | `koncur-linux-amd64` |
| Linux (arm64) | `koncur-linux-arm64` |
| macOS (Intel) | `koncur-darwin-amd64` |
| macOS (Apple Silicon) | `koncur-darwin-arm64` |
| Windows (amd64) | `koncur-windows-amd64.exe` |

## Quick Start

```bash
# Download the binary for your platform (example: Linux amd64)
curl -Lo koncur https://github.com/${REPO}/releases/download/latest/koncur-linux-amd64
chmod +x koncur

# Download the test archive
curl -Lo koncur-tests.tar.gz https://github.com/${REPO}/releases/download/latest/koncur-tests.tar.gz

# Run tests against your Tackle Hub
./koncur run --test-archive koncur-tests.tar.gz -t tackle-hub -c target.yaml
```

See the [Local Testing with Custom Images](docs/local-testing-custom-images.md) guide for more details.
