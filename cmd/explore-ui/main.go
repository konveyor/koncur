package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/konveyor/test-harness/pkg/config"
)

// Report stores all exploration findings
type Report struct {
	Timestamp     time.Time
	TackleUIURL   string
	TestUsed      string
	TestDef       *config.TestDefinition
	Selectors     map[string]SelectorInfo
	Screenshots   []string
	HTMLSamples   map[string]string
	Steps         []StepResult
	Issues        []string
	TotalDuration time.Duration
}

// SelectorInfo documents a discovered selector
type SelectorInfo struct {
	Element      string
	BestSelector string
	Alternatives []string
	Method       string
	TriedCount   int
}

// StepResult records step execution outcome
type StepResult struct {
	Number     int
	Name       string
	Duration   time.Duration
	Success    bool
	Error      string
	Screenshot string
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("🚀 Tackle UI Exploration Tool")
	log.Println(strings.Repeat("=", 60))

	// Load test definition
	testPath := "tests/tackle-testapp-package-filter/test.yaml"
	absPath, _ := filepath.Abs(testPath)
	test, err := config.Load(absPath)
	if err != nil {
		log.Fatalf("❌ Failed to load test: %v", err)
	}
	log.Printf("✅ Loaded test: %s", test.Name)

	// Initialize report
	report := &Report{
		Timestamp:   time.Now(),
		TackleUIURL: "http://localhost:8080",
		TestUsed:    "tackle-testapp-package-filter",
		TestDef:     test,
		Selectors:   make(map[string]SelectorInfo),
		HTMLSamples: make(map[string]string),
	}

	// Create output directories
	os.MkdirAll(".koncur/docs/screenshots", 0755)

	// Run exploration
	startTime := time.Now()
	if err := exploreUI(test, report); err != nil {
		log.Printf("⚠️  Exploration encountered errors: %v", err)
	}
	report.TotalDuration = time.Since(startTime)

	// Generate report
	if err := generateReport(report); err != nil {
		log.Fatalf("❌ Failed to generate report: %v", err)
	}

	// Print summary
	printSummary(report)
}

func exploreUI(test *config.TestDefinition, report *Report) error {
	// Launch playwright
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("failed to start playwright: %w", err)
	}
	defer pw.Stop()

	log.Println("🌐 Launching Chromium browser...")

	// Launch browser (headed mode, visible)
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false),
		SlowMo:   playwright.Float(500),
	})
	if err != nil {
		return fmt.Errorf("failed to launch browser: %w", err)
	}
	defer browser.Close()

	// Create page
	page, err := browser.NewPage()
	if err != nil {
		return fmt.Errorf("failed to create page: %w", err)
	}

	// Execute workflow steps
	steps := []struct {
		num  int
		name string
		fn   func(playwright.Page, *config.TestDefinition, *Report) error
	}{
		{1, "Navigate to Landing Page", step01_NavigateToLanding},
		{2, "Navigate to Applications", step02_NavigateToApplications},
		{3, "Click Create Application", step03_ClickCreateApplication},
		{4, "Fill Application Form", step04_FillApplicationForm},
		{5, "Submit Application", step05_SubmitApplication},
		{6, "Navigate to Analysis Config", step06_NavigateToAnalysisConfig},
		{7, "Configure Analysis", step07_ConfigureAnalysis},
		{8, "Start Analysis", step08_StartAnalysis},
		{9, "Wait for Completion", step09_WaitForCompletion},
		{10, "Navigate to Results", step10_NavigateToResults},
		{11, "Extract Insights Structure", step11_ExtractInsightsStructure},
		{12, "Extract Sample Insights", step12_ExtractSampleInsights},
		{13, "Document Tags", step13_DocumentTags},
	}

	for _, step := range steps {
		executeStep(page, test, report, step.num, step.name, step.fn)
	}

	return nil
}

