package config

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// Validate checks if a test definition is valid
func Validate(test *TestDefinition) error {
	// Run struct validation
	if err := validate.Struct(test); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Custom validation: ExpectedOutput must have exactly one of Result or File
	if err := validateExpectedOutput(&test.Expect.Output); err != nil {
		return err
	}

	return nil
}

// validateExpectedOutput ensures exactly one of Result or File is set
func validateExpectedOutput(output *ExpectedOutput) error {
	hasResult := len(output.Result) > 0
	hasFile := output.File != ""

	if !hasResult && !hasFile {
		return fmt.Errorf("expected output must specify either 'result' or 'file'")
	}

	if hasResult && hasFile {
		return fmt.Errorf("expected output cannot specify both 'result' and 'file'")
	}

	return nil
}
