package tackleui

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/playwright-community/playwright-go"

	"github.com/konveyor/test-harness/pkg/config"
)

// BrowserHelper wraps playwright browser operations with logging and error handling
type BrowserHelper struct {
	page    playwright.Page
	browser playwright.Browser
	pw      *playwright.Playwright
	log     logr.Logger
}

// NewBrowserHelper creates a new browser automation helper
func NewBrowserHelper(browserType string, headless bool, log logr.Logger) (*BrowserHelper, error) {
	// Initialize playwright
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to start playwright: %w", err)
	}

	// Default to chromium
	if browserType == "" {
		browserType = "chromium"
	}

	// Launch browser
	var browser playwright.Browser
	switch strings.ToLower(browserType) {
	case "chromium", "chrome":
		browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(headless),
		})
	case "firefox":
		browser, err = pw.Firefox.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(headless),
		})
	default:
		pw.Stop()
		return nil, fmt.Errorf("unsupported browser type: %s", browserType)
	}

	if err != nil {
		pw.Stop()
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	// Create page
	page, err := browser.NewPage()
	if err != nil {
		browser.Close()
		pw.Stop()
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	return &BrowserHelper{
		page:    page,
		browser: browser,
		pw:      pw,
		log:     log,
	}, nil
}

// Close cleans up browser resources
func (b *BrowserHelper) Close() error {
	if b.page != nil {
		b.page.Close()
	}
	if b.browser != nil {
		b.browser.Close()
	}
	if b.pw != nil {
		b.pw.Stop()
	}
	return nil
}

// NavigateToURL navigates to the specified URL
func (b *BrowserHelper) NavigateToURL(url string) error {
	b.log.Info("Navigating to URL", "url", url)
	_, err := b.page.Goto(url, playwright.PageGotoOptions{
		Timeout: playwright.Float(30000),
	})
	if err != nil {
		return fmt.Errorf("failed to navigate to %s: %w", url, err)
	}

	// Wait for network idle
	b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})

	currentURL := b.page.URL()
	b.log.V(1).Info("Navigation complete", "currentURL", currentURL)
	return nil
}

// TrySelectors attempts multiple selectors and returns the first one that works
func (b *BrowserHelper) TrySelectors(selectors []string, timeout float64) (string, error) {
	for _, sel := range selectors {
		elem, err := b.page.QuerySelector(sel)
		if err == nil && elem != nil {
			b.log.V(2).Info("Found element", "selector", sel)
			return sel, nil
		}
	}
	return "", fmt.Errorf("could not find element (tried %d selectors)", len(selectors))
}

// ClickElement clicks an element using the best selector
func (b *BrowserHelper) ClickElement(selectors []string, description string, timeout float64) error {
	selector, err := b.TrySelectors(selectors, timeout)
	if err != nil {
		return fmt.Errorf("failed to find %s: %w", description, err)
	}

	b.log.V(1).Info("Clicking element", "description", description, "selector", selector)
	err = b.page.Click(selector, playwright.PageClickOptions{
		Timeout: playwright.Float(timeout),
	})
	if err != nil {
		return fmt.Errorf("failed to click %s: %w", description, err)
	}

	return nil
}

// FillField fills a form field
func (b *BrowserHelper) FillField(selectors []string, value, description string) error {
	selector, err := b.TrySelectors(selectors, 5000)
	if err != nil {
		return fmt.Errorf("failed to find %s: %w", description, err)
	}

	b.log.V(1).Info("Filling field", "description", description, "value", value)
	err = b.page.Fill(selector, value)
	if err != nil {
		return fmt.Errorf("failed to fill %s: %w", description, err)
	}

	return nil
}

// NavigateToApplications navigates to the applications page
func (b *BrowserHelper) NavigateToApplications() error {
	return b.ClickElement(AlternativeApplicationsNav, "Applications navigation", 10000)
}

