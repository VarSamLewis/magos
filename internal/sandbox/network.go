package sandbox

import (
	"fmt"
	"os/exec"
	"strings"
)

type Network struct {
	tapDevice string
	bridge    string
	vmIP      string
	hostIP    string
}

func NewNetwork(bridge, hostIP, vmIP string) (*Network, error) {
	n := &Network{
		tapDevice: "tap-magos",
		bridge:    bridge,
		vmIP:      vmIP,
		hostIP:    hostIP,
	}

	if err := n.createTapDevice(); err != nil {
		return nil, err
	}

	if err := n.addToBridge(); err != nil {
		n.cleanupTapDevice()
		return nil, err
	}

	if err := n.configureHostIP(); err != nil {
		n.cleanupTapDevice()
		return nil, err
	}

	return n, nil
}

func (n *Network) TAPDevice() string {
	return n.tapDevice
}

func (n *Network) createTapDevice() error {
	cmd := exec.Command("sudo", "ip", "tuntap", "add", "dev", n.tapDevice, "mode", "tap")
	if out, err := cmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(out), "File exists") {
			return fmt.Errorf("create tap: %w: %s", err, out)
		}
	}

	cmd = exec.Command("sudo", "ip", "link", "set", n.tapDevice, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("set tap up: %w: %s", err, out)
	}

	return nil
}

func (n *Network) addToBridge() error {
	if n.bridge == "" {
		return nil
	}

	cmd := exec.Command("sudo", "ip", "link", "set", n.tapDevice, "master", n.bridge)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("add to bridge: %w: %s", err, out)
	}

	return nil
}

func (n *Network) configureHostIP() error {
	cmd := exec.Command("sudo", "ip", "addr", "add", n.hostIP+"/24", "dev", n.tapDevice)
	if out, err := cmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(out), "File exists") {
			return fmt.Errorf("add IP: %w: %s", err, out)
		}
	}

	return nil
}

func (n *Network) Cleanup() error {
	n.cleanupTapDevice()
	return nil
}

func (n *Network) cleanupTapDevice() error {
	cmd := exec.Command("sudo", "ip", "link", "del", n.tapDevice)
	if out, err := cmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(out), "No such device") {
			return fmt.Errorf("delete tap: %w: %s", err, out)
		}
	}
	return nil
}
