package sandbox

import (
	"os"
	"path/filepath"
)

type Config struct {
	AssetsDir  string
	KernelPath string
	RootfsPath string

	VMemoryMB   int
	VCPUCount   int
	WorkspaceMB int
	Timeout     int

	NetworkBridge string
	NetworkIP     string
	VMIP          string
}

func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	assetsDir := filepath.Join(homeDir, ".config", "magos", "assets")

	return &Config{
		AssetsDir:     assetsDir,
		KernelPath:    filepath.Join(assetsDir, "vmlinux"),
		RootfsPath:    filepath.Join(assetsDir, "rootfs.ext4"),
		VMemoryMB:     2048,
		VCPUCount:     2,
		WorkspaceMB:   4096,
		Timeout:       30 * 60,
		NetworkBridge: "br0",
		NetworkIP:     "172.100.0.1",
		VMIP:          "172.100.0.2",
	}
}