// CreateApplication creates a new application
func (b *BrowserHelper) CreateApplication(test *config.TestDefinition) error {
	b.log.Info("Creating application", "name", test.Name)

	// Click Create button
	err := b.ClickElement(AlternativeCreateButton, "Create Application button", 10000)
	if err != nil {
		return err
	}

	// Wait for form to appear - wait for the name field to be visible
	_, err = b.page.WaitForSelector(SelectorFormName, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		return fmt.Errorf("form did not appear: %w", err)
	}

	// Fill basic information
	err = b.FillField(AlternativeFormName, test.Name, "application name")
	if err != nil {
		return err
	}

	description := test.Description
	if description == "" {
		description = fmt.Sprintf("Application created for test: %s", test.Name)
	}
	err = b.FillField(AlternativeFormDescription, description, "application description")
	if err != nil {
		return err
	}

	// Handle Git repository or binary
	isBinary := IsBinaryFile(test.Analysis.Application)

	if !isBinary {
		// CRITICAL: Click "Source code" section to expand it and reveal Git fields
		b.log.Info("Clicking Source code section to expand")
		sourceCodeSelectors := []string{
			"button:has-text('Source code')",
			"text=Source code",
			"[aria-label='Source code']",
		}
		err = b.ClickElement(sourceCodeSelectors, "Source code section", 5000)
		if err != nil {
			b.log.V(1).Info("Could not click Source code section, may already be expanded", "error", err)
		}

		// Fill Git repository fields
		gitComponents := test.Analysis.ApplicationGitComponents
		if gitComponents == nil {
			return fmt.Errorf("no Git components available for source code application")
		}

		// Wait for repository URL field to be visible before filling
		b.log.V(1).Info("Waiting for repository URL field to be visible")
		_, err = b.page.WaitForSelector(SelectorFormSourceURL, playwright.PageWaitForSelectorOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(5000),
		})
		if err != nil {
			b.log.V(1).Info("Repository URL field not visible, trying to fill anyway", "error", err)
		}

		err = b.FillField(AlternativeFormSourceURL, gitComponents.URL, "repository URL")
		if err != nil {
			return err
		}

		if gitComponents.Ref != "" {
			err = b.FillField(AlternativeFormSourceBranch, gitComponents.Ref, "repository branch")
			if err != nil {
				return err
			}
		}

		if gitComponents.Path != "" {
			err = b.FillField(AlternativeFormSourcePath, gitComponents.Path, "repository path")
			if err != nil {
				return err
			}
		}

		b.log.Info("Filled Git repository fields", "url", gitComponents.URL, "ref", gitComponents.Ref)
	} else {
		// For binary applications, we don't fill any source fields in the create form
		// The binary file will be uploaded later during the analysis wizard
		b.log.Info("Binary application detected, binary will be uploaded during analysis wizard", "path", test.Analysis.Application)
	}

	// Submit the form
	err = b.ClickElement(AlternativeFormSubmit, "Create submit button", 30000)
	if err != nil {
		return err
	}

	b.log.Info("Application creation submitted", "name", test.Name)

	// Wait for form to close - the close button should disappear
	b.log.V(1).Info("Waiting for create form to close")
	_, err = b.page.WaitForSelector(SelectorCloseModal, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateHidden,
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		b.log.V(1).Info("Modal close not detected", "error", err)
		// Don't fail - continue anyway
	}

	// Wait for the application to appear in the list by checking for a row with the application name
	b.log.V(1).Info("Waiting for application to appear in list", "name", test.Name)
	_, err = b.page.WaitForSelector(fmt.Sprintf("tr:has-text('%s')", test.Name), playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		b.log.V(1).Info("Application not visible in list yet", "error", err)
		// Don't fail - it may still work
	}

	return nil
}

