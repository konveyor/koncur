package targets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/konveyor/test-harness/pkg/util"
)

// IsBinaryFile returns true if the path appears to be a binary artifact (.jar, .war, or .ear)
func IsBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jar" || ext == ".war" || ext == ".ear"
}

// GitURLComponents represents a parsed Git URL with optional branch/tag and path
type GitURLComponents struct {
	URL    string // Base Git URL
	Ref    string // Branch or tag (optional)
	Path   string // Path within repository (optional)
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

// CloneGitRepository clones a Git repository and returns the path to the cloned directory
// or subdirectory if specified in the GitURLComponents
func CloneGitRepository(ctx context.Context, components *GitURLComponents, workDir string, cloneName string) (string, error) {
	log := util.GetLogger()

	// Clone the git repository into workDir/cloneName folder
	cloneDir := filepath.Join(workDir, cloneName)

	// Get absolute path for clone directory
	absCloneDir, err := filepath.Abs(cloneDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Determine the final input directory (may be a subdirectory if path is specified)
	var absInputDir string
	if components.Path != "" {
		absInputDir = filepath.Join(absCloneDir, components.Path)
	} else {
		absInputDir = absCloneDir
	}

	// Check if directory already exists
	if _, err := os.Stat(absInputDir); err == nil {
		log.Info("Repository already exists, skipping clone", "dest", absInputDir)
		return absInputDir, nil
	}

	log.Info("Cloning git repository", "url", components.URL, "ref", components.Ref, "path", components.Path, "dest", absCloneDir)

	// Build git clone command
	var gitArgs []string
	if components.Ref != "" {
		gitArgs = []string{"clone", "--depth", "1", "--branch", components.Ref, components.URL, absCloneDir}
	} else {
		gitArgs = []string{"clone", "--depth", "1", components.URL, absCloneDir}
	}

	// Execute git clone
	result, err := ExecuteCommand(ctx, "git", gitArgs, ".", 5*60*1000000000) // 5 minute timeout for clone
	if err != nil {
		log.Info("Git clone failed", "error", err.Error(), "exitCode", result.ExitCode, "stderr", result.Stderr)
		return "", fmt.Errorf("git clone failed: %w", err)
	}

	log.Info("Git clone completed successfully")

	// Remove .git directory to save space and avoid git-related issues
	gitDir := filepath.Join(absCloneDir, ".git")
	if err := os.RemoveAll(gitDir); err != nil {
		log.Info("Warning: failed to remove .git directory", "error", err.Error())
		// Don't fail the entire operation if we can't remove .git
	} else {
		log.Info("Removed .git directory", "path", gitDir)
	}

	// Verify the target path exists if specified
	if components.Path != "" {
		if _, err := os.Stat(absInputDir); err != nil {
			return "", fmt.Errorf("specified path does not exist in repository: %s: %w", components.Path, err)
		}
		log.Info("Using subdirectory from repository", "path", components.Path, "fullPath", absInputDir)
	}

	return absInputDir, nil
}
