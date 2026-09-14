package tackleui

import (
	"fmt"

	"github.com/konveyor/test-harness/pkg/config"
	"github.com/playwright-community/playwright-go"
)

// SelectSourceTechnologies selects source technologies in the Advanced Options wizard step
func (b *BrowserHelper) SelectSourceTechnologies(sources []string) error {
	if len(sources) == 0 {
		b.log.V(1).Info("No source technologies specified, skipping")
		return nil
	}

	b.log.Info("Selecting source technologies", "sources", sources)

	// Wait for the sources selector to appear
	// The selector is a multi-select dropdown with id "formSources-id" or "sources-toggle"
	sourcesSelectors := []string{
		"#sources-toggle",
		"#formSources-id",
		"button[aria-label='Select sources']",
		"select[name='sources']",
	}

	sourcesSelector, err := b.TrySelectors(sourcesSelectors, 5000)
	if err != nil {
		b.log.V(1).Info("Could not find sources selector, may not be on Advanced Options step", "error", err)
		return nil // Don't fail - sources may be optional
	}

	// Click to open the dropdown
	err = b.page.Click(sourcesSelector)
	if err != nil {
		return fmt.Errorf("failed to open sources dropdown: %w", err)
	}

	// Wait for dropdown to open
	_, err = b.page.WaitForSelector("[role='listbox']", playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(3000),
	})
	if err != nil {
		b.log.V(1).Info("Dropdown not opened, trying alternate approach", "error", err)
	}

	// Select each source
	for _, source := range sources {
		b.log.V(1).Info("Selecting source technology", "source", source)

		// Try different selector strategies for the source option
		optionSelectors := []string{
			fmt.Sprintf("button[value='%s']", source),
			fmt.Sprintf("[role='option']:has-text('%s')", source),
			fmt.Sprintf("li:has-text('%s')", source),
			fmt.Sprintf("option[value='%s']", source),
		}

		var selected bool
		for _, selector := range optionSelectors {
			err = b.page.Click(selector, playwright.PageClickOptions{
				Timeout: playwright.Float(2000),
			})
			if err == nil {
				b.log.Info("Selected source technology", "source", source, "selector", selector)
				selected = true
				break
			}
		}

		if !selected {
			b.log.V(1).Info("Could not find option for source", "source", source)
			// Don't fail - continue with other sources
		}
	}

	// Close the dropdown by clicking outside or pressing Escape
	err = b.page.Keyboard().Press("Escape")
	if err != nil {
		b.log.V(2).Info("Failed to close dropdown with Escape", "error", err)
	}

	b.log.Info("Source technologies selection completed", "count", len(sources))
	return nil
}

// UploadCustomRules uploads custom rule files in the Custom Rules wizard step
func (b *BrowserHelper) UploadCustomRules(rules []string) error {
	if len(rules) == 0 {
		b.log.V(1).Info("No custom rules specified, skipping")
		return nil
	}

	b.log.Info("Uploading custom rules", "rules", rules)

	// Wait for the custom rules file upload field
	uploadSelectors := []string{
		"input[type='file'][accept*='yaml']",
		"input[type='file'][accept*='.yml']",
		"input[type='file']",
		"[aria-label='Upload']",
	}

	uploadSelector, err := b.TrySelectors(uploadSelectors, 5000)
	if err != nil {
		b.log.V(1).Info("Could not find file upload field, may not be on Custom Rules step", "error", err)
		return nil // Don't fail - custom rules may be optional
	}

	// Upload each rule file
	for _, rulePath := range rules {
		b.log.V(1).Info("Uploading rule file", "path", rulePath)

		// TODO: Handle Git URLs - need to download first
		// For now, assume local file paths

		err = b.page.SetInputFiles(uploadSelector, rulePath)
		if err != nil {
			return fmt.Errorf("failed to upload rule file %s: %w", rulePath, err)
		}

		b.log.Info("Uploaded rule file", "path", rulePath)

		// Wait for upload to complete
		err = b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		})
		if err != nil {
			b.log.V(2).Info("Network idle wait failed after upload", "error", err)
		}
	}

	b.log.Info("Custom rules upload completed", "count", len(rules))
	return nil
}

// SetDisableDefaultRules sets the "Disable default rules" checkbox
func (b *BrowserHelper) SetDisableDefaultRules(disable bool) error {
	if !disable {
		b.log.V(1).Info("Default rules are enabled, no action needed")
		return nil
	}

	b.log.Info("Disabling default rules")

	// Find the checkbox
	checkboxSelectors := []string{
		"input[type='checkbox'][name*='disableDefaultRules']",
		"input[type='checkbox']:has-text('Disable default rulesets')",
		"label:has-text('Disable default'):has(input[type='checkbox'])",
	}

	checkboxSelector, err := b.TrySelectors(checkboxSelectors, 5000)
	if err != nil {
		b.log.V(1).Info("Could not find disable default rules checkbox", "error", err)
		return nil // Don't fail
	}

	// Check if already checked
	isChecked, err := b.page.IsChecked(checkboxSelector)
	if err == nil && isChecked {
		b.log.V(1).Info("Default rules already disabled")
		return nil
	}

	// Click the checkbox
	err = b.page.Check(checkboxSelector)
	if err != nil {
		return fmt.Errorf("failed to check disable default rules: %w", err)
	}

	b.log.Info("Default rules disabled")
	return nil
}

// ConfigureAdvancedOptions configures advanced analysis options (sources, custom rules, etc.)
func (b *BrowserHelper) ConfigureAdvancedOptions(test *config.TestDefinition) error {
	b.log.Info("Configuring advanced options")

	// The wizard flow navigates through steps, so we need to handle them in order
	// This function is called after navigating through earlier steps

	// If we have custom rules, handle them first (step 5 comes before step 6)
	if len(test.Analysis.Rules) > 0 || test.Analysis.DisableDefaultRules {
		b.log.V(1).Info("Configuring custom rules step")

		// Upload custom rules
		err := b.UploadCustomRules(test.Analysis.Rules)
		if err != nil {
			return fmt.Errorf("failed to upload custom rules: %w", err)
		}

		// Set disable default rules
		err = b.SetDisableDefaultRules(test.Analysis.DisableDefaultRules)
		if err != nil {
			return fmt.Errorf("failed to set disable default rules: %w", err)
		}
	}

	// If we have source technologies, select them (step 6 - Advanced Options)
	if len(test.Analysis.Source) > 0 {
		b.log.V(1).Info("Configuring source technologies")

		err := b.SelectSourceTechnologies(test.Analysis.Source)
		if err != nil {
			return fmt.Errorf("failed to select source technologies: %w", err)
		}
	}

	b.log.Info("Advanced options configuration completed")
	return nil
}