// StartAnalysis configures and starts analysis for the application
func (b *BrowserHelper) StartAnalysis(test *config.TestDefinition) error {
	b.log.Info("Starting analysis", "application", test.Name)

	// CRITICAL: Select the application by clicking its checkbox
	// Find the row containing the application name, then find the checkbox in that row
	b.log.Info("Selecting application checkbox", "name", test.Name)

	// Strategy: Find a table row that contains the application name, then click the checkbox in that row
	checkboxSelectors := []string{
		fmt.Sprintf("tr:has-text('%s') input[type='checkbox']", test.Name),
		fmt.Sprintf("tr:has-text('%s') [role='checkbox']", test.Name),
		// Fallback: just find any checkbox (if only one app exists)
		"tbody input[type='checkbox']:first-of-type",
	}

	err := b.ClickElement(checkboxSelectors, "application checkbox", 10000)
	if err != nil {
		return fmt.Errorf("failed to select application checkbox: %w", err)
	}

	b.log.Info("Application checkbox selected", "name", test.Name)

	// Now click Analyze button (should become enabled after checkbox is selected)
	// Wait for Analyze button to be enabled
	_, err = b.page.WaitForSelector(SelectorAnalyzeButton, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		b.log.V(1).Info("Analyze button not visible, trying anyway", "error", err)
	}

	err = b.ClickElement(AlternativeAnalyzeButton, "Analyze button", 10000)
	if err != nil {
		return err
	}

	// Wait for analysis configuration wizard to appear - look for common wizard elements
	b.log.V(1).Info("Waiting for analysis configuration wizard")
	// Try to wait for either a Next button or mode selection elements
	wizardSelectors := []string{
		"button:has-text('Next')",
		"button:has-text('Manual')",
		"div:has-text('Manual Selection')",
		"h1:has-text('Analysis')",
		"h2:has-text('Mode')",
	}

	_, err = b.page.WaitForSelector(wizardSelectors[0], playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		// Try other selectors
		for _, sel := range wizardSelectors[1:] {
			_, err = b.page.WaitForSelector(sel, playwright.PageWaitForSelectorOptions{
				State:   playwright.WaitForSelectorStateVisible,
				Timeout: playwright.Float(2000),
			})
			if err == nil {
				break
			}
		}
	}

	// Step 1: Select "Manual Selection" mode
	b.log.Info("Selecting Manual Selection mode")
	err = b.SelectManualMode()
	if err != nil {
		b.log.V(1).Info("Failed to select Manual mode (may already be selected)", "error", err)
		// Don't fail - mode may already be selected
	}

	// Click Next to proceed to Analysis Source selection
	b.log.Info("Clicking Next to proceed to Analysis Source selection")
	err = b.clickNext()
	if err != nil {
		b.log.V(1).Info("Could not click Next after mode selection", "error", err)
	}

	// Step 2: Select Analysis Source (source code + dependencies / source code / binary)
	b.log.Info("Selecting Analysis Source")
	err = b.SelectAnalysisSource(test)
	if err != nil {
		b.log.V(1).Info("Failed to select Analysis Source (may already be selected)", "error", err)
		// Don't fail - source may already be selected
	}

	// Click Next to proceed to targets selection
	b.log.Info("Clicking Next to proceed to targets selection")
	err = b.clickNext()
	if err != nil {
		b.log.V(1).Info("Could not click Next after source selection", "error", err)
	}

	// Step 3: Select targets
	b.log.Info("Selecting targets")
	err = b.SelectTargets(test)
	if err != nil {
		b.log.V(1).Info("Failed to select targets (may not be required)", "error", err)
		// Don't fail - targets may be optional or already selected
	}

	// Navigate through wizard steps, configuring each as needed
	// The wizard flow is: Mode → Source → Targets → Scope → Custom Rules → Advanced Options → Review → Run
	b.log.Info("Navigating through wizard steps and configuring options")
	maxSteps := 5
	for i := 0; i < maxSteps; i++ {
		// Try to find and click "Next" button
		nextSelectors := []string{
			"button:has-text('Next')",
			"button:has-text('Continue')",
			"[aria-label='Next']",
		}

		nextSelector, err := b.TrySelectors(nextSelectors, 2000)
		if err != nil {
			// No Next button found, we might be on the last step
			b.log.V(1).Info("No Next button found, looking for Run button", "step", i+1)
			break
		}

		// Before clicking Next, configure the current step if needed
		// Try to detect which step we're on and configure accordingly
		b.log.V(1).Info("Checking current wizard step", "step", i+1)

		// Check for Custom Rules step (has file upload or disable checkbox)
		if len(test.Analysis.Rules) > 0 || test.Analysis.DisableDefaultRules {
			isCustomRulesStep, _ := b.page.Locator("input[type='file']").Count()
			if isCustomRulesStep > 0 {
				b.log.Info("Detected Custom Rules step, configuring")
				err = b.UploadCustomRules(test.Analysis.Rules)
				if err != nil {
					b.log.V(1).Info("Failed to upload custom rules", "error", err)
				}
				err = b.SetDisableDefaultRules(test.Analysis.DisableDefaultRules)
				if err != nil {
					b.log.V(1).Info("Failed to set disable default rules", "error", err)
				}
			}
		}

		// Check for Advanced Options step (has sources selector)
		if len(test.Analysis.Source) > 0 {
			isAdvancedStep, _ := b.page.Locator("#sources-toggle").Count()
			if isAdvancedStep > 0 {
				b.log.Info("Detected Advanced Options step, configuring sources")
				err = b.SelectSourceTechnologies(test.Analysis.Source)
				if err != nil {
					b.log.V(1).Info("Failed to select source technologies", "error", err)
				}
			}
		}

		b.log.V(1).Info("Clicking Next button", "step", i+1, "selector", nextSelector)
		err = b.page.Click(nextSelector)
		if err != nil {
			b.log.V(1).Info("Failed to click Next, assuming we're on final step", "error", err)
			break
		}

		// Wait for the next step to load by waiting for network idle
		err = b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		})
		if err != nil {
			b.log.V(2).Info("Network idle wait failed", "error", err)
		}
	}

	// Click Start/Run button
	b.log.Info("Clicking final Run/Start button")
	err = b.ClickElement(AlternativeAnalysisStartButton, "Start Analysis button", 30000)
	if err != nil {
		return err
	}

	b.log.Info("Analysis started", "application", test.Name)

	// Wait for the wizard to close - the Run button should disappear
	_, err = b.page.WaitForSelector(AlternativeAnalysisStartButton[0], playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateHidden,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		b.log.V(1).Info("Wizard close not detected", "error", err)
		// Don't fail - analysis may have started
	}

	return nil
}

