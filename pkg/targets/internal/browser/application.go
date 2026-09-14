package browser

import (
	"fmt"

	"github.com/playwright-community/playwright-go"

	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/targets/internal/tackleui"
)

// CreateApplication creates a new application
func (h *Helper) CreateApplication(test *config.TestDefinition) error {
	h.log.Info("Creating application", "name", test.Name)

	// Click Create button
	err := h.ClickElement(tackleui.AlternativeCreateButton, "Create Application button", 10000)
	if err != nil {
		return err
	}

	// Wait for form to appear - wait for the name field to be visible
	_, err = h.page.WaitForSelector(tackleui.SelectorFormName, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		return fmt.Errorf("form did not appear: %w", err)
	}

	// Fill basic information
	err = h.FillField(tackleui.AlternativeFormName, test.Name, "application name")
	if err != nil {
		return err
	}

	description := test.Description
	if description == "" {
		description = fmt.Sprintf("Application created for test: %s", test.Name)
	}
	err = h.FillField(tackleui.AlternativeFormDescription, description, "application description")
	if err != nil {
		return err
	}

	// Handle Git repository or binary
	isBinary := IsBinaryFile(test.Analysis.Application)

	if !isBinary {
		// CRITICAL: Click "Source code" section to expand it and reveal Git fields
		h.log.Info("Clicking Source code section to expand")
		sourceCodeSelectors := []string{
			"button:has-text('Source code')",
			"text=Source code",
			"[aria-label='Source code']",
		}
		err = h.ClickElement(sourceCodeSelectors, "Source code section", 5000)
		if err != nil {
			h.log.V(1).Info("Could not click Source code section, may already be expanded", "error", err)
		}

		// Fill Git repository fields
		gitComponents := test.Analysis.ApplicationGitComponents
		if gitComponents == nil {
			return fmt.Errorf("no Git components available for source code application")
		}

		// Wait for repository URL field to be visible before filling
		h.log.V(1).Info("Waiting for repository URL field to be visible")
		_, err = h.page.WaitForSelector(tackleui.SelectorFormSourceURL, playwright.PageWaitForSelectorOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(5000),
		})
		if err != nil {
			h.log.V(1).Info("Repository URL field not visible, trying to fill anyway", "error", err)
		}

		err = h.FillField(tackleui.AlternativeFormSourceURL, gitComponents.URL, "repository URL")
		if err != nil {
			return err
		}

		if gitComponents.Ref != "" {
			err = h.FillField(tackleui.AlternativeFormSourceBranch, gitComponents.Ref, "repository branch")
			if err != nil {
				return err
			}
		}

		if gitComponents.Path != "" {
			err = h.FillField(tackleui.AlternativeFormSourcePath, gitComponents.Path, "repository path")
			if err != nil {
				return err
			}
		}

		h.log.Info("Filled Git repository fields", "url", gitComponents.URL, "ref", gitComponents.Ref)
	} else {
		// For binary applications, we don't fill any source fields in the create form
		// The binary file will be uploaded later during the analysis wizard
		h.log.Info("Binary application detected, binary will be uploaded during analysis wizard", "path", test.Analysis.Application)
	}

	// Submit the form
	err = h.ClickElement(tackleui.AlternativeFormSubmit, "Create submit button", 30000)
	if err != nil {
		return err
	}

	h.log.Info("Application creation submitted", "name", test.Name)

	// Wait for form to close - the close button should disappear
	h.log.V(1).Info("Waiting for create form to close")
	_, err = h.page.WaitForSelector(tackleui.SelectorCloseModal, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateHidden,
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		h.log.V(1).Info("Modal close not detected", "error", err)
		// Don't fail - continue anyway
	}

	// Wait for the application to appear in the list by checking for a row with the application name
	h.log.V(1).Info("Waiting for application to appear in list", "name", test.Name)
	_, err = h.page.WaitForSelector(fmt.Sprintf("tr:has-text('%s')", test.Name), playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		h.log.V(1).Info("Application not visible in list yet", "error", err)
		// Don't fail - it may still work
	}

	return nil
}

// DeleteApplication deletes an application by name
func (h *Helper) DeleteApplication(appName string) error {
	h.log.Info("Deleting application", "name", appName)

	// Navigate to applications page if not already there
	err := h.NavigateToApplications()
	if err != nil {
		h.log.V(1).Info("Failed to navigate to applications for cleanup", "error", err)
		// Continue anyway
	}

	// Find the application row by name
	appRowSelector := fmt.Sprintf("tr:has-text('%s')", appName)
	_, err = h.TrySelectors([]string{appRowSelector}, 5000)
	if err != nil {
		return fmt.Errorf("application not found in list: %w", err)
	}

	// Find and click the kebab menu (actions dropdown) for this application
	kebabSelectors := []string{
		fmt.Sprintf("tr:has-text('%s') button[aria-label='Kebab toggle']", appName),
		fmt.Sprintf("tr:has-text('%s') [aria-label='Application actions']", appName),
		fmt.Sprintf("tr:has-text('%s') button[aria-label*='menu']", appName),
	}

	err = h.ClickElement(kebabSelectors, "application actions menu", 5000)
	if err != nil {
		return fmt.Errorf("failed to open actions menu: %w", err)
	}

	// Wait for menu to open - look for Delete option to appear
	_, err = h.page.WaitForSelector("button:has-text('Delete')", playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		h.log.V(1).Info("Delete option not immediately visible", "error", err)
	}

	// Click Delete option
	deleteSelectors := []string{
		"button:has-text('Delete')",
		"a:has-text('Delete')",
		"[aria-label='Delete']",
	}

	err = h.ClickElement(deleteSelectors, "delete option", 5000)
	if err != nil {
		return fmt.Errorf("failed to click delete: %w", err)
	}

	// Wait for confirmation dialog to appear
	dialogSelectors := []string{
		"button:has-text('Confirm')",
		"button:has-text('Delete')",
		"[role='dialog']",
	}
	_, err = h.page.WaitForSelector(dialogSelectors[0], playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		// Try other selectors
		for _, sel := range dialogSelectors[1:] {
			_, err = h.page.WaitForSelector(sel, playwright.PageWaitForSelectorOptions{
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

	err = h.ClickElement(confirmSelectors, "delete confirmation", 5000)
	if err != nil {
		return fmt.Errorf("failed to confirm deletion: %w", err)
	}

	h.log.Info("Application deletion initiated", "name", appName)

	// Wait for deletion to complete - the row should disappear
	_, err = h.page.WaitForSelector(fmt.Sprintf("tr:has-text('%s')", appName), playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateHidden,
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		h.log.V(1).Info("Application row still visible after delete", "error", err)
		// Don't fail - deletion may have succeeded anyway
	}

	return nil
}
