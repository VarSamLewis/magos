.PHONY: all rootfs vmlinux setup clean

ASSETS_DIR ?= $(HOME)/.config/magos/assets
ROOTFS_FILE := $(ASSETS_DIR)/rootfs.ext4
VMLINUX_FILE := $(ASSETS_DIR)/vmlinux
WORK_DIR := /var/tmp/magos-rootfs-build
SIZE_MB := 2048

all: setup

setup: $(VMLINUX_FILE) $(ROOTFS_FILE)
	@echo "Setup complete! Run ./magos to start."

$(ASSETS_DIR):
	mkdir -p $(ASSETS_DIR)

$(VMLINUX_FILE): $(ASSETS_DIR)
	@echo "Downloading Linux kernel..."
	curl -L -o $(VMLINUX_FILE) https://s3.amazonaws.com/spec.ccfc.min/img/hello/kernel/vmlinux

$(WORK_DIR):
	mkdir -p $(WORK_DIR)

$(ROOTFS_FILE): $(WORK_DIR) $(VMLINUX_FILE)
	@echo "Building rootfs..."
	rm -f $(ROOTFS_FILE)
	rm -rf $(WORK_DIR)/*

	@echo "Setting up base rootfs directory..."
	mkdir -p $(WORK_DIR)/rootfs

	@echo "Running pacstrap (Arch Linux)..."
	sudo pacstrap -c -M $(WORK_DIR)/rootfs \
		base \
		base-devel \
		git \
		go \
		openssh \
		busybox \
		iproute2 \
		curl \
		wget

	@echo "Copying Magos binary..."
	sudo cp ./magos $(WORK_DIR)/rootfs/usr/bin/magos || true

	@echo "Creating init script..."
	{ \
		echo '#!/bin/sh'; \
		echo 'mount -t proc proc /proc'; \
		echo 'mount -t sysfs sys /sys'; \
		echo 'mount -t devtmpfs dev /dev'; \
		echo 'ip link set eth0 up'; \
		echo 'ip addr add 172.100.0.2/24 dev eth0'; \
		echo 'ip route add default via 172.100.0.1'; \
		echo '[ -f /mnt/workspace/.magos-env ] && source /mnt/workspace/.magos-env'; \
		echo 'mount -t ext4 /dev/vdb /mnt/workspace || true'; \
		echo 'mkdir -p /mnt/workspace'; \
		echo 'cd /mnt/workspace'; \
		echo 'chmod 600 .magos-env 2>/dev/null || true'; \
		echo '/usr/bin/magos --sandbox-mode --prompt-file /mnt/workspace/.magos-prompt'; \
		echo 'poweroff -f'; \
	} | sudo tee $(WORK_DIR)/rootfs/init > /dev/null
	sudo chmod +x $(WORK_DIR)/rootfs/init

	@echo "Creating disk image..."
	dd if=/dev/zero of=$(ROOTFS_FILE) bs=1M count=$(SIZE_MB)
	mke2fs -t ext4 -F $(ROOTFS_FILE)

	@echo "Copying rootfs to image..."
	sudo mkdir -p $(WORK_DIR)/mount
	sudo mount -o loop $(ROOTFS_FILE) $(WORK_DIR)/mount
	sudo cp -a $(WORK_DIR)/rootfs/. $(WORK_DIR)/mount/
	sudo umount $(WORK_DIR)/mount

	@echo "Cleaning up..."
	sudo rm -rf $(WORK_DIR)

	@echo "Rootfs built at $(ROOTFS_FILE)"

clean:
	sudo rm -rf $(WORK_DIR)
	rm -f $(ROOTFS_FILE)
	rm -f $(VMLINUX_FILE)
