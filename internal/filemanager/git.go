package filemanager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager handles git operations and file locking for the current project.
type Manager struct {
	projectRoot string
	locks       sync.Map
}

// WorktreeInfo contains information about a git worktree.
type WorktreeInfo struct {
	Path      string
	Branch    string
	SessionID string
}

// NewManager creates a git manager for the current working directory.
func NewManager() (*Manager, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	root, err := findGitRoot(cwd)
	if err != nil {
		return nil, err
	}

	return &Manager{projectRoot: root}, nil
}

// findGitRoot walks up directory tree to find .git directory.
func findGitRoot(dir string) (string, error) {
	for {
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a git repository")
		}
		dir = parent
	}
}

// CreateWorktree creates a new git worktree with a feature branch.
func (m *Manager) CreateWorktree(sessionID string) (string, error) {
	magosDir := filepath.Join(m.projectRoot, ".magos")
	if err := os.MkdirAll(magosDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .magos directory: %w", err)
	}

	wtPath := filepath.Join(magosDir, "wt_"+sessionID)
	branchName := "magos/" + sessionID

	cmd := exec.Command("git", "worktree", "add", wtPath, "-b", branchName)
	cmd.Dir = m.projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to create worktree: %w\nOutput: %s", err, output)
	}

	return wtPath, nil
}

// MergeWorktree merges the worktree's branch back to main.
func (m *Manager) MergeWorktree(sessionID string) error {
	branchName := "magos/" + sessionID

	checkoutCmd := exec.Command("git", "checkout", "main")
	checkoutCmd.Dir = m.projectRoot
	if output, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to checkout main: %w\nOutput: %s", err, output)
	}

	mergeCmd := exec.Command("git", "merge", branchName, "--no-ff", "-m",
		fmt.Sprintf("Merge Magos session %s", sessionID))
	mergeCmd.Dir = m.projectRoot

	if output, err := mergeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to merge branch: %w\nOutput: %s", err, output)
	}

	return nil
}

// CleanupWorktree removes the worktree and deletes the branch.
func (m *Manager) CleanupWorktree(sessionID string) error {
	wtPath := filepath.Join(m.projectRoot, ".magos", "wt_"+sessionID)
	branchName := "magos/" + sessionID

	removeCmd := exec.Command("git", "worktree", "remove", wtPath, "--force")
	removeCmd.Dir = m.projectRoot
	if output, err := removeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove worktree: %w\nOutput: %s", err, output)
	}

	branchCmd := exec.Command("git", "branch", "-D", branchName)
	branchCmd.Dir = m.projectRoot
	if output, err := branchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete branch: %w\nOutput: %s", err, output)
	}

	return nil
}

// ListWorktrees returns all active Magos worktrees.
func (m *Manager) ListWorktrees() ([]WorktreeInfo, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = m.projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	return parseWorktrees(string(output)), nil
}

// parseWorktrees parses the output of git worktree list --porcelain.
func parseWorktrees(output string) []WorktreeInfo {
	var worktrees []WorktreeInfo
	lines := strings.Split(output, "\n")

	var current WorktreeInfo
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			branch := strings.TrimPrefix(line, "branch refs/heads/")
			current.Branch = branch

			if strings.HasPrefix(branch, "magos/") {
				current.SessionID = strings.TrimPrefix(branch, "magos/")
			}
		} else if line == "" && current.Path != "" {
			if strings.Contains(current.Path, ".magos/wt_") {
				worktrees = append(worktrees, current)
			}
			current = WorktreeInfo{}
		}
	}

	return worktrees
}

// GetDiff returns the diff between the worktree branch and main.
func (m *Manager) GetDiff(sessionID string) (string, error) {
	branchName := "magos/" + sessionID

	cmd := exec.Command("git", "diff", "main..."+branchName)
	cmd.Dir = m.projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	return string(output), nil
}

// LockFile blocks until the file is available, then locks it for the given session.
func (m *Manager) LockFile(sessionID, filePath string) {
	for {
		_, loaded := m.locks.LoadOrStore(filePath, sessionID)
		if !loaded {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// UnlockFile releases the lock on a file.
func (m *Manager) UnlockFile(filePath string) {
	m.locks.Delete(filePath)
}

// IsLocked checks if a file is currently locked and returns the session ID if it is.
func (m *Manager) IsLocked(filePath string) (bool, string) {
	if sessionID, ok := m.locks.Load(filePath); ok {
		return true, sessionID.(string)
	}
	return false, ""
}

// ListLocks returns all currently locked files and their session IDs.
func (m *Manager) ListLocks() map[string]string {
	locks := make(map[string]string)
	m.locks.Range(func(key, value interface{}) bool {
		locks[key.(string)] = value.(string)
		return true
	})
	return locks
}
