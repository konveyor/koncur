package targets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/konveyor/tackle2-hub/shared/binding"
	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/targets/internal/browser"
	"github.com/konveyor/test-harness/pkg/targets/internal/hubapi"
	"github.com/konveyor/test-harness/pkg/util"
)

// TackleUITarget implements Target for Tackle UI automation
type TackleUITarget struct {
	url           string
	hubURL        string
	username      string
	password      string
	browser       string
	headless      bool
	mavenSettings string
}

// NewTackleUITarget creates a new Tackle UI automation target
func NewTackleUITarget(cfg *config.TackleUIConfig) (*TackleUITarget, error) {
	if cfg == nil {
		return nil, fmt.Errorf("tackle ui configuration is required")
	}

	browser := cfg.Browser
	if browser == "" {
		browser = "chrome"
	}

	// Default Hub URL to <URL>/hub if not specified
	hubURL := cfg.HubURL
	if hubURL == "" {
		hubURL = strings.TrimSuffix(cfg.URL, "/") + "/hub"
	}

	return &TackleUITarget{
		url:           cfg.URL,
		hubURL:        hubURL,
		username:      cfg.Username,
		password:      cfg.Password,
		browser:       browser,
		headless:      cfg.Headless,
		mavenSettings: cfg.MavenSettings,
	}, nil
}

// Name returns the target name
func (t *TackleUITarget) Name() string {
	return "tackle-ui"
}

