package validation

import (
	"fmt"
	"strings"
)

// Validator is the interface all language validators must implement.
type Validator interface {
	// Validate runs all validation tools and returns results.
	Validate() (*ValidationResult, error)
}

// ValidationResult contains the outcome of running validators.
type ValidationResult struct {
	Language string            `json:"language"`
	Passed   bool              `json:"passed"`
	Errors   []ValidationError `json:"errors"`
}

// ValidationError represents a single validation issue.
type ValidationError struct {
	Tool     string `json:"tool"`     // Name of the tool (ruff, pyright, golangci-lint, etc)
	File     string `json:"file"`     // Relative file path
	Line     int    `json:"line"`     // Line number (1-indexed)
	Column   int    `json:"column"`   // Column number (1-indexed)
	Code     string `json:"code"`     // Error code (E501, F401, etc)
	Message  string `json:"message"`  // Human-readable message
	Severity string `json:"severity"` // error, warning, info
}

// FormatForLLM formats validation errors for inclusion in LLM prompt.
func (r *ValidationResult) FormatForLLM() string {
	if r.Passed {
		return "All validation checks passed."
	}

	var builder strings.Builder
	builder.WriteString("Validation failed with the following errors:\n\n")

	for _, err := range r.Errors {
		builder.WriteString(fmt.Sprintf("[%s] %s:%d:%d - %s (%s): %s\n",
			err.Severity,
			err.File,
			err.Line,
			err.Column,
			err.Code,
			err.Tool,
			err.Message,
		))
	}

	return builder.String()
}
