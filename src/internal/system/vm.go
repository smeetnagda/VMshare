package system

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// runVBoxCommand executes a VirtualBox command and handles errors.
func runVBoxCommand(args ...string) error {
	cmd := exec.Command("VBoxManage", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("VBoxManage error: %s, output: %s", err, output)
	}
	return nil
}

// InstallSSHInsideVM ensures SSH is installed and started in the guest OS.
func InstallSSHInsideVM(vmName string) error {
	log.Printf("Ensuring SSH is installed in the guest OS of VM: %s\n", vmName)
	// Wait for VirtualBox guest session to be ready.
	for i := 0; i < 10; i++ {
		cmd := exec.Command("VBoxManage", "guestcontrol", vmName, "run",
			"--exe", "/bin/echo", "--username", "ubuntu", "--password", "ubuntu",
			"--", "/bin/echo", "guest session ready")
		err := cmd.Run()
		if err == nil {
			log.Println("Guest session is available.")
			break
		}
		log.Println("Guest session not available yet, retrying in 5s...")
		time.Sleep(5 * time.Second)
	}

	// Run commands inside the VM to install SSH.
	commands := []string{
		"sudo apt update",
		"sudo apt install -y openssh-server",
		"sudo systemctl enable ssh",
		"sudo systemctl start ssh",
	}

	for _, cmdStr := range commands {
		err := exec.Command("VBoxManage", "guestcontrol", vmName, "run",
			"--exe", "/bin/sh", "--username", "ubuntu", "--password", "ubuntu",
			"--", "/bin/sh", "-c", cmdStr).Run()
		if err != nil {
			return fmt.Errorf("failed to execute command in VM: %s, error: %v", cmdStr, err)
		}
	}
	log.Println("SSH successfully installed and started inside VM!")
	return nil
}

// configureGraphics sets the appropriate graphics controller for the VM.
func configureGraphics(vmName string) error {
	log.Println("Configuring graphics for VM...")
	var graphicsController string
	if runtime.GOARCH == "arm64" {
		graphicsController = "vmsvga" // Apple Silicon
	} else {
		graphicsController = "vboxsvga" // x86_64 (Intel/AMD)
	}
	err := runVBoxCommand("modifyvm", vmName, "--graphicscontroller", graphicsController)
	if err != nil {
		return fmt.Errorf("failed to set graphics controller: %v", err)
	}
	log.Println("Graphics configuration complete!")
	return nil
}

// WaitForSSH waits until the SSH server is available on localhost:2222.
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

// InjectSSHKey copies the provided SSH key into the guest VM's authorized_keys.
func InjectSSHKey(vmName, sshKey string) error {
	log.Printf("Injecting SSH Key into VM: %s\n", vmName)
	// Wait for SSH to be ready.
	err := WaitForSSH()
	if err != nil {
		return fmt.Errorf("failed to inject SSH key: SSH not available: %v", err)
	}
	// Copy SSH Key to VM's authorized_keys file using SSH.
	authCmd := fmt.Sprintf(`echo "%s" | ssh -p 2222 ubuntu@localhost 'mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys'`, sshKey)
	insertKeyCmd := exec.Command("bash", "-c", authCmd)
	output, err := insertKeyCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to inject SSH key: %v, output: %s", err, output)
	}
	log.Println("SSH Key successfully added to VM!")
	return nil
}

// DeleteVM forcefully removes a VM and its files.
func DeleteVM(vmName string) error {
	log.Printf("Deleting VM: %s\n", vmName)
	// Try to power off the VM first (in case it's still running).
	_ = exec.Command("VBoxManage", "controlvm", vmName, "poweroff").Run()
	// Try to unregister the VM.
	cmd := exec.Command("VBoxManage", "unregistervm", vmName, "--delete")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: Could not unregister VM %s: %v, output: %s\n", vmName, err, string(output))
	}
	// Remove any remaining files manually.
	vmPath := filepath.Join(os.Getenv("HOME"), "VirtualBox VMs", vmName)
	if _, err := os.Stat(vmPath); !os.IsNotExist(err) {
		log.Printf("Cleaning up VM directory: %s\n", vmPath)
		err := os.RemoveAll(vmPath)
		if err != nil {
			return fmt.Errorf("failed to remove VM directory: %v", err)
		}
	}
	// Ensure the VM is fully unlocked before proceeding.
	time.Sleep(2 * time.Second)
	log.Println("VM deleted successfully!")
	return nil
}

