package browser

import (
	"fmt"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// SelectSourceTechnologies selects source technologies in the Advanced Options wizard step
func (h *Helper) SelectSourceTechnologies(sources []string) error {
	if len(sources) == 0 {
		h.log.V(1).Info("No source technologies specified, skipping")
		return nil
	}

	h.log.Info("Selecting source technologies", "sources", sources)

	// Wait for the sources selector to appear
	// The selector is a multi-select dropdown with id "formSources-id" or "sources-toggle"
	sourcesSelectors := []string{
		"#sources-toggle",
		"#formSources-id",
		"button[aria-label='Select sources']",
		"select[name='sources']",
	}

	sourcesSelector, err := h.TrySelectors(sourcesSelectors, 5000)
	if err != nil {
		h.log.V(1).Info("Could not find sources selector, may not be on Advanced Options step", "error", err)
		return nil // Don't fail - sources may be optional
	}

	// Click to open the dropdown
	err = h.page.Click(sourcesSelector)
	if err != nil {
		return fmt.Errorf("failed to open sources dropdown: %w", err)
	}

	// Wait for dropdown to open
	_, err = h.page.WaitForSelector("[role='listbox']", playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(3000),
	})
	if err != nil {
		h.log.V(1).Info("Dropdown not opened, trying alternate approach", "error", err)
	}

	// Select each source
	for _, source := range sources {
		h.log.V(1).Info("Selecting source technology", "source", source)

		// Try different selector strategies for the source option
		optionSelectors := []string{
			fmt.Sprintf("button[value='%s']", source),
			fmt.Sprintf("[role='option']:has-text('%s')", source),
			fmt.Sprintf("li:has-text('%s')", source),
			fmt.Sprintf("option[value='%s']", source),
		}

		var selected bool
		for _, selector := range optionSelectors {
			err = h.page.Click(selector, playwright.PageClickOptions{
				Timeout: playwright.Float(2000),
			})
			if err == nil {
				h.log.Info("Selected source technology", "source", source, "selector", selector)
				selected = true
				break
			}
		}

		if !selected {
			h.log.V(1).Info("Could not find option for source", "source", source)
			// Don't fail - continue with other sources
		}
	}

	// Close the dropdown by clicking outside or pressing Escape
	err = h.page.Keyboard().Press("Escape")
	if err != nil {
		h.log.V(2).Info("Failed to close dropdown with Escape", "error", err)
	}

	h.log.Info("Source technologies selection completed", "count", len(sources))
	return nil
}

// UploadCustomRules uploads custom rule files or configures Git repository URLs in the Custom Rules wizard step
func (h *Helper) UploadCustomRules(rules []string) error {
	if len(rules) == 0 {
		h.log.V(1).Info("No custom rules specified, skipping")
		return nil
	}

	h.log.Info("Starting custom rules configuration", "count", len(rules), "rules", rules)

	// Process each rule - could be a local file path or a Git URL
	for i, rule := range rules {
		if isGitURL(rule) {
			// This is a Git repository URL
			h.log.Info("Adding custom rules from Git repository", "url", rule, "index", i)
			err := h.addCustomRulesFromGit(rule, i)
			if err != nil {
				return fmt.Errorf("failed to add custom rules from Git URL %s: %w", rule, err)
			}
		} else {
			// This is a local file path
			h.log.Info("Uploading custom rules from local file", "path", rule, "index", i)
			err := h.addCustomRulesFromFile(rule, i)
			if err != nil {
				return fmt.Errorf("failed to upload custom rule file %s: %w", rule, err)
			}
		}
	}

	h.log.Info("Custom rules configuration completed", "count", len(rules))
	return nil
}

