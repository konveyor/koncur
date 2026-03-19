package targets

import (
	"regexp"
	"strings"
)

// ParseLabelSelector parses a label selector string into included and excluded labels.
// The label selector format supports:
// - OR operations with "||"
// - AND operations with "&&"
// - Negation with "!" prefix for exclusions
// - Key-value pairs in format "key=value"
// - Key-only labels for existence checks (e.g. "konveyor.io/target")
//
// Examples:
//   - "konveyor.io/target=cloud-readiness || konveyor.io/target=linux" -> Included: ["konveyor.io/target=cloud-readiness", "konveyor.io/target=linux"]
//   - "!konveyor.io/target=windows" -> Excluded: ["konveyor.io/target=windows"]
//   - "konveyor.io/target=quarkus || !konveyor.io/source=java8" -> Included: ["konveyor.io/target=quarkus"], Excluded: ["konveyor.io/source=java8"]
//   - "konveyor.io/target" -> Included: ["konveyor.io/target"]
func ParseLabelSelector(selector string) Labels {
	labels := Labels{
		Included: []string{},
		Excluded: []string{},
	}

	// Remove all whitespace from the selector
	selector = strings.ReplaceAll(selector, " ", "")

	if selector == "" {
		return labels
	}

	// Extract all label expressions from the selector.
	// Matches both key=value pairs and key-only labels (existence checks).
	// Pattern: optional "!" + one or more chars (not parens/operators),
	// optionally followed by "=" + value chars.
	re := regexp.MustCompile(`!?[^\)\(|&=]+(?:=[^\)\(|&]+)?`)
	matches := re.FindAllString(selector, -1)

	for _, match := range matches {
		// Check if it's an exclusion (starts with !)
		if strings.HasPrefix(match, "!") {
			// Remove the ! prefix and add to excluded
			excluded := strings.TrimPrefix(match, "!")
			labels.Excluded = append(labels.Excluded, excluded)
		} else {
			// Add to included
			labels.Included = append(labels.Included, match)
		}
	}

	return labels
}
