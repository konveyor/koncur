package browser

import (
	"fmt"

	"github.com/playwright-community/playwright-go"

	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/targets/internal/tackleui"
)

// StartAnalysis configures and starts analysis for the application
func (h *Helper) StartAnalysis(test *config.TestDefinition) error {
	h.log.Info("Starting analysis", "application", test.Name)

	// CRITICAL: Select the application by clicking its checkbox
	// Find the row containing the application name, then find the checkbox in that row
	h.log.Info("Selecting application checkbox", "name", test.Name)

	// Strategy: Find a table row that contains the application name, then click the checkbox in that row
	checkboxSelectors := []string{
		fmt.Sprintf("tr:has-text('%s') input[type='checkbox']", test.Name),
		fmt.Sprintf("tr:has-text('%s') [role='checkbox']", test.Name),
		// Fallback: just find any checkbox (if only one app exists)
		"tbody input[type='checkbox']:first-of-type",
	}

	err := h.ClickElement(checkboxSelectors, "application checkbox", 10000)
	if err != nil {
		return fmt.Errorf("failed to select application checkbox: %w", err)
	}

	h.log.Info("Application checkbox selected", "name", test.Name)

	// Now click Analyze button (should become enabled after checkbox is selected)
	// Wait for Analyze button to be enabled
	_, err = h.page.WaitForSelector(tackleui.SelectorAnalyzeButton, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		h.log.V(1).Info("Analyze button not visible, trying anyway", "error", err)
	}

	err = h.ClickElement(tackleui.AlternativeAnalyzeButton, "Analyze button", 10000)
	if err != nil {
		return err
	}

	// Wait for analysis configuration wizard to appear - look for common wizard elements
	h.log.V(1).Info("Waiting for analysis configuration wizard")
	// Try to wait for either a Next button or mode selection elements
	wizardSelectors := []string{
		"button:has-text('Next')",
		"button:has-text('Manual')",
		"div:has-text('Manual Selection')",
		"h1:has-text('Analysis')",
		"h2:has-text('Mode')",
	}

	_, err = h.page.WaitForSelector(wizardSelectors[0], playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		// Try other selectors
		for _, sel := range wizardSelectors[1:] {
			_, err = h.page.WaitForSelector(sel, playwright.PageWaitForSelectorOptions{
				State:   playwright.WaitForSelectorStateVisible,
				Timeout: playwright.Float(2000),
			})
			if err == nil {
				break
			}
		}
	}

	// Step 1: Select "Manual Selection" mode
	h.log.Info("Selecting Manual Selection mode")
	err = h.SelectManualMode()
	if err != nil {
		h.log.V(1).Info("Failed to select Manual mode (may already be selected)", "error", err)
		// Don't fail - mode may already be selected
	}

	// Click Next to proceed to Analysis Source selection
	h.log.Info("Clicking Next to proceed to Analysis Source selection")
	err = h.clickNext()
	if err != nil {
		h.log.V(1).Info("Could not click Next after mode selection", "error", err)
	}

	// Step 2: Select Analysis Source (source code + dependencies / source code / binary)
	h.log.Info("Selecting Analysis Source")
	err = h.SelectAnalysisSource(test)
	if err != nil {
		h.log.V(1).Info("Failed to select Analysis Source (may already be selected)", "error", err)
		// Don't fail - source may already be selected
	}

	// Click Next to proceed to targets selection
	h.log.Info("Clicking Next to proceed to targets selection")
	err = h.clickNext()
	if err != nil {
		h.log.V(1).Info("Could not click Next after source selection", "error", err)
	}

	// Step 3: Select targets
	h.log.Info("Selecting targets")
	err = h.SelectTargets(test)
	if err != nil {
		h.log.V(1).Info("Failed to select targets (may not be required)", "error", err)
		// Don't fail - targets may be optional or already selected
	}

	// Wait for target selections to register before proceeding
	h.page.WaitForTimeout(1500)

	// Navigate through wizard steps, configuring each as needed
	// The wizard flow is: Mode → Source → Targets → Scope → Custom Rules → Advanced Options → Review → Run
	h.log.Info("Navigating through wizard steps and configuring options")
	maxSteps := 5
	for i := 0; i < maxSteps; i++ {
		// Try to find and click "Next" button
		nextSelectors := []string{
			"button:has-text('Next')",
			"button:has-text('Continue')",
			"[aria-label='Next']",
		}

		nextSelector, err := h.TrySelectors(nextSelectors, 2000)
		if err != nil {
			// No Next button found, we might be on the last step
			h.log.V(1).Info("No Next button found, looking for Run button", "step", i+1)
			break
		}

		// Before clicking Next, configure the current step if needed
		// Try to detect which step we're on and configure accordingly
		h.log.V(1).Info("Checking current wizard step", "step", i+1)

		// Check for Custom Rules step
		// Always detect this step, even if we don't need to configure anything
		// Look for multiple indicators: file upload, tabs (Upload/Repository), or disable checkbox
		hasFileUpload, _ := h.page.Locator("input[type='file']").Count()
		hasRepositoryTab, _ := h.page.Locator("button:has-text('Repository')").Count()
		hasUploadTab, _ := h.page.Locator("button:has-text('Upload')").Count()
		hasDisableCheckbox, _ := h.page.Locator("input[type='checkbox']:has-text('Disable')").Count()

		isCustomRulesStep := hasFileUpload > 0 || hasRepositoryTab > 0 || hasUploadTab > 0 || hasDisableCheckbox > 0

		if isCustomRulesStep {
			h.log.Info("Detected Custom Rules step",
				"hasFileUpload", hasFileUpload > 0,
				"hasRepositoryTab", hasRepositoryTab > 0,
				"hasUploadTab", hasUploadTab > 0,
				"hasDisableCheckbox", hasDisableCheckbox > 0)

			if len(test.Analysis.Rules) > 0 {
				h.log.Info("Configuring custom rules")
				err = h.UploadCustomRules(test.Analysis.Rules)
				if err != nil {
					h.log.V(1).Info("Failed to upload custom rules", "error", err)
				}
			} else {
				h.log.Info("No custom rules to configure, using defaults")
			}

			if test.Analysis.DisableDefaultRules {
				h.log.Info("Disabling default rules")
				err = h.SetDisableDefaultRules(test.Analysis.DisableDefaultRules)
				if err != nil {
					h.log.V(1).Info("Failed to set disable default rules", "error", err)
				}
			} else {
				h.log.Info("Default rules will be enabled (no action needed)")
			}
		}

		// Check for Advanced Options step (has sources selector)
		if len(test.Analysis.Source) > 0 {
			isAdvancedStep, _ := h.page.Locator("#sources-toggle").Count()
			if isAdvancedStep > 0 {
				h.log.Info("Detected Advanced Options step, configuring sources")
				err = h.SelectSourceTechnologies(test.Analysis.Source)
				if err != nil {
					h.log.V(1).Info("Failed to select source technologies", "error", err)
				}
			}
		}

		h.log.V(1).Info("Clicking Next button", "step", i+1, "selector", nextSelector)
		err = h.page.Click(nextSelector)
		if err != nil {
			h.log.V(1).Info("Failed to click Next, assuming we're on final step", "error", err)
			break
		}

		// Wait for the next step to load by waiting for network idle
		err = h.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		})
		if err != nil {
			h.log.V(2).Info("Network idle wait failed", "error", err)
		}
	}

	// Click Start/Run button
	h.log.Info("Clicking final Run/Start button")
	err = h.ClickElement(tackleui.AlternativeAnalysisStartButton, "Start Analysis button", 30000)
	if err != nil {
		return err
	}

	h.log.Info("Analysis started", "application", test.Name)

	// Wait for the wizard to close - the Run button should disappear
	_, err = h.page.WaitForSelector(tackleui.AlternativeAnalysisStartButton[0], playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateHidden,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		h.log.V(1).Info("Wizard close not detected", "error", err)
		// Don't fail - analysis may have started
	}

	return nil
}

