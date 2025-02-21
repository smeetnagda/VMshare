package system

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
	"time"
)

// Guest credentials and path to Guest Additions ISO (adjust these as needed)
const guestUsername = "ubuntu"
const guestPassword = "ubuntu"
const guestAdditionsISO = "/Applications/VirtualBox.app/Contents/MacOS/VBoxGuestAdditions.iso"

// CreateVM initializes a new VM, configures its graphics settings, sets up SSH access
// (including provisioning SSH on the guest), and schedules deletion.
func CreateVM(vmName, sshKey string, duration time.Duration) error {
	log.Println("Checking system resources before VM creation...")

	// Check if the VM already exists and delete it if found.
	exists, err := vmExists(vmName)
	if err != nil {
		return fmt.Errorf("failed to check if VM exists: %v", err)
	}
	if exists {
		log.Printf("VM with name %s already exists. Deleting it before creating a new one.\n", vmName)
		if err := DeleteVM(vmName); err != nil {
			return fmt.Errorf("failed to delete existing VM: %v", err)
		}
	}

	log.Printf("Attempting to create VM: %s\n", vmName)
	cmd := exec.Command("VBoxManage", "createvm", "--name", vmName, "--register")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create VM: %v", err)
	}

	// Configure graphics to use VMSVGA and set video RAM.
	if err := configureVMGraphics(vmName); err != nil {
		return fmt.Errorf("failed to configure VM graphics: %v", err)
	}

	// Set up networking, start the VM, provision SSH in the guest, and inject the SSH key.
	if err := setupSSH(vmName, sshKey); err != nil {
		return fmt.Errorf("failed to set up SSH: %v", err)
	}

	log.Println("VM created successfully!")
	go scheduleDeletion(vmName, duration)
	return nil
}

// vmExists checks whether a VM with the given name is already registered.
func vmExists(vmName string) (bool, error) {
	cmd := exec.Command("VBoxManage", "list", "vms")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	// Each line is like: "vmName" {UUID} – search for the quoted VM name.
	searchStr := fmt.Sprintf("\"%s\"", vmName)
	return strings.Contains(string(output), searchStr), nil
}

// configureVMGraphics sets the graphics controller to VMSVGA and reduces video RAM.
func configureVMGraphics(vmName string) error {
	log.Printf("Configuring graphics for VM: %s", vmName)
	cmd := exec.Command("VBoxManage", "modifyvm", vmName, "--graphicscontroller", "VMSVGA")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set graphics controller: %v", err)
	}
	cmd = exec.Command("VBoxManage", "modifyvm", vmName, "--vram", "16")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set video RAM: %v", err)
	}
	log.Println("Graphics configuration complete!")
	return nil
}

// setupSSH configures SSH access by setting up NAT port forwarding, starting the VM,
// waiting for the SSH port to be reachable, ensuring guest SSH is provisioned, and injecting the SSH key.
func setupSSH(vmName, sshKey string) error {
	log.Printf("Setting up SSH access for VM: %s", vmName)
	cmds := [][]string{
		{"VBoxManage", "modifyvm", vmName, "--nic1", "nat"},
		{"VBoxManage", "modifyvm", vmName, "--natpf1", "guestssh,tcp,,2222,,22"},
		{"VBoxManage", "startvm", vmName, "--type", "headless"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed command %v: %v", args, err)
		}
	}

	// Wait for the forwarded SSH port to become available.
	if err := waitForSSH("2222", 24, 5*time.Second); err != nil {
		return fmt.Errorf("SSH server didn't become available: %v", err)
	}

	// Ensure that the guest has SSH installed and provisioned.
	if err := ensureGuestSSHInstalled(vmName, guestUsername, guestPassword); err != nil {
		return fmt.Errorf("failed to provision SSH on guest: %v", err)
	}

	// Inject the SSH public key into the guest's authorized_keys.
	if err := injectSSHKey(sshKey); err != nil {
		return fmt.Errorf("failed to inject SSH key: %v", err)
	}

	log.Println("SSH setup complete!")
	return nil
}

// waitForSSH repeatedly attempts to establish a TCP connection on the given port until it is available.
func waitForSSH(port string, maxRetries int, delay time.Duration) error {
	addr := fmt.Sprintf("localhost:%s", port)
	log.Printf("Waiting for SSH server on %s...", addr)
	for i := 0; i < maxRetries; i++ {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close()
			log.Printf("SSH server is available on %s", addr)
			return nil
		}
		log.Printf("Attempt %d: SSH not available yet, retrying in %v...", i+1, delay)
		time.Sleep(delay)
	}
	return fmt.Errorf("SSH server not available on %s after %d retries", addr, maxRetries)
}

// waitForGuestSession loops indefinitely until a guest session is available,
// meaning that VBoxManage guestcontrol commands can be successfully run.
func waitForGuestSession(vmName, username, password string, delay time.Duration) {
	log.Printf("Waiting indefinitely for guest session on VM: %s...", vmName)
	for {
		cmd := exec.Command("VBoxManage", "guestcontrol", vmName, "run",
			"--username", username,
			"--password", password,
			"--", "echo", "ok")
		if err := cmd.Run(); err == nil {
			log.Printf("Guest session is available on VM: %s", vmName)
			return
		}
		log.Printf("Guest session not available yet, retrying in %v...", delay)
		time.Sleep(delay)
	}
}

