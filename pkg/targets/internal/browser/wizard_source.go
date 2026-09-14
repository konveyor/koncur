package browser

import (
	"fmt"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/konveyor/analyzer-lsp/provider"
	"github.com/konveyor/test-harness/pkg/config"
)

// SelectAnalysisSource selects the analysis source type (source code + dependencies / source code / binary)
func (h *Helper) SelectAnalysisSource(test *config.TestDefinition) error {
	h.log.V(1).Info("Looking for Analysis Source selection")

	// Determine which source type to select based on the application
	// The application field determines whether we need to upload a binary or use source code
	var sourceType string
	var shouldUploadBinary bool

	// Strip "binary:" prefix if present (legacy format)
	application := test.Analysis.Application
	if strings.HasPrefix(application, "binary:") {
		application = application[7:] // Remove "binary:" prefix
		h.log.V(1).Info("Stripped binary: prefix", "original", test.Analysis.Application, "cleaned", application)
	}

	// Check if the application is a binary file
	h.log.Info("Checking if application is binary", "application", application, "isBinary", IsBinaryFile(application))
	if IsBinaryFile(application) {
		// This is a binary file - we need to upload it
		sourceType = "Upload a local binary"
		shouldUploadBinary = true
		h.log.Info("Detected binary file, will upload", "application", application)
	} else {
		// This is source code - determine if we want dependencies or not
		if test.Analysis.AnalysisMode == provider.SourceOnlyAnalysisMode {
			sourceType = "Source code"
			h.log.Info("Selecting Source code mode (no dependencies)")
		} else {
			// Default to source + dependencies for source code
			sourceType = "Source code + dependencies"
			h.log.Info("Selecting Source code + dependencies mode")
		}
	}

	// The Analysis Source is a dropdown menu (MenuToggle component)
	// We need to:
	// 1. Click the toggle button to open the dropdown
	// 2. Click the menu item with the desired source type

	h.log.Info("Opening Analysis Source dropdown")

	// Click the dropdown toggle button
	toggleSelectors := []string{
		"#analysis-source-toggle",
		"[data-testid='analysis-source-select-toggle']",
		"button[aria-label='Analysis source dropdown toggle']",
		".pf-v5-c-menu-toggle:has-text('Source code')",
	}

	var toggleClicked bool
	for _, toggleSel := range toggleSelectors {
		err := h.page.Click(toggleSel, playwright.PageClickOptions{
			Timeout: playwright.Float(2000),
		})
		if err == nil {
			h.log.Info("Clicked Analysis Source dropdown toggle", "selector", toggleSel)
			toggleClicked = true
			break
		}
	}

	if !toggleClicked {
		h.log.V(1).Info("Could not find dropdown toggle, trying direct menu item click")
	} else {
		// Wait for dropdown menu to appear
		h.page.WaitForTimeout(500)
	}

	// Now click the menu item with the desired source type
	// IMPORTANT: Use exact text match, not :has-text() which does partial matching
	// "Source code" would match both "Source code" and "Source code + dependencies"
	// Use JavaScript to find exact text match
	menuItemSelector := fmt.Sprintf(`button[role='option'] >> text="%s"`, sourceType)

	h.log.Info("Looking for menu item", "sourceType", sourceType, "selector", menuItemSelector)

	menuItemSelectors := []string{
		menuItemSelector,
		// Fallback: try using getByRole with exact name
		fmt.Sprintf(`button[role='option']:has(.pf-v5-c-menu__item-text:text-is("%s"))`, sourceType),
	}

	for _, menuSel := range menuItemSelectors {
		err := h.page.Click(menuSel, playwright.PageClickOptions{
			Timeout: playwright.Float(2000),
		})
		if err == nil {
			h.log.Info("Selected Analysis Source from dropdown", "type", sourceType, "selector", menuSel)

			// Wait for menu to close and selection to be processed
			h.page.WaitForTimeout(1000)

			// Verify the toggle button now shows the correct source type
			toggleTextSelector := "#analysis-source-toggle .pf-v5-c-menu-toggle__text"
			_, verifyErr := h.page.WaitForSelector(fmt.Sprintf("%s:has-text('%s')", toggleTextSelector, sourceType), playwright.PageWaitForSelectorOptions{
				State:   playwright.WaitForSelectorStateAttached,
				Timeout: playwright.Float(3000),
			})
			if verifyErr == nil {
				h.log.Info("Verified Analysis Source selection", "type", sourceType)
			} else {
				h.log.Info("Could not verify Analysis Source selection in toggle text", "type", sourceType, "error", verifyErr)
				// Try clicking the menu item again
				h.page.Click(toggleSelectors[0], playwright.PageClickOptions{Timeout: playwright.Float(1000)})
				h.page.WaitForTimeout(500)
				h.page.Click(menuSel, playwright.PageClickOptions{Timeout: playwright.Float(2000)})
				h.page.WaitForTimeout(1000)
			}

			// If we need to upload a binary file, do it now
			if shouldUploadBinary {
				h.log.Info("Uploading binary file", "path", application)
				err = h.UploadBinary(application)
				if err != nil {
					return fmt.Errorf("failed to upload binary: %w", err)
				}
			}

			return nil
		}
	}

	// If we couldn't find it, log but don't fail
	h.log.V(1).Info("Could not find Analysis Source selector, may already be selected", "type", sourceType)
	return nil
}

