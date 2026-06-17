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

// PrepareRules handles rules that may be Git URLs or local paths.
// Returns a list of prepared rule paths ready for use by any target.
func PrepareRules(ctx context.Context, rules []config.CustomRule, testDir, workDir string) ([]string, error) {
	if len(rules) == 0 {
		return nil, nil
	}

	log := util.GetLogger()
	preparedRules := make([]string, 0, len(rules))

	for i, rule := range rules {
		if rule.File != nil {
			if filepath.IsAbs(rule.File.FilePath) {
				preparedRules = append(preparedRules, rule.File.FilePath)
			} else {
				preparedRules = append(preparedRules, filepath.Join(testDir, rule.File.FilePath))
			}
		} else if rule.Git != nil {
			components := rule.Git.GetCompents()
			log.Info("Cloning rules repository", "rule", rule.Git.GitRepo)
			cloneName := fmt.Sprintf("rules-%d", i)
			clonedPath, err := CloneGitRepository(ctx, components, workDir, cloneName)
			if err != nil {
				return nil, fmt.Errorf("failed to clone rules repository %s: %w", rule.Git.GitRepo, err)
			}
			preparedRules = append(preparedRules, clonedPath)
		}
	}

	return preparedRules, nil
}

// IsBinaryFile returns true if the path appears to be a binary artifact (.jar, .war, or .ear)
func IsBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jar" || ext == ".war" || ext == ".ear"
}

// IsMavenCoordinate returns true if the path is a Maven coordinate (mvn://)
func IsMavenCoordinate(path string) bool {
	return strings.HasPrefix(path, "mvn://")
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

	// Keep .git as an empty directory. The builtin provider uses ".git" as an
	// exclusion pattern; if the directory is missing, it falls back to a regex
	// where "." is a wildcard, matching "github" in paths and excluding all files.
	gitDir := filepath.Join(absCloneDir, ".git")
	if err := os.RemoveAll(gitDir); err != nil {
		return "", fmt.Errorf("failed to remove .git directory: %w", err)
	}
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		return "", fmt.Errorf("failed to recreate .git directory: %w", err)
	}
	log.Info("Cleaned .git directory contents", "path", gitDir)

	// Verify the target path exists if specified
	if components.Path != "" {
		if _, err := os.Stat(absInputDir); err != nil {
			return "", fmt.Errorf("specified path does not exist in repository: %s: %w", components.Path, err)
		}
		log.Info("Using subdirectory from repository", "path", components.Path, "fullPath", absInputDir)
	}

	return absInputDir, nil
}
