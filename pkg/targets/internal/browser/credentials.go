package browser

import (
	"fmt"
	"os"

	"github.com/playwright-community/playwright-go"
)

// CreateMavenIdentity creates a Maven settings credential/identity in the Administrator view
func (h *Helper) CreateMavenIdentity(baseURL, name, settingsPath string) error {
	h.log.Info("Creating Maven identity", "name", name, "settingsPath", settingsPath)

	// Read the Maven settings file
	settingsContent, err := os.ReadFile(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read Maven settings file %s: %w", settingsPath, err)
	}

	// Navigate directly to Credentials/Identities page
	// In Tackle UI, this is /identities which is in the Administration perspective
	h.log.V(1).Info("Navigating to Credentials/Identities page")
	identitiesURL := baseURL + "/identities"
	_, err = h.page.Goto(identitiesURL)
	if err != nil {
		return fmt.Errorf("failed to navigate to identities page: %w", err)
	}

	// Wait for Credentials page to load
	err = h.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		h.log.V(2).Info("Network idle wait failed", "error", err)
	}

	// Check if identity already exists
	existingSelector := fmt.Sprintf("td:has-text('%s')", name)
	count, err := h.page.Locator(existingSelector).Count()
	if err == nil && count > 0 {
		h.log.Info("Maven identity already exists, skipping creation", "name", name)
		return nil
	}

	// Click Create button
	h.log.Info("Looking for Create button on identities page")
	createSelectors := []string{
		"button:has-text('Create')",
		"button:has-text('Create new')",
		"button:has-text('Create credential')",
		"button:has-text('Create identity')",
		"[aria-label='Create']",
		"[aria-label='Create credential']",
		"[aria-label='Create identity']",
		"button[data-testid='create-button']",
	}
	err = h.ClickElement(createSelectors, "Create button", 10000)
	if err != nil {
		return fmt.Errorf("failed to click Create button: %w", err)
	}
	h.log.Info("Create button clicked successfully")

	// Wait for Create Identity form to appear
	_, err = h.page.WaitForSelector("input[name='name']", playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		return fmt.Errorf("Create Identity form did not appear: %w", err)
	}

	// Fill identity name
	err = h.FillField([]string{"input[name='name']", "#name"}, name, "identity name")
	if err != nil {
		return fmt.Errorf("failed to fill identity name: %w", err)
	}

	// Fill description (optional but good practice)
	descriptionSelectors := []string{"input[name='description']", "textarea[name='description']", "#description"}
	descSelector, descErr := h.TrySelectors(descriptionSelectors, 2000)
	if descErr == nil {
		h.log.V(1).Info("Filling description field")
		err = h.page.Fill(descSelector, "Maven settings for koncur tests")
		if err != nil {
			h.log.V(1).Info("Failed to fill description (non-critical)", "error", err)
		}
	}

	// Select "Maven Settings File" type
	// Modern PatternFly uses dropdown buttons, not <select> elements
	h.log.Info("Selecting Maven Settings File credential type")

	// Wait for the credential type field to appear after the form loads
	h.log.V(1).Info("Waiting for credential type field to appear")
	err = h.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		h.log.V(1).Info("Network idle wait failed", "error", err)
	}

	// Strategy 1: Try PatternFly dropdown (most common in modern UI)
	dropdownSelectors := []string{
		"#type-select-toggle",
		"button[aria-label='Type select dropdown toggle']",
		"button:has-text('Select a credential type')",
		"button:has-text('Select credential type')",
		"button:has-text('Select a type')",
		"button:has-text('Type')",
		"button[aria-label='Credential type']",
		"button[aria-label*='type']",
		"button[aria-label*='Type']",
		"#credential-type-toggle",
		"#type-toggle",
		"[data-testid='credential-type-toggle']",
		"button.pf-v5-c-select__toggle:has-text('Select...')",
		"[id*='toggle']:has-text('Select')",
	}

	h.log.Info("Looking for credential type dropdown", "selectorsCount", len(dropdownSelectors))
	err = h.ClickElement(dropdownSelectors, "credential type dropdown", 10000)
	if err != nil {
		// Strategy 2: Try native <select> element (older UI)
		h.log.Info("Dropdown not found, trying select element", "dropdownError", err)
		typeSelectors := []string{
			"select[name='type']",
			"#type",
			"select[name='credentialType']",
			"#credentialType",
			"select:has(option:has-text('Maven Settings File'))",
			"select",
		}

		h.log.V(1).Info("Trying select element selectors", "count", len(typeSelectors))
		typeSelector, err := h.TrySelectors(typeSelectors, 5000)
		if err != nil {
			return fmt.Errorf("could not find credential type selector - tried %d dropdown selectors and %d select selectors: %w",
				len(dropdownSelectors), len(typeSelectors), err)
		}

		// Use select element
		_, err = h.page.SelectOption(typeSelector, playwright.SelectOptionValues{
			Values: &[]string{"maven"},
		})
		if err != nil {
			return fmt.Errorf("failed to select maven from select element: %w", err)
		}
	} else {
		// Dropdown clicked successfully - now select Maven option
		h.log.V(1).Info("Dropdown opened, selecting Maven Settings File option")

		mavenOptionSelectors := []string{
			"button:has-text('Maven Settings File')",
			"li:has-text('Maven Settings File')",
			"[role='option']:has-text('Maven Settings File')",
			"[role='menuitem']:has-text('Maven Settings File')",
		}

		err = h.ClickElement(mavenOptionSelectors, "Maven Settings File option", 5000)
		if err != nil {
			return fmt.Errorf("failed to select Maven Settings File from dropdown: %w", err)
		}
		h.log.Info("Maven Settings File type selected")
	}

	// Wait for Maven-specific fields to appear
	h.log.Info("Waiting for Maven-specific fields to appear")
	err = h.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		h.log.V(2).Info("Network idle wait failed after type selection", "error", err)
	}

	// Upload or paste Maven settings content
	// Try file upload first
	h.log.V(1).Info("Looking for Maven settings input field")
	fileInputSelectors := []string{
		"input[type='file']",
		"input[accept*='xml']",
		"input[name='file']",
	}

	fileSelector, err := h.TrySelectors(fileInputSelectors, 2000)
	if err == nil {
		// File upload field exists
		h.log.Info("Found file upload field, uploading Maven settings file", "path", settingsPath)
		err = h.page.SetInputFiles(fileSelector, settingsPath)
		if err != nil {
			return fmt.Errorf("failed to upload Maven settings file: %w", err)
		}
		h.log.Info("Maven settings file uploaded successfully")
	} else {
		// Try textarea/text input
		h.log.Info("File upload not found, trying textarea for Maven settings content")
		textAreaSelectors := []string{
			"textarea[name='settings']",
			"textarea[name='content']",
			"textarea",
		}

		err = h.FillField(textAreaSelectors, string(settingsContent), "Maven settings content")
		if err != nil {
			return fmt.Errorf("failed to fill Maven settings content: %w", err)
		}
		h.log.Info("Maven settings content filled in textarea")
	}

	// Submit the form
	h.log.Info("Looking for submit button to create credential")
	submitSelectors := []string{
		"button[type='submit']:has-text('Create')",
		"button[type='submit']:has-text('Save')",
		"button:has-text('Create')",
		"button:has-text('Save')",
		"button:has-text('Submit')",
		"footer button:has-text('Create')",
		"footer button[type='submit']",
		"[aria-label='Create']",
	}

	err = h.ClickElement(submitSelectors, "Submit button", 10000)
	if err != nil {
		return fmt.Errorf("failed to submit identity form: %w", err)
	}
	h.log.Info("Submit button clicked, waiting for form to close")

	// Check if there's an error alert that appeared immediately after submit
	alertSelectors := []string{
		".pf-v5-c-alert.pf-m-danger",
		".pf-v5-c-alert.pf-m-warning",
		"[role='alert']",
	}
	for _, alertSel := range alertSelectors {
		alertCount, _ := h.page.Locator(alertSel).Count()
		if alertCount > 0 {
			alertText, _ := h.page.Locator(alertSel).First().TextContent()
			h.log.Info("Alert detected after submit", "message", alertText)
		}
	}

	// Wait for form to close and identity to appear in the list
	h.log.V(1).Info("Waiting for identity creation to complete")

	// First wait for network to settle
	err = h.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(15000),
	})
	if err != nil {
		h.log.V(1).Info("Network idle wait failed after submit", "error", err)
	}

	// Wait for the identity to appear in the table (it should appear after creation)
	h.log.V(1).Info("Waiting for identity to appear in list", "name", name)
	identityRowSelector := fmt.Sprintf("td:has-text('%s')", name)
	_, err = h.page.WaitForSelector(identityRowSelector, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10000),
	})

	// If wait fails, check if it's there anyway (sometimes the wait times out but element exists)
	count, verifyErr := h.page.Locator(identityRowSelector).Count()
	if verifyErr != nil || count == 0 {
		// Identity not found - the form submission may have failed
		// Check for validation errors on the form
		errorSelectors := []string{
			".pf-v5-c-alert.pf-m-danger",
			".pf-v5-c-form__helper-text.pf-m-error",
			"[aria-invalid='true']",
			".pf-m-error",
			".pf-c-form__helper-text--error",
		}

		var errorMessages []string
		for _, errSel := range errorSelectors {
			errCount, _ := h.page.Locator(errSel).Count()
			if errCount > 0 {
				// Try to get error message text
				errText, _ := h.page.Locator(errSel).First().TextContent()
				if errText != "" {
					h.log.Info("Form validation error detected", "selector", errSel, "error", errText)
					errorMessages = append(errorMessages, errText)
				}
			}
		}

		if len(errorMessages) > 0 {
			return fmt.Errorf("identity creation failed with validation errors: %v", errorMessages)
		}
		return fmt.Errorf("identity was not created - not found in list after submission (possible validation error)")
	}

	h.log.Info("Maven identity created and verified in list", "name", name)
	return nil
}

