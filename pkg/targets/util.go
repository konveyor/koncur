package targets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/util"
)

// IsBinaryFile returns true if the path appears to be a binary artifact (.jar, .war, or .ear)
func IsBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jar" || ext == ".war" || ext == ".ear"
}

// CloneGitRepository clones a Git repository and returns the path to the cloned directory
// or subdirectory if specified in the GitURLComponents
func CloneGitRepository(ctx context.Context, components *config.GitURLComponents, workDir string, cloneName string) (string, error) {
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

	// Clean .git directory contents to save space, but keep the empty directory.
	// The builtin provider's file search uses ".git" as an excluded directory pattern.
	// If the directory exists, it's matched via stat (correct behavior: exclude files inside .git/).
	// If removed entirely, the pattern falls through to regexp.Compile(".git") where "."
	// matches any character, causing ".git" to match "github" in absolute paths and
	// incorrectly excluding ALL files from analysis.
	gitDir := filepath.Join(absCloneDir, ".git")
	entries, err := os.ReadDir(gitDir)
	if err == nil {
		for _, entry := range entries {
			os.RemoveAll(filepath.Join(gitDir, entry.Name()))
		}
		log.Info("Cleaned .git directory contents", "path", gitDir)
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
