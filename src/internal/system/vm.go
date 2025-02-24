package system

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
	"os"
	"path/filepath"
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

	// Copy SSH Key to VM's authorized_keys file using SSH
	authCmd := fmt.Sprintf(`echo "%s" >> ~/.ssh/authorized_keys`, sshKey)
	insertKeyCmd := exec.Command("ssh", "-p", "2222", "ubuntu@localhost", authCmd)
	if err := insertKeyCmd.Run(); err != nil {
		return fmt.Errorf("failed to inject SSH key: %v", err)
	}

	log.Println("SSH Key successfully added to VM!")
	return nil
}

// DeleteVM forcefully removes a VM and all its files
func DeleteVM(vmName string) error {
	log.Printf("Deleting VM: %s\n", vmName)

	// Unregister the VM first
	cmd := exec.Command("VBoxManage", "unregistervm", vmName, "--delete")
	if err := cmd.Run(); err != nil {
		log.Printf("Warning: Could not unregister VM %s: %v\n", vmName, err)
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

	log.Println("VM deleted successfully!")
	return nil
}

func configureNetworking(vmName string) error {
	log.Println("Configuring networking for VM...")

	// Enable NAT networking with port forwarding for SSH (host:2222 -> guest:22)
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



// ManageVM creates or deletes a VM while handling errors like locked states
func ManageVM(vmName string, action string) error {
	switch action {
	case "create":
		log.Printf("Attempting to create VM: %s\n", vmName)

		// Check if the VM exists
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

		// Create VM
		err = runVBoxCommand("createvm", "--name", vmName, "--register")
		if err != nil {
			return fmt.Errorf("failed to create VM: %v", err)
		}

		// Configure VM Networking (NAT with Port Forwarding for SSH)
		err = configureNetworking(vmName)
		if err != nil {
			return fmt.Errorf("failed to configure networking: %v", err)
		}

		// Start the VM
		err = StartVM(vmName)
		if err != nil {
			return fmt.Errorf("failed to start VM: %v", err)
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

	// Check if VM is running
	output, err := exec.Command("VBoxManage", "list", "runningvms").Output()
	if err == nil && strings.Contains(string(output), vmName) {
		log.Printf("VM %s is already running.\n", vmName)
		return nil
	}

	// If VM is locked, force power off
	err = exec.Command("VBoxManage", "controlvm", vmName, "poweroff").Run()
	if err != nil {
		log.Printf("No need to power off: VM %s was not running.\n", vmName)
	}

	// Start VM
	err = exec.Command("VBoxManage", "startvm", vmName, "--type", "headless").Run()
	if err != nil {
		return fmt.Errorf("failed to start VM: %v", err)
	}
	log.Printf("VM %s started successfully.\n", vmName)
	return nil
}

// WaitForSSH waits for SSH to become available on the VM
