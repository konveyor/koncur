package browser

import (
	"fmt"
	"strings"

	"github.com/playwright-community/playwright-go"

	"github.com/konveyor/test-harness/pkg/config"
)

// SelectTargets selects migration targets in the analysis wizard
func (h *Helper) SelectTargets(test *config.TestDefinition) error {
	// Determine which targets to select
	var targetLabels []string
	var selectAllTargets bool

	// First, check if targets are explicitly specified in the test
	if len(test.Analysis.Target) > 0 {
		targetLabels = test.Analysis.Target
	} else if test.Analysis.LabelSelector != "" {
		// Check if labelSelector is just "konveyor.io/target" without a specific value
		// This means select ALL available targets
		if test.Analysis.LabelSelector == "konveyor.io/target" {
			h.log.Info("Label selector is 'konveyor.io/target' without value - selecting ALL targets")
			selectAllTargets = true
		} else {
			// Extract specific targets from label selector
			targetLabels = extractTargetsFromLabelSelector(test.Analysis.LabelSelector)
			h.log.Info("Extracted targets from label selector", "labelSelector", test.Analysis.LabelSelector, "targets", targetLabels)
		}
	}

	if !selectAllTargets && len(targetLabels) == 0 {
		h.log.V(1).Info("No targets specified, skipping target selection")
		return nil
	}

	// If selectAllTargets is true, we need to find all target cards on the page and select them
	if selectAllTargets {
		return h.selectAllTargetCards()
	}

	// Map label values to target card names
	// This mapping comes from tackle2-seed/resources/targets.yaml
	labelToCardName := map[string]string{
		"cloud-readiness":  "Containerization",
		"quarkus":          "Quarkus",
		"linux":            "Linux",
		"eap8":             "Application server migration to",
		"eap7":             "Application server migration to",
		"spring6":          "Spring Framework",
		"spring-boot3":     "Spring Boot",
		"openjdk11":        "OpenJDK",
		"openjdk17":        "OpenJDK",
		"openjdk21":        "OpenJDK",
		"openjdk":          "OracleJDK to OpenJDK",
		"jakarta-ee":       "Jakarta EE 9",
		"jws6":             "JBoss Web Server 6",
		"openliberty":      "Open Liberty",
		"camel3":           "Camel",
		"camel4":           "Camel",
		"azure-appservice": "Azure",
		"azure-aks":        "Azure",
	}

	h.log.Info("Selecting targets", "labels", targetLabels)

	// For each target label, find the corresponding card and click it
	for _, targetLabel := range targetLabels {
		cardName, exists := labelToCardName[targetLabel]
		if !exists {
			// If no mapping exists, try using the label as the card name
			cardName = targetLabel
			h.log.V(1).Info("No mapping for target label, using label as card name", "label", targetLabel)
		}

		h.log.V(1).Info("Looking for target card", "label", targetLabel, "cardName", cardName)

		// Target cards have a checkbox that needs to be checked
		// Use Check() which will only check if unchecked (won't toggle)
		checkboxSelector := fmt.Sprintf("#target-%s-select", cardName)

		h.log.Info("Checking target checkbox", "label", targetLabel, "cardName", cardName, "selector", checkboxSelector)
		err := h.page.Check(checkboxSelector, playwright.PageCheckOptions{
			Timeout: playwright.Float(5000),
			Force:   playwright.Bool(true), // Force check even if not visible (checkbox might be styled/hidden)
		})

		var selected bool
		if err == nil {
			h.log.Info("Successfully checked target checkbox", "label", targetLabel, "cardName", cardName)
			selected = true
		} else {
			h.log.Info("Failed to check checkbox, trying to click card div", "label", targetLabel, "error", err)
			// Fallback: try clicking the card div
			cardDivSelector := fmt.Sprintf("#target-%s", cardName)
			clickErr := h.page.Click(cardDivSelector, playwright.PageClickOptions{
				Timeout: playwright.Float(2000),
			})
			if clickErr == nil {
				h.log.Info("Clicked target card div", "label", targetLabel, "cardName", cardName)
				selected = true
			}
		}

		if selected {
			// Wait for the card to get the pf-m-selected class
			// The card div with data-target-name should have pf-m-selected class when selected
			cardSelectedSelector := fmt.Sprintf("#target-%s.pf-m-selected", cardName)
			_, waitErr := h.page.WaitForSelector(cardSelectedSelector, playwright.PageWaitForSelectorOptions{
				State:   playwright.WaitForSelectorStateAttached,
				Timeout: playwright.Float(2000),
			})
			if waitErr == nil {
				h.log.Info("Verified target card has pf-m-selected class", "label", targetLabel, "cardName", cardName)
			} else {
				h.log.V(1).Info("Could not verify pf-m-selected class but card was checked/clicked", "label", targetLabel, "cardName", cardName)
			}
		} else {
			h.log.V(1).Info("Could not select target card", "label", targetLabel, "cardName", cardName)
		}
	}

	return nil
}

