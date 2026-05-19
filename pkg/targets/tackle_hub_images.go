package targets

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/konveyor/test-harness/pkg/config"
	"github.com/konveyor/test-harness/pkg/util"
)

// buildCRPatch constructs a JSON merge patch for the Tackle CR spec
// from the configured image overrides.
func buildCRPatch(images *config.TackleHubImages) (string, error) {
	if images == nil {
		return "", nil
	}

	spec := map[string]string{}

	if images.Hub != "" {
		spec["hub_image_fqin"] = images.Hub
	}
	if images.Analyzer != "" {
		spec["analyzer_fqin"] = images.Analyzer
	}
	if images.JavaProvider != "" {
		spec["provider_java_image_fqin"] = images.JavaProvider
	}
	if images.GenericProvider != "" {
		spec["provider_python_image_fqin"] = images.GenericProvider
		spec["provider_nodejs_image_fqin"] = images.GenericProvider
	}
	if images.CsharpProvider != "" {
		spec["provider_c_sharp_image_fqin"] = images.CsharpProvider
	}
	if images.Runner != "" {
		spec["kantra_fqin"] = images.Runner
	}
	if images.DiscoveryAddon != "" {
		spec["language_discovery_fqin"] = images.DiscoveryAddon
	}
	if images.PlatformAddon != "" {
		spec["platform_fqin"] = images.PlatformAddon
	}

	if len(spec) == 0 {
		return "", nil
	}

	patch := map[string]interface{}{
		"spec": spec,
	}

	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return "", fmt.Errorf("failed to marshal patch: %w", err)
	}

	return string(patchJSON), nil
}

// applyImageOverrides patches the Tackle CR with custom images and waits
// for the Hub to become ready with the new configuration.
func applyImageOverrides(ctx context.Context, cfg *config.TackleHubConfig) error {
	log := util.GetLogger()

	if !cfg.HasImageOverrides() {
		return nil
	}

	patch, err := buildCRPatch(cfg.Images)
	if err != nil {
		return fmt.Errorf("failed to build CR patch: %w", err)
	}
	if patch == "" {
		return nil
	}

	namespace := cfg.GetNamespace()
	crName := cfg.GetCRName()

	log.Info("Patching Tackle CR with image overrides",
		"namespace", namespace,
		"crName", crName,
		"patch", patch,
	)

	// Find kubectl
	kubectl, err := exec.LookPath("kubectl")
	if err != nil {
		return fmt.Errorf("kubectl not found in PATH (required for image overrides): %w", err)
	}

	// Patch the Tackle CR
	patchArgs := []string{
		"patch", "tackle", crName,
		"-n", namespace,
		"--type", "merge",
		"-p", patch,
	}

	patchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(patchCtx, kubectl, patchArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to patch Tackle CR: %w\nOutput: %s", err, string(output))
	}
	log.Info("Tackle CR patched successfully", "output", strings.TrimSpace(string(output)))

	// Wait for the Hub pod to become ready with the updated images.
	// The operator will reconcile and restart pods as needed.
	log.Info("Waiting for Tackle Hub to be ready with updated images...")
	err = waitForHubReady(ctx, kubectl, namespace)
	if err != nil {
		return fmt.Errorf("tackle Hub did not become ready after image update: %w", err)
	}

	log.Info("Tackle Hub is ready with updated images")
	return nil
}

// waitForHubReady polls until the Hub pod is ready or the context is cancelled.
func waitForHubReady(ctx context.Context, kubectl, namespace string) error {
	log := util.GetLogger()

	// Give the operator time to start reconciling
	select {
	case <-time.After(10 * time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}

	// Poll for up to 10 minutes
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("timed out waiting for Hub to be ready")
		default:
		}

		cmd := exec.CommandContext(waitCtx, kubectl,
			"wait", "--for=condition=ready",
			"pod", "-l", "app.kubernetes.io/name=tackle-hub",
			"-n", namespace,
			"--timeout=30s",
		)
		output, err := cmd.CombinedOutput()
		if err == nil {
			log.V(1).Info("Hub pod is ready", "output", strings.TrimSpace(string(output)))
			return nil
		}

		log.V(1).Info("Hub not ready yet, retrying...", "output", strings.TrimSpace(string(output)))

		select {
		case <-time.After(10 * time.Second):
		case <-waitCtx.Done():
			return fmt.Errorf("timed out waiting for Hub to be ready")
		}
	}
}