// InstallGuestAdditions attaches the Guest Additions ISO and runs its installer.
// It automatically creates the IDE controller (and handles locked sessions) if needed.
func InstallGuestAdditions(vmName string) error {
	log.Println("Installing VirtualBox Guest Additions...")
	const guestAdditionsISO = "/Applications/VirtualBox.app/Contents/MacOS/VBoxGuestAdditions.iso"

	// Attempt to attach Guest Additions ISO.
	err := runVBoxCommand("storageattach", vmName,
		"--storagectl", "IDE",
		"--port", "1", "--device", "0",
		"--type", "dvddrive",
		"--medium", guestAdditionsISO)
	if err != nil {
		// Handle missing IDE controller.
		if strings.Contains(err.Error(), "Could not find a controller named") {
			log.Println("IDE storage controller not found. Creating the controller automatically...")
			errCtrl := runVBoxCommand("storagectl", vmName, "--add", "ide", "--name", "IDE")
			if errCtrl != nil {
				// If VM is locked, power it off, add the controller, and then restart.
				if strings.Contains(errCtrl.Error(), "already locked for a session") {
					log.Println("VM is locked for a session. Powering off VM to modify storage controllers...")
					if errPO := exec.Command("VBoxManage", "controlvm", vmName, "poweroff").Run(); errPO != nil {
						return fmt.Errorf("failed to power off VM: %v", errPO)
					}
					time.Sleep(2 * time.Second)
					errCtrl = runVBoxCommand("storagectl", vmName, "--add", "ide", "--name", "IDE")
					if errCtrl != nil {
						return fmt.Errorf("failed to add IDE storage controller after powering off VM: %v", errCtrl)
					}
					if errStart := StartVM(vmName); errStart != nil {
						return fmt.Errorf("failed to restart VM after modifying storage controllers: %v", errStart)
					}
				} else {
					return fmt.Errorf("failed to add IDE storage controller: %v", errCtrl)
				}
			}
			// Retry attaching the Guest Additions ISO after adding the controller.
			err = runVBoxCommand("storageattach", vmName,
				"--storagectl", "IDE",
				"--port", "1", "--device", "0",
				"--type", "dvddrive",
				"--medium", guestAdditionsISO)
			if err != nil {
				return fmt.Errorf("failed to attach Guest Additions ISO after adding IDE controller: %v", err)
			}
		} else if strings.Contains(err.Error(), "already locked for a session") {
			// Handle locked state when attaching the ISO.
			log.Println("VM is locked for a session. Powering off VM to attach Guest Additions ISO...")
			if errPO := exec.Command("VBoxManage", "controlvm", vmName, "poweroff").Run(); errPO != nil {
				return fmt.Errorf("failed to power off VM: %v", errPO)
			}
			time.Sleep(2 * time.Second)
			err = runVBoxCommand("storageattach", vmName,
				"--storagectl", "IDE",
				"--port", "1", "--device", "0",
				"--type", "dvddrive",
				"--medium", guestAdditionsISO)
			if err != nil {
				return fmt.Errorf("failed to attach Guest Additions ISO after powering off VM: %v", err)
			}
			if errStart := StartVM(vmName); errStart != nil {
				return fmt.Errorf("failed to restart VM after ISO attachment: %v", errStart)
			}
		} else {
			return fmt.Errorf("failed to attach Guest Additions ISO: %v", err)
		}
	}
	log.Println("Guest Additions ISO attached. Running installation script...")
	// Run installation script inside the guest.
	err = runVBoxCommand("guestcontrol", vmName, "run",
		"--exe", "/bin/sh", "--username", "ubuntu", "--password", "ubuntu",
		"--", "/bin/sh", "-c", "sudo mount /dev/cdrom /mnt && sudo /mnt/VBoxLinuxAdditions.run")
	if err != nil {
		return fmt.Errorf("failed to install Guest Additions: %v", err)
	}
	log.Println("Guest Additions installed successfully!")
	return nil
}

// configureNetworking sets up NAT networking and port forwarding (host:2222 -> guest:22).
func configureNetworking(vmName string) error {
	log.Println("Configuring networking for VM...")
	err := runVBoxCommand("modifyvm", vmName, "--nic1", "nat")
	if err != nil {
		return fmt.Errorf("failed to set NAT networking: %v", err)
	}
	err = runVBoxCommand("modifyvm", vmName, "--natpf1", "guestssh,tcp,,2222,,22")
	if err != nil {
		return fmt.Errorf("failed to configure SSH port forwarding: %v", err)
	}
	log.Println("Networking configured successfully!")
	return nil
}

// ManageVM creates or deletes a VM and configures it, handling locked states.
func ManageVM(vmName string, action string) error {
	switch action {
	case "create":
		log.Printf("Attempting to create VM: %s\n", vmName)
		// Ensure the VM does not already exist.
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
		// Create the VM.
		err = runVBoxCommand("createvm", "--name", vmName, "--register")
		if err != nil {
			return fmt.Errorf("failed to create VM: %v", err)
		}
		// Configure Graphics.
		err = configureGraphics(vmName)
		if err != nil {
			return fmt.Errorf("failed to configure graphics: %v", err)
		}
		// Configure Networking.
		err = configureNetworking(vmName)
		if err != nil {
			return fmt.Errorf("failed to configure networking: %v", err)
		}
		// Start the VM.
		err = StartVM(vmName)
		if err != nil {
			return fmt.Errorf("failed to start VM: %v", err)
		}
		// Install Guest Additions.
		err = InstallGuestAdditions(vmName)
		if err != nil {
			return fmt.Errorf("failed to install Guest Additions: %v", err)
		}
		// Ensure SSH is installed inside the VM.
		err = InstallSSHInsideVM(vmName)
		if err != nil {
			return fmt.Errorf("failed to install SSH: %v", err)
		}
		// Wait for SSH to be ready.
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

// StartVM checks if the VM is running and starts it, handling locked states.
func StartVM(vmName string) error {
	log.Printf("Starting VM: %s\n", vmName)
	// Check if the VM is already running.
	output, err := exec.Command("VBoxManage", "list", "runningvms").Output()
	if err == nil && strings.Contains(string(output), vmName) {
		log.Printf("VM %s is already running.\n", vmName)
		return nil
	}
	// If VM might be locked, attempt to power it off.
	err = exec.Command("VBoxManage", "controlvm", vmName, "poweroff").Run()
	if err != nil {
		log.Printf("No need to power off: VM %s was not running.\n", vmName)
	}
	// Start the VM in headless mode.
	cmd := exec.Command("VBoxManage", "startvm", vmName, "--type", "headless")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start VM: %v", err)
	}
	log.Printf("VM %s started successfully.\n", vmName)
	return nil
}
