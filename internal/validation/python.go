package validation

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// PythonValidator runs Python validation tools (ruff, pyright) in VM.
type PythonValidator struct {
	projectPath string
}

// NewPythonValidator creates a validator for Python projects.
func NewPythonValidator(projectPath string) *PythonValidator {
	return &PythonValidator{
		projectPath: projectPath,
	}
}

// Validate runs all Python validators and returns structured errors.
func (v *PythonValidator) Validate() (*ValidationResult, error) {
	result := &ValidationResult{
		Language: "python",
		Errors:   []ValidationError{},
	}

	// Run ruff
	ruffErrors, err := v.runRuff()
	if err != nil {
		return nil, fmt.Errorf("ruff execution failed: %w", err)
	}
	result.Errors = append(result.Errors, ruffErrors...)

	// Run pyright
	pyrightErrors, err := v.runPyright()
	if err != nil {
		return nil, fmt.Errorf("pyright execution failed: %w", err)
	}
	result.Errors = append(result.Errors, pyrightErrors...)

	result.Passed = len(result.Errors) == 0

	return result, nil
}

// runRuff executes ruff with configured rules.
func (v *PythonValidator) runRuff() ([]ValidationError, error) {
	// Ruff configuration embedded as CLI args
	// select = ["E", "F", "S", "C90", "I"]
	// max-complexity = 10
	cmd := exec.Command("ruff", "check",
		"--select", "E,F,S,C90,I",
		"--config", "mccabe.max-complexity=10",
		"--output-format", "json",
		v.projectPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ruff exits non-zero if issues found, which is expected
		if len(output) == 0 {
			return nil, fmt.Errorf("ruff failed: %w", err)
		}
	}

	return v.parseRuffOutput(output)
}

// RuffOutput represents ruff's JSON output structure.
type RuffOutput struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Location struct {
		Row    int `json:"row"`
		Column int `json:"column"`
	} `json:"location"`
	Filename string `json:"filename"`
}

// parseRuffOutput parses ruff JSON output into ValidationErrors.
func (v *PythonValidator) parseRuffOutput(output []byte) ([]ValidationError, error) {
	if len(output) == 0 {
		return []ValidationError{}, nil
	}

	var ruffIssues []RuffOutput
	if err := json.Unmarshal(output, &ruffIssues); err != nil {
		return nil, fmt.Errorf("failed to parse ruff output: %w", err)
	}

	var errors []ValidationError
	for _, issue := range ruffIssues {
		relPath, _ := filepath.Rel(v.projectPath, issue.Filename)
		if relPath == "" {
			relPath = issue.Filename
		}

		errors = append(errors, ValidationError{
			Tool:     "ruff",
			File:     relPath,
			Line:     issue.Location.Row,
			Column:   issue.Location.Column,
			Code:     issue.Code,
			Message:  issue.Message,
			Severity: getSeverityFromRuffCode(issue.Code),
		})
	}

	return errors, nil
}

// getSeverityFromRuffCode maps ruff error codes to severity levels.
func getSeverityFromRuffCode(code string) string {
	if strings.HasPrefix(code, "E") || strings.HasPrefix(code, "F") {
		return "error"
	}
	if strings.HasPrefix(code, "S") {
		return "warning" // Security issues
	}
	if strings.HasPrefix(code, "C90") {
		return "warning" // Complexity
	}
	return "info"
}

// runPyright executes pyright type checker.
func (v *PythonValidator) runPyright() ([]ValidationError, error) {
	cmd := exec.Command("pyright",
		"--outputjson",
		v.projectPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Pyright exits non-zero if type errors found
		if len(output) == 0 {
			return nil, fmt.Errorf("pyright failed: %w", err)
		}
	}

	return v.parsePyrightOutput(output)
}

// PyrightOutput represents pyright's JSON output structure.
type PyrightOutput struct {
	GeneralDiagnostics []struct {
		File     string `json:"file"`
		Severity string `json:"severity"`
		Message  string `json:"message"`
		Range    struct {
			Start struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"start"`
		} `json:"range"`
		Rule string `json:"rule"`
	} `json:"generalDiagnostics"`
}

// parsePyrightOutput parses pyright JSON output into ValidationErrors.
func (v *PythonValidator) parsePyrightOutput(output []byte) ([]ValidationError, error) {
	if len(output) == 0 {
		return []ValidationError{}, nil
	}

	var pyrightResult PyrightOutput
	if err := json.Unmarshal(output, &pyrightResult); err != nil {
		return nil, fmt.Errorf("failed to parse pyright output: %w", err)
	}

	var errors []ValidationError
	for _, diag := range pyrightResult.GeneralDiagnostics {
		relPath, _ := filepath.Rel(v.projectPath, diag.File)
		if relPath == "" {
			relPath = diag.File
		}

		errors = append(errors, ValidationError{
			Tool:     "pyright",
			File:     relPath,
			Line:     diag.Range.Start.Line + 1, // Pyright uses 0-indexed lines
			Column:   diag.Range.Start.Character + 1,
			Code:     diag.Rule,
			Message:  diag.Message,
			Severity: mapPyrightSeverity(diag.Severity),
		})
	}

	return errors, nil
}

// mapPyrightSeverity converts pyright severity to standard levels.
func mapPyrightSeverity(severity string) string {
	switch strings.ToLower(severity) {
	case "error":
		return "error"
	case "warning":
		return "warning"
	case "information":
		return "info"
	default:
		return "info"
	}
}
