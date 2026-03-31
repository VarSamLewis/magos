package sandbox

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

type Sandbox struct {
	cfg       *Config
	logger    *slog.Logger
	worktree  string
	workspace *Workspace
	network   *Network
	vm        *VM
}

func NewSandbox(logger *slog.Logger, cfg *Config) *Sandbox {
	return &Sandbox{
		cfg:    cfg,
		logger: logger,
	}
}

func (s *Sandbox) Start(worktreePath, prompt, apiKey string) error {
	s.logger.Info("starting sandbox", "worktree", worktreePath)

	s.worktree = worktreePath

	s.logger.Info("creating workspace image")
	workspace, err := NewWorkspace(worktreePath, s.cfg.WorkspaceMB)
	if err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	s.workspace = workspace

	if err := s.writeEnvironment(prompt, apiKey); err != nil {
		s.workspace.Cleanup()
		return fmt.Errorf("write environment: %w", err)
	}

	s.logger.Info("setting up network")
	network, err := NewNetwork(s.cfg.NetworkBridge, s.cfg.NetworkIP, s.cfg.VMIP)
	if err != nil {
		s.workspace.Cleanup()
		return fmt.Errorf("setup network: %w", err)
	}
	s.network = network

	s.logger.Info("starting VM")
	vm := NewVM(s.cfg, s.workspace.ImagePath())
	if err := vm.Start(); err != nil {
		network.Cleanup()
		s.workspace.Cleanup()
		return fmt.Errorf("start VM: %w", err)
	}
	s.vm = vm

	return nil
}

func (s *Sandbox) writeEnvironment(prompt, apiKey string) error {
	if err := s.workspace.Mount(); err != nil {
		return fmt.Errorf("mount workspace: %w", err)
	}
	defer s.workspace.Unmount()

	envPath := s.workspace.MountPath() + "/.magos-env"
	promptPath := s.workspace.MountPath() + "/.magos-prompt"

	envContent := fmt.Sprintf("MAGOS_ANTHROPIC_API_KEY=%s\n", apiKey)
	if err := s.writeFile(envPath, envContent); err != nil {
		return fmt.Errorf("write env: %w", err)
	}

	if err := s.writeFile(promptPath, prompt); err != nil {
		return fmt.Errorf("write prompt: %w", err)
	}

	return nil
}

func (s *Sandbox) writeFile(path, content string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

func (s *Sandbox) Wait(timeout time.Duration) error {
	s.logger.Info("waiting for VM", "timeout", timeout)
	return s.vm.Wait(timeout)
}

func (s *Sandbox) ExtractChanges() error {
	s.logger.Info("extracting changes from workspace")
	return s.workspace.ExtractChanges(s.worktree)
}

func (s *Sandbox) Stop() error {
	s.logger.Info("stopping sandbox")

	if s.vm != nil {
		if err := s.vm.Stop(); err != nil {
			s.logger.Error("stop VM", "error", err)
		}
	}

	if s.network != nil {
		if err := s.network.Cleanup(); err != nil {
			s.logger.Error("cleanup network", "error", err)
		}
	}

	if s.workspace != nil {
		if err := s.workspace.Cleanup(); err != nil {
			s.logger.Error("cleanup workspace", "error", err)
		}
	}

	return nil
}