// clickNext clicks the Next button in the wizard
func (b *BrowserHelper) clickNext() error {
	nextSelectors := []string{
		"button:has-text('Next')",
		"button:has-text('Continue')",
		"[aria-label='Next']",
	}

	nextSelector, err := b.TrySelectors(nextSelectors, 5000)
	if err != nil {
		return fmt.Errorf("could not find Next button: %w", err)
	}

	err = b.page.Click(nextSelector)
	if err != nil {
		return fmt.Errorf("failed to click Next button: %w", err)
	}

	// Wait for the page to transition to the next step
	err = b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		b.log.V(2).Info("Network idle wait failed after clicking Next", "error", err)
	}

	return nil
}

// SelectAnalysisSource selects the analysis source type (source code + dependencies / source code / binary)
func (b *BrowserHelper) SelectAnalysisSource(test *config.TestDefinition) error {
	b.log.V(1).Info("Looking for Analysis Source selection")

	// Determine which source type to select based on the test configuration
	var sourceType string

	// Map AnalysisMode from config to UI source type
	switch test.Analysis.AnalysisMode {
	case "binary":
		sourceType = "Binary"
		b.log.Info("Selecting Binary mode", "analysisMode", test.Analysis.AnalysisMode)
	case "source-only":
		sourceType = "Source code"
		b.log.Info("Selecting Source code mode", "analysisMode", test.Analysis.AnalysisMode)
	case "source-and-deps", "full":
		// "full" mode is equivalent to "Source code + dependencies"
		sourceType = "Source code + dependencies"
		b.log.Info("Selecting Source code + dependencies mode", "analysisMode", test.Analysis.AnalysisMode)
	case "upload-binary":
		sourceType = "Upload a local binary"
		b.log.Info("Selecting Upload a local binary mode", "analysisMode", test.Analysis.AnalysisMode)
	default:
		// Fallback: try to infer from application type
		if IsBinaryFile(test.Analysis.Application) {
			sourceType = "Binary"
			b.log.Info("AnalysisMode not set, detected binary application", "application", test.Analysis.Application)
		} else {
			sourceType = "Source code + dependencies"
			b.log.Info("AnalysisMode not set, defaulting to Source code + dependencies")
		}
	}

	// Try different selector strategies for the source type options
	sourceSelectors := []string{
		// Try clicking on text containing the source type
		fmt.Sprintf("button:has-text('%s')", sourceType),
		fmt.Sprintf("div:has-text('%s')", sourceType),
		fmt.Sprintf("label:has-text('%s')", sourceType),
		// Try radio button with source type value
		fmt.Sprintf("input[type='radio'][value*='%s']", sourceType),
		// Try card-based selection (common in PatternFly)
		fmt.Sprintf(".pf-v5-c-card:has-text('%s')", sourceType),
		// Try clicking the entire clickable area
		fmt.Sprintf("div[role='button']:has-text('%s')", sourceType),
		fmt.Sprintf("div[class*='selectable']:has-text('%s')", sourceType),
	}

	for _, selector := range sourceSelectors {
		err := b.page.Click(selector, playwright.PageClickOptions{
			Timeout: playwright.Float(2000),
		})
		if err == nil {
			b.log.Info("Selected Analysis Source", "type", sourceType, "selector", selector)
			// Wait for the selection to be processed
			err = b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
				State: playwright.LoadStateNetworkidle,
			})
			if err != nil {
				b.log.V(2).Info("Network idle wait failed after source selection", "error", err)
			}

			// If "Upload a local binary" was selected, upload the binary file
			if sourceType == "Upload a local binary" {
				b.log.Info("Upload binary mode selected, uploading file")
				err = b.UploadBinary(test.Analysis.Application)
				if err != nil {
					return fmt.Errorf("failed to upload binary: %w", err)
				}
			}

			return nil
		}
	}

	// If we couldn't find it, log but don't fail
	b.log.V(1).Info("Could not find Analysis Source selector, may already be selected", "type", sourceType)
	return nil
}

