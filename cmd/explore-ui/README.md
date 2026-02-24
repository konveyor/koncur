# Tackle UI Exploration Tool

Automated browser-based exploration of Tackle UI to document workflow selectors and DOM structure.

## Purpose

This tool uses go-playwright to:
1. Execute a complete Tackle UI analysis workflow
2. Document CSS selectors for all interactive elements
3. Capture HTML structure of insights/results
4. Generate screenshots at each workflow step
5. Produce comprehensive documentation for implementing `tackle_ui.go`

## Prerequisites

1. **Tackle UI running** at `http://localhost:8080`
2. **Go 1.25+** installed
3. **Playwright browsers** installed (see setup below)

## Setup

### 1. Install go-playwright dependency (already done)

```bash
# This was already done during tool creation
go get github.com/playwright-community/playwright-go
go mod tidy
```

### 2. Install Chromium browser

```bash
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install chromium
```

This downloads the Chromium browser to `~/.cache/ms-playwright/` (or similar on macOS/Windows).

**Expected output:**
```
Downloading Chromium 130.0.6723.31 (playwright build v1148) - 286.9 Mb
Chromium 130.0.6723.31 (playwright build v1148) downloaded to /Users/yourname/.cache/ms-playwright/chromium-1148
```

## Usage

```bash
# Ensure Tackle UI is running
curl http://localhost:8080

# Run exploration from project root
go run cmd/explore-ui/main.go
```

## What It Does

1. Loads test: `tests/tackle-testapp-package-filter/test.yaml`
2. Launches visible Chromium browser (SlowMo: 500ms)
3. Executes 13 workflow steps:
   - Navigate to Tackle UI
   - Create application with Git repository
   - Configure analysis with label selector
   - Start analysis
   - Wait for completion (~10 minutes)
   - Navigate to results
   - Extract insights structure
4. Generates report and screenshots

### Workflow Steps in Detail

| Step | Name | Description | Duration |
|------|------|-------------|----------|
| 1 | Navigate to Landing Page | Load Tackle UI, find navigation | ~2s |
| 2 | Navigate to Applications | Click Applications link | ~2s |
| 3 | Click Create Application | Open create application form | ~2s |
| 4 | Fill Application Form | Enter name, description, Git URL, branch | ~3s |
| 5 | Submit Application | Create the application | ~3s |
| 6 | Navigate to Analysis Config | Find and click Analyze button | ~2s |
| 7 | Configure Analysis | Fill label selector and other fields | ~2s |
| 8 | Start Analysis | Submit analysis configuration | ~2s |
| 9 | Wait for Completion | Poll status until "Succeeded" | ~10min |
| 10 | Navigate to Results | Click Results/Insights link | ~2s |
| 11 | Extract Insights Structure | Capture insights DOM structure | ~3s |
| 12 | Extract Sample Insights | Capture first 3 insight rows | ~2s |
| 13 | Document Tags | Find and document tags section | ~2s |

## Outputs

### Report

**Location:** `.koncur/docs/tackle-ui-exploration.md`

Contains:
- Workflow execution summary (duration, successes, failures)
- Detailed step-by-step log with timings
- All discovered selectors organized by element
- HTML structure samples (insights container, rows, etc.)
- Issues encountered during exploration
- Recommendations for implementation

### Screenshots

**Location:** `.koncur/docs/screenshots/step-*.png`

13 screenshots (one per workflow step):
- `step-01.png` - Landing page
- `step-02.png` - Applications page
- `step-03.png` - Create application form opened
- `step-04.png` - Form filled with test data
- `step-05.png` - Application created
- `step-06.png` - Analysis configuration opened
- `step-07.png` - Analysis configured
- `step-08.png` - Analysis started
- `step-09.png` - Analysis running/complete
- `step-10.png` - Results page navigation
- `step-11.png` - Insights structure extracted
- `step-12.png` - Sample insights captured
- `step-13.png` - Tags documented

## Expected Duration

- **Total time:** ~10-12 minutes
  - Setup/navigation: ~2 minutes
  - Analysis execution: ~10 minutes (from test timeout)
  - Results extraction: ~30 seconds

## Notes

- **Application Created:** "Tackle Testapp public with package filter"
- **NOT DELETED:** Application remains in Tackle UI for manual inspection
- **Browser Visible:** You'll see the browser window during execution (SlowMo: 500ms)
- **Manual Cleanup:** Delete the test application manually after reviewing results

## Troubleshooting

### "playwright not found" or "Failed to launch browser"

```bash
# Install Chromium browser
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install chromium
```

### "Tackle UI not accessible"

```bash
# Check if Tackle is running
curl http://localhost:8080

# If not running, start Tackle (adjust for your setup)
# Example using kind/kubectl:
kubectl port-forward svc/tackle-ui 8080:8080
```

### Browser crashes or hangs

- **Check available memory:** Chromium needs ~500MB RAM
- **Close other applications** to free up resources
- **Try headless mode:** Edit `main.go` line 105: `Headless: playwright.Bool(true)`
- **Increase timeout:** Edit test file to increase timeout from 10m to 15m

