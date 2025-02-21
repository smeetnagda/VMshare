package system

import (
	"fmt"
	"log"
	"os/exec"
	"time"
)

// CreateVM initializes a new VM, sets up SSH authentication, and schedules deletion.
func CreateVM(vmName, sshKey string, duration time.Duration) error {
	log.Println("Checking system resources before VM creation...")
	// (You already have resource check logic in `CheckResources`)

	log.Printf("Attempting to create VM: %s\n", vmName)
	cmd := exec.Command("VBoxManage", "createvm", "--name", vmName, "--register")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create VM: %v", err)
	}

	// Configure VM with SSH access
	if err := setupSSH(vmName, sshKey); err != nil {
		return fmt.Errorf("failed to set up SSH: %v", err)
	}

	log.Println("VM created successfully!")
	
	// Schedule deletion
	go scheduleDeletion(vmName, duration)
	return nil
}

// setupSSH configures SSH access by injecting the renter’s SSH public key.
func setupSSH(vmName, sshKey string) error {
	// Assuming the VM OS is a Linux variant, we configure SSH keys
	log.Printf("Setting up SSH access for VM: %s\n", vmName)

	// Enable SSH and configure authorized keys
	cmds := [][]string{
		{"VBoxManage", "modifyvm", vmName, "--nic1", "nat"},
		{"VBoxManage", "modifyvm", vmName, "--natpf1", "guestssh,tcp,,2222,,22"},
		{"VBoxManage", "startvm", vmName, "--type", "headless"},
	}

	for _, cmdArgs := range cmds {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed command: %v", cmdArgs)
		}
	}

	// Inject SSH key inside VM
	authCmd := fmt.Sprintf(`echo "%s" >> ~/.ssh/authorized_keys`, sshKey)
	insertKeyCmd := exec.Command("ssh", "-p", "2222", "user@localhost", authCmd)
	if err := insertKeyCmd.Run(); err != nil {
		return fmt.Errorf("failed to inject SSH key: %v", err)
	}

	log.Println("SSH setup complete!")
	return nil
}

// scheduleDeletion deletes the VM after the rental duration ends.
func scheduleDeletion(vmName string, duration time.Duration) {
	log.Printf("Scheduling deletion of VM: %s in %v\n", vmName, duration)
	time.Sleep(duration)

	log.Printf("Deleting VM: %s\n", vmName)
	if err := DeleteVM(vmName); err != nil {
		log.Printf("Error deleting VM: %v\n", err)
	}
}

// DeleteVM removes the VM.
func DeleteVM(vmName string) error {
	cmd := exec.Command("VBoxManage", "unregistervm", vmName, "--delete")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete VM: %v", err)
	}

	log.Printf("VM %s deleted successfully!\n", vmName)
	return nil
}