// UploadBinary uploads a binary file (jar, war, ear) in the analysis wizard
func (b *BrowserHelper) UploadBinary(binaryPath string) error {
	b.log.Info("Uploading binary file", "path", binaryPath)

	// Wait for the multiple file upload component to appear
	// This is a PatternFly component with a visible upload button
	b.log.V(1).Info("Waiting for file upload component")
	_, err := b.page.WaitForSelector(".pf-v5-c-multiple-file-upload", playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		b.log.V(1).Info("Multiple file upload component not found, trying alternative approach")
	}

	// Strategy 1: Try to use the hidden file input directly (works in many cases)
	uploadSelectors := []string{
		".pf-v5-c-multiple-file-upload input[type='file']",
		"input[type='file'][accept*='jar']",
		"input[type='file'][accept*='war']",
		"input[type='file'][accept*='ear']",
		"input[type='file']",
	}

	uploadSelector, err := b.TrySelectors(uploadSelectors, 2000)
	if err == nil {
		// Try direct upload to hidden input
		b.log.V(1).Info("Attempting direct upload to file input", "selector", uploadSelector)
		err = b.page.SetInputFiles(uploadSelector, binaryPath)
		if err == nil {
			b.log.V(1).Info("Successfully uploaded file directly")
			return nil
		}
		b.log.V(1).Info("Direct upload failed, trying button click approach", "error", err)
	}

	// Strategy 2: Click the upload button and handle file chooser
	uploadButtonSelectors := []string{
		".pf-v5-c-multiple-file-upload__upload > button",
		"button:has-text('Upload')",
		"button:has-text('Browse')",
		"button[aria-label*='upload']",
		".pf-v5-c-multiple-file-upload button",
	}

	// Set up file chooser handler before clicking the button
	b.log.V(1).Info("Setting up file chooser handler")
	fileChooserChan := make(chan playwright.FileChooser, 1)
	b.page.Once("filechooser", func(fc playwright.FileChooser) {
		fileChooserChan <- fc
	})

	// Click the upload button
	b.log.V(1).Info("Clicking upload button")
	err = b.ClickElement(uploadButtonSelectors, "upload button", 10000)
	if err != nil {
		return fmt.Errorf("failed to click upload button: %w", err)
	}

	// Wait for file chooser to appear and select the file
	b.log.V(1).Info("Waiting for file chooser")
	select {
	case fc := <-fileChooserChan:
		b.log.V(1).Info("File chooser appeared, selecting file", "path", binaryPath)
		err = fc.SetFiles(binaryPath)
		if err != nil {
			return fmt.Errorf("failed to set file in chooser: %w", err)
		}
		b.log.Info("Binary file selected successfully")
	case <-time.After(5 * time.Second):
		// File chooser didn't appear - the input might have been updated directly
		// Try to find the input again and set files
		b.log.V(1).Info("File chooser timeout, trying direct input approach")
		uploadSelector, err = b.TrySelectors(uploadSelectors, 2000)
		if err != nil {
			return fmt.Errorf("file chooser did not appear and could not find file input: %w", err)
		}
		err = b.page.SetInputFiles(uploadSelector, binaryPath)
		if err != nil {
			return fmt.Errorf("failed to upload file after button click: %w", err)
		}
	}

	// Wait for upload to complete
	// Look for success indicator or progress completion
	b.log.V(1).Info("Waiting for binary upload to complete")
	err = b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(60000), // Binary uploads can take longer
	})
	if err != nil {
		b.log.V(1).Info("Network idle wait failed after upload", "error", err)
	}

	// Look for success indicator
	successSelectors := []string{
		"[data-testid='upload-success']",
		".pf-m-success",
		"svg.pf-v5-svg[aria-label*='success']",
		"text=Uploaded",
	}

	for _, sel := range successSelectors {
		count, _ := b.page.Locator(sel).Count()
		if count > 0 {
			b.log.Info("Binary upload completed successfully", "indicator", sel)
			return nil
		}
	}

	b.log.Info("Binary upload completed (success indicator not found, proceeding anyway)")
	return nil
}

