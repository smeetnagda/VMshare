package system

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
	"os"
	"path/filepath"
	"runtime"
)
// runVBoxCommand executes a VirtualBox command and handles errors
func runVBoxCommand(args ...string) error {
	cmd := exec.Command("VBoxManage", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("VBoxManage error: %s, output: %s", err, output)
	}
	return nil
}

func InstallSSHInsideVM(vmName string) error {
	log.Printf("Ensuring SSH is installed in the guest OS of VM: %s\n", vmName)

	// Wait for VirtualBox guest session to be ready
	for i := 0; i < 10; i++ {
		cmd := exec.Command("VBoxManage", "guestcontrol", vmName, "run", "--exe", "/bin/echo", "--username", "ubuntu", "--password", "ubuntu", "--", "/bin/echo", "guest session ready")
		err := cmd.Run()
		if err == nil {
			log.Println("Guest session is available.")
			break
		}
		log.Println("Guest session not available yet, retrying in 5s...")
		time.Sleep(5 * time.Second)
	}

	// Run commands inside the VM to install SSH
	commands := []string{
		"sudo apt update",
		"sudo apt install -y openssh-server",
		"sudo systemctl enable ssh",
		"sudo systemctl start ssh",
	}

	for _, cmd := range commands {
		err := exec.Command("VBoxManage", "guestcontrol", vmName, "run",
			"--exe", "/bin/sh", "--username", "ubuntu", "--password", "ubuntu",
			"--", "/bin/sh", "-c", cmd).Run()
		if err != nil {
			return fmt.Errorf("failed to execute command in VM: %s, error: %v", cmd, err)
		}
	}

	log.Println("SSH successfully installed and started inside VM!")
	return nil
}

func configureGraphics(vmName string) error {
	log.Println("Configuring graphics for VM...")

	// Use VMSVGA for Apple Silicon, VBoxSVGA for x86, fallback to None
	var graphicsController string
	if runtime.GOARCH == "arm64" {
		graphicsController = "vmsvga" // Apple Silicon
	} else {
		graphicsController = "vboxsvga" // Intel/AMD
	}

	err := runVBoxCommand("modifyvm", vmName, "--graphicscontroller", graphicsController)
	if err != nil {
		log.Printf("⚠️ Graphics controller %s failed, retrying with VBoxVGA", graphicsController)
		_ = runVBoxCommand("modifyvm", vmName, "--graphicscontroller", "vboxvga")
	}

	log.Println("✅ Graphics configuration complete!")
	return nil
}

func WaitForSSH() error {
	log.Println("Waiting for SSH server to be available on localhost:2222...")

	for i := 0; i < 10; i++ {
		cmd := exec.Command("nc", "-zv", "localhost", "2222")
		err := cmd.Run()
		if err == nil {
			log.Println("SSH server is available on localhost:2222")
			return nil
		}
		log.Println("SSH not available yet, retrying in 5s...")
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("SSH did not become available in time")
}


func InjectSSHKey(vmName, sshKey string) error {
	log.Printf("Injecting SSH Key into VM: %s\n", vmName)

	// Wait for SSH to be ready
	err := WaitForSSH()
	if err != nil {
		return fmt.Errorf("failed to inject SSH key: SSH not available: %v", err)
	}

	// Copy SSH Key to VM's authorized_keys file using SSH
	authCmd := fmt.Sprintf(`echo "%s" | ssh -p 2222 ubuntu@localhost 'mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys'`, sshKey)
	insertKeyCmd := exec.Command("bash", "-c", authCmd)
	output, err := insertKeyCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to inject SSH key: %v, output: %s", err, output)
	}

	log.Println("SSH Key successfully added to VM!")
	return nil
}


