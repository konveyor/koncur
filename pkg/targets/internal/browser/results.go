package browser

import "github.com/konveyor/test-harness/pkg/targets/internal/tackleui"

// GetPageHTML returns the full HTML content of the current page
func (h *Helper) GetPageHTML() (string, error) {
	return h.page.Content()
}

// ExtractInsightsHTML extracts the insights section HTML for parsing
func (h *Helper) ExtractInsightsHTML() (string, error) {
	h.log.Info("Extracting insights HTML")

	// Try to find insights container
	selector, err := h.TrySelectors(tackleui.AlternativeInsightsContainer, 5000)
	if err != nil {
		// Fallback to full page HTML
		h.log.V(1).Info("Insights container not found, using full page HTML")
		return h.GetPageHTML()
	}

	elem, err := h.page.QuerySelector(selector)
	if err != nil || elem == nil {
		return h.GetPageHTML()
	}

	html, err := elem.InnerHTML()
	if err != nil {
		return h.GetPageHTML()
	}

	h.log.Info("Extracted insights HTML", "size", len(html))
	return html, nil
}