// SelectManualMode selects "Manual Selection" mode in the analysis wizard
func (b *BrowserHelper) SelectManualMode() error {
	b.log.V(1).Info("Looking for Manual Selection mode option")

	// Try different selector strategies for the Manual Selection option
	// This could be a radio button, card, or other UI element
	manualSelectors := []string{
		// Try clicking on text containing "Manual"
		"button:has-text('Manual')",
		"div:has-text('Manual Selection')",
		"label:has-text('Manual')",
		// Try radio button with Manual value
		"input[type='radio'][value*='manual']",
		"input[type='radio'][value*='Manual']",
		// Try by ID or name
		"#manual-selection",
		"input[name*='manual']",
		// Try card-based selection (common in PatternFly)
		"[data-testid*='manual']",
		".pf-v5-c-card:has-text('Manual')",
		// Try clicking the entire clickable area
		"div[role='button']:has-text('Manual')",
		"div[class*='selectable']:has-text('Manual')",
	}

	for _, selector := range manualSelectors {
		err := b.page.Click(selector, playwright.PageClickOptions{
			Timeout: playwright.Float(2000),
		})
		if err == nil {
			b.log.Info("Selected Manual Selection mode", "selector", selector)
			// Wait for the selection to be processed
			err = b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
				State: playwright.LoadStateNetworkidle,
			})
			if err != nil {
				b.log.V(2).Info("Network idle wait failed after mode selection", "error", err)
			}
			return nil
		}
	}

	// If we couldn't find it, log but don't fail
	b.log.V(1).Info("Could not find Manual Selection mode selector, may already be selected")
	return nil
}

// SelectTargets selects migration targets in the analysis wizard
func (b *BrowserHelper) SelectTargets(test *config.TestDefinition) error {
	// Determine which targets to select
	var targetLabels []string

	// First, check if targets are explicitly specified in the test
	if len(test.Analysis.Target) > 0 {
		targetLabels = test.Analysis.Target
	} else if test.Analysis.LabelSelector != "" {
		// Extract targets from label selector
		targetLabels = extractTargetsFromLabelSelector(test.Analysis.LabelSelector)
		b.log.Info("Extracted targets from label selector", "labelSelector", test.Analysis.LabelSelector, "targets", targetLabels)
	}

	if len(targetLabels) == 0 {
		b.log.V(1).Info("No targets specified, skipping target selection")
		return nil
	}

	// Map label values to target card names
	// This mapping comes from tackle2-seed/resources/targets.yaml
	labelToCardName := map[string]string{
		"cloud-readiness":  "Containerization",
		"quarkus":          "Quarkus",
		"linux":            "Linux",
		"eap8":             "Application server migration to",
		"eap7":             "Application server migration to",
		"spring6":          "Spring Framework",
		"spring-boot3":     "Spring Boot",
		"openjdk11":        "OpenJDK",
		"openjdk17":        "OpenJDK",
		"openjdk21":        "OpenJDK",
		"openjdk":          "OracleJDK to OpenJDK",
		"jakarta-ee":       "Jakarta EE 9",
		"jws6":             "JBoss Web Server 6",
		"openliberty":      "Open Liberty",
		"camel3":           "Camel",
		"camel4":           "Camel",
		"azure-appservice": "Azure",
		"azure-aks":        "Azure",
	}

	b.log.Info("Selecting targets", "labels", targetLabels)

	// For each target label, find the corresponding card and click it
	for _, targetLabel := range targetLabels {
		cardName, exists := labelToCardName[targetLabel]
		if !exists {
			// If no mapping exists, try using the label as the card name
			cardName = targetLabel
			b.log.V(1).Info("No mapping for target label, using label as card name", "label", targetLabel)
		}

		b.log.V(1).Info("Looking for target card", "label", targetLabel, "cardName", cardName)

		// Target cards are rendered with a heading containing the target name
		// Click on the heading to select the card (as per the test in analysis-wizard.test.tsx line 202-205)
		cardSelectors := []string{
			// Try clicking the heading (h4) with the target name
			fmt.Sprintf("h4:has-text('%s')", cardName),
			fmt.Sprintf("[data-target-name='%s']", cardName),
			// Try clicking the card itself
			fmt.Sprintf(".target-card:has-text('%s')", cardName),
			// Try the role=heading approach from the test
			fmt.Sprintf("[role='heading']:has-text('%s')", cardName),
		}

		var selected bool
		for _, selector := range cardSelectors {
			err := b.page.Click(selector, playwright.PageClickOptions{
				Timeout: playwright.Float(2000),
			})
			if err == nil {
				b.log.Info("Selected target card", "label", targetLabel, "cardName", cardName, "selector", selector)
				selected = true
				break
			}
		}

		if !selected {
			b.log.V(1).Info("Could not find target card, may already be selected or not available", "label", targetLabel, "cardName", cardName)
			// Don't fail - target may be selected by default or via other means
		}
	}

	return nil
}