// addCustomRulesFromFile uploads a local rule file
func (h *Helper) addCustomRulesFromFile(filePath string, index int) error {
	// If this is not the first rule, we may need to add another rule source
	if index > 0 {
		// Click "Add" or "Add source" button to add another rule
		addButtonSelectors := []string{
			"button:has-text('Add')",
			"button:has-text('Add source')",
			"button[aria-label='Add rule source']",
		}
		err := h.ClickElement(addButtonSelectors, "add rule source button", 5000)
		if err != nil {
			h.log.V(1).Info("Could not find add rule source button, trying to upload anyway", "error", err)
		}
	}

	// Make sure we're on the "Upload" tab (default)
	uploadTabSelectors := []string{
		"button[data-ouia-component-type='PF5/TabButton']:has-text('Upload')",
		"button.pf-v5-c-tabs__link:has-text('Upload')",
		"[role='tab']:has-text('Upload')",
	}
	err := h.ClickElement(uploadTabSelectors, "Upload tab", 2000)
	if err != nil {
		h.log.V(1).Info("Could not click Upload tab, may already be active", "error", err)
	}

	// Wait for the file upload field
	uploadSelectors := []string{
		"input[type='file'][accept*='yaml']",
		"input[type='file'][accept*='.yml']",
		"input[type='file']",
	}

	uploadSelector, err := h.TrySelectors(uploadSelectors, 5000)
	if err != nil {
		return fmt.Errorf("could not find file upload field: %w", err)
	}

	// Upload the file
	h.log.V(1).Info("Uploading rule file", "path", filePath)
	err = h.page.SetInputFiles(uploadSelector, filePath)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	// Wait for upload to complete
	err = h.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		h.log.V(2).Info("Network idle wait failed after upload", "error", err)
	}

	h.log.Info("Rule file uploaded successfully", "path", filePath)
	return nil
}

// addCustomRulesFromGit configures a Git repository URL for custom rules
func (h *Helper) addCustomRulesFromGit(gitURL string, index int) error {
	// If this is not the first rule, we may need to add another rule source
	if index > 0 {
		// Click "Add" or "Add source" button to add another rule
		addButtonSelectors := []string{
			"button:has-text('Add')",
			"button:has-text('Add source')",
			"button[aria-label='Add rule source']",
		}
		err := h.ClickElement(addButtonSelectors, "add rule source button", 5000)
		if err != nil {
			h.log.V(1).Info("Could not find add rule source button, trying to continue anyway", "error", err)
		}
	}

	// Click the "Repository" tab
	repositoryTabSelectors := []string{
		"button[data-ouia-component-type='PF5/TabButton']:has-text('Repository')",
		"button.pf-v5-c-tabs__link:has-text('Repository')",
		"[role='tab']:has-text('Repository')",
		".pf-v5-c-tabs__item-text:has-text('Repository')",
	}

	h.log.V(1).Info("Clicking Repository tab")
	err := h.ClickElement(repositoryTabSelectors, "Repository tab", 5000)
	if err != nil {
		return fmt.Errorf("failed to click Repository tab: %w", err)
	}

	// Wait for repository type dropdown to appear
	h.log.V(1).Info("Waiting for repository type dropdown")
	_, err = h.page.WaitForSelector("select", playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		h.log.V(1).Info("Repository type dropdown not immediately visible", "error", err)
	}

	// Select repository type as "git"
	repositoryTypeSelectors := []string{
		"select[name='repositoryType']",
		"#repositoryType",
		"select",
	}

	h.log.Info("Selecting repository type: git")
	repoTypeSelector, err := h.TrySelectors(repositoryTypeSelectors, 5000)
	if err != nil {
		h.log.V(1).Info("Could not find repository type selector", "error", err)
	} else {
		_, err = h.page.SelectOption(repoTypeSelector, playwright.SelectOptionValues{
			Values: &[]string{"git"},
		})
		if err != nil {
			h.log.V(1).Info("Failed to select git repository type", "error", err)
		} else {
			h.log.Info("Repository type set to git")
		}
	}

	// Wait for repository URL input to appear after selecting type
	_, err = h.page.WaitForSelector("input[type='text']", playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		h.log.V(1).Info("Repository fields not immediately visible", "error", err)
	}

	// Parse the Git URL to extract repository, branch, and path
	// Format: https://github.com/user/repo#branch/path
	repoURL, branch, path := parseGitURL(gitURL)

	h.log.Info("Parsed Git URL", "url", repoURL, "branch", branch, "path", path)

	// Wait for repository URL input to be visible after clicking Repository tab
	repoURLSelectors := []string{
		"input[name='sourceRepository']",
		"#sourceRepository",
		"input[name*='repositoryURL']",
		"input[name*='repository']",
		"input[name*='url']",
		"input[placeholder*='Repository']",
	}

	// Wait for the field to appear
	h.log.V(1).Info("Waiting for repository URL field to appear")
	_, err = h.page.WaitForSelector(repoURLSelectors[0], playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		h.log.V(1).Info("sourceRepository field not found, trying other selectors", "error", err)
	}

	h.log.Info("Filling repository URL field", "url", repoURL)
	err = h.FillField(repoURLSelectors, repoURL, "repository URL")
	if err != nil {
		return fmt.Errorf("failed to fill repository URL: %w", err)
	}
	h.log.Info("Repository URL filled successfully", "url", repoURL)

	// Fill branch if present
	if branch != "" {
		branchSelectors := []string{
			"input[name='sourceBranch']",
			"#sourceBranch",
			"input[name*='branch']",
			"input[placeholder*='Branch']",
		}
		h.log.Info("Filling branch field", "branch", branch)
		err = h.FillField(branchSelectors, branch, "branch")
		if err != nil {
			h.log.V(1).Info("Could not fill branch field", "error", err)
			// Don't fail - branch might be optional
		} else {
			h.log.Info("Branch filled successfully", "branch", branch)
		}
	}

	// Fill root path if present
	if path != "" {
		pathSelectors := []string{
			"input[name='rootPath']",
			"#rootPath",
			"input[name='sourcePath']",
			"#sourcePath",
			"input[name*='path']",
			"input[placeholder*='Path']",
		}
		h.log.Info("Filling root path field", "path", path)
		err = h.FillField(pathSelectors, path, "root path")
		if err != nil {
			h.log.V(1).Info("Could not fill root path field", "error", err)
			// Don't fail - path might be optional
		} else {
			h.log.Info("Root path filled successfully", "path", path)
		}
	}

	h.log.Info("Git repository URL configured", "url", repoURL, "branch", branch, "path", path)
	return nil
}