// clickNext clicks the Next button in the wizard
func (h *Helper) clickNext() error {
	nextSelectors := []string{
		"button:has-text('Next')",
		"button:has-text('Continue')",
		"[aria-label='Next']",
	}

	nextSelector, err := h.TrySelectors(nextSelectors, 5000)
	if err != nil {
		return fmt.Errorf("could not find Next button: %w", err)
	}

	err = h.page.Click(nextSelector)
	if err != nil {
		return fmt.Errorf("failed to click Next button: %w", err)
	}

	// Wait for the page to transition to the next step
	err = h.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		h.log.V(2).Info("Network idle wait failed after clicking Next", "error", err)
	}

	return nil
}

// SelectManualMode selects "Manual Selection" mode in the analysis wizard
func (h *Helper) SelectManualMode() error {
	h.log.V(1).Info("Looking for Manual Selection mode option")

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
		err := h.page.Click(selector, playwright.PageClickOptions{
			Timeout: playwright.Float(2000),
		})
		if err == nil {
			h.log.Info("Selected Manual Selection mode", "selector", selector)
			// Wait for the selection to be processed
			err = h.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
				State: playwright.LoadStateNetworkidle,
			})
			if err != nil {
				h.log.V(2).Info("Network idle wait failed after mode selection", "error", err)
			}
			return nil
		}
	}

	// If we couldn't find it, log but don't fail
	h.log.V(1).Info("Could not find Manual Selection mode selector, may already be selected")
	return nil
}