// Execute runs analysis via Tackle UI browser automation
func (t *TackleUITarget) Execute(ctx context.Context, test *config.TestDefinition) (*ExecutionResult, error) {
	log := util.GetLogger()
	start := time.Now()

	// Prepare work directory
	workDir, err := PrepareWorkDir(test.GetWorkDir(), test.Name)
	if err != nil {
		return nil, err
	}

	log.Info("Executing Tackle UI analysis", "workDir", workDir, "url", t.url, "mavenSettings", t.mavenSettings)

	// Resolve application path if it's a local file
	// For binary files or source code paths, make them absolute
	testDir := test.GetTestDir()
	if test.Analysis.Application != "" {
		// Strip "binary:" prefix if present
		application := test.Analysis.Application
		if strings.HasPrefix(application, "binary:") {
			application = application[7:]
		}

		// Check if it's a local file (not a URL)
		if !strings.HasPrefix(application, "http://") &&
			!strings.HasPrefix(application, "https://") &&
			!strings.HasPrefix(application, "git://") &&
			!strings.HasPrefix(application, "mvn://") {

			// It's a local file - resolve to absolute path
			var absPath string
			if filepath.IsAbs(application) {
				absPath = application
			} else {
				// Relative path - resolve relative to test directory
				absPath = filepath.Join(testDir, application)
			}

			// Check if file exists
			if _, err := os.Stat(absPath); err != nil {
				return nil, fmt.Errorf("application file not found: %s", absPath)
			}

			// Update the test object with resolved path
			test.Analysis.Application = absPath
			log.Info("Resolved application path", "original", application, "absolute", absPath)
		}
	}

	// Resolve custom rules paths if they are local files
	for i, rule := range test.Analysis.Rules {
		// Check if it's a local file (not a URL)
		if !strings.HasPrefix(rule, "http://") &&
			!strings.HasPrefix(rule, "https://") &&
			!strings.HasPrefix(rule, "git://") &&
			!strings.HasPrefix(rule, "git@") {

			// It's a local file - resolve to absolute path
			var absPath string
			if filepath.IsAbs(rule) {
				absPath = rule
			} else {
				// Relative path - resolve relative to test directory
				absPath = filepath.Join(testDir, rule)
			}

			// Check if file exists
			if _, err := os.Stat(absPath); err != nil {
				return nil, fmt.Errorf("rule file not found: %s", absPath)
			}

			// Update the rules array with resolved path
			test.Analysis.Rules[i] = absPath
			log.Info("Resolved rule path", "original", rule, "absolute", absPath)
		}
	}

	// Step 1: Initialize browser
	browserHelper, err := browser.New(t.browser, t.headless, log)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize browser: %w", err)
	}
	defer browserHelper.Close()

	// Step 2: Navigate to Tackle UI
	err = browserHelper.NavigateToURL(t.url)
	if err != nil {
		return nil, fmt.Errorf("failed to navigate to Tackle UI: %w", err)
	}

	// Step 3: Login (if credentials provided)
	// Note: Current implementation assumes auth is disabled
	// TODO: Implement login flow for auth-enabled instances
	if t.username != "" && t.password != "" {
		log.Info("Login not yet implemented, assuming auth is disabled")
	}

	// Step 4: Navigate to applications page
	err = browserHelper.NavigateToApplications()
	if err != nil {
		return nil, fmt.Errorf("failed to navigate to applications: %w", err)
	}

	// Step 5: Create shared Maven identity if test requires it
	var mavenIdentityName string
	if test.RequireMavenSettings {
		if t.mavenSettings == "" {
			return nil, fmt.Errorf("test requires Maven settings but none configured in target config")
		}

		// Use a shared identity name for all tests using the same settings file
		// This avoids creating duplicate identities for multiple tests
		mavenIdentityName = "koncur-maven-settings"
		log.Info("Creating/reusing shared Maven identity", "name", mavenIdentityName)
		err = browserHelper.CreateMavenIdentity(t.url, mavenIdentityName, t.mavenSettings)
		if err != nil {
			return nil, fmt.Errorf("failed to create Maven identity: %w", err)
		}

		// Navigate back to Applications after creating identity
		// Use direct URL navigation since we're in the Administration perspective
		err = browserHelper.NavigateToApplications(t.url)
		if err != nil {
			return nil, fmt.Errorf("failed to navigate back to applications: %w", err)
		}
	}

	// Step 6: Create application
	log.Info("Creating application", "name", test.Name)
	err = browserHelper.CreateApplication(test)
	if err != nil {
		// Cleanup: try to delete the application if creation fails partway through
		log.V(1).Info("Application creation failed, attempting cleanup", "error", err)
		if cleanupErr := browserHelper.DeleteApplication(test.Name); cleanupErr != nil {
			log.V(1).Info("Failed to cleanup application after creation error", "cleanupError", cleanupErr)
		}
		return nil, fmt.Errorf("failed to create application: %w", err)
	}

	// Step 7: Associate Maven identity with application if needed
	if mavenIdentityName != "" {
		log.Info("Associating Maven identity with application", "app", test.Name, "identity", mavenIdentityName)
		err = browserHelper.AssociateMavenIdentityWithApplication(t.url, test.Name, mavenIdentityName)
		if err != nil {
			return nil, fmt.Errorf("failed to associate Maven identity: %w", err)
		}
	}

	// Step 8: Start analysis
	err = browserHelper.StartAnalysis(test)
	if err != nil {
		return nil, fmt.Errorf("failed to start analysis: %w", err)
	}

	// Step 9: Wait for analysis to complete
	err = browserHelper.WaitForAnalysisComplete(test.GetTimeout())
	if err != nil {
		return nil, fmt.Errorf("analysis did not complete: %w", err)
	}

	// Step 10: Navigate to results to extract the application ID
	err = browserHelper.NavigateToResults(t.url, test.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to navigate to results: %w", err)
	}

	// Step 11: Extract application ID from the URL
	appID, err := browserHelper.GetApplicationIDFromURL()
	if err != nil {
		return nil, fmt.Errorf("failed to get application ID: %w", err)
	}
	log.Info("Extracted application ID", "appID", appID)

	// Step 12: Initialize Hub API client
	log.Info("Initializing Hub API client", "hubURL", t.hubURL)
	hubClient := binding.New(t.hubURL)
	// Set authentication if credentials provided
	if t.username != "" && t.password != "" {
		err := hubClient.Login(t.username, t.password)
		if err != nil {
			return nil, fmt.Errorf("failed to login to Hub API: %w", err)
		}
	}

	// Step 13: Get the latest task ID for this application
	// We need to find the analysis task that just completed
	tasks, err := hubClient.Task.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	var taskID uint
	for _, task := range tasks {
		// Find the most recent analyzer task for this application
		if task.Application != nil && task.Application.ID == appID && task.Kind == "analyzer" {
			taskID = task.ID
			break // Tasks are typically sorted by most recent first
		}
	}

	if taskID == 0 {
		return nil, fmt.Errorf("could not find analysis task for application ID %d", appID)
	}
	log.Info("Found analysis task", "taskID", taskID)

	// Step 14: Fetch analysis results from Hub API
	log.Info("Fetching analysis results from Hub API")
	output, err := hubapi.FetchAnalysisResults(hubClient, appID, taskID, test.Analysis.DisableDefaultRules)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch analysis results: %w", err)
	}

	outputDir := filepath.Join(workDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	outputFile := filepath.Join(outputDir, "output.yaml")
	if err := os.WriteFile(outputFile, output, 0644); err != nil {
		return nil, fmt.Errorf("failed to write output file: %w", err)
	}

	log.Info("Successfully wrote analysis results", "file", outputFile)

	duration := time.Since(start)
	result := &ExecutionResult{
		ExitCode:   0,
		Duration:   duration,
		OutputFile: outputFile,
		WorkDir:    workDir,
	}

	return result, nil
}
