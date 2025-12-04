package targets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/konveyor/tackle2-hub/api"
	"github.com/konveyor/tackle2-hub/binding"
	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/util"
)

const (
	// TaskStateCreated indicates task has been created
	TaskStateCreated = "Created"
	// TaskStateReady indicates task is ready to run
	TaskStateReady = "Ready"
	// TaskStateRunning indicates task is running
	TaskStateRunning = "Running"
	// TaskStateSucceeded indicates task completed successfully
	TaskStateSucceeded = "Succeeded"
	// TaskStateFailed indicates task failed
	TaskStateFailed = "Failed"
)

// TackleHubTarget implements Target for Tackle Hub API
type TackleHubTarget struct {
	url    string
	client *binding.RichClient
}

// NewTackleHubTarget creates a new Tackle Hub API target
func NewTackleHubTarget(cfg *config.TackleHubConfig) (*TackleHubTarget, error) {
	if cfg == nil {
		return nil, fmt.Errorf("tackle hub configuration is required")
	}

	client := binding.New(cfg.URL)

	// Set authentication token if provided
	if cfg.Token != "" {
		client.Client.Login.Token = cfg.Token
	} else if cfg.Username != "" && cfg.Password != "" {
		client.Client.Login.User = cfg.Username
		client.Client.Login.Password = cfg.Password
	} else {
		return nil, fmt.Errorf("either token or username/password required")
	}

	return &TackleHubTarget{
		url:    cfg.URL,
		client: client,
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

	// Step 3: Poll for task completion
	log.Info("Polling for task completion", "taskID", task.ID)
	err = t.pollTaskCompletion(ctx, task.ID, test.GetTimeout())
	if err != nil {
		return nil, fmt.Errorf("task failed or timed out: %w", err)
	}
	log.Info("Analysis task completed successfully", "taskID", task.ID)

	// Step 4: Download results
	log.Info("Downloading analysis results", "applicationID", app.ID)
	outputFile, err := t.downloadResults(app.ID, workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to download results: %w", err)
	}

	duration := time.Since(start)
	result := &ExecutionResult{
		ExitCode:   0,
		Duration:   duration,
		OutputFile: outputFile,
		WorkDir:    workDir,
	}

	return result, nil
}

// createApplication creates a new application in Tackle Hub
func (t *TackleHubTarget) createApplication(test *config.TestDefinition) (*api.Application, error) {
	app := &api.Application{
		Name:        test.Name,
		Description: test.Description,
		Repository: &api.Repository{
			Kind:   "git", // or detect based on test.Analysis.Application
			URL:    test.Analysis.Application,
		},
	}

	err := t.client.Application.Create(app)
	if err != nil {
		return nil, err
	}

	return app, nil
}

// createAnalysisTask creates an analysis task for the application
func (t *TackleHubTarget) createAnalysisTask(ctx context.Context, test *config.TestDefinition, app *api.Application) (*api.Task, error) {
	// Build task data with analysis configuration
	taskData := map[string]interface{}{
		"mode": map[string]interface{}{
			"artifact": "",
		},
		"targets": []string{},
		"sources": []string{},
	}

	// Set analysis mode
	switch test.Analysis.AnalysisMode {
	case "source-only":
		taskData["mode"].(map[string]interface{})["binary"] = false
	default:
		taskData["mode"].(map[string]interface{})["binary"] = true
	}

	// Add label selector
	if test.Analysis.LabelSelector != "" {
		taskData["labelSelector"] = test.Analysis.LabelSelector
	}

	task := &api.Task{
		Name:        fmt.Sprintf("Analysis: %s", test.Name),
		Kind:        "analyzer", // analyzer task kind
		Addon:       "analyzer",
		Application: &api.Ref{ID: app.ID},
		Data:        taskData,
	}

	err := t.client.Task.Create(task)
	if err != nil {
		return nil, err
	}

	return task, nil
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
			case TaskStateRunning, TaskStateReady, TaskStateCreated:
				// Continue polling
				continue
			default:
				return fmt.Errorf("unexpected task state: %s", task.State)
			}
		}
	}
}

// downloadResults downloads the analysis results from the application bucket
func (t *TackleHubTarget) downloadResults(appID uint, workDir string) (string, error) {
	log := util.GetLogger()

	// Create output directory
	outputDir := filepath.Join(workDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Get application bucket
	bucket := t.client.Application.Bucket(appID)

	// Download output.yaml from the bucket
	outputFile := filepath.Join(outputDir, "output.yaml")
	log.Info("Downloading output.yaml", "from", "bucket", "to", outputFile)

	// Get the output.yaml from the analysis results
	// The path in the bucket is typically: /windup/report/output.yaml or similar
	err := bucket.Get("/windup/report/output.yaml", outputFile)
	if err != nil {
		// Try alternate path
		err = bucket.Get("/analyzer/output.yaml", outputFile)
		if err != nil {
			return "", fmt.Errorf("failed to download output.yaml: %w", err)
		}
	}

	return outputFile, nil
}
