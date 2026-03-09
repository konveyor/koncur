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
//
// Examples:
//   - "konveyor.io/target=cloud-readiness || konveyor.io/target=linux" -> Included: ["konveyor.io/target=cloud-readiness", "konveyor.io/target=linux"]
//   - "!konveyor.io/target=windows" -> Excluded: ["konveyor.io/target=windows"]
//   - "konveyor.io/target=quarkus || !konveyor.io/source=java8" -> Included: ["konveyor.io/target=quarkus"], Excluded: ["konveyor.io/source=java8"]
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

	// Extract all label key-value pairs from the selector
	// Pattern matches: optional "!" + key + "=" + value
	// where key and value contain any characters except parentheses and logical operators
	re := regexp.MustCompile(`!?[^\)\(|&]+=[^\)\(|&]+`)
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