// injectSSHKey injects the provided public SSH key into the VM's authorized_keys file; it retries on transient failures.
func injectSSHKey(sshKey string) error {
	maxRetries := 3
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		authCmd := fmt.Sprintf(`echo "%s" >> ~/.ssh/authorized_keys`, sshKey)
		insertKeyCmd := exec.Command("ssh",
			"-p", "2222",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"user@localhost",
			authCmd)
		output, err := insertKeyCmd.CombinedOutput()
		if err == nil {
			return nil
		}
		lastErr = err
		log.Printf("Attempt %d: SSH key injection failed. Output:\n%s\nError: %v", i+1, string(output), err)
		time.Sleep(time.Duration(5*(i+1)) * time.Second)
	}
	return fmt.Errorf("SSH key injection failed after %d attempts: %v", maxRetries, lastErr)
}

// ensureGuestSSHInstalled checks if sshd is present in the guest; if not,
// it waits for a guest session indefinitely, updates Guest Additions, and installs openssh-server.
func ensureGuestSSHInstalled(vmName, username, password string) error {
	log.Printf("Ensuring SSH is installed in the guest OS of VM: %s", vmName)

	// Wait indefinitely for a guest session.
	waitForGuestSession(vmName, username, password, 5*time.Second)

	// Check for the presence of sshd.
	cmdCheck := exec.Command("VBoxManage", "guestcontrol", vmName, "run",
		"--username", username,
		"--password", password,
		"--", "bash", "-c", "command -v sshd")
	output, err := cmdCheck.CombinedOutput()
	if err != nil || len(strings.TrimSpace(string(output))) == 0 {
		log.Println("sshd not found in guest. Attempting to update Guest Additions...")
		if err := updateGuestAdditions(vmName); err != nil {
			return fmt.Errorf("failed to update guest additions: %v", err)
		}
		// Wait extra time for the guest to initialize after updating Guest Additions.
		time.Sleep(60 * time.Second)
		output, err = exec.Command("VBoxManage", "guestcontrol", vmName, "run",
			"--username", username,
			"--password", password,
			"--", "bash", "-c", "command -v sshd").CombinedOutput()
		if err != nil || len(strings.TrimSpace(string(output))) == 0 {
			return fmt.Errorf("sshd still not found in guest after updating guest additions: %s", string(output))
		}
		log.Println("Guest additions updated and sshd now available. Installing openssh-server...")
		installCmd := exec.Command("VBoxManage", "guestcontrol", vmName, "run",
			"--username", username,
			"--password", password,
			"--", "bash", "-c", "sudo apt-get update && sudo apt-get install -y openssh-server")
		installOutput, installErr := installCmd.CombinedOutput()
		if installErr != nil {
			return fmt.Errorf("failed to install openssh-server: %s, error: %v", string(installOutput), installErr)
		}
		enableCmd := exec.Command("VBoxManage", "guestcontrol", vmName, "run",
			"--username", username,
			"--password", password,
			"--", "bash", "-c", "sudo systemctl enable ssh --now")
		enableOutput, enableErr := enableCmd.CombinedOutput()
		if enableErr != nil {
			return fmt.Errorf("failed to enable/start ssh service: %s, error: %v", string(enableOutput), enableErr)
		}
		log.Println("openssh-server installed and enabled successfully.")
	} else {
		log.Printf("sshd is present in guest: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

// updateGuestAdditions updates (or installs) the Guest Additions in the guest.
// Note: The guestcontrol updateguestadditions command ignores guest credentials.
func updateGuestAdditions(vmName string) error {
	log.Printf("Updating Guest Additions for VM: %s", vmName)
	cmd := exec.Command("VBoxManage", "guestcontrol", vmName, "updateguestadditions",
		"--source", guestAdditionsISO,
		"--wait-start")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update guest additions: %s, error: %v", string(output), err)
	}
	return nil
}

// scheduleDeletion deletes the VM after the specified duration.
func scheduleDeletion(vmName string, duration time.Duration) {
	log.Printf("Scheduling deletion of VM: %s in %v", vmName, duration)
	time.Sleep(duration)
	log.Printf("Deleting VM: %s", vmName)
	if err := DeleteVM(vmName); err != nil {
		log.Printf("Error deleting VM: %v", err)
	}
}

// DeleteVM attempts to power off (if necessary) then unregisters and deletes the VM.
func DeleteVM(vmName string) error {
	_ = exec.Command("VBoxManage", "controlvm", vmName, "poweroff").Run()
	cmd := exec.Command("VBoxManage", "unregistervm", vmName, "--delete")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete VM: %v", err)
	}
	log.Printf("VM %s deleted successfully!", vmName)
	return nil
}