func executeStep(page playwright.Page, test *config.TestDefinition, report *Report, num int, name string, fn func(playwright.Page, *config.TestDefinition, *Report) error) {
	log.Printf("\n▶️  Step %d: %s", num, name)

	stepStart := time.Now()
	err := fn(page, test, report)
	duration := time.Since(stepStart)

	result := StepResult{
		Number:   num,
		Name:     name,
		Duration: duration,
		Success:  err == nil,
	}

	if err != nil {
		result.Error = err.Error()
		log.Printf("   ❌ Failed: %v", err)
		report.Issues = append(report.Issues, fmt.Sprintf("Step %d (%s): %v", num, name, err))
	} else {
		log.Printf("   ✅ Complete (%.2fs)", duration.Seconds())
	}

	// Take screenshot
	screenshotPath := fmt.Sprintf(".koncur/docs/screenshots/step-%02d.png", num)
	page.Screenshot(playwright.PageScreenshotOptions{
		Path: playwright.String(screenshotPath),
	})
	result.Screenshot = screenshotPath
	report.Screenshots = append(report.Screenshots, screenshotPath)

	report.Steps = append(report.Steps, result)
}

// Helper functions

func trySelectors(page playwright.Page, elementName string, selectors []string) (string, error) {
	for _, sel := range selectors {
		elem, err := page.QuerySelector(sel)
		if err == nil && elem != nil {
			log.Printf("   ✓ Found %s: %s", elementName, sel)
			return sel, nil
		}
	}
	return "", fmt.Errorf("could not find %s (tried %d selectors)", elementName, len(selectors))
}

func recordSelector(report *Report, key, element, selector, method string, alternatives []string) {
	report.Selectors[key] = SelectorInfo{
		Element:      element,
		BestSelector: selector,
		Alternatives: alternatives,
		Method:       method,
		TriedCount:   len(alternatives) + 1,
	}
}

