package cli

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/konveyor/test-harness/pkg/validator"
	yaml "gopkg.in/yaml.v2"
)

// TestResult represents the result of a single test execution
// Validation errors are grouped with each test
type TestResult struct {
	Name             string                      `json:"name" yaml:"name" xml:"name,attr"`
	TestFile         string                      `json:"testFile" yaml:"testFile" xml:"testFile,attr"`
	Status           string                      `json:"status" yaml:"status" xml:"status,attr"`
	Duration         string                      `json:"duration" yaml:"duration" xml:"time,attr"`
	ExitCode         int                         `json:"exitCode,omitempty" yaml:"exitCode,omitempty" xml:"exitCode,omitempty"`
	ExpectedExitCode int                         `json:"expectedExitCode,omitempty" yaml:"expectedExitCode,omitempty" xml:"expectedExitCode,omitempty"`
	ValidationErrors []validator.ValidationError `json:"validationErrors,omitempty" yaml:"validationErrors,omitempty" xml:"validationErrors>error,omitempty"`
	ErrorMessage     string                      `json:"errorMessage,omitempty" yaml:"errorMessage,omitempty" xml:"errorMessage,omitempty"`
	RuleSetsCount    int                         `json:"ruleSetsCount,omitempty" yaml:"ruleSetsCount,omitempty" xml:"ruleSetsCount,omitempty"`
	FilteredFrom     int                         `json:"filteredFrom,omitempty" yaml:"filteredFrom,omitempty" xml:"filteredFrom,omitempty"`
}

// TestSummary contains results for all tests in a run
type TestSummary struct {
	Total    int          `json:"total" yaml:"total" xml:"total,attr"`
	Passed   int          `json:"passed" yaml:"passed" xml:"passed,attr"`
	Failed   int          `json:"failed" yaml:"failed" xml:"failed,attr"`
	Skipped  int          `json:"skipped" yaml:"skipped" xml:"skipped,attr"`
	Duration string       `json:"duration" yaml:"duration" xml:"time,attr"`
	Tests    []TestResult `json:"tests" yaml:"tests" xml:"testcase"`
}

// JUnitTestSuite represents a JUnit XML test suite
type JUnitTestSuite struct {
	XMLName   xml.Name         `xml:"testsuite"`
	Name      string           `xml:"name,attr"`
	Tests     int              `xml:"tests,attr"`
	Failures  int              `xml:"failures,attr"`
	Skipped   int              `xml:"skipped,attr"`
	Time      string           `xml:"time,attr"`
	TestCases []JUnitTestCase  `xml:"testcase"`
}

// JUnitTestCase represents a single test case in JUnit XML format
type JUnitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
	Skipped   *JUnitSkipped `xml:"skipped,omitempty"`
}

// JUnitFailure represents a test failure in JUnit XML format
type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

// JUnitSkipped represents a skipped test in JUnit XML format
type JUnitSkipped struct {
	Message string `xml:"message,attr"`
}

// OutputFormat represents the output format for test results
type OutputFormat string

const (
	OutputFormatConsole OutputFormat = "console"
	OutputFormatJSON    OutputFormat = "json"
	OutputFormatYAML    OutputFormat = "yaml"
	OutputFormatJUnit   OutputFormat = "junit"
)

// FormatResults outputs the test results in the specified format
func FormatResults(summary *TestSummary, format OutputFormat) (string, error) {
	switch format {
	case OutputFormatJSON:
		return formatJSON(summary)
	case OutputFormatYAML:
		return formatYAML(summary)
	case OutputFormatJUnit:
		return formatJUnit(summary)
	default:
		return "", fmt.Errorf("unsupported output format: %s", format)
	}
}

// formatJSON formats the test results as JSON
func formatJSON(summary *TestSummary) (string, error) {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data), nil
}

// formatYAML formats the test results as YAML
func formatYAML(summary *TestSummary) (string, error) {
	data, err := yaml.Marshal(summary)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}
	return string(data), nil
}

// formatJUnit formats the test results as JUnit XML
func formatJUnit(summary *TestSummary) (string, error) {
	suite := JUnitTestSuite{
		Name:      "koncur-tests",
		Tests:     summary.Total,
		Failures:  summary.Failed,
		Skipped:   summary.Skipped,
		Time:      summary.Duration,
		TestCases: make([]JUnitTestCase, 0, len(summary.Tests)),
	}

	for _, result := range summary.Tests {
		testCase := JUnitTestCase{
			Name:      result.Name,
			ClassName: "koncur",
			Time:      result.Duration,
		}

		if result.Status == "failed" {
			failureMessage := result.ErrorMessage
			if failureMessage == "" && len(result.ValidationErrors) > 0 {
				failureMessage = fmt.Sprintf("%d validation error(s)", len(result.ValidationErrors))
			}

			// Build detailed failure content with validation errors grouped under this test
			content := ""
			if result.ExitCode != result.ExpectedExitCode {
				content += fmt.Sprintf("Exit code mismatch: expected %d, got %d\n", result.ExpectedExitCode, result.ExitCode)
			}
			if len(result.ValidationErrors) > 0 {
				content += fmt.Sprintf("\nValidation Errors (%d):\n", len(result.ValidationErrors))
				for i, verr := range result.ValidationErrors {
					content += fmt.Sprintf("[%d] %s: %s\n", i+1, verr.Path, verr.Message)
				}
			}

			testCase.Failure = &JUnitFailure{
				Message: failureMessage,
				Type:    "ValidationError",
				Content: content,
			}
		} else if result.Status == "skipped" {
			testCase.Skipped = &JUnitSkipped{
				Message: "Test marked as skipped",
			}
		}

		suite.TestCases = append(suite.TestCases, testCase)
	}

	data, err := xml.MarshalIndent(suite, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JUnit XML: %w", err)
	}

	return xml.Header + string(data), nil
}

// parseDuration converts a time.Duration to a string in seconds (for JUnit compatibility)
func parseDuration(d time.Duration) string {
	return fmt.Sprintf("%.3f", d.Seconds())
}
