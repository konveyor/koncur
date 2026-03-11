package targets

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"github.com/konveyor/tackle2-hub/shared/api"
	"github.com/konveyor/tackle2-hub/shared/binding"
	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/parser"
	"github.com/konveyor/test-harness/pkg/util"
	"go.lsp.dev/uri"
	"gopkg.in/yaml.v2"
)

const (
	// TaskStateCreated indicates task has been created
	TaskStateCreated = "Created"
	// TaskStateReady indicates task is ready to run
	TaskStateReady = "Ready"
	// TaskStatePending indicates task is pending
	TaskStatePending = "Pending"
	// TaskStatePostponed indicates task is postponed
	TaskStatePostponed = "Postponed"
	// TaskStateRunning indicates task is running
	TaskStateRunning = "Running"
	// TaskStateSucceeded indicates task completed successfully
	TaskStateSucceeded = "Succeeded"
	// TaskStateFailed indicates task failed
	TaskStateFailed = "Failed"
)

type Data struct {
	// Verbosity level.
	Verbosity int `json:"verbosity"`
	// Mode options.
	Mode Mode `json:"mode"`
	// Scope options.
	Scope Scope `json:"scope"`
	// Rules options.
	Rules Rules `json:"rules"`
	// Tagger options.
	Tagger Tagger `json:"tagger"`
}

type Mode struct {
	Discovery bool   `json:"discovery"`
	Binary    bool   `json:"binary"`
	Artifact  string `json:"artifact"`
	WithDeps  bool   `json:"withDeps"`
	//
	path struct {
		appDir string
		binary string
	}
}
type Scope struct {
	WithKnownLibs bool `json:"withKnownLibs"`
	Packages      struct {
		Included []string `json:"included,omitempty"`
		Excluded []string `json:"excluded,omitempty"`
	} `json:"packages"`
}
type Rules struct {
	Path         string          `json:"path"`
	Repository   *api.Repository `json:"repository"`
	Identity     *api.Ref        `json:"identity"`
	Labels       Labels          `json:"labels"`
	RuleSets     []api.Ref       `json:"ruleSets"`
	repositories []string
	rules        []string
}
type Labels struct {
	Included []string `json:"included,omitempty"`
	Excluded []string `json:"excluded,omitempty"`
}
type Tagger struct {
	Enabled bool   `json:"enabled"`
	Source  string `json:"source"`
}

// TackleHubTarget implements Target for Tackle Hub API
type TackleHubTarget struct {
	url           string
	client        *binding.RichClient
	mavenSettings string
}

// NewTackleHubTarget creates a new Tackle Hub API target
func NewTackleHubTarget(cfg *config.TackleHubConfig) (*TackleHubTarget, error) {
	if cfg == nil {
		return nil, fmt.Errorf("tackle hub configuration is required")
	}

	// Apply image overrides to the Tackle CR if configured
	if cfg.HasImageOverrides() {
		if err := applyImageOverrides(context.Background(), cfg); err != nil {
			return nil, fmt.Errorf("failed to apply image overrides: %w", err)
		}
	}

	client := binding.New(cfg.URL)

	// Set authentication if provided (optional for instances with auth disabled)
	if cfg.Token != "" {
		client.Client.Use(api.Login{Token: cfg.Token})
	} else if cfg.Username != "" && cfg.Password != "" {
		err := client.Login(cfg.Username, cfg.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to login: %w", err)
		}
	}
	// If no credentials provided, assume auth is disabled on the Tackle instance

	return &TackleHubTarget{
		url:           cfg.URL,
		client:        client,
		mavenSettings: cfg.MavenSettings,
	}, nil
}

// Name returns the target name
func (t *TackleHubTarget) Name() string {
	return "tackle-hub"
}