// isGitURL checks if a string is a Git repository URL
func isGitURL(s string) bool {
	return strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "git://") ||
		strings.HasPrefix(s, "git@")
}

// parseGitURL parses a Git URL that may contain branch and path information
// Format: https://github.com/user/repo#branch/path
// Returns: (url, branch, path)
func parseGitURL(gitURL string) (string, string, string) {
	// Check if there's a # fragment
	parts := strings.SplitN(gitURL, "#", 2)
	if len(parts) == 1 {
		// No fragment - just return the URL
		return gitURL, "", ""
	}

	baseURL := parts[0]
	fragment := parts[1]

	// The fragment might be "branch/path" or just "branch"
	// Split on first / to separate branch from path
	fragmentParts := strings.SplitN(fragment, "/", 2)
	branch := fragmentParts[0]
	path := ""
	if len(fragmentParts) > 1 {
		path = fragmentParts[1]
	}

	return baseURL, branch, path
}

// SetDisableDefaultRules sets the "Disable default rules" checkbox
func (h *Helper) SetDisableDefaultRules(disable bool) error {
	if !disable {
		h.log.V(1).Info("Default rules are enabled, no action needed")
		return nil
	}

	h.log.Info("Disabling default rules")

	// Find the checkbox
	checkboxSelectors := []string{
		"input[type='checkbox'][name*='disableDefaultRules']",
		"input[type='checkbox']:has-text('Disable default rulesets')",
		"label:has-text('Disable default'):has(input[type='checkbox'])",
	}

	checkboxSelector, err := h.TrySelectors(checkboxSelectors, 5000)
	if err != nil {
		h.log.V(1).Info("Could not find disable default rules checkbox", "error", err)
		return nil // Don't fail
	}

	// Check if already checked
	isChecked, err := h.page.IsChecked(checkboxSelector)
	if err == nil && isChecked {
		h.log.V(1).Info("Default rules already disabled")
		return nil
	}

	// Click the checkbox
	err = h.page.Check(checkboxSelector)
	if err != nil {
		return fmt.Errorf("failed to check disable default rules: %w", err)
	}

	h.log.Info("Default rules disabled")
	return nil
}