### "Failed to find element" errors

This is normal during exploration - the tool tries multiple selectors and documents what works. If critical elements can't be found:

1. Check Tackle UI version matches expectations
2. Manually inspect the UI to see if element structure changed
3. Screenshot will show the current state for debugging

### Analysis never completes

- Check Tackle Hub logs for errors
- Verify the test application can be analyzed successfully via Tackle Hub API
- Ensure Maven settings are configured if required

## Next Steps

After successful exploration:

1. **Review the report:**
   ```bash
   cat .koncur/docs/tackle-ui-exploration.md
   ```

2. **Examine screenshots:**
   ```bash
   ls -la .koncur/docs/screenshots/
   open .koncur/docs/screenshots/step-11.png  # macOS
   ```

3. **Verify all critical selectors documented:**
   - Applications navigation
   - Create application button and form fields
   - Analysis configuration fields
   - Insights container and row selectors

4. **Use findings to implement:**
   - `pkg/targets/internal/tackleui/selectors.go` - Constants from report
   - `pkg/targets/internal/tackleui/browser.go` - Browser automation helpers
   - `pkg/targets/tackle_ui.go` - Main target implementation

## Cleaning Up

After exploration:

```bash
# Delete the test application from Tackle UI manually
# Or via API:
# curl -X DELETE http://localhost:8080/hub/applications/{id}

# Optionally remove screenshots
rm -rf .koncur/docs/screenshots/

# Optionally remove report
rm .koncur/docs/tackle-ui-exploration.md
```

## Example Output

```
2026/02/19 20:30:00 🚀 Tackle UI Exploration Tool
2026/02/19 20:30:00 ============================================================
2026/02/19 20:30:00 ✅ Loaded test: Tackle Testapp public with package filter
2026/02/19 20:30:01 🌐 Launching Chromium browser...

2026/02/19 20:30:03 ▶️  Step 1: Navigate to Landing Page
2026/02/19 20:30:04    Landing URL: http://localhost:8080/applications
2026/02/19 20:30:04    ✓ Found Applications navigation: text=Applications
2026/02/19 20:30:04    ✅ Complete (1.20s)

[... steps 2-13 ...]

2026/02/19 20:42:15 ============================================================
2026/02/19 20:42:15 📊 EXPLORATION SUMMARY
2026/02/19 20:42:15 ============================================================
2026/02/19 20:42:15 Duration: 12m 15s
2026/02/19 20:42:15 Steps: 13
2026/02/19 20:42:15 Selectors Found: 24
2026/02/19 20:42:15 Screenshots: 13
2026/02/19 20:42:15 HTML Samples: 5
2026/02/19 20:42:15 
2026/02/19 20:42:15 📝 Report: .koncur/docs/tackle-ui-exploration.md
2026/02/19 20:42:15 📸 Screenshots: .koncur/docs/screenshots/
2026/02/19 20:42:15 
2026/02/19 20:42:15 ✅ Exploration complete!
2026/02/19 20:42:15 ============================================================
```

## Success Criteria

The exploration is successful when:

- ✅ All 13 steps execute (some failures are OK, we document them)
- ✅ Report contains at least 10+ selectors documented
- ✅ HTML samples include insights container structure
- ✅ All 13 screenshots captured
- ✅ Analysis completes successfully (status: "Succeeded")
- ✅ Can identify how to extract:
  - Ruleset name
  - Rule ID  
  - Description
  - Category
  - Effort
  - Incidents (file, line, message)

## Technical Details

### Selector Discovery Strategy

The tool tries multiple selector strategies for each element:

1. **data-testid** - Most stable (if available)
2. **aria-label** - Accessibility attributes
3. **text content** - Using `:has-text()` for buttons
4. **name attribute** - For form inputs
5. **id attribute** - Fallback
6. **CSS classes** - Last resort (least stable)

The report documents which selector worked for each element.

### Browser Configuration

- **Browser:** Chromium (latest playwright build)
- **Mode:** Headed (visible window)
- **SlowMo:** 500ms between actions (for visibility)
- **Timeout:** 30s for page loads, 10s for element waits
- **Screenshots:** PNG format, full page

### Error Handling

- **Non-fatal failures:** Tool continues even if steps fail
- **Multiple selector attempts:** Tries all selectors before failing
- **Graceful degradation:** Captures full page HTML if specific elements not found
- **Detailed logging:** Every attempt and failure is logged

## FAQ

**Q: Do I need to run this every time I make changes?**
A: No, run it once to generate the initial documentation. Re-run only if Tackle UI changes significantly.

**Q: Can I run this in CI/CD?**
A: Yes, but use headless mode and ensure Tackle UI is accessible.

**Q: Why does it take so long?**
A: The test runs a full analysis (~10 minutes). This is necessary to document the complete workflow including results extraction.

**Q: Can I use a different test?**
A: Yes, edit `main.go` line 54 to point to a different test file. Ensure the test uses Git repository (not binary).

**Q: What if selectors change?**
A: Re-run the exploration tool. The implementation should use the selector constants, making updates easy.