// UploadBinary uploads a binary file (jar, war, ear) in the analysis wizard
func (h *Helper) UploadBinary(binaryPath string) error {
	h.log.Info("Uploading binary file", "path", binaryPath)

	// Wait for the multiple file upload component to appear
	// This is a PatternFly component with a visible upload button
	h.log.V(1).Info("Waiting for file upload component")
	_, err := h.page.WaitForSelector(".pf-v5-c-multiple-file-upload", playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		h.log.V(1).Info("Multiple file upload component not found, trying alternative approach")
	}

	// Strategy 1: Try to use the hidden file input directly (works in many cases)
	uploadSelectors := []string{
		".pf-v5-c-multiple-file-upload input[type='file']",
		"input[type='file'][accept*='jar']",
		"input[type='file'][accept*='war']",
		"input[type='file'][accept*='ear']",
		"input[type='file']",
	}

	uploadSelector, err := h.TrySelectors(uploadSelectors, 2000)
	if err == nil {
		// Try direct upload to hidden input
		h.log.V(1).Info("Attempting direct upload to file input", "selector", uploadSelector)
		err = h.page.SetInputFiles(uploadSelector, binaryPath)
		if err == nil {
			h.log.V(1).Info("Successfully uploaded file directly")
			return nil
		}
		h.log.V(1).Info("Direct upload failed, trying button click approach", "error", err)
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
	h.log.V(1).Info("Setting up file chooser handler")
	fileChooserChan := make(chan playwright.FileChooser, 1)
	h.page.Once("filechooser", func(fc playwright.FileChooser) {
		fileChooserChan <- fc
	})

	// Click the upload button
	h.log.V(1).Info("Clicking upload button")
	err = h.ClickElement(uploadButtonSelectors, "upload button", 10000)
	if err != nil {
		return fmt.Errorf("failed to click upload button: %w", err)
	}

	// Wait for file chooser to appear and select the file
	h.log.V(1).Info("Waiting for file chooser")
	select {
	case fc := <-fileChooserChan:
		h.log.V(1).Info("File chooser appeared, selecting file", "path", binaryPath)
		err = fc.SetFiles(binaryPath)
		if err != nil {
			return fmt.Errorf("failed to set file in chooser: %w", err)
		}
		h.log.Info("Binary file selected successfully")
	case <-time.After(5 * time.Second):
		// File chooser didn't appear - the input might have been updated directly
		// Try to find the input again and set files
		h.log.V(1).Info("File chooser timeout, trying direct input approach")
		uploadSelector, err = h.TrySelectors(uploadSelectors, 2000)
		if err != nil {
			return fmt.Errorf("file chooser did not appear and could not find file input: %w", err)
		}
		err = h.page.SetInputFiles(uploadSelector, binaryPath)
		if err != nil {
			return fmt.Errorf("failed to upload file after button click: %w", err)
		}
	}

	// Wait for upload to complete
	// Look for success indicator or progress completion
	h.log.V(1).Info("Waiting for binary upload to complete")
	err = h.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(60000), // Binary uploads can take longer
	})
	if err != nil {
		h.log.V(1).Info("Network idle wait failed after upload", "error", err)
	}

	// Look for success indicator
	successSelectors := []string{
		"[data-testid='upload-success']",
		".pf-m-success",
		"svg.pf-v5-svg[aria-label*='success']",
		"text=Uploaded",
	}

	for _, sel := range successSelectors {
		count, _ := h.page.Locator(sel).Count()
		if count > 0 {
			h.log.Info("Binary upload completed successfully", "indicator", sel)
			return nil
		}
	}

	h.log.Info("Binary upload completed (success indicator not found, proceeding anyway)")
	return nil
}
