package config

import (
	"strings"
)

// GitURLComponents represents a parsed Git URL with optional branch/tag and path
type GitURLComponents struct {
	URL  string // Base Git URL
	Ref  string // Branch or tag (optional)
	Path string // Path within repository (optional)
}

// ParseGitURLWithPath parses a Git URL that may contain a reference and path
// Format: https://github.com/org/repo#branch/path/to/dir
func ParseGitURLWithPath(gitURL string) *GitURLComponents {
	components := &GitURLComponents{}

	// Check if it contains a reference marker
	if strings.Contains(gitURL, "#") {
		parts := strings.SplitN(gitURL, "#", 2)
		components.URL = parts[0]

		if len(parts) > 1 {
			// Split the reference on "/" to separate branch from path
			refParts := strings.SplitN(parts[1], "/", 2)
			components.Ref = refParts[0]
			if len(refParts) > 1 {
				components.Path = refParts[1]
			}
		}
	} else {
		components.URL = gitURL
	}

	return components
}

// IsGitURL checks if the given string is a Git URL
func IsGitURL(str string) bool {
	return strings.HasPrefix(str, "http://") ||
		strings.HasPrefix(str, "https://") ||
		strings.HasPrefix(str, "git@") ||
		strings.Contains(str, "#")
}