package browser

import "strings"

// IsBinaryFile checks if the application is a binary file
func IsBinaryFile(application string) bool {
	return strings.HasSuffix(application, ".jar") ||
		strings.HasSuffix(application, ".war") ||
		strings.HasSuffix(application, ".ear") ||
		strings.HasPrefix(application, "mvn://")
}

// contains checks if string contains any of the substrings (case-insensitive)
func contains(s string, substrs ...string) bool {
	s = strings.ToLower(s)
	for _, substr := range substrs {
		if strings.Contains(s, strings.ToLower(substr)) {
			return true
		}
	}
	return false
}
