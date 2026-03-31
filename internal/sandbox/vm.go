package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type VM struct {
	cfg        *Config
	workspace  string
	cmd        *exec.Cmd
	ctx        context.Context
	cancel     context.CancelFunc
	jailerRoot string
	pidFile    string
}

func NewVM(cfg *Config, workspacePath string) *VM {
	ctx, cancel := context.WithCancel(context.Background())
	return &VM{
		cfg:        cfg,
		workspace:  workspacePath,
		ctx:        ctx,
		cancel:     cancel,
		jailerRoot: "/var/run/magos-jailer",
		pidFile:    "/tmp/magos-firecracker.pid",
	}
}

func (v *VM) Start() error {
	if err := os.MkdirAll(v.jailerRoot, 0755); err != nil {
		return fmt.Errorf("create jailer root: %w", err)
	}

	cfgPath := filepath.Join(v.jailerRoot, "firecracker.config.json")
	if err := v.writeConfig(cfgPath); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	firecrackerBin, err := exec.LookPath("firecracker")
	if err != nil {
		return fmt.Errorf("firecracker not found: %w", err)
	}

	cmd := exec.CommandContext(v.ctx, firecrackerBin,
		"--config-file", cfgPath,
		"--api-sock", filepath.Join(v.jailerRoot, "firecracker.sock"),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start firecracker: %w", err)
	}

	v.cmd = cmd
	return nil
}

func (v *VM) writeConfig(path string) error {
	configContent := fmt.Sprintf(`{
		"boot-source": {
			"kernel-image-path": %q,
			"initrd-path": "",
			"boot-args": "console=ttyS0 reboot=k panic=1 pci=off"
		},
		"machine-config": {
			"vcpu-count": %d,
			"mem-size-mib": %d
		},
		"drives": [
			{
				"drive-id": "rootfs",
				"path-on-host": %q,
				"is-root-device": true,
				"is-read-only": true
			},
			{
				"drive-id": "workspace",
				"path-on-host": %q,
				"is-root-device": false,
				"is-read-only": false
			}
		],
		"network-interfaces": [
			{
				"guest-device-name": "eth0",
				"host-dev-name": "tap-magos"
			}
		]
	}`, v.cfg.KernelPath, v.cfg.VCPUCount, v.cfg.VMemoryMB, v.cfg.RootfsPath, v.workspace)

	return os.WriteFile(path, []byte(configContent), 0644)
}

func (v *VM) Wait(timeout time.Duration) error {
	done := make(chan error, 1)

	go func() {
		done <- v.cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		v.Stop()
		return fmt.Errorf("VM timed out after %v", timeout)
	case <-v.ctx.Done():
		return v.ctx.Err()
	}
}

func (v *VM) Stop() error {
	v.cancel()

	if v.cmd != nil && v.cmd.Process != nil {
		v.cmd.Process.Signal(os.Interrupt)
		time.Sleep(2 * time.Second)
		if v.cmd.Process != nil {
			v.cmd.Process.Kill()
		}
	}

	os.RemoveAll(v.jailerRoot)
	return nil
}
