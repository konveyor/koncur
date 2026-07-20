package browser

import (
	"fmt"
	"strings"
	"time"

	"github.com/konveyor/test-harness/pkg/targets/internal/tackleui"
)

// WaitForAnalysisComplete polls until analysis completes or times out
func (h *Helper) WaitForAnalysisComplete(timeout time.Duration) error {
	h.log.Info("Waiting for analysis to complete", "timeout", timeout)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	lastStatus := ""

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("analysis timeout after %v", timeout)
		}

		// Wait for the Analysis status column to show "Completed"
		// The structure is: <td data-label="Analysis"><div>...<div>Completed</div></div></td>
		// Look for the success icon (checkmark SVG) combined with "Completed" text
		completedSelectors := []string{
			// Match the Analysis column with success icon and "Completed" text
			"td[data-label='Analysis']:has-text('Completed') .pf-m-success",
			// Fallback: just look for "Completed" in the Analysis column
			"td[data-label='Analysis']:has-text('Completed')",
			// Alternative: look for the success icon SVG in Analysis column
			"td[data-label='Analysis'] .pf-v5-c-icon__content.pf-m-success",
		}

		// Try to find the completed status indicator
		var foundCompleted bool
		for _, selector := range completedSelectors {
			count, err := h.page.Locator(selector).Count()
			if err == nil && count > 0 {
				// Check if the text actually contains "Completed"
				elem, err := h.page.QuerySelector(selector)
				if err == nil && elem != nil {
					text, _ := elem.TextContent()
					if strings.Contains(strings.ToLower(text), "completed") {
						h.log.Info("Analysis completed successfully", "selector", selector)
						return nil
					}
					foundCompleted = true
					break
				}
			}
		}

		if foundCompleted {
			h.log.Info("Analysis completed successfully")
			return nil
		}

		// Also check for other status indicators as fallback
		selector, err := h.TrySelectors(tackleui.AlternativeStatusIndicator, 2000)
		if err == nil {
			elem, err := h.page.QuerySelector(selector)
			if err == nil && elem != nil {
				text, _ := elem.TextContent()
				text = strings.TrimSpace(text)

				if text != lastStatus {
					h.log.Info("Analysis status", "status", text)
					lastStatus = text
				}

				// Check if completed
				if contains(text, tackleui.StatusSucceeded, "completed", "success") {
					h.log.Info("Analysis completed successfully")
					return nil
				}

				// Check if failed
				if contains(text, tackleui.StatusFailed, "error", "fail") {
					return fmt.Errorf("analysis failed: %s", text)
				}
			}
		}

		<-ticker.C
	}
}