// extractTargetsFromLabelSelector extracts target values from a label selector
// Example: "konveyor.io/target=cloud-readiness || konveyor.io/target=linux" -> ["cloud-readiness", "linux"]
func extractTargetsFromLabelSelector(selector string) []string {
	targets := []string{}

	// Split by OR operator
	parts := strings.Split(selector, "||")

	for _, part := range parts {
		part = strings.TrimSpace(part)

		// Skip empty parts and exclusions (starting with !)
		if part == "" || strings.HasPrefix(part, "!") {
			continue
		}

		// Check if this is a target label (konveyor.io/target=value)
		if strings.Contains(part, "konveyor.io/target=") {
			// Extract the value after the =
			parts := strings.SplitN(part, "=", 2)
			if len(parts) == 2 {
				target := strings.TrimSpace(parts[1])
				if target != "" {
					targets = append(targets, target)
				}
			}
		}
	}

	return targets
}

// selectAllTargetCards selects all available target cards in the wizard
func (h *Helper) selectAllTargetCards() error {
	h.log.Info("Selecting all available target cards")

	// Find all target card headings (h4 elements within target cards)
	// Based on the UI structure, each target card has an h4 heading that we click to select
	cardSelectors := []string{
		"h4[data-target]",                  // If cards have data-target attribute
		".pf-v5-c-card h4",                 // Target cards with h4 headings
		"[role='heading'][aria-level='4']", // h4 elements by role
	}

	// Try to find all h4 headings
	for _, selector := range cardSelectors {
		h.log.V(1).Info("Trying selector for target cards", "selector", selector)
		count, err := h.page.Locator(selector).Count()
		h.log.Info("Selector result", "selector", selector, "count", count, "error", err)
		if err == nil && count > 0 {
			h.log.Info("Found target cards", "selector", selector, "count", count)

			// Get all the cards
			successCount := 0
			failCount := 0
			for i := 0; i < count; i++ {
				locator := h.page.Locator(selector).Nth(i)

				// Get the text first for better logging
				text, _ := locator.TextContent()
				h.log.V(1).Info("Attempting to click target card", "index", i, "text", text)

				// Wait for the element to be visible and stable
				err := locator.WaitFor(playwright.LocatorWaitForOptions{
					State:   playwright.WaitForSelectorStateVisible,
					Timeout: playwright.Float(3000),
				})
				if err != nil {
					h.log.Info("Target card not visible", "index", i, "text", text, "error", err)
					failCount++
					continue
				}

				// Try to click this card with force option in case it's partially obscured
				err = locator.Click(playwright.LocatorClickOptions{
					Timeout: playwright.Float(5000),
					Force:   playwright.Bool(true), // Force click even if element is obscured
				})
				if err != nil {
					h.log.Info("Failed to click target card", "index", i, "text", text, "error", err)
					failCount++
					continue
				}

				h.log.Info("Selected target card", "index", i, "text", text)
				successCount++
			}

			h.log.Info("Target card selection complete", "total", count, "selected", successCount, "failed", failCount)

			if successCount == 0 {
				h.log.Info("No target cards were successfully selected - all clicks failed")
				// Don't return error, continue to fallback strategies
			} else {
				return nil
			}
		}
	}

	// If we couldn't find cards with the above selectors, try a more general approach
	h.log.Info("Could not find target cards with specific selectors, trying general approaches")

	// Strategy 1: Look for any h4 elements on the page
	h.log.V(1).Info("Trying to find h4 elements")
	generalLocator := h.page.Locator("h4")
	count, err := generalLocator.Count()
	h.log.Info("Found h4 elements", "count", count, "error", err)

	if err == nil && count > 0 {
		h.log.Info("Found h4 elements, attempting to select as target cards", "count", count)
		selectedCount := 0
		for i := 0; i < count; i++ {
			locator := generalLocator.Nth(i)
			text, _ := locator.TextContent()

			// Skip if the text doesn't look like a target name
			if text == "" || len(text) > 100 {
				h.log.V(1).Info("Skipping h4 element", "index", i, "text", text, "reason", "empty or too long")
				continue
			}

			err := locator.Click()
			if err != nil {
				h.log.V(1).Info("Failed to click h4 element", "index", i, "text", text, "error", err)
				continue
			}

			h.log.Info("Selected target card", "index", i, "text", text)
			selectedCount++
		}

		if selectedCount > 0 {
			h.log.Info("Selected target cards via h4 elements", "count", selectedCount)
			return nil
		}
	}

	// Strategy 2: Look for clickable cards with specific class or structure
	h.log.Info("Trying to find clickable cards")
	cardLocators := []string{
		"div.pf-v5-c-card[role='button']",
		"div.pf-v5-c-card.pf-m-selectable",
		"button[data-ouia-component-type='Card']",
	}

	for _, cardLoc := range cardLocators {
		h.log.V(1).Info("Trying card selector", "selector", cardLoc)
		count, err := h.page.Locator(cardLoc).Count()
		h.log.Info("Card selector result", "selector", cardLoc, "count", count, "error", err)

		if err == nil && count > 0 {
			h.log.Info("Found clickable cards, selecting all", "selector", cardLoc, "count", count)
			for i := 0; i < count; i++ {
				locator := h.page.Locator(cardLoc).Nth(i)
				err := locator.Click()
				if err != nil {
					h.log.V(1).Info("Failed to click card", "index", i, "error", err)
					continue
				}

				text, _ := locator.TextContent()
				h.log.Info("Selected card", "index", i, "text", text)
			}
			return nil
		}
	}

	h.log.Info("No target cards found using any strategy - step may not be visible or already configured")
	return nil
}