// Execute runs analysis via Tackle Hub API
func (t *TackleHubTarget) Execute(ctx context.Context, test *config.TestDefinition) (*ExecutionResult, error) {
	log := util.GetLogger()
	start := time.Now()

	// Validate maven settings requirement
	if test.RequireMavenSettings && t.mavenSettings == "" {
		return nil, fmt.Errorf("test requires maven settings but none configured in target config")
	}

	// Prepare work directory
	workDir, err := PrepareWorkDir(test.GetWorkDir(), test.Name)
	if err != nil {
		return nil, err
	}

	log.Info("Executing Tackle Hub analysis", "workDir", workDir)

	// Step 1: Create or find application
	log.Info("Creating application", "name", test.Name)
	app, err := t.createApplication(test)
	if err != nil {
		return nil, fmt.Errorf("failed to create application: %w", err)
	}
	log.Info("Application created", "id", app.ID, "name", app.Name)

	// Step 2: Create analysis task
	log.Info("Creating analysis task", "applicationID", app.ID)
	task, err := t.createAnalysisTask(ctx, test, app)
	if err != nil {
		return nil, fmt.Errorf("failed to create analysis task: %w", err)
	}
	log.Info("Analysis task created", "taskID", task.ID)

	// Step 2.5: Submit the task to move it to Ready state
	log.Info("Submitting task", "taskID", task.ID)
	err = t.submitTask(task.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to submit task: %w", err)
	}
	log.Info("Task submitted", "taskID", task.ID)

	// Step 3: Poll for task completion
	log.Info("Polling for task completion", "taskID", task.ID)
	err = t.pollTaskCompletion(ctx, task.ID, test.GetTimeout())
	if err != nil {
		return nil, fmt.Errorf("task failed or timed out: %w", err)
	}
	log.Info("Analysis task completed successfully", "taskID", task.ID)

	// Use the binding's Analysis methods to fetch insights
	insights, err := t.client.Application.Select(app.ID).Analysis.ListInsights()
	if err != nil {
		return nil, fmt.Errorf("failed to get analysis insights: %w", err)
	}
	log.Info("Retrieved analysis insights", "count", len(insights))

	rulesetToInsightConverted := map[string]konveyor.RuleSet{}
	for _, insight := range insights {
		rs := rulesetToInsightConverted[insight.RuleSet]
		rs.Name = insight.RuleSet
		if rs.Insights == nil {
			rs.Insights = map[string]konveyor.Violation{}
		}
		if rs.Violations == nil {
			rs.Violations = map[string]konveyor.Violation{}
		}
		incidents := []konveyor.Incident{}
		for _, i := range insight.Incidents {
			// Normalize paths to match expected output format
			i.File = parser.NormalizePath(i.File)
			// Handle empty file paths (summary insights without specific file)
			var fileURI uri.URI
			if i.File == "" {
				fileURI = ""
			} else {
				fileURI = uri.File(i.File)
			}
			incidents = append(incidents, konveyor.Incident{
				URI:        fileURI,
				Message:    i.Message,
				CodeSnip:   i.CodeSnip,
				LineNumber: &i.Line,
			})
		}
		links := []konveyor.Link{}
		for _, l := range insight.Links {
			links = append(links, konveyor.Link{
				URL:   l.URL,
				Title: l.Title,
			})
		}

		v := konveyor.Violation{
			Description: insight.Description,
			Category:    (*konveyor.Category)(&insight.Category),
			Labels:      insight.Labels,
			Incidents:   incidents,
			Links:       links,
			Effort:      &insight.Effort,
		}

		if insight.Effort == 0 {
			rs.Insights[insight.Rule] = v
		} else {
			rs.Violations[insight.Rule] = v
		}
		rulesetToInsightConverted[insight.RuleSet] = rs
	}
	// Get tags from application
	appTag := t.client.Application.Tags(app.ID)
	tags, err := appTag.List()
	if err != nil {
		return nil, err
	}

	// Only include discovery/technology-usage rulesets if default rules are enabled
	if !test.Analysis.DisableDefaultRules {
		// Ensure discovery-rules and technology-usage rulesets exist
		if _, exists := rulesetToInsightConverted["discovery-rules"]; !exists {
			rulesetToInsightConverted["discovery-rules"] = konveyor.RuleSet{
				Name: "discovery-rules",
				Tags: []string{},
			}
		}
		if _, exists := rulesetToInsightConverted["technology-usage"]; !exists {
			rulesetToInsightConverted["technology-usage"] = konveyor.RuleSet{
				Name: "technology-usage",
				Tags: []string{},
			}
		}
		// Add tags to appropriate rulesets based on source
		for _, tag := range tags {
			switch tag.Source {
			case "language-discovery":
				rs := rulesetToInsightConverted["discovery-rules"]
				rs.Tags = append(rs.Tags, tag.Name)
				rulesetToInsightConverted["discovery-rules"] = rs
			case "tech-discovery":
				rs := rulesetToInsightConverted["technology-usage"]
				rs.Tags = append(rs.Tags, tag.Name)
				rulesetToInsightConverted["technology-usage"] = rs
			}
		}
	} else {
		// When default rules are disabled, remove discovery/technology-usage rulesets
		delete(rulesetToInsightConverted, "discovery-rules")
		delete(rulesetToInsightConverted, "technology-usage")
	}
	a, err := t.client.Task.Get(task.ID)
	if err != nil {
		return nil, err
	}
	for _, e := range a.Errors {
		if strings.Contains(e.Description, "[Analyzer]") {
			analysisError := strings.TrimPrefix(e.Description, "[Analyzer]")
			parts := strings.Split(analysisError, ":")
			if len(parts) < 2 {
				continue
			}
			log.Info("got error parts", "error part1", parts[0], "error part 2", parts[1])
			ruleParts := strings.Split(strings.TrimSpace(parts[0]), ".")
			log.Info("got rule parts", "rule part", ruleParts)
			if len(ruleParts) != 2 {
				continue
			}
			r, ok := rulesetToInsightConverted[ruleParts[0]]
			log.Info("searching rulesetToInsightConverted for ruleset", "ok", ok, "r", r, "name", ruleParts[0], "keys", slices.Sorted(maps.Keys(rulesetToInsightConverted)))
			if r, ok := rulesetToInsightConverted[ruleParts[0]]; ok {
				log.Info("got ruleset", "error part1", parts[0], "error part 2", parts[1])
				if r.Errors == nil {
					r.Errors = map[string]string{}
				}
				r.Errors[ruleParts[1]] = strings.TrimSpace(strings.Join(parts[1:], ":"))
				rulesetToInsightConverted[ruleParts[0]] = r
			}
		}
	}

	// Get errors associated with rulesets
	output, err := yaml.Marshal(slices.Collect(maps.Values(rulesetToInsightConverted)))
	if err != nil {
		return nil, err
	}

	// Create output directory
	outputDir := filepath.Join(workDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write output to file
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

// createApplication creates a new application in Tackle Hub or finds existing one
func (t *TackleHubTarget) createApplication(test *config.TestDefinition) (*api.Application, error) {
	log := util.GetLogger()

	// First, try to find an existing application with the same name
	apps, err := t.client.Application.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list applications: %w", err)
	}

	// Look for existing application with matching name
	for _, existingApp := range apps {
		if existingApp.Name == test.Name {
			log.Info("Found existing application", "id", existingApp.ID, "name", existingApp.Name)

			// Update identities if maven settings configured
			if t.mavenSettings != "" {
				err = t.attachMavenIdentity(&existingApp)
				if err != nil {
					return nil, fmt.Errorf("failed to attach maven identity: %w", err)
				}
			}

			return &existingApp, nil
		}
	}

	// Application doesn't exist, create new one
	app := &api.Application{
		Name:        test.Name,
		Description: test.Description,
	}

	// Check if this is a binary analysis (based on file extension)
	isBinary := IsBinaryFile(test.Analysis.Application)

	// Only set repository for source code analysis
	if !isBinary {
		// Use parsed Git components if available, otherwise parse the URL
		if test.Analysis.ApplicationGitComponents != nil {
			app.Repository = &api.Repository{
				Kind:   "git",
				URL:    test.Analysis.ApplicationGitComponents.URL,
				Branch: test.Analysis.ApplicationGitComponents.Ref,
				Path:   test.Analysis.ApplicationGitComponents.Path,
			}
		} else {
			// Fallback to simple parsing (for backward compatibility)
			repoURL, branch := parseGitURL(test.Analysis.Application)
			app.Repository = &api.Repository{
				Kind:   "git",
				URL:    repoURL,
				Branch: branch,
			}
		}
	}

	err = t.client.Application.Create(app)
	if err != nil {
		return nil, err
	}

	// Attach maven identity if configured
	if t.mavenSettings != "" {
		err = t.attachMavenIdentity(app)
		if err != nil {
			return nil, fmt.Errorf("failed to attach maven identity: %w", err)
		}
	}

	return app, nil
}

// uploadBinary uploads a binary file to the application's bucket
func (t *TackleHubTarget) uploadBinary(task *api.Task, binaryPath string, testDir string) error {
	log := util.GetLogger()

	// Resolve the binary path (handle both absolute and relative paths)
	var absPath string
	var err error

	if filepath.IsAbs(binaryPath) {
		absPath = binaryPath
	} else {
		// Relative path - resolve relative to test directory
		absPath = filepath.Join(testDir, binaryPath)
	}

	// Verify file exists
	if _, err = os.Stat(absPath); err != nil {
		return fmt.Errorf("binary file not found at %s: %w", absPath, err)
	}

	log.Info("Uploading binary file", "path", absPath, "task", task.ID)

	// Get application bucket
	bucket := t.client.Bucket.Content(task.Bucket.ID)

	// Upload the binary to the bucket
	// The file will be stored at /binary in the bucket
	err = bucket.Put(absPath, fmt.Sprintf("/binary/%v", filepath.Base(absPath)))
	if err != nil {
		return fmt.Errorf("failed to upload binary: %w", err)
	}

	log.Info("Successfully uploaded binary", "path", absPath, "task", task.ID, "bucket_content")
	return nil
}

// createAnalysisTask creates an analysis task for the application
func (t *TackleHubTarget) createAnalysisTask(ctx context.Context, test *config.TestDefinition, app *api.Application) (*api.Task, error) {
	log := util.GetLogger()
	// Build task data with analysis configuration
	taskData := Data{}
	// For testing purpose's we want discovery and tags to be applied
	// from this task
	taskData.Tagger.Enabled = true

	// Check if this is a binary analysis
	isBinary := IsBinaryFile(test.Analysis.Application)

	if isBinary {
		// Binary mode
		taskData.Mode.Binary = true
		taskData.Mode.Artifact = fmt.Sprintf("/binary/%v", test.Analysis.Application) // Path where binary is stored in bucket
		log.Info("Configuring binary analysis mode", "artifact", taskData.Mode.Artifact)
	} else {
		// Source code mode
		// Set analysis mode
		switch test.Analysis.AnalysisMode {
		case "source-only":
			taskData.Mode.WithDeps = false
		default:
			taskData.Mode.WithDeps = true
		}
	}

	// Add label selector
	if test.Analysis.LabelSelector != "" {
		taskData.Rules.Labels = ParseLabelSelector(test.Analysis.LabelSelector)
	}

	// Handle rules that may be Git URLs
	// Tackle Hub uses repositories for rules, so we'll prepare them differently
	err := t.prepareRulesForHub(ctx, test, &taskData)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare rules: %w", err)
	}

	taskData.Verbosity = 1
	log.V(1).Info("Using task data", "data", taskData)

	task := &api.Task{
		Name:        fmt.Sprintf("Analysis: %s", test.Name),
		Kind:        "analyzer", // analyzer task kind
		Addon:       "analyzer",
		Extensions:  test.Analysis.Extensions,
		Application: &api.Ref{ID: app.ID},
		Data:        taskData,
		State:       "Created",
	}

	// Debug: log the task before creating
	log.V(1).Info("Creating task", "name", task.Name, "kind", task.Kind, "addon", task.Addon, "appID", app.ID)

	err = t.client.Task.Create(task)
	if err != nil {
		return nil, err
	}
	if isBinary {
		err = t.uploadBinary(task, test.Analysis.Application, test.GetTestDir())
		if err != nil {
			return nil, err
		}
	}
	task.State = "Ready"
	err = t.client.Task.Update(task)
	if err != nil {
		return nil, err
	}

	return task, nil
}