// AssociateMavenIdentityWithApplication associates a Maven identity with an application
func (h *Helper) AssociateMavenIdentityWithApplication(baseURL, appName, identityName string) error {
	h.log.Info("Associating Maven identity with application", "app", appName, "identity", identityName)

	// Navigate back to Applications using direct URL
	err := h.NavigateToApplications(baseURL)
	if err != nil {
		return fmt.Errorf("failed to navigate to applications: %w", err)
	}

	// Wait for applications page
	err = h.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		h.log.V(2).Info("Network idle wait failed", "error", err)
	}

	// Find the application row
	appRowSelector := fmt.Sprintf("tr:has-text('%s')", appName)
	_, err = h.page.WaitForSelector(appRowSelector, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		return fmt.Errorf("application not found: %s", appName)
	}

	// Click the Kebab menu (three dots) for the application
	h.log.Info("Looking for Kebab menu for application", "app", appName)
	kebabSelectors := []string{
		fmt.Sprintf("tr:has-text('%s') button[aria-label='Kebab toggle']", appName),
		fmt.Sprintf("tr:has-text('%s') button.pf-v5-c-menu-toggle.pf-m-plain", appName),
		fmt.Sprintf("tr:has-text('%s') button[data-ouia-component-type='PF5/MenuToggle']", appName),
	}

	err = h.ClickElement(kebabSelectors, "Kebab menu", 5000)
	if err != nil {
		return fmt.Errorf("failed to click Kebab menu: %w", err)
	}
	h.log.Info("Kebab menu opened")

	// Wait for menu to appear and click "Manage credentials"
	h.log.Info("Looking for 'Manage credentials' menu item")
	manageCredentialsSelectors := []string{
		"span.pf-v5-c-menu__item-text:has-text('Manage credentials')",
		"button:has-text('Manage credentials')",
		"[role='menuitem']:has-text('Manage credentials')",
	}

	_, err = h.page.WaitForSelector(manageCredentialsSelectors[0], playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		h.log.V(1).Info("Manage credentials not immediately visible", "error", err)
	}

	err = h.ClickElement(manageCredentialsSelectors, "Manage credentials", 5000)
	if err != nil {
		return fmt.Errorf("failed to click Manage credentials: %w", err)
	}
	h.log.Info("Clicked 'Manage credentials'")

	// Wait for the credentials form to appear
	err = h.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		h.log.V(2).Info("Network idle wait failed", "error", err)
	}

	// Find the Maven settings typeahead input
	h.log.Info("Looking for Maven settings typeahead input")
	mavenInputSelectors := []string{
		"#maven-settings-toggle-select-typeahead",
		"input[aria-label='Maven settings']",
		"input[placeholder='Select...'][type='text']",
	}

	mavenInputSelector, err := h.TrySelectors(mavenInputSelectors, 5000)
	if err != nil {
		return fmt.Errorf("failed to find Maven settings input: %w", err)
	}

	// Type the identity name into the typeahead field
	h.log.Info("Typing Maven identity name into typeahead", "identity", identityName)
	err = h.page.Fill(mavenInputSelector, identityName)
	if err != nil {
		return fmt.Errorf("failed to fill Maven identity name: %w", err)
	}

	// Wait for dropdown options to appear and select the identity
	h.log.V(1).Info("Waiting for identity option to appear in dropdown")
	identityOptionSelectors := []string{
		fmt.Sprintf("button:has-text('%s')", identityName),
		fmt.Sprintf("li:has-text('%s')", identityName),
		fmt.Sprintf("[role='option']:has-text('%s')", identityName),
	}

	// Wait for the option to appear
	_, err = h.page.WaitForSelector(identityOptionSelectors[0], playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(3000),
	})
	if err != nil {
		h.log.V(1).Info("Identity option not immediately visible, trying to click anyway", "error", err)
	}

	// Click the identity option from the dropdown
	err = h.ClickElement(identityOptionSelectors, "Maven identity option", 5000)
	if err != nil {
		h.log.V(1).Info("Could not click identity option, may be auto-selected", "error", err)
		// Don't fail - the typeahead might have auto-selected it
	}

	// Save/Submit the form
	saveSelectors := []string{
		"button[type='submit']:has-text('Save')",
		"button:has-text('Save')",
		"button:has-text('Update')",
	}

	err = h.ClickElement(saveSelectors, "Save button", 10000)
	if err != nil {
		return fmt.Errorf("failed to save application: %w", err)
	}

	// Wait for save to complete
	err = h.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		h.log.V(1).Info("Network idle wait failed after save", "error", err)
	}

	h.log.Info("Maven identity associated with application", "app", appName, "identity", identityName)
	return nil
}