func parseGitURL(gitURL string) (url, branch string) {
	parts := strings.SplitN(gitURL, "#", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return gitURL, "main"
}

func contains(s string, substrs ...string) bool {
	s = strings.ToLower(s)
	for _, substr := range substrs {
		if strings.Contains(s, strings.ToLower(substr)) {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Step 1: Navigate to Landing Page
func step01_NavigateToLanding(page playwright.Page, test *config.TestDefinition, report *Report) error {
	_, err := page.Goto("http://localhost:8080", playwright.PageGotoOptions{
		Timeout: playwright.Float(30000),
	})
	if err != nil {
		return fmt.Errorf("failed to navigate: %w", err)
	}

	// Wait for page to load
	page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})

	currentURL := page.URL()
	log.Printf("   Landing URL: %s", currentURL)

	// Try to find Applications navigation
	selectors := []string{
		"a[href*='application']",
		"text=Applications",
		"[aria-label='Applications']",
		"nav >> text=Applications",
	}

	selector, err := trySelectors(page, "Applications navigation", selectors)
	if err == nil {
		recordSelector(report, "nav-applications", "Applications Navigation Link", selector, "css", selectors)
	}

	return nil
}

// Step 2: Navigate to Applications
func step02_NavigateToApplications(page playwright.Page, test *config.TestDefinition, report *Report) error {
	selectors := []string{
		"text=Applications",
		"a[href*='application']",
		"[aria-label='Applications']",
	}

	selector, err := trySelectors(page, "Applications link", selectors)
	if err != nil {
		return err
	}

	err = page.Click(selector, playwright.PageClickOptions{
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		return fmt.Errorf("failed to click applications: %w", err)
	}

	page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})

	log.Printf("   Applications page URL: %s", page.URL())

	// Find Create Application button
	createSelectors := []string{
		"button:has-text('Create')",
		"text=Create application",
		"[aria-label*='Create']",
		"button[data-testid='create-application']",
	}

	createSel, err := trySelectors(page, "Create Application button", createSelectors)
	if err == nil {
		recordSelector(report, "create-app-button", "Create Application Button", createSel, "text", createSelectors)
	}

	return nil
}

// Step 3: Click Create Application
func step03_ClickCreateApplication(page playwright.Page, test *config.TestDefinition, report *Report) error {
	selector := report.Selectors["create-app-button"].BestSelector
	if selector == "" {
		selector = "button:has-text('Create')"
	}

	err := page.Click(selector, playwright.PageClickOptions{
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		return fmt.Errorf("failed to click create: %w", err)
	}

	// Wait for form to appear
	time.Sleep(2 * time.Second)

	// Document form fields
	formFields := map[string][]string{
		"name": {
			"input[name='name']",
			"#name",
			"input[placeholder*='name' i]",
		},
		"description": {
			"textarea[name='description']",
			"#description",
			"textarea[placeholder*='description' i]",
		},
		"repository-url": {
			"input[name='source.url']",
			"#sourceRepository",
			"[aria-label='source repository url']",
		},
		"repository-branch": {
			"input[name='source.branch']",
			"#branch",
			"[aria-label='Repository branch']",
		},
		"repository-path": {
			"input[name='source.path']",
			"#rootPath",
			"[aria-label='source repository root path']",
		},
	}

	for fieldName, selectors := range formFields {
		selector, err := trySelectors(page, fmt.Sprintf("form field: %s", fieldName), selectors)
		if err == nil {
			recordSelector(report, "form-"+fieldName, "Form Field: "+fieldName, selector, "name", selectors)
		}
	}

	return nil
}

// Step 4: Fill Application Form
func step04_FillApplicationForm(page playwright.Page, test *config.TestDefinition, report *Report) error {
	appName := test.Name
	appDesc := test.Description
	if appDesc == "" {
		appDesc = "Test application created by exploration tool"
	}
	repoURL := test.Analysis.Application

	gitURL, gitBranch := parseGitURL(repoURL)

	// Fill name
	if sel := report.Selectors["form-name"].BestSelector; sel != "" {
		err := page.Fill(sel, appName)
		if err != nil {
			return fmt.Errorf("failed to fill name: %w", err)
		}
		log.Printf("   ✓ Filled name: %s", appName)
	}

	// Fill description
	if sel := report.Selectors["form-description"].BestSelector; sel != "" {
		page.Fill(sel, appDesc)
		log.Printf("   ✓ Filled description")
	}

	// Fill repository URL
	if sel := report.Selectors["form-repository-url"].BestSelector; sel != "" {
		err := page.Fill(sel, gitURL)
		if err != nil {
			return fmt.Errorf("failed to fill repository URL: %w", err)
		}
		log.Printf("   ✓ Filled repository URL: %s", gitURL)
	}

	// Fill branch
	if sel := report.Selectors["form-repository-branch"].BestSelector; sel != "" {
		page.Fill(sel, gitBranch)
		log.Printf("   ✓ Filled branch: %s", gitBranch)
	}

	// Fill repository path (optional, usually empty)
	if sel := report.Selectors["form-repository-path"].BestSelector; sel != "" {
		page.Fill(sel, "")
		log.Printf("   ✓ Filled repository path (empty)")
	}

	return nil
}

// Step 5: Submit Application
func step05_SubmitApplication(page playwright.Page, test *config.TestDefinition, report *Report) error {
	submitSelectors := []string{
		"button:has-text('Create')",
		"button:has-text('Save')",
		"button[type='submit']",
		"[aria-label='Create application']",
	}

	selector, err := trySelectors(page, "Submit button", submitSelectors)
	if err != nil {
		return err
	}

	err = page.Click(selector)
	if err != nil {
		return fmt.Errorf("failed to submit: %w", err)
	}

	recordSelector(report, "form-submit", "Form Submit Button", selector, "text", submitSelectors)

	log.Printf("   ✓ Application created (not deleted)")

	// Wait for form to close
	time.Sleep(3 * time.Second)

	return nil
}

// Step 6: Navigate to Analysis Config
func step06_NavigateToAnalysisConfig(page playwright.Page, test *config.TestDefinition, report *Report) error {
	// Find the newly created application and click "Analyze"
	analyzeSelectors := []string{
		"button:has-text('Analyze')",
		"[aria-label='Analyze']",
		"a:has-text('Analyze')",
	}

	selector, err := trySelectors(page, "Analyze button", analyzeSelectors)
	if err != nil {
		return err
	}

	err = page.Click(selector, playwright.PageClickOptions{
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		return fmt.Errorf("failed to click analyze: %w", err)
	}

	recordSelector(report, "analyze-button", "Analyze Button", selector, "text", analyzeSelectors)

	time.Sleep(2 * time.Second)
	return nil
}

// Step 7: Configure Analysis
func step07_ConfigureAnalysis(page playwright.Page, test *config.TestDefinition, report *Report) error {
	// Fill analysis configuration from test definition

	// Label selector
	if test.Analysis.LabelSelector != "" {
		labelSelectors := []string{
			"input[name='labelSelector']",
			"[id*='label']",
			"input[placeholder*='label' i]",
		}

		for _, sel := range labelSelectors {
			err := page.Fill(sel, test.Analysis.LabelSelector, playwright.PageFillOptions{
				Timeout: playwright.Float(2000),
			})
			if err == nil {
				log.Printf("   ✓ Filled label selector: %s", test.Analysis.LabelSelector)
				recordSelector(report, "analysis-label-selector", "Label Selector Input", sel, "name", labelSelectors)
				break
			}
		}
	}

	// TODO: Document other analysis config fields (targets, sources, rules, etc.)
	// For now, we'll proceed with defaults

	return nil
}

// Step 8: Start Analysis
func step08_StartAnalysis(page playwright.Page, test *config.TestDefinition, report *Report) error {
	startSelectors := []string{
		"button:has-text('Run')",
		"button:has-text('Start')",
		"button:has-text('Analyze')",
		"[aria-label*='Run']",
	}

	selector, err := trySelectors(page, "Start Analysis button", startSelectors)
	if err != nil {
		return err
	}

	err = page.Click(selector)
	if err != nil {
		return fmt.Errorf("failed to start analysis: %w", err)
	}

	recordSelector(report, "start-analysis-button", "Start Analysis Button", selector, "text", startSelectors)

	time.Sleep(2 * time.Second)
	return nil
}

// Step 9: Wait for Completion (CRITICAL - handles 10min timeout)
func step09_WaitForCompletion(page playwright.Page, test *config.TestDefinition, report *Report) error {
	timeout := test.GetTimeout()
	log.Printf("   Waiting for analysis (timeout: %v)...", timeout)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	statusSelectors := []string{
		"[data-testid='status']",
		".status",
		"[aria-label*='status' i]",
		"td:has-text('Status')",
	}

	lastStatus := ""

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("analysis timeout after %v", timeout)
		}

		// Try to find status
		for _, sel := range statusSelectors {
			elem, err := page.QuerySelector(sel)
			if err == nil && elem != nil {
				text, _ := elem.TextContent()
				text = strings.TrimSpace(text)

				if text != lastStatus {
					log.Printf("   Status: %s", text)
					lastStatus = text
				}

				if contains(text, "succeeded", "completed", "success") {
					log.Printf("   ✅ Analysis completed!")
					recordSelector(report, "status-indicator", "Analysis Status Indicator", sel, "css", statusSelectors)
					return nil
				}

				if contains(text, "failed", "error") {
					return fmt.Errorf("analysis failed: %s", text)
				}

				break
			}
		}

		<-ticker.C
	}
}

// Step 10: Navigate to Results
func step10_NavigateToResults(page playwright.Page, test *config.TestDefinition, report *Report) error {
	// Try to navigate to results/insights view
	resultsSelectors := []string{
		"text=Results",
		"text=Insights",
		"a:has-text('Reports')",
		"[aria-label*='result' i]",
	}

	for _, sel := range resultsSelectors {
		err := page.Click(sel, playwright.PageClickOptions{
			Timeout: playwright.Float(2000),
		})
		if err == nil {
			log.Printf("   ✓ Navigated to results: %s", sel)
			recordSelector(report, "results-link", "Results/Insights Link", sel, "text", resultsSelectors)
			break
		}
	}

	time.Sleep(2 * time.Second)

	log.Printf("   Results URL: %s", page.URL())
	return nil
}

// Step 11: Extract Insights Structure (CRITICAL)
func step11_ExtractInsightsStructure(page playwright.Page, test *config.TestDefinition, report *Report) error {
	log.Printf("   Extracting insights DOM structure...")

	// Find insights container
	containerSelectors := []string{
		"[data-testid='insights']",
		"[data-testid='insights-table']",
		"table[aria-label*='insight' i]",
		"#insights",
		".insights-table",
	}

	var containerHTML string
	var containerSel string

	for _, sel := range containerSelectors {
		elem, err := page.QuerySelector(sel)
		if err == nil && elem != nil {
			html, _ := elem.InnerHTML()
			containerHTML = html
			containerSel = sel
			log.Printf("   ✓ Found insights container: %s", sel)
			break
		}
	}

	if containerHTML == "" {
		// Fallback: capture entire page
		containerHTML, _ = page.Content()
		log.Printf("   ⚠️  Using full page HTML (container not found)")
	}

	report.HTMLSamples["insights-container"] = containerHTML

	if containerSel != "" {
		recordSelector(report, "insights-container", "Insights Container", containerSel, "data-testid", containerSelectors)

		// Try to find rows
		rowSelectors := []string{
			containerSel + " tr[data-testid='insight-row']",
			containerSel + " tbody tr",
			containerSel + " [role='row']",
		}

		for _, rowSel := range rowSelectors {
			rows, err := page.QuerySelectorAll(rowSel)
			if err == nil && len(rows) > 0 {
				log.Printf("   ✓ Found %d insight rows: %s", len(rows), rowSel)
				recordSelector(report, "insight-rows", "Insight Rows", rowSel, "css", rowSelectors)
				break
			}
		}
	}

	return nil
}

// Step 12: Extract Sample Insights
func step12_ExtractSampleInsights(page playwright.Page, test *config.TestDefinition, report *Report) error {
	// Get the row selector we found
	rowSel := report.Selectors["insight-rows"].BestSelector
	if rowSel == "" {
		log.Printf("   ⚠️  No row selector found, skipping sample extraction")
		return nil
	}

	rows, err := page.QuerySelectorAll(rowSel)
	if err != nil || len(rows) == 0 {
		return nil
	}

	// Capture first 3 rows as samples
	sampleCount := min(3, len(rows))
	log.Printf("   Capturing %d sample rows...", sampleCount)

	for i := 0; i < sampleCount; i++ {
		rowHTML, _ := rows[i].InnerHTML()
		report.HTMLSamples[fmt.Sprintf("insight-row-sample-%d", i+1)] = rowHTML
	}

	return nil
}

// Step 13: Document Tags
func step13_DocumentTags(page playwright.Page, test *config.TestDefinition, report *Report) error {
	// Try to find tags section
	tagSelectors := []string{
		"[data-testid='tags']",
		".tags",
		"[aria-label*='tag' i]",
	}

	for _, sel := range tagSelectors {
		elem, err := page.QuerySelector(sel)
		if err == nil && elem != nil {
			html, _ := elem.InnerHTML()
			report.HTMLSamples["tags-section"] = html
			log.Printf("   ✓ Found tags section: %s", sel)
			recordSelector(report, "tags-section", "Tags Section", sel, "css", tagSelectors)
			return nil
		}
	}

	log.Printf("   ⚠️  Tags section not found")
	return nil
}

// Report generation

func generateReport(report *Report) error {
	os.MkdirAll(".koncur/docs", 0755)

	f, err := os.Create(".koncur/docs/tackle-ui-exploration.md")
	if err != nil {
		return err
	}
	defer f.Close()

	// Header
	fmt.Fprintf(f, "# Tackle UI Exploration Report\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", report.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(f, "**Tackle UI URL:** %s\n\n", report.TackleUIURL)
	fmt.Fprintf(f, "**Test Used:** %s\n\n", report.TestUsed)
	fmt.Fprintf(f, "**Application Created:** %s (**NOT DELETED**)\n\n", report.TestDef.Name)

	// Execution Summary
	fmt.Fprintf(f, "## Workflow Execution Summary\n\n")
	fmt.Fprintf(f, "- **Total Duration:** %v\n", report.TotalDuration.Round(time.Second))
	fmt.Fprintf(f, "- **Steps:** %d\n", len(report.Steps))

	successCount := 0
	for _, step := range report.Steps {
		if step.Success {
			successCount++
		}
	}
	fmt.Fprintf(f, "- **Successes:** %d\n", successCount)
	fmt.Fprintf(f, "- **Failures:** %d\n\n", len(report.Steps)-successCount)

	// Detailed Steps
	fmt.Fprintf(f, "## Detailed Steps\n\n")
	for _, step := range report.Steps {
		status := "✅"
		if !step.Success {
			status = "❌"
		}
		fmt.Fprintf(f, "%d. %s **%s** (%.2fs)\n", step.Number, status, step.Name, step.Duration.Seconds())
		if step.Error != "" {
			fmt.Fprintf(f, "   - Error: `%s`\n", step.Error)
		}
		if step.Screenshot != "" {
			fmt.Fprintf(f, "   - Screenshot: `%s`\n", step.Screenshot)
		}
	}
	fmt.Fprintf(f, "\n")

	// Discovered Selectors
	fmt.Fprintf(f, "## Discovered Selectors\n\n")
	fmt.Fprintf(f, "| Element | Selector | Method |\n")
	fmt.Fprintf(f, "|---------|----------|--------|\n")
	for _, info := range report.Selectors {
		fmt.Fprintf(f, "| %s | `%s` | %s |\n", info.Element, info.BestSelector, info.Method)
	}
	fmt.Fprintf(f, "\n")

	// HTML Samples
	if len(report.HTMLSamples) > 0 {
		fmt.Fprintf(f, "## HTML Structure Samples\n\n")
		for name, html := range report.HTMLSamples {
			fmt.Fprintf(f, "### %s\n\n", name)
			// Truncate very long HTML
			if len(html) > 5000 {
				html = html[:5000] + "\n... (truncated)"
			}
			fmt.Fprintf(f, "```html\n%s\n```\n\n", html)
		}
	}

	// Issues
	if len(report.Issues) > 0 {
		fmt.Fprintf(f, "## Issues Encountered\n\n")
		for _, issue := range report.Issues {
			fmt.Fprintf(f, "- %s\n", issue)
		}
		fmt.Fprintf(f, "\n")
	}

	// Recommendations
	fmt.Fprintf(f, "## Recommendations for Implementation\n\n")
	fmt.Fprintf(f, "1. Use `data-testid` attributes where available (most stable)\n")
	fmt.Fprintf(f, "2. Fallback to `aria-label` for accessibility selectors\n")
	fmt.Fprintf(f, "3. Use text matching (`:has-text()`) for buttons\n")
	fmt.Fprintf(f, "4. Avoid CSS classes (likely to change)\n")
	fmt.Fprintf(f, "5. Poll status every 10 seconds for analysis completion\n")
	fmt.Fprintf(f, "6. Extract insights by iterating over table rows\n")
	fmt.Fprintf(f, "7. Handle missing/optional fields gracefully\n\n")

	return nil
}

func printSummary(report *Report) {
	log.Println("\n" + strings.Repeat("=", 60))
	log.Println("📊 EXPLORATION SUMMARY")
	log.Println(strings.Repeat("=", 60))
	log.Printf("Duration: %v", report.TotalDuration.Round(time.Second))
	log.Printf("Steps: %d", len(report.Steps))
	log.Printf("Selectors Found: %d", len(report.Selectors))
	log.Printf("Screenshots: %d", len(report.Screenshots))
	log.Printf("HTML Samples: %d", len(report.HTMLSamples))
	if len(report.Issues) > 0 {
		log.Printf("Issues: %d", len(report.Issues))
	}
	log.Println("")
	log.Println("📝 Report: .koncur/docs/tackle-ui-exploration.md")
	log.Println("📸 Screenshots: .koncur/docs/screenshots/")
	log.Println("")
	log.Println("✅ Exploration complete!")
	log.Println(strings.Repeat("=", 60))
}
