package parser

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"go.lsp.dev/uri"
	"gopkg.in/yaml.v3"
)

// javaBinPattern matches ephemeral java-bin temp directories in URIs.
var javaBinPattern = regexp.MustCompile(`.*/java-bin-\d+/`)

// ParseOutput reads and parses the analyzer output.yaml file
func ParseOutput(outputFile string) ([]konveyor.RuleSet, error) {
	data, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read output file %s: %w", outputFile, err)
	}

	var rulesets []konveyor.RuleSet
	if err := yaml.Unmarshal(data, &rulesets); err != nil {
		return nil, fmt.Errorf("failed to parse output YAML: %w", err)
	}

	return rulesets, nil
}

// FilterRuleSets filters out rulesets that don't have violations, insights, or tags
// This is used to normalize output for comparison, removing empty rulesets
func FilterRuleSets(rulesets []konveyor.RuleSet) []konveyor.RuleSet {
	var filtered []konveyor.RuleSet
	for _, rs := range rulesets {
		// Keep rulesets that have violations, insights, or tags
		if len(rs.Violations) > 0 || len(rs.Insights) > 0 || len(rs.Tags) > 0 {
			filtered = append(filtered, rs)
		}
	}
	return filtered
}

// NormalizeRuleSets normalizes rulesets for comparison by removing dynamic content.
// workDir is optional; if provided, it will be stripped from incident URIs (used by kai-rpc
// which runs locally and produces absolute paths instead of container-relative /source/ paths).
func NormalizeRuleSets(rulesets []konveyor.RuleSet, testDir string, workDir ...string) ([]konveyor.RuleSet, error) {
	wd := ""
	if len(workDir) > 0 && workDir[0] != "" {
		if abs, err := filepath.Abs(workDir[0]); err == nil {
			wd = filepath.ToSlash(abs)
		}
	}
	normalizedRuleSets := []konveyor.RuleSet{}
	var returnError error
	for _, rs := range rulesets {
		newRuleSet := konveyor.RuleSet{
			Name:        rs.Name,
			Description: rs.Description,
			Tags:        rs.Tags,
			Violations:  map[string]konveyor.Violation{},
			Insights:    map[string]konveyor.Violation{},
			Errors:      rs.Errors,
			Unmatched:   rs.Unmatched,
			Skipped:     rs.Skipped,
		}
		for k, violation := range rs.Violations {
			newViolation, err := normalizeViolation(violation, testDir, wd)
			// Skip this for now
			if err != nil {
				returnError = errors.Join(returnError, err)
				continue
			}
			newRuleSet.Violations[k] = newViolation
		}
		for k, insight := range rs.Insights {
			newInsight, err := normalizeViolation(insight, testDir, wd)
			// Skip this for now
			if err != nil {
				continue
			}
			newRuleSet.Insights[k] = newInsight
		}
		normalizedRuleSets = append(normalizedRuleSets, newRuleSet)
	}
	return normalizedRuleSets, returnError
}

func normalizeViolation(violation konveyor.Violation, testDir, workDir string) (konveyor.Violation, error) {
	newViolation := konveyor.Violation{
		Description: violation.Description,
		Category:    violation.Category,
		Labels:      violation.Labels,
		Incidents:   []konveyor.Incident{},
		Links:       violation.Links,
		Extras:      violation.Extras,
		Effort:      violation.Effort,
	}

	var returnErr error
	for _, inc := range violation.Incidents {
		inc, err := normalizeIncident(inc, testDir, workDir)
		if err != nil {
			returnErr = errors.Join(returnErr, err)
		}
		newViolation.Incidents = append(newViolation.Incidents, inc)
	}
	return newViolation, returnErr
}

// NormalizePath converts container-specific paths to canonical format for comparison.
// This handles:
//   - Maven repository paths: /root/.m2/repository/, /cache/m2/, /addon/.m2/repository/ -> /m2/
//   - Tackle Hub paths: /shared/source/{reponame}/ -> /source/, /opt/input/source -> /source
func NormalizePath(path string) string {
	// Normalize Maven repository paths
	if strings.Contains(path, "/root/.m2/repository") {
		path = strings.ReplaceAll(path, "/root/.m2/repository/", "/m2/")
	}
	if strings.Contains(path, "/cache/m2/") {
		path = strings.ReplaceAll(path, "/cache/m2/", "/m2/")
	}
	// Providers should all be running in the addon dir now
	if strings.Contains(path, "/addon/.m2/repository") {
		path = strings.ReplaceAll(path, "/addon/.m2/repository/", "/m2/")
	}

	// Normalize Tackle Hub container paths
	// /shared/source/path -> /source/path
	if strings.Contains(path, "/shared/source/") {
		path = strings.ReplaceAll(path, "/shared/source/", "/source/")
	}
	if strings.Contains(path, "/opt/input/source") {
		path = strings.ReplaceAll(path, "/opt/input/source", "/source")
	}

	return path
}

// normalizeIncident normalizes file paths in incidents to match the expected output format
// This applies the same normalization that saveFilteredOutput does when generating expected output
func normalizeIncident(incident konveyor.Incident, testDir, workDir string) (konveyor.Incident, error) {
	// Marshal to YAML to normalize paths using string replacement (same approach as generate)
	// Normalize paths by removing the test directory path
	if incident.URI == "" || !strings.Contains(string(incident.URI), "file://") {
		return incident, nil
	}
	newIncident := konveyor.Incident{
		URI:        incident.URI,
		Message:    incident.Message,
		CodeSnip:   incident.CodeSnip,
		LineNumber: incident.LineNumber,
		Variables:  incident.Variables,
	}
	// For windows, we need to normailze to slash
	fileName := string(incident.URI)
	getFilePath := strings.TrimPrefix(fileName, "file://")
	if strings.Contains(getFilePath, `/\`) {
		getFilePath = fmt.Sprintf("\\%v", strings.TrimLeft(getFilePath, `/\`))
	}
	toSlashFilePaths := filepath.ToSlash(getFilePath)

	fileName = toSlashFilePaths
	if testDir != "" {
		normalizedTestDir := filepath.ToSlash(testDir)
		fileName = strings.ReplaceAll(fileName, normalizedTestDir, "")
	}

	// Strip workDir from URIs (used by kai-rpc which runs locally with absolute paths).
	// workDir is pre-resolved to absolute and slash-normalized by NormalizeRuleSets.
	if workDir != "" {
		fileName = strings.ReplaceAll(fileName, workDir, "")
	}

	if strings.HasPrefix(fileName, "//") {
		fileName = strings.Replace(fileName, "//", "/", 1)
		fileName = fmt.Sprintf("/%s", strings.TrimLeft(fileName, "/"))
	}

	fileName = fmt.Sprintf("file://%s", fileName)

	// Apply shared path normalization (maven, tackle hub paths)
	fileName = NormalizePath(fileName)

	// Normalize ephemeral java-bin paths (containers, temp dirs) to /source/
	// This handles macOS (/var/folders/.../T/), Linux (/tmp/), and container storage
	fileName = javaBinPattern.ReplaceAllString(fileName, "file:///source/")

	if fileName == "" {
		return newIncident, fmt.Errorf("fileName went to empty: %s", incident.URI)
	}
	newIncident.URI = uri.URI(fileName)
	return newIncident, nil
}
