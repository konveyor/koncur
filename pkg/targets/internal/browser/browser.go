package browser

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/playwright-community/playwright-go"
)

// Helper wraps playwright browser operations with logging and error handling
type Helper struct {
	page    playwright.Page
	browser playwright.Browser
	pw      *playwright.Playwright
	log     logr.Logger
}

// New creates a new browser automation helper
func New(browserType string, headless bool, log logr.Logger) (*Helper, error) {
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

	return &Helper{
		page:    page,
		browser: browser,
		pw:      pw,
		log:     log,
	}, nil
}

// Close cleans up browser resources
func (h *Helper) Close() error {
	if h.page != nil {
		h.page.Close()
	}
	if h.browser != nil {
		h.browser.Close()
	}
	if h.pw != nil {
		h.pw.Stop()
	}
	return nil
}

// TrySelectors attempts multiple selectors and returns the first one that works
func (h *Helper) TrySelectors(selectors []string, timeout float64) (string, error) {
	for _, sel := range selectors {
		elem, err := h.page.QuerySelector(sel)
		if err == nil && elem != nil {
			h.log.V(2).Info("Found element", "selector", sel)
			return sel, nil
		}
	}
	return "", fmt.Errorf("could not find element (tried %d selectors)", len(selectors))
}

// ClickElement clicks an element using the best selector
func (h *Helper) ClickElement(selectors []string, description string, timeout float64) error {
	selector, err := h.TrySelectors(selectors, timeout)
	if err != nil {
		return fmt.Errorf("failed to find %s: %w", description, err)
	}

	h.log.V(1).Info("Clicking element", "description", description, "selector", selector)
	err = h.page.Click(selector, playwright.PageClickOptions{
		Timeout: playwright.Float(timeout),
	})
	if err != nil {
		return fmt.Errorf("failed to click %s: %w", description, err)
	}

	return nil
}

// FillField fills a form field
func (h *Helper) FillField(selectors []string, value, description string) error {
	selector, err := h.TrySelectors(selectors, 5000)
	if err != nil {
		return fmt.Errorf("failed to find %s: %w", description, err)
	}

	h.log.V(1).Info("Filling field", "description", description, "value", value)
	err = h.page.Fill(selector, value)
	if err != nil {
		return fmt.Errorf("failed to fill %s: %w", description, err)
	}

	return nil
}
