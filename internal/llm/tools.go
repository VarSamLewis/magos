package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ToolCall represents a parsed tool call from LLM response.
type ToolCall struct {
	Tool  string                 `json:"tool"`
	Args  map[string]interface{} `json:"-"`
	Raw   map[string]interface{}
}

// FileModification represents a file to be modified.
type FileModification struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// BuildSystemPrompt creates the system prompt with tool definitions.
func BuildSystemPrompt(projectRoot string, fileTree []string) string {
	fileTreeStr := strings.Join(fileTree, "\n")

	return fmt.Sprintf(`You are Magos, an AI coding assistant with access to the following tools:

## Available Tools

### read_files
Read the contents of files from the project.
Input: {"tool": "read_files", "path": "relative/path/to/file.go"}
If path is empty or a directory, reads all files in that directory.
If path is a file, reads that specific file.
Output: File contents

### modify_files
Modify or create files in the codebase.
Input: {
  "tool": "modify_files",
  "files": [
    {"path": "auth.go", "content": "package auth\n\nfunc Login() {}"},
    {"path": "routes.go", "content": "..."}
  ]
}
Output: Files will be created/modified in a git worktree, validated, and merged.

### get_definition
Get the type definition of a symbol using LSP.
Input: {"tool": "get_definition", "symbol": "User", "file": "models/user.go"}
Output: Type definition

### list_symbols
List all symbols (functions, types, variables) in a file.
Input: {"tool": "list_symbols", "file": "auth.go"}
Output: List of symbols with their types

## Project Context

Project Root: %s

File Tree:
%s

## Instructions

When you need to use a tool, output a JSON block in a code fence like this:

` + "```json\n" + `{"tool": "read_file", "path": "auth.go"}
` + "```" + `

I will execute the tool and provide the result. Then you can continue or call more tools.

When you're ready to make changes, use modify_files with the complete new file contents.
Only call modify_files when you have all the information you need and are ready to write code.
`, projectRoot, fileTreeStr)
}

// ParseToolCalls extracts tool calls from LLM response text.
func ParseToolCalls(text string) []ToolCall {
	var toolCalls []ToolCall

	jsonBlockRegex := regexp.MustCompile("```json\\s*\\n([^`]+)\\n```")
	matches := jsonBlockRegex.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		jsonStr := strings.TrimSpace(match[1])

		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
			continue
		}

		tool, ok := raw["tool"].(string)
		if !ok {
			continue
		}

		toolCall := ToolCall{
			Tool: tool,
			Args: make(map[string]interface{}),
			Raw:  raw,
		}

		for k, v := range raw {
			if k != "tool" {
				toolCall.Args[k] = v
			}
		}

		toolCalls = append(toolCalls, toolCall)
	}

	return toolCalls
}

// GetStringArg safely extracts a string argument from tool call.
func (tc *ToolCall) GetStringArg(key string) (string, bool) {
	val, ok := tc.Args[key]
	if !ok {
		return "", false
	}

	str, ok := val.(string)
	return str, ok
}

// GetFileModifications extracts file modifications from modify_files tool call.
func (tc *ToolCall) GetFileModifications() ([]FileModification, error) {
	if tc.Tool != "modify_files" {
		return nil, fmt.Errorf("not a modify_files tool call")
	}

	filesInterface, ok := tc.Args["files"]
	if !ok {
		return nil, fmt.Errorf("missing 'files' argument")
	}

	filesSlice, ok := filesInterface.([]interface{})
	if !ok {
		return nil, fmt.Errorf("'files' is not an array")
	}

	var modifications []FileModification
	for _, fileInterface := range filesSlice {
		fileMap, ok := fileInterface.(map[string]interface{})
		if !ok {
			continue
		}

		path, pathOk := fileMap["path"].(string)
		content, contentOk := fileMap["content"].(string)

		if pathOk && contentOk {
			modifications = append(modifications, FileModification{
				Path:    path,
				Content: content,
			})
		}
	}

	return modifications, nil
}

// ExecuteReadFiles reads files from the specified path.
func ExecuteReadFiles(projectRoot, path string) (string, error) {
	targetPath := filepath.Join(projectRoot, path)

	if path == "" {
		return readAllFiles(projectRoot)
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return "", fmt.Errorf("path not found: %s", path)
	}

	if info.IsDir() {
		return readAllFiles(targetPath)
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}

	return fmt.Sprintf("File: %s\n\n%s", path, string(content)), nil
}

// maxReadBytes is the total byte budget for readAllFiles output.
const maxReadBytes = 100_000

// readAllFiles recursively reads all files in a directory.
func readAllFiles(dir string) (string, error) {
	var result strings.Builder
	var fileCount int

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == ".magos" || info.Name() == "node_modules" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		if result.Len() >= maxReadBytes {
			return filepath.SkipAll
		}

		relPath, _ := filepath.Rel(dir, path)

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		remaining := maxReadBytes - result.Len()
		if len(content) > remaining {
			content = content[:remaining]
			result.WriteString(fmt.Sprintf("\n--- File: %s (truncated) ---\n%s\n", relPath, string(content)))
		} else {
			result.WriteString(fmt.Sprintf("\n--- File: %s ---\n%s\n", relPath, string(content)))
		}
		fileCount++

		return nil
	})

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Read %d files:\n%s", fileCount, result.String()), nil
}

// ExecuteModifyFiles writes files to the worktree path.
func ExecuteModifyFiles(worktreePath string, modifications []FileModification) (string, error) {
	if worktreePath == "" {
		return "", fmt.Errorf("worktree path not provided")
	}

	var results []string

	for _, mod := range modifications {
		targetPath := filepath.Join(worktreePath, mod.Path)

		dir := filepath.Dir(targetPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory for %s: %w", mod.Path, err)
		}

		if err := os.WriteFile(targetPath, []byte(mod.Content), 0644); err != nil {
			return "", fmt.Errorf("failed to write file %s: %w", mod.Path, err)
		}

		results = append(results, mod.Path)
	}

	return fmt.Sprintf("Modified %d file(s): %s", len(results), strings.Join(results, ", ")), nil
}