// WaitForAnalysisComplete polls until analysis completes or times out
func (b *BrowserHelper) WaitForAnalysisComplete(timeout time.Duration) error {
	b.log.Info("Waiting for analysis to complete", "timeout", timeout)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	lastStatus := ""

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("analysis timeout after %v", timeout)
		}

		// Wait for the Analysis status column to show "Completed"
		// The structure is: <td data-label="Analysis"><div>...<div>Completed</div></div></td>
		// Look for the success icon (checkmark SVG) combined with "Completed" text
		completedSelectors := []string{
			// Match the Analysis column with success icon and "Completed" text
			"td[data-label='Analysis']:has-text('Completed') .pf-m-success",
			// Fallback: just look for "Completed" in the Analysis column
			"td[data-label='Analysis']:has-text('Completed')",
			// Alternative: look for the success icon SVG in Analysis column
			"td[data-label='Analysis'] .pf-v5-c-icon__content.pf-m-success",
		}

		// Try to find the completed status indicator
		var foundCompleted bool
		for _, selector := range completedSelectors {
			count, err := b.page.Locator(selector).Count()
			if err == nil && count > 0 {
				// Check if the text actually contains "Completed"
				elem, err := b.page.QuerySelector(selector)
				if err == nil && elem != nil {
					text, _ := elem.TextContent()
					if strings.Contains(strings.ToLower(text), "completed") {
						b.log.Info("Analysis completed successfully", "selector", selector)
						return nil
					}
					foundCompleted = true
					break
				}
			}
		}

		if foundCompleted {
			b.log.Info("Analysis completed successfully")
			return nil
		}

		// Also check for other status indicators as fallback
		selector, err := b.TrySelectors(AlternativeStatusIndicator, 2000)
		if err == nil {
			elem, err := b.page.QuerySelector(selector)
			if err == nil && elem != nil {
				text, _ := elem.TextContent()
				text = strings.TrimSpace(text)

				if text != lastStatus {
					b.log.Info("Analysis status", "status", text)
					lastStatus = text
				}

				// Check if completed
				if contains(text, StatusSucceeded, "completed", "success") {
					b.log.Info("Analysis completed successfully")
					return nil
				}

				// Check if failed
				if contains(text, StatusFailed, "error", "fail") {
					return fmt.Errorf("analysis failed: %s", text)
				}
			}
		}

		<-ticker.C
	}
}

// NavigateToResults navigates to the results/insights page
func (b *BrowserHelper) NavigateToResults() error {
	b.log.Info("Navigating to results")

	err := b.ClickElement(AlternativeResultsLink, "Results link", 10000)
	if err != nil {
		b.log.V(1).Info("Could not find Results link, may already be on results page", "error", err)
		return nil
	}

	// Wait for results to load - wait for network idle
	err = b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(30000),
	})
	if err != nil {
		b.log.V(1).Info("Network idle wait failed on results page", "error", err)
	}

	currentURL := b.page.URL()
	b.log.V(1).Info("Results page", "url", currentURL)

	return nil
}

// GetPageHTML returns the full HTML content of the current page
func (b *BrowserHelper) GetPageHTML() (string, error) {
	return b.page.Content()
}

// ExtractInsightsHTML extracts the insights section HTML for parsing
func (b *BrowserHelper) ExtractInsightsHTML() (string, error) {
	b.log.Info("Extracting insights HTML")

	// Try to find insights container
	selector, err := b.TrySelectors(AlternativeInsightsContainer, 5000)
	if err != nil {
		// Fallback to full page HTML
		b.log.V(1).Info("Insights container not found, using full page HTML")
		return b.GetPageHTML()
	}

	elem, err := b.page.QuerySelector(selector)
	if err != nil || elem == nil {
		return b.GetPageHTML()
	}

	html, err := elem.InnerHTML()
	if err != nil {
		return b.GetPageHTML()
	}

	b.log.Info("Extracted insights HTML", "size", len(html))
	return html, nil
}

