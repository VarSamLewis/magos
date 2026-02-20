package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Runner orchestrates validation across multiple validators.
type Runner struct {
	projectPath string
	validators  []Validator
}

// NewRunner creates a validation runner for the given project path.
// It auto-detects languages and initializes appropriate validators.
func NewRunner(projectPath string) (*Runner, error) {
	if _, err := os.Stat(projectPath); err != nil {
		return nil, fmt.Errorf("project path does not exist: %w", err)
	}

	runner := &Runner{
		projectPath: projectPath,
		validators:  []Validator{},
	}

	// Auto-detect languages and add validators
	if err := runner.detectAndAddValidators(); err != nil {
		return nil, err
	}

	return runner, nil
}

// detectAndAddValidators scans project for language indicators.
func (r *Runner) detectAndAddValidators() error {
	// Check for Python
	pythonFiles := []string{
		filepath.Join(r.projectPath, "pyproject.toml"),
		filepath.Join(r.projectPath, "requirements.txt"),
		filepath.Join(r.projectPath, "setup.py"),
	}

	hasPythonProject := false
	for _, f := range pythonFiles {
		if _, err := os.Stat(f); err == nil {
			hasPythonProject = true
			break
		}
	}

	// Also check for any .py files in the project
	if !hasPythonProject {
		entries, err := os.ReadDir(r.projectPath)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".py") {
					hasPythonProject = true
					break
				}
			}
		}
	}

	if hasPythonProject {
		r.validators = append(r.validators, NewPythonValidator(r.projectPath))
	}

	// TODO: Add Go detection (go.mod)
	// TODO: Add Rust detection (Cargo.toml)
	// TODO: Add JavaScript/TypeScript detection (package.json)

	if len(r.validators) == 0 {
		return fmt.Errorf("no supported languages detected in project")
	}

	return nil
}

// Run executes all validators in parallel and aggregates results.
func (r *Runner) Run() (*AggregateResult, error) {
	aggregate := &AggregateResult{
		Results: make(map[string]*ValidationResult),
		Passed:  true,
	}

	// Run validators sequentially for now
	// TODO: Parallelize with goroutines + channels
	for _, validator := range r.validators {
		result, err := validator.Validate()
		if err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}

		aggregate.Results[result.Language] = result
		if !result.Passed {
			aggregate.Passed = false
		}
	}

	return aggregate, nil
}

// AggregateResult contains results from all validators.
type AggregateResult struct {
	Passed  bool                           `json:"passed"`
	Results map[string]*ValidationResult `json:"results"`
}

// FormatForLLM formats all validation results for LLM feedback.
func (a *AggregateResult) FormatForLLM() string {
	if a.Passed {
		return "All validation checks passed across all languages."
	}

	var builder strings.Builder
	builder.WriteString("Validation failed:\n\n")

	for lang, result := range a.Results {
		if !result.Passed {
			builder.WriteString(fmt.Sprintf("=== %s ===\n", lang))
			builder.WriteString(result.FormatForLLM())
			builder.WriteString("\n")
		}
	}

	return builder.String()
}
