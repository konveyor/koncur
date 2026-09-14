package hubapi

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
	"github.com/konveyor/tackle2-hub/shared/binding"
	"github.com/konveyor/test-harness/pkg/parser"
	"github.com/konveyor/test-harness/pkg/util"
	"go.lsp.dev/uri"
	"gopkg.in/yaml.v2"
)

// FetchAnalysisResults fetches analysis results from Hub API and converts them to konveyor format
func FetchAnalysisResults(client *binding.RichClient, appID uint, taskID uint, disableDefaultRules bool) ([]byte, error) {
	log := util.GetLogger()

	// Fetch insights from Hub API
	insights, err := client.Application.Select(appID).Analysis.ListInsights()
	if err != nil {
		return nil, fmt.Errorf("failed to get analysis insights: %w", err)
	}
	log.Info("Retrieved analysis insights", "count", len(insights))

	// Convert insights to ruleset format
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

		// Convert incidents
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

		// Convert links
		links := []konveyor.Link{}
		for _, l := range insight.Links {
			links = append(links, konveyor.Link{
				URL:   l.URL,
				Title: l.Title,
			})
		}

		// Create violation
		v := konveyor.Violation{
			Description: insight.Description,
			Category:    (*konveyor.Category)(&insight.Category),
			Labels:      insight.Labels,
			Incidents:   incidents,
			Links:       links,
			Effort:      &insight.Effort,
		}

		// Categorize by effort (0 = insight, >0 = violation)
		if insight.Effort == 0 {
			rs.Insights[insight.Rule] = v
		} else {
			rs.Violations[insight.Rule] = v
		}
		rulesetToInsightConverted[insight.RuleSet] = rs
	}

	// Get tags from application
	appTag := client.Application.Tags(appID)
	tags, err := appTag.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	// Only include discovery/technology-usage rulesets if default rules are enabled
	if !disableDefaultRules {
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

	// Get errors associated with rulesets
	task, err := client.Task.Get(taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	for _, e := range task.Errors {
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

	// Convert to YAML
	output, err := yaml.Marshal(slices.Collect(maps.Values(rulesetToInsightConverted)))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal output: %w", err)
	}

	return output, nil
}
