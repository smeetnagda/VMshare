package system

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// runCmd is a helper for exec.Command
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// createCloudInitISO writes user-data and meta-data and builds seed.iso
func createCloudInitISO(vmName, sshKey, outDir string) (string, error) {
	userData := fmt.Sprintf(`#cloud-config
users:
  - name: ubuntu
    ssh-authorized-keys:
      - %s
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
`, sshKey)
	metaData := fmt.Sprintf("instance-id: %s\nlocal-hostname: %s\n", vmName, vmName)

	seedDir := filepath.Join(outDir, "seed-"+vmName)
	if err := os.MkdirAll(seedDir, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(seedDir, "user-data"), []byte(userData), 0644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(seedDir, "meta-data"), []byte(metaData), 0644); err != nil {
		return "", err
	}

	isoPath := filepath.Join(outDir, vmName+"-seed.iso")
	if err := runCmd("mkisofs", "-o", isoPath,
       "-V", "cidata",
       "-J", "-r",
       filepath.Join(seedDir, "user-data"),
      filepath.Join(seedDir, "meta-data")); err != nil {
       return "", fmt.Errorf("mkisofs error: %v", err)
	}
	return isoPath, nil
}

// StartVM launches a QEMU ARM64 VM with SSH port forwarding.
func StartVM(vmName, sshKey string, duration time.Duration) error {
    imagePath := filepath.Join(os.Getenv("HOME"), "qemu-images", "ubuntu-24.04-server-arm64.img")
    if _, err := os.Stat(imagePath); err != nil {
        return fmt.Errorf("cloud image not found at %s", imagePath)
    }

    // Use /tmp/vmrentals as our work area so it's easy to tail.
    workDir := filepath.Join("/tmp", "vmrentals", vmName)
    if err := os.MkdirAll(workDir, 0755); err != nil {
        return err
    }

    // 1) cloud-init ISO...
    seedISO, err := createCloudInitISO(vmName, sshKey, workDir)
    if err != nil {
        return err
    }
    log.Printf("✅ Generated cloud-init ISO: %s", seedISO)

    // 2) qcow2 overlay...
    vmDisk := filepath.Join(workDir, vmName+".qcow2")
    if err := runCmd("qemu-img", "create", "-f", "qcow2", "-b", imagePath, "-F", "raw", vmDisk); err != nil {
        return fmt.Errorf("qemu-img error: %v", err)
    }

    // 3) Launch QEMU with monitor & serial log
    serialLog := filepath.Join(workDir, "serial.log")
    qemuArgs := []string{
        "-machine", "virt,accel=hvf",
        "-cpu", "host",
        "-smp", "2",
        "-m", "2048",
		"-bios", filepath.Join(os.Getenv("HOMEBREW_PREFIX"), "share", "qemu", "edk2-aarch64-code.fd"),
        "-monitor", "tcp:127.0.0.1:4444,server,nowait",
        "-serial", "file:" + serialLog,       // will capture Linux serial console once it starts

        "-display", "curses",                 // show the VGA console (UEFI shell & kernel)
 
        "-drive", "file=" + vmDisk + ",if=virtio,format=qcow2",
        "-drive", "file=" + seedISO + ",if=ide,media=cdrom,readonly=on",
		"-boot", "order=d",
        "-netdev", "user,id=net0,hostfwd=tcp::2222-:22",
        "-device", "virtio-net-pci,netdev=net0",
        "-nographic",
    }

    cmd := exec.Command("qemu-system-aarch64", qemuArgs...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    log.Printf("🚀 Starting QEMU VM %s...", vmName)
    if err := cmd.Start(); err != nil {
        return fmt.Errorf("failed to launch QEMU: %v", err)
    }

    // 4) auto‐shutdown after duration...
    go func() {
        time.Sleep(duration)
        log.Printf("⏰ Time expired, stopping VM %s...", vmName)
        cmd.Process.Kill()
    }()

    // 5) let the VM run
    if err := cmd.Wait(); err != nil {
        log.Printf("❗ QEMU exited: %v", err)
    }
    log.Printf("✅ VM %s stopped; cleaning up %s", vmName, workDir)
    return nil
}


// DeleteVM cleans up the temporary directory for a VM.
func DeleteVM(vmName string) error {
	workDir := filepath.Join(os.TempDir(), "vmrentals", vmName)
	log.Printf("🗑️ Cleaning up %s...", workDir)
	return os.RemoveAll(workDir)
}