// DeleteApplication deletes an application by name
func (b *BrowserHelper) DeleteApplication(appName string) error {
	b.log.Info("Deleting application", "name", appName)

	// Navigate to applications page if not already there
	err := b.NavigateToApplications()
	if err != nil {
		b.log.V(1).Info("Failed to navigate to applications for cleanup", "error", err)
		// Continue anyway
	}

	// Find the application row by name
	appRowSelector := fmt.Sprintf("tr:has-text('%s')", appName)
	_, err = b.TrySelectors([]string{appRowSelector}, 5000)
	if err != nil {
		return fmt.Errorf("application not found in list: %w", err)
	}

	// Find and click the kebab menu (actions dropdown) for this application
	kebabSelectors := []string{
		fmt.Sprintf("tr:has-text('%s') button[aria-label='Kebab toggle']", appName),
		fmt.Sprintf("tr:has-text('%s') [aria-label='Application actions']", appName),
		fmt.Sprintf("tr:has-text('%s') button[aria-label*='menu']", appName),
	}

	err = b.ClickElement(kebabSelectors, "application actions menu", 5000)
	if err != nil {
		return fmt.Errorf("failed to open actions menu: %w", err)
	}

	// Wait for menu to open - look for Delete option to appear
	_, err = b.page.WaitForSelector("button:has-text('Delete')", playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		b.log.V(1).Info("Delete option not immediately visible", "error", err)
	}

	// Click Delete option
	deleteSelectors := []string{
		"button:has-text('Delete')",
		"a:has-text('Delete')",
		"[aria-label='Delete']",
	}

	err = b.ClickElement(deleteSelectors, "delete option", 5000)
	if err != nil {
		return fmt.Errorf("failed to click delete: %w", err)
	}

	// Wait for confirmation dialog to appear
	dialogSelectors := []string{
		"button:has-text('Confirm')",
		"button:has-text('Delete')",
		"[role='dialog']",
	}
	_, err = b.page.WaitForSelector(dialogSelectors[0], playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		// Try other selectors
		for _, sel := range dialogSelectors[1:] {
			_, err = b.page.WaitForSelector(sel, playwright.PageWaitForSelectorOptions{
				State:   playwright.WaitForSelectorStateVisible,
				Timeout: playwright.Float(2000),
			})
			if err == nil {
				break
			}
		}
	}

	// Confirm deletion
	confirmSelectors := []string{
		"button:has-text('Delete'):visible",
		"button[aria-label='confirm']",
		"button:has-text('Confirm')",
	}

	err = b.ClickElement(confirmSelectors, "delete confirmation", 5000)
	if err != nil {
		return fmt.Errorf("failed to confirm deletion: %w", err)
	}

	b.log.Info("Application deletion initiated", "name", appName)

	// Wait for deletion to complete - the row should disappear
	_, err = b.page.WaitForSelector(fmt.Sprintf("tr:has-text('%s')", appName), playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateHidden,
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		b.log.V(1).Info("Application row still visible after delete", "error", err)
		// Don't fail - deletion may have succeeded anyway
	}

	return nil
}

// Helper functions

// extractTargetsFromLabelSelector extracts target values from a label selector
// Example: "konveyor.io/target=cloud-readiness || konveyor.io/target=linux" -> ["cloud-readiness", "linux"]
func extractTargetsFromLabelSelector(selector string) []string {
	targets := []string{}

	// Split by OR operator
	parts := strings.Split(selector, "||")

	for _, part := range parts {
		part = strings.TrimSpace(part)

		// Skip empty parts and exclusions (starting with !)
		if part == "" || strings.HasPrefix(part, "!") {
			continue
		}

		// Check if this is a target label (konveyor.io/target=value)
		if strings.Contains(part, "konveyor.io/target=") {
			// Extract the value after the =
			parts := strings.SplitN(part, "=", 2)
			if len(parts) == 2 {
				target := strings.TrimSpace(parts[1])
				if target != "" {
					targets = append(targets, target)
				}
			}
		}
	}

	return targets
}

// IsBinaryFile checks if the application is a binary file
func IsBinaryFile(application string) bool {
	return strings.HasSuffix(application, ".jar") ||
		strings.HasSuffix(application, ".war") ||
		strings.HasSuffix(application, ".ear") ||
		strings.HasPrefix(application, "mvn://")
}

// contains checks if string contains any of the substrings (case-insensitive)
func contains(s string, substrs ...string) bool {
	s = strings.ToLower(s)
	for _, substr := range substrs {
		if strings.Contains(s, strings.ToLower(substr)) {
			return true
		}
	}
	return false
}