// DeleteVM forcefully removes a VM and all its files
func DeleteVM(vmName string) error {
	log.Printf("Deleting VM: %s\n", vmName)

	// Try to power off the VM first (in case it's still running)
	_ = exec.Command("VBoxManage", "controlvm", vmName, "poweroff").Run()

	// Try to unregister the VM
	cmd := exec.Command("VBoxManage", "unregistervm", vmName, "--delete")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: Could not unregister VM %s: %v, output: %s\n", vmName, err, string(output))
	}

	// Remove any remaining files manually
	vmPath := filepath.Join(os.Getenv("HOME"), "VirtualBox VMs", vmName)
	if _, err := os.Stat(vmPath); !os.IsNotExist(err) {
		log.Printf("Cleaning up VM directory: %s\n", vmPath)
		err := os.RemoveAll(vmPath)
		if err != nil {
			return fmt.Errorf("failed to remove VM directory: %v", err)
		}
	}

	// Ensure the VM is fully unlocked before proceeding
	time.Sleep(2 * time.Second)

	log.Println("VM deleted successfully!")
	return nil
}

func InstallGuestAdditions(vmName string) error {
    log.Println("Installing VirtualBox Guest Additions...")

    // **Step 1: Power off the VM to release locks**
    log.Println("🔄 Ensuring VM is powered off before modifying storage controllers...")
    _ = exec.Command("VBoxManage", "controlvm", vmName, "poweroff").Run()
    time.Sleep(5 * time.Second) // Allow time for shutdown

    // **Step 2: Release any active VirtualBox session locks**
    log.Println("🔓 Releasing any active VirtualBox session locks...")
    _ = exec.Command("VBoxManage", "closemedium", "disk", "all", "--delete").Run()
    time.Sleep(3 * time.Second)

    // **Step 3: Ensure the IDE storage controller exists**
    log.Println("🛠 Checking or creating IDE controller...")
    err := runVBoxCommand("storagectl", vmName, "--name", "IDE", "--add", "ide")
    if err != nil && strings.Contains(err.Error(), "already locked for a session") {
        log.Println("⚠️ VM is locked. Retrying after forced power off...")
        _ = exec.Command("VBoxManage", "controlvm", vmName, "poweroff").Run()
        time.Sleep(5 * time.Second)

        err = runVBoxCommand("storagectl", vmName, "--name", "IDE", "--add", "ide")
        if err != nil {
            return fmt.Errorf("failed to add IDE controller: %v", err)
        }
    }

    // **Step 4: Attach Guest Additions ISO**
    guestAdditionsISO := "/Applications/VirtualBox.app/Contents/MacOS/VBoxGuestAdditions.iso"
    err = runVBoxCommand("storageattach", vmName, "--storagectl", "IDE",
        "--port", "1", "--device", "0", "--type", "dvddrive", "--medium", guestAdditionsISO)
    if err != nil {
        return fmt.Errorf("failed to attach Guest Additions ISO: %v", err)
    }

    log.Println("✅ Guest Additions ISO attached. Running installation script...")

    // **Step 5: Restart the VM before running the installation**
    log.Println("🔄 Restarting VM before running Guest Additions installation...")
    startErr := exec.Command("VBoxManage", "startvm", vmName, "--type", "headless").Run()
    if startErr != nil {
        return fmt.Errorf("failed to start VM after modifications: %v", startErr)
    }

    // **Step 6: Run Guest Additions installer inside the VM**
    err = runVBoxCommand("guestcontrol", vmName, "run",
        "--exe", "/bin/sh", "--username", "ubuntu", "--password", "ubuntu",
        "--", "/bin/sh", "-c", "sudo mount /dev/cdrom /mnt && sudo /mnt/VBoxLinuxAdditions.run")
    if err != nil {
        return fmt.Errorf("failed to install Guest Additions: %v", err)
    }

    log.Println("✅ Guest Additions installed successfully!")
    return nil
}



func vmExists(vmName string) (bool, error) {
    output, err := exec.Command("VBoxManage", "list", "vms").CombinedOutput()
    if err != nil {
        return false, fmt.Errorf("failed to list VMs: %v", err)
    }
    return strings.Contains(string(output), fmt.Sprintf("\"%s\"", vmName)), nil
}

