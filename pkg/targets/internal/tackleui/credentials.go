package tackleui

import (
	"fmt"
	"os"

	"github.com/playwright-community/playwright-go"
)

// CreateMavenIdentity creates a Maven settings credential/identity in the Administrator view
func (b *BrowserHelper) CreateMavenIdentity(baseURL, name, settingsPath string) error {
	b.log.Info("Creating Maven identity", "name", name, "settingsPath", settingsPath)

	// Read the Maven settings file
	settingsContent, err := os.ReadFile(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read Maven settings file %s: %w", settingsPath, err)
	}

	// Navigate directly to Credentials/Identities page
	// In Tackle UI, this is /identities which is in the Administration perspective
	b.log.V(1).Info("Navigating to Credentials/Identities page")
	identitiesURL := baseURL + "/identities"
	_, err = b.page.Goto(identitiesURL)
	if err != nil {
		return fmt.Errorf("failed to navigate to identities page: %w", err)
	}

	// Wait for Credentials page to load
	err = b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		b.log.V(2).Info("Network idle wait failed", "error", err)
	}

	// Check if identity already exists
	existingSelector := fmt.Sprintf("td:has-text('%s')", name)
	count, err := b.page.Locator(existingSelector).Count()
	if err == nil && count > 0 {
		b.log.Info("Maven identity already exists, skipping creation", "name", name)
		return nil
	}

	// Click Create button
	createSelectors := []string{
		"button:has-text('Create')",
		"button:has-text('Create new')",
		"[aria-label='Create']",
	}
	err = b.ClickElement(createSelectors, "Create button", 10000)
	if err != nil {
		return fmt.Errorf("failed to click Create button: %w", err)
	}

	// Wait for Create Identity form to appear
	_, err = b.page.WaitForSelector("input[name='name']", playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		return fmt.Errorf("Create Identity form did not appear: %w", err)
	}

	// Fill identity name
	err = b.FillField([]string{"input[name='name']", "#name"}, name, "identity name")
	if err != nil {
		return fmt.Errorf("failed to fill identity name: %w", err)
	}

	// Select "Maven Settings File" type
	b.log.V(1).Info("Selecting Maven Settings File type")
	typeSelectors := []string{
		"select[name='type']",
		"#type",
		"select:has(option:has-text('Maven Settings File'))",
	}

	typeSelector, err := b.TrySelectors(typeSelectors, 5000)
	if err != nil {
		return fmt.Errorf("failed to find type selector: %w", err)
	}

	_, err = b.page.SelectOption(typeSelector, playwright.SelectOptionValues{
		Values: &[]string{"maven"},
	})
	if err != nil {
		// Try clicking a dropdown if select doesn't work
		b.log.V(1).Info("Select option failed, trying dropdown approach", "error", err)

		// Look for dropdown toggle
		dropdownSelectors := []string{
			"button:has-text('Select a credential type')",
			"button[aria-label='Credential type']",
			"div:has-text('Select a credential type')",
		}

		err = b.ClickElement(dropdownSelectors, "credential type dropdown", 5000)
		if err != nil {
			return fmt.Errorf("failed to open credential type dropdown: %w", err)
		}

		// Select Maven option
		mavenOptionSelectors := []string{
			"button:has-text('Maven Settings File')",
			"li:has-text('Maven Settings File')",
			"[role='option']:has-text('Maven Settings File')",
		}

		err = b.ClickElement(mavenOptionSelectors, "Maven Settings File option", 5000)
		if err != nil {
			return fmt.Errorf("failed to select Maven Settings File: %w", err)
		}
	}

	// Wait for Maven-specific fields to appear
	err = b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		b.log.V(2).Info("Network idle wait failed after type selection", "error", err)
	}

	// Upload or paste Maven settings content
	// Try file upload first
	fileInputSelectors := []string{
		"input[type='file']",
		"input[accept*='xml']",
		"input[name='file']",
	}

	fileSelector, err := b.TrySelectors(fileInputSelectors, 2000)
	if err == nil {
		// File upload field exists
		b.log.V(1).Info("Using file upload for Maven settings")
		err = b.page.SetInputFiles(fileSelector, settingsPath)
		if err != nil {
			return fmt.Errorf("failed to upload Maven settings file: %w", err)
		}
	} else {
		// Try textarea/text input
		b.log.V(1).Info("Using textarea for Maven settings content")
		textAreaSelectors := []string{
			"textarea[name='settings']",
			"textarea[name='content']",
			"textarea",
		}

		err = b.FillField(textAreaSelectors, string(settingsContent), "Maven settings content")
		if err != nil {
			return fmt.Errorf("failed to fill Maven settings content: %w", err)
		}
	}

	// Submit the form
	submitSelectors := []string{
		"button[type='submit']:has-text('Create')",
		"button:has-text('Save')",
		"button:has-text('Submit')",
	}

	err = b.ClickElement(submitSelectors, "Submit button", 10000)
	if err != nil {
		return fmt.Errorf("failed to submit identity form: %w", err)
	}

	// Wait for form to close
	b.log.V(1).Info("Waiting for identity creation to complete")
	err = b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(15000),
	})
	if err != nil {
		b.log.V(1).Info("Network idle wait failed after submit", "error", err)
	}

	b.log.Info("Maven identity created successfully", "name", name)
	return nil
}