// prepareRulesForHub handles rules that may be Git URLs for Tackle Hub
// Tackle Hub handles rules differently - it uses repositories rather than file paths
func (t *TackleHubTarget) prepareRulesForHub(_ context.Context, test *config.TestDefinition, taskData *Data) error {
	if len(test.Analysis.Rules) == 0 || len(test.Analysis.RulesGitComponents) == 0 {
		return nil
	}

	log := util.GetLogger()

	if len(test.Analysis.RulesGitComponents) != 1 {
		return fmt.Errorf("tackle hub can only handle a single repository for custom rules")
	}

	taskData.Rules.Repository = &api.Repository{
		Kind:   "git",
		URL:    test.Analysis.RulesGitComponents[0].URL,
		Branch: test.Analysis.RulesGitComponents[0].Ref,
		Path:   test.Analysis.RulesGitComponents[0].Path,
	}

	log.Info("Using rules", "rules", taskData.Rules)

	return nil
}

// pollTaskCompletion polls the task until it completes or times out
func (t *TackleHubTarget) pollTaskCompletion(ctx context.Context, taskID uint, timeout time.Duration) error {
	log := util.GetLogger()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Until(deadline)):
			return fmt.Errorf("task timeout after %v", timeout)
		case <-ticker.C:
			task, err := t.client.Task.Get(taskID)
			if err != nil {
				return fmt.Errorf("failed to get task status: %w", err)
			}

			log.V(1).Info("Task status", "taskID", taskID, "state", task.State)

			switch task.State {
			case TaskStateSucceeded:
				return nil
			case TaskStateFailed:
				return fmt.Errorf("task failed: %v", task.Errors)
			case TaskStateRunning, TaskStateReady, TaskStateCreated, TaskStatePending, TaskStatePostponed:
				// Continue polling
				continue
			default:
				return fmt.Errorf("unexpected task state: %s", task.State)
			}
		}
	}
}

