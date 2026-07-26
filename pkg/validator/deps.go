package validator

import (
	"fmt"
	"reflect"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
)

// compares normalized flat dependency lists from dependencies.yaml
func ValidateDependenciesFlat(actual, expected []konveyor.DepsFlatItem) *ValidationResult {
	result := &ValidationResult{Passed: true, Errors: []ValidationError{}}

	key := func(it konveyor.DepsFlatItem) string {
		return it.FileURI + "\x00" + it.Provider
	}

	actualByKey := make(map[string]konveyor.DepsFlatItem, len(actual))
	for _, it := range actual {
		actualByKey[key(it)] = it
	}

	for _, exp := range expected {
		k := key(exp)
		act, ok := actualByKey[k]
		if !ok {
			result.Errors = append(result.Errors, ValidationError{
				Path:    fmt.Sprintf("dependencies/%s", k),
				Message: "No matching entry for fileURI+provider in actual dependencies output",
			})
			continue
		}
		if !reflect.DeepEqual(act.Dependencies, exp.Dependencies) {
			result.Errors = append(result.Errors, ValidationError{
				Path:    fmt.Sprintf("dependencies/%s/deps", k),
				Message: "Dependency list differs from expected (after normalization)",
			})
		}
	}

	expectedKeys := make(map[string]bool, len(expected))
	for _, exp := range expected {
		expectedKeys[key(exp)] = true
	}
	for _, act := range actual {
		k := key(act)
		if !expectedKeys[k] {
			result.Errors = append(result.Errors, ValidationError{
				Path:    fmt.Sprintf("dependencies/%s", k),
				Message: "Unexpected fileURI+provider in actual dependencies output",
			})
		}
	}

	result.Passed = len(result.Errors) == 0
	return result
}
