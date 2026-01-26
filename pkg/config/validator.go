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

	return nil
}