func configureNetworking(vmName string) error {
    log.Println("Configuring networking for VM...")

    // Set NAT for internet access (default networking mode)
    err := runVBoxCommand("modifyvm", vmName, "--nic1", "nat")
    if err != nil {
        return fmt.Errorf("failed to set NAT networking: %v", err)
    }

    // REMOVE host-only adapter (vboxnet0) if present
    err = runVBoxCommand("modifyvm", vmName, "--nic2", "none")
    if err != nil {
        log.Println("Warning: Could not remove host-only adapter (vboxnet0), continuing...")
    }

    // Set up port forwarding for SSH access
    err = runVBoxCommand("modifyvm", vmName, "--natpf1", "guestssh,tcp,,2222,,22")
    if err != nil {
        return fmt.Errorf("failed to configure SSH port forwarding: %v", err)
    }

    log.Println("✅ Networking configured successfully!")
    return nil
}

func ensureVMDeleted(vmName string) error {
    exists, _ := vmExists(vmName)
    if !exists {
        return nil
    }

    log.Printf("Deleting VM: %s", vmName)

    // Power off if running
    _ = runVBoxCommand("controlvm", vmName, "poweroff")
    time.Sleep(2 * time.Second) // Allow shutdown

    // Unregister and delete VM
    _ = runVBoxCommand("unregistervm", vmName, "--delete")

    // Remove any leftover files
    vmPath := filepath.Join(os.Getenv("HOME"), "VirtualBox VMs", vmName)
    _ = os.RemoveAll(vmPath)

    log.Println("✅ VM deleted successfully!")
    return nil
}



// ManageVM creates or deletes a VM while handling errors like locked states
func ManageVM(vmName string, action string) error {
	switch action {
	case "create":
		log.Printf("Attempting to create VM: %s\n", vmName)

		// Ensure the VM does not exist
		existingVMs, err := exec.Command("VBoxManage", "list", "vms").Output()
		if err != nil {
			return fmt.Errorf("failed to list VMs: %v", err)
		}

		if strings.Contains(string(existingVMs), vmName) {
			log.Printf("VM with name %s already exists. Deleting it before creating a new one.\n", vmName)
			if err := DeleteVM(vmName); err != nil {
				return fmt.Errorf("failed to delete existing VM: %v", err)
			}
		}

		// Create the VM
		err = runVBoxCommand("createvm", "--name", vmName, "--register")
		if err != nil {
			return fmt.Errorf("failed to create VM: %v", err)
		}

		// Configure Graphics
		err = configureGraphics(vmName)
		if err != nil {
			return fmt.Errorf("failed to configure graphics: %v", err)
		}

		// Configure Networking
		err = configureNetworking(vmName)
		if err != nil {
			return fmt.Errorf("failed to configure networking: %v", err)
		}

		// Start the VM
		err = StartVM(vmName)
		if err != nil {
			return fmt.Errorf("failed to start VM: %v", err)

		}
		err = InstallGuestAdditions(vmName)
		if err != nil {
			return fmt.Errorf("failed to install Guest Additions: %v", err)
		}

		// Ensure SSH is installed inside the VM
		err = InstallSSHInsideVM(vmName)
		if err != nil {
			return fmt.Errorf("failed to install SSH: %v", err)
		}

		// Wait for SSH to be ready
		err = WaitForSSH()
		if err != nil {
			return fmt.Errorf("failed to establish SSH connection: %v", err)
		}

		log.Println("VM is fully configured and ready for access.")
		return nil

	case "delete":
		return DeleteVM(vmName)

	default:
		return fmt.Errorf("invalid action: %s", action)
	}
}



// StartVM attempts to start the VM, handling locked states
func StartVM(vmName string) error {
	log.Printf("Starting VM: %s\n", vmName)

	// Check if the VM is already running
	output, err := exec.Command("VBoxManage", "list", "runningvms").Output()
	if err == nil && strings.Contains(string(output), vmName) {
		log.Printf("VM %s is already running.\n", vmName)
		return nil
	}

	// If VM is locked, power it off first
	exec.Command("VBoxManage", "controlvm", vmName, "poweroff").Run()
	time.Sleep(2 * time.Second) // Allow time for shutdown

	// Try to start the VM and capture detailed output
	cmd := exec.Command("VBoxManage", "startvm", vmName, "--type", "headless")
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Failed to start VM. Error details:\n%s", string(output))
		return fmt.Errorf("failed to start VM: %v", err)
	}

	log.Printf("✅ VM %s started successfully.", vmName)
	return nil
}


// WaitForSSH waits for SSH to become available on the VM