// submitTask submits a task to the task manager for processing
func (t *TackleHubTarget) submitTask(taskID uint) error {
	path := fmt.Sprintf("/tasks/%d/submit", taskID)
	// The submit endpoint doesn't return a body, but we need to pass something
	// to the Put method. Pass nil and ignore the Unmarshal(nil) error.
	err := t.client.Client.Put(path, nil)
	if err != nil && err.Error() != "json: Unmarshal(nil)" {
		return err
	}
	return nil
}

// attachMavenIdentity creates or finds a maven settings identity and attaches it to the application
func (t *TackleHubTarget) attachMavenIdentity(app *api.Application) error {
	log := util.GetLogger()

	// Read maven settings file
	settingsContent, err := os.ReadFile(t.mavenSettings)
	if err != nil {
		return fmt.Errorf("failed to read maven settings file %s: %w", t.mavenSettings, err)
	}

	identityName := fmt.Sprintf("maven-settings-%s", app.Name)

	// Check if identity already exists
	identities, err := t.client.Identity.List()
	if err != nil {
		return fmt.Errorf("failed to list identities: %w", err)
	}

	var identity *api.Identity
	for _, existing := range identities {
		if existing.Name == identityName && existing.Kind == "maven" {
			identity = &existing
			log.Info("Found existing maven identity", "id", identity.ID, "name", identity.Name)
			break
		}
	}

	// Create identity if it doesn't exist
	if identity == nil {
		identity = &api.Identity{
			Name:        identityName,
			Kind:        "maven",
			Description: fmt.Sprintf("Maven settings for %s", app.Name),
			Settings:    string(settingsContent),
		}

		err = t.client.Identity.Create(identity)
		if err != nil {
			return fmt.Errorf("failed to create maven identity: %w", err)
		}
		log.Info("Created maven identity", "id", identity.ID, "name", identity.Name)
	}

	// Attach identity to application by adding it to identities list
	identityRef := api.IdentityRef{ID: identity.ID, Role: "maven"}

	// Check if identity is already attached
	alreadyAttached := false
	for _, ref := range app.Identities {
		if ref.ID == identity.ID {
			alreadyAttached = true
			break
		}
	}

	if !alreadyAttached {
		app.Identities = append(app.Identities, identityRef)
		err = t.client.Application.Update(app)
		if err != nil {
			return fmt.Errorf("failed to update application with identity: %w", err)
		}
		log.Info("Attached maven identity to application", "appID", app.ID, "identityID", identity.ID)
	} else {
		log.Info("Maven identity already attached to application", "appID", app.ID, "identityID", identity.ID)
	}

	return nil
}

// parseGitURL parses a git URL that may contain a branch reference (e.g., URL#branch)
// and returns the base URL and branch separately.
// This is kept for backward compatibility, but prefer using config.ParseGitURLWithPath
func parseGitURL(gitURL string) (url, branch string) {
	components := config.ParseGitURLWithPath(gitURL)
	url = components.URL
	// For backward compatibility, combine ref and path with /
	if components.Path != "" {
		branch = components.Ref + "/" + components.Path
	} else {
		branch = components.Ref
	}
	return url, branch
}
