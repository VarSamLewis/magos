package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Workspace struct {
	imagePath string
	mountPath string
	sizeMB    int
}

func NewWorkspace(worktreePath string, sizeMB int) (*Workspace, error) {
	workspace := &Workspace{
		imagePath: filepath.Join(os.TempDir(), "magos-workspace.ext4"),
		mountPath: filepath.Join(os.TempDir(), "magos-workspace-mount"),
		sizeMB:    sizeMB,
	}

	if err := workspace.createImage(); err != nil {
		return nil, fmt.Errorf("create image: %w", err)
	}

	if err := workspace.mount(); err != nil {
		return nil, fmt.Errorf("mount: %w", err)
	}

	if err := workspace.copyFiles(worktreePath); err != nil {
		workspace.unmount()
		return nil, fmt.Errorf("copy files: %w", err)
	}

	if err := workspace.unmount(); err != nil {
		return nil, fmt.Errorf("unmount: %w", err)
	}

	return workspace, nil
}

func (w *Workspace) ImagePath() string {
	return w.imagePath
}

func (w *Workspace) MountPath() string {
	return w.mountPath
}

func (w *Workspace) Mount() error {
	return w.mount()
}

func (w *Workspace) Unmount() error {
	return w.unmount()
}

func (w *Workspace) createImage() error {
	if err := os.Remove(w.imagePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove old image: %w", err)
	}

	cmd := exec.Command("dd", "if=/dev/zero", "of="+w.imagePath, "bs=1M", fmt.Sprintf("count=%d", w.sizeMB))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("dd: %w: %s", err, out)
	}

	cmd = exec.Command("mke2fs", "-t", "ext4", "-F", w.imagePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mke2fs: %w: %s", err, out)
	}

	return nil
}

func (w *Workspace) mount() error {
	if err := os.MkdirAll(w.mountPath, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	cmd := exec.Command("sudo", "mount", "-o", "loop", w.imagePath, w.mountPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mount: %w: %s", err, out)
	}

	return nil
}

func (w *Workspace) unmount() error {
	cmd := exec.Command("sudo", "umount", w.mountPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("unmount: %w: %s", err, out)
	}

	os.Remove(w.mountPath)
	return nil
}

func (w *Workspace) copyFiles(worktreePath string) error {
	cmd := exec.Command("sudo", "cp", "-a", worktreePath+"/.", w.mountPath+"/")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cp: %w: %s", err, out)
	}

	return nil
}

func (w *Workspace) ExtractChanges(hostWorktreePath string) error {
	if err := w.mount(); err != nil {
		return fmt.Errorf("mount for extract: %w", err)
	}

	cmd := exec.Command("rsync", "-a", "--exclude=.magos/", w.mountPath+"/", hostWorktreePath+"/")
	if out, err := cmd.CombinedOutput(); err != nil {
		w.unmount()
		return fmt.Errorf("rsync: %w: %s", err, out)
	}

	return w.unmount()
}

func (w *Workspace) Cleanup() error {
	os.Remove(w.imagePath)
	return nil
}