// AssociateMavenIdentityWithApplication associates a Maven identity with an application
func (b *BrowserHelper) AssociateMavenIdentityWithApplication(appName, identityName string) error {
	b.log.Info("Associating Maven identity with application", "app", appName, "identity", identityName)

	// Navigate back to Applications
	err := b.NavigateToApplications()
	if err != nil {
		return fmt.Errorf("failed to navigate to applications: %w", err)
	}

	// Wait for applications page
	err = b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		b.log.V(2).Info("Network idle wait failed", "error", err)
	}

	// Find the application row
	appRowSelector := fmt.Sprintf("tr:has-text('%s')", appName)
	_, err = b.page.WaitForSelector(appRowSelector, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		return fmt.Errorf("application not found: %s", appName)
	}

	// Click on the application to open details/edit
	// Try clicking the pencil/edit icon
	editSelectors := []string{
		fmt.Sprintf("tr:has-text('%s') button[aria-label*='Edit']", appName),
		fmt.Sprintf("tr:has-text('%s') svg", appName), // Pencil icon
		fmt.Sprintf("tr:has-text('%s') .pf-v5-c-table__action button", appName),
	}

	err = b.ClickElement(editSelectors, "edit application button", 5000)
	if err != nil {
		b.log.V(1).Info("Could not click edit button, trying application name", "error", err)
		// Try clicking the application name itself
		nameSelector := fmt.Sprintf("tr:has-text('%s') td[data-label='Name']", appName)
		err = b.ClickElement([]string{nameSelector}, "application name", 5000)
		if err != nil {
			return fmt.Errorf("failed to open application: %w", err)
		}
	}

	// Wait for edit form or details drawer to appear
	err = b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		b.log.V(2).Info("Network idle wait failed", "error", err)
	}

	// Look for Maven identity dropdown/select
	// The field might be labeled "Maven" or "Maven settings"
	mavenSelectSelectors := []string{
		"select[name='maven']",
		"#maven",
		"button:has-text('Select Maven')",
		"[aria-label*='Maven']",
	}

	mavenSelector, err := b.TrySelectors(mavenSelectSelectors, 5000)
	if err != nil {
		return fmt.Errorf("failed to find Maven identity selector: %w", err)
	}

	// Try to select the identity
	// If it's a select element
	_, err = b.page.SelectOption(mavenSelector, playwright.SelectOptionValues{
		Labels: &[]string{identityName},
	})
	if err != nil {
		// Try dropdown approach
		b.log.V(1).Info("Select option failed, trying dropdown click", "error", err)

		err = b.page.Click(mavenSelector)
		if err != nil {
			return fmt.Errorf("failed to open Maven identity dropdown: %w", err)
		}

		// Click the identity option
		identityOptionSelectors := []string{
			fmt.Sprintf("button:has-text('%s')", identityName),
			fmt.Sprintf("li:has-text('%s')", identityName),
			fmt.Sprintf("[role='option']:has-text('%s')", identityName),
		}

		err = b.ClickElement(identityOptionSelectors, "Maven identity option", 5000)
		if err != nil {
			return fmt.Errorf("failed to select Maven identity: %w", err)
		}
	}

	// Save/Submit the form
	saveSelectors := []string{
		"button[type='submit']:has-text('Save')",
		"button:has-text('Save')",
		"button:has-text('Update')",
	}

	err = b.ClickElement(saveSelectors, "Save button", 10000)
	if err != nil {
		return fmt.Errorf("failed to save application: %w", err)
	}

	// Wait for save to complete
	err = b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		b.log.V(1).Info("Network idle wait failed after save", "error", err)
	}

	b.log.Info("Maven identity associated with application", "app", appName, "identity", identityName)
	return nil
}
