package browser

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/konveyor/test-harness/pkg/targets/internal/tackleui"
)

// NavigateToURL navigates to the specified URL
func (h *Helper) NavigateToURL(url string) error {
	h.log.Info("Navigating to URL", "url", url)
	_, err := h.page.Goto(url, playwright.PageGotoOptions{
		Timeout: playwright.Float(30000),
	})
	if err != nil {
		return fmt.Errorf("failed to navigate to %s: %w", url, err)
	}

	// Wait for network idle
	h.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})

	currentURL := h.page.URL()
	h.log.V(1).Info("Navigation complete", "currentURL", currentURL)
	return nil
}

// NavigateToApplications navigates to the applications page
// Accepts an optional baseURL - if provided, uses direct navigation which works across perspectives
// If not provided, tries to click the Applications nav link
func (h *Helper) NavigateToApplications(baseURL ...string) error {
	// If baseURL provided, use direct navigation (works across perspectives)
	if len(baseURL) > 0 && baseURL[0] != "" {
		applicationsURL := baseURL[0] + "/applications"
		h.log.Info("Navigating directly to applications page", "url", applicationsURL)
		return h.NavigateToURL(applicationsURL)
	}

	// Otherwise, try clicking the Applications nav link (only works within Migration perspective)
	h.log.V(1).Info("Trying to click Applications navigation link")
	return h.ClickElement(tackleui.AlternativeApplicationsNav, "Applications navigation", 10000)
}

// NavigateToResults navigates to the results/insights page for the application
// baseURL: the base Tackle UI URL (e.g., "https://tackle-ui.example.com")
// appName: the name of the application to view results for
func (h *Helper) NavigateToResults(baseURL, appName string) error {
	h.log.Info("Navigating to results", "appName", appName)

	// We should be on the applications page after WaitForAnalysisComplete
	// Flow:
	// 1. Click the application name to open the drawer
	// 2. Click the "Details" tab in the drawer
	// 3. Click the "Insights" link in the Details tab
	// 4. This navigates to /insights/single-app/:id

	// Step 1: Find and click the application row (clickable row, not a link)
	h.log.Info("Looking for application in table", "appName", appName)

	// First, let's verify we're on the applications page and can see the table
	currentURL := h.page.URL()
	h.log.V(1).Info("Current URL before clicking app", "url", currentURL)

	// Wait for the applications table to be visible
	_, tableErr := h.page.WaitForSelector("table", playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if tableErr != nil {
		h.log.Info("Applications table not found")
	}

	// The table uses clickable rows (pf-m-clickable), not links
	// Try multiple selectors to find and click the application row
	appRowSelectors := []string{
		// Click the clickable row
		fmt.Sprintf("tr.pf-m-clickable:has-text('%s')", appName),
		// Click the Name column in the row
		fmt.Sprintf("tr:has-text('%s') td[data-label='Name']", appName),
		// Click anywhere in the row containing the app name
		fmt.Sprintf("tr:has-text('%s')", appName),
		// Try with data-item-name attribute
		fmt.Sprintf("tr[data-item-name='%s']", appName),
	}

	var foundRow bool
	var lastErr error
	for i, sel := range appRowSelectors {
		h.log.Info("Trying selector", "index", i+1, "selector", sel)
		err := h.page.Click(sel, playwright.PageClickOptions{
			Timeout: playwright.Float(5000),
		})
		if err == nil {
			h.log.Info("Successfully clicked application row", "selector", sel)
			foundRow = true
			break
		} else {
			h.log.V(1).Info("Selector failed", "selector", sel, "error", err.Error())
			lastErr = err
		}
	}

	if !foundRow {
		return fmt.Errorf("could not find or click application row after trying all selectors: %w", lastErr)
	}

	h.log.V(1).Info("Clicked application name, waiting for drawer")

	// Wait for the drawer to appear
	_, drawerErr := h.page.WaitForSelector("div[class*='drawer' i], div[role='dialog']", playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if drawerErr != nil {
		return fmt.Errorf("application drawer did not appear: %w", drawerErr)
	}

	// Step 2: Click the "Details" tab
	h.log.V(1).Info("Clicking Details tab")
	detailsTabErr := h.ClickElement([]string{
		"button:has-text('Details')",
		"a:has-text('Details')",
		"span:has-text('Details')",
		".pf-v5-c-tabs__item-text:has-text('Details')",
	}, "Details tab", 5000)

	if detailsTabErr != nil {
		h.log.V(1).Info("Could not click Details tab, maybe already on it", "error", detailsTabErr)
	}

	// Wait a moment for the tab content to load
	h.page.WaitForTimeout(500)

	// Step 3: Click the "Insights" link
	h.log.V(1).Info("Clicking Insights link")
	insightsLinkErr := h.ClickElement([]string{
		"a[href*='/insights/single-app/']",
		"a:has-text('Insights')",
	}, "Insights link", 5000)

	if insightsLinkErr != nil {
		return fmt.Errorf("could not find Insights link: %w", insightsLinkErr)
	}

	// Wait for navigation to complete
	navErr := h.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(15000),
	})
	if navErr != nil {
		h.log.V(1).Info("Network idle wait failed", "error", navErr)
	}

	// Wait for the insights table to appear
	_, insightsTableErr := h.page.WaitForSelector("table, [data-testid='insights-table']", playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(15000),
	})
	if insightsTableErr != nil {
		h.log.V(1).Info("Insights table did not appear", "error", insightsTableErr)
		return fmt.Errorf("insights table not found: %w", insightsTableErr)
	}

	finalURL := h.page.URL()
	h.log.Info("On single-app insights page", "url", finalURL)

	return nil
}

// GetApplicationIDFromURL extracts the application ID from the current URL
// Expected URL format: /insights/single-app/:id or /issues/single-app/:id
func (h *Helper) GetApplicationIDFromURL() (uint, error) {
	rawURL := h.page.URL()
	h.log.V(1).Info("Extracting application ID from URL", "url", rawURL)

	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return 0, fmt.Errorf("failed to parse URL %s: %w", rawURL, err)
	}

	// Get the path (without query parameters)
	path := parsedURL.Path

	// Expected patterns:
	// - /insights/single-app/123
	// - /issues/single-app/123

	var appID uint64
	// Try insights pattern
	if strings.HasPrefix(path, "/insights/single-app/") {
		idStr := strings.TrimPrefix(path, "/insights/single-app/")
		appID, err = strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse application ID from %s: %w", idStr, err)
		}
	} else if strings.HasPrefix(path, "/issues/single-app/") {
		idStr := strings.TrimPrefix(path, "/issues/single-app/")
		appID, err = strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse application ID from %s: %w", idStr, err)
		}
	} else {
		return 0, fmt.Errorf("URL path does not match expected pattern (/insights/single-app/:id or /issues/single-app/:id): %s", path)
	}

	h.log.Info("Extracted application ID from URL", "appID", appID)
	return uint(appID), nil
}
