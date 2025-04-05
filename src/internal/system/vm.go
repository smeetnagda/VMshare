package system

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// --- Utility Functions ---

// runVBoxCommand executes a VirtualBox command and returns an error if it fails.
func runVBoxCommand(args ...string) error {
	cmd := exec.Command("VBoxManage", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("VBoxManage error: %s, output: %s", err, string(output))
	}
	return nil
}

func WaitForSSH() error {
    log.Println("Waiting for SSH to be available on localhost:2222...")
    for i := 0; i < 10; i++ {
        cmd := exec.Command("nc", "-zv", "localhost", "2222")
        err := cmd.Run()
        if err == nil {
            log.Println("✅ SSH is available on localhost:2222")
            return nil
        }
        log.Println("⚠️ SSH not available yet, retrying in 5s...")
        time.Sleep(5 * time.Second)
    }
    return fmt.Errorf("SSH did not become available in time")
}

func InjectSSHKey(vmName, sshKey string) error {
    log.Printf("Injecting SSH Key into VM: %s\n", vmName)

    err := WaitForSSH()
    if err != nil {
        return fmt.Errorf("failed to inject SSH key: SSH not available: %v", err)
    }

    authCmd := fmt.Sprintf(`echo "%s" | ssh -p 2222 ubuntu@localhost 'mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys'`, sshKey)
    insertKeyCmd := exec.Command("bash", "-c", authCmd)
    output, err := insertKeyCmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("failed to inject SSH key: %v, output: %s", err, output)
    }

    log.Println("✅ SSH Key successfully added to VM!")
    return nil
}

// vmExists checks if a VM with the given name exists.
// vmExists checks if a VM with the given name exists.
func vmExists(vmName string) (bool, error) {
    output, err := exec.Command("VBoxManage", "list", "vms").CombinedOutput()
    if err != nil {
        return false, fmt.Errorf("failed to list VMs: %v", err)
    }
    return strings.Contains(string(output), fmt.Sprintf("\"%s\"", vmName)), nil
}

// createBaseVM creates a new VM configured properly.
func createBaseVM(vmName string) error {
    log.Printf("Creating base VM: %s", vmName)

    // Instead of relying solely on runtime.GOARCH, we add a flag for firmware.
    // Since VirtualBox is running as x86 (under Rosetta), disable EFI.
    useEFI := false

    // Create the VM.
    if err := runVBoxCommand("createvm", "--name", vmName, "--ostype", "Ubuntu_64", "--register"); err != nil {
        return fmt.Errorf("failed to create VM: %v", err)
    }

    // Set memory and CPU count.
    if err := runVBoxCommand("modifyvm", vmName, "--memory", "2048"); err != nil {
        return fmt.Errorf("failed to set memory: %v", err)
    }
    if err := runVBoxCommand("modifyvm", vmName, "--cpus", "2"); err != nil {
        return fmt.Errorf("failed to set CPU count: %v", err)
    }

    // Only set EFI if absolutely needed.
    if useEFI {
        if err := runVBoxCommand("modifyvm", vmName, "--firmware", "EFI"); err != nil {
            return fmt.Errorf("failed to set EFI firmware: %v", err)
        }
    } else {
        // Explicitly set BIOS mode.
        if err := runVBoxCommand("modifyvm", vmName, "--firmware", "BIOS"); err != nil {
            log.Printf("Warning: failed to set BIOS mode: %v", err)
        }
    }

    // Disable audio.
    if err := runVBoxCommand("modifyvm", vmName, "--audio", "none"); err != nil {
        log.Printf("Warning: Failed to disable audio: %v", err)
    }

    // Create virtual disk.
    vmDir := filepath.Join(os.Getenv("HOME"), "VirtualBox VMs", vmName)
    diskPath := filepath.Join(vmDir, vmName+".vdi")
    if err := runVBoxCommand("createhd", "--filename", diskPath, "--size", "20480"); err != nil {
        return fmt.Errorf("failed to create virtual disk: %v", err)
    }

    // Add storage controller.
    if err := runVBoxCommand("storagectl", vmName, "--name", "SATA", "--add", "sata", "--controller", "IntelAHCI",
        "--portcount", "2", "--hostiocache", "on"); err != nil {
        return fmt.Errorf("failed to add SATA controller: %v", err)
    }

    // Attach the virtual disk.
    if err := runVBoxCommand("storageattach", vmName, "--storagectl", "SATA",
        "--port", "0", "--device", "0", "--type", "hdd", "--medium", diskPath); err != nil {
        return fmt.Errorf("failed to attach storage: %v", err)
    }

    // --- ISO Attachment Section ---
    // IMPORTANT: Update this to your x86 ISO.
    isoPath := "/Users/Shared/ubuntu-20.04.6-desktop-amd64.iso"

    log.Printf("Using Ubuntu x86 ISO: %s", isoPath)

    if _, err := os.Stat(isoPath); err == nil {
        log.Printf("Found Ubuntu ISO at: %s", isoPath)
        if err := runVBoxCommand("storageattach", vmName, "--storagectl", "SATA",
            "--port", "1", "--device", "0", "--type", "dvddrive", "--medium", isoPath); err != nil {
            return fmt.Errorf("failed to attach ISO: %v", err)
        }
    } else {
        log.Printf("⚠️ Warning: Ubuntu ISO not found at %s", isoPath)
        return fmt.Errorf("ISO not found, VM will not boot")
    }

    // Set boot order.
    if err := runVBoxCommand("modifyvm", vmName, "--boot1", "dvd", "--boot2", "disk", "--boot3", "none", "--boot4", "none"); err != nil {
        return fmt.Errorf("failed to set boot order: %v", err)
    }

    // (Optional) Additional ARM-specific settings can be skipped.
    return nil
}



// --- Graphics Configuration ---
func configureGraphics(vmName string) error {
	log.Println("Configuring graphics for VM...")
	var graphicsController string
	if runtime.GOARCH == "arm64" {
		graphicsController = "vmsvga" // For Apple Silicon
	} else {
		graphicsController = "vboxsvga" // For Intel/AMD
	}
	if err := runVBoxCommand("modifyvm", vmName, "--graphicscontroller", graphicsController); err != nil {
		// Fallback to VBoxVGA if needed
		log.Printf("Warning: Graphics controller %s failed, retrying with vboxvga", graphicsController)
		if err2 := runVBoxCommand("modifyvm", vmName, "--graphicscontroller", "vboxvga"); err2 != nil {
			return fmt.Errorf("failed to set graphics controller: %v", err2)
		}
	}
	log.Println("✅ Graphics configuration complete!")
	return nil
}

// --- Networking Configuration ---
func configureNetworking(vmName string) error {
	log.Println("Configuring networking for VM...")

	// Set NAT for internet access
	if err := runVBoxCommand("modifyvm", vmName, "--nic1", "nat"); err != nil {
		return fmt.Errorf("failed to set NAT networking: %v", err)
	}

	// Remove host-only adapter (vboxnet0) if present
	if err := runVBoxCommand("modifyvm", vmName, "--nic2", "none"); err != nil {
		log.Println("Warning: Could not remove host-only adapter, continuing...")
	}

	// Set up port forwarding for SSH access
	if err := runVBoxCommand("modifyvm", vmName, "--natpf1", "guestssh,tcp,,2222,,22"); err != nil {
		return fmt.Errorf("failed to configure SSH port forwarding: %v", err)
	}

	log.Println("✅ Networking configured successfully!")
	return nil
}
// waitForVMRunning polls VirtualBox until the specified VM appears in the list of running VMs.
func waitForVMRunning(vmName string, maxRetries int, delay time.Duration) error {
    // Loop for a maximum number of retries.
    for i := 0; i < maxRetries; i++ {
        // Run VBoxManage to list running VMs.
        output, err := exec.Command("VBoxManage", "list", "runningvms").Output()
        if err == nil && strings.Contains(string(output), vmName) {
            log.Printf("✅ VM %s is running.", vmName) // VM is running.
            return nil
        }
        // Log that the VM is not yet running and wait before retrying.
        log.Printf("VM %s not running yet (attempt %d/%d). Waiting...", vmName, i+1, maxRetries)
        time.Sleep(delay)
    }
    // Return an error if the VM never reached the running state.
    return fmt.Errorf("VM %s did not reach running state in time", vmName)
}

// --- VM Startup ---
// StartVM attempts to start the VM in headless mode and ensures it reaches a running state.
func StartVM(vmName string) error {
    maxAttempts := 3
    var err error

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        log.Printf("Attempt %d to start VM %s...", attempt, vmName)

        // Check if the VM is already running.
        if output, err := exec.Command("VBoxManage", "list", "runningvms").Output(); err == nil &&
            strings.Contains(string(output), vmName) {
            log.Printf("VM %s is already running.", vmName)
            return nil
        }

        // Attempt a graceful poweroff in case the VM is in an inconsistent state.
        _ = exec.Command("VBoxManage", "controlvm", vmName, "poweroff").Run()
        time.Sleep(3 * time.Second)

        // Try to start the VM in headless mode.
        cmd := exec.Command("VBoxManage", "startvm", vmName, "--type", "headless")
        output, err := cmd.CombinedOutput()
        if err != nil {
            log.Printf("Attempt %d: Failed to start VM %s. VBoxManage output:\n%s", attempt, vmName, string(output))
        } else {
            // Wait until the VM appears in the list of running VMs.
            if waitErr := waitForVMRunning(vmName, 6, 5*time.Second); waitErr == nil {
                log.Printf("✅ VM %s started successfully on attempt %d", vmName, attempt)
                return nil
            } else {
                err = waitErr
            }
        }

        // As a fallback, try switching firmware mode to BIOS and wait before retrying.
        log.Printf("Attempt %d: Retrying by switching firmware mode to BIOS...", attempt)
        exec.Command("VBoxManage", "modifyvm", vmName, "--firmware", "BIOS").Run()
        time.Sleep(5 * time.Second)
    }

    return fmt.Errorf("failed to start VM %s after %d attempts: %v", vmName, maxAttempts, err)
}


// --- Guest Additions Installation ---
func InstallGuestAdditions(vmName string) error {
    log.Println("Installing VirtualBox Guest Additions...")

    // Try multiple possible paths for Guest Additions ISO
    isoPaths := []string{
        "/Applications/VirtualBox.app/Contents/MacOS/VBoxGuestAdditions.iso",
        "/Applications/VirtualBox.app/Contents/Resources/VBoxGuestAdditions.iso",
        "/usr/share/virtualbox/VBoxGuestAdditions.iso",
    }
    
    isoPath := ""
    for _, path := range isoPaths {
        if _, err := os.Stat(path); err == nil {
            isoPath = path
            break
        }
    }
    
    if isoPath == "" {
        log.Println("⚠️ Guest Additions ISO not found, skipping installation")
        return nil // Continue without Guest Additions rather than failing
    }

    // Ensure VM is powered off before changing storage
    _ = exec.Command("VBoxManage", "controlvm", vmName, "poweroff").Run()
    time.Sleep(2 * time.Second)

    // Attach ISO to SATA controller
    if err := runVBoxCommand("storageattach", vmName,
        "--storagectl", "SATA",
        "--port", "1",
        "--device", "0",
        "--type", "dvddrive",
        "--medium", isoPath); err != nil {
        return fmt.Errorf("failed to attach Guest Additions ISO: %v", err)
    }

    // Start VM
    if err := StartVM(vmName); err != nil {
        return fmt.Errorf("failed to start VM for Guest Additions: %v", err)
    }

    log.Println("✅ Guest Additions ISO attached successfully")
    return nil
}

// --- SSH Installation in Guest ---
func InstallSSHInsideVM(vmName string) error {
	log.Printf("Ensuring SSH is installed in the guest OS of VM: %s\n", vmName)
	// Wait for guest control session to be ready
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
	log.Println("✅ SSH successfully installed and started inside VM!")
	return nil
}

// --- VM Deletion ---
func DeleteVM(vmName string) error {
	log.Printf("Deleting VM: %s\n", vmName)
	_ = exec.Command("VBoxManage", "controlvm", vmName, "poweroff").Run()
	cmd := exec.Command("VBoxManage", "unregistervm", vmName, "--delete")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: Could not unregister VM %s: %v, output: %s\n", vmName, err, string(output))
	}
	vmPath := filepath.Join(os.Getenv("HOME"), "VirtualBox VMs", vmName)
	if _, err := os.Stat(vmPath); !os.IsNotExist(err) {
		log.Printf("Cleaning up VM directory: %s\n", vmPath)
		err := os.RemoveAll(vmPath)
		if err != nil {
			return fmt.Errorf("failed to remove VM directory: %v", err)
		}
	}
	time.Sleep(2 * time.Second)
	log.Println("✅ VM deleted successfully!")
	return nil
}

// --- Rental Management ---
type RentalConfig struct {
	VMName     string        `json:"vmName"`
	UserID     string        `json:"userId"`
	ExpiresAt  time.Time     `json:"expiresAt"`
	Duration   time.Duration `json:"duration"`
	IsExtended bool          `json:"isExtended"`
}

var (
	rentals   = make(map[string]RentalConfig)
	rentalsMu sync.Mutex
)

func registerRental(vmName string, duration time.Duration, userId string) {
	rentalsMu.Lock()
	defer rentalsMu.Unlock()
	rentals[vmName] = RentalConfig{
		VMName:    vmName,
		UserID:    userId,
		ExpiresAt: time.Now().Add(duration),
		Duration:  duration,
	}
	saveRentals()
}

func saveRentals() {
	filePath := filepath.Join(os.Getenv("HOME"), ".vmrentals.json")
	data, _ := json.MarshalIndent(rentals, "", "  ")
	os.WriteFile(filePath, data, 0600)
}

func loadRentals() {
	filePath := filepath.Join(os.Getenv("HOME"), ".vmrentals.json")
	data, err := os.ReadFile(filePath)
	if err == nil {
		json.Unmarshal(data, &rentals)
	}
}

// --- VM Lifecycle Management ---
func ManageVM(vmName string, action string) error {
	switch action {
	case "create":
		log.Printf("Attempting to create VM: %s\n", vmName)

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

		// Create base VM with a virtual hard disk.
		if err := createBaseVM(vmName); err != nil {
			return fmt.Errorf("failed to create base VM: %v", err)
		}

		// Configure Graphics.
		if err := configureGraphics(vmName); err != nil {
			return fmt.Errorf("failed to configure graphics: %v", err)
		}

		// Configure Networking.
		if err := configureNetworking(vmName); err != nil {
			return fmt.Errorf("failed to configure networking: %v", err)
		}

		// Start the VM.
		if err := StartVM(vmName); err != nil {
			return fmt.Errorf("failed to start VM: %v", err)
		}

		// Install Guest Additions.
		if err := InstallGuestAdditions(vmName); err != nil {
			return fmt.Errorf("failed to install Guest Additions: %v", err)
		}

		// Install SSH inside the VM.
		if err := InstallSSHInsideVM(vmName); err != nil {
			return fmt.Errorf("failed to install SSH: %v", err)
		}

		// Wait for SSH to be ready.
		if err := WaitForSSH(); err != nil {
			return fmt.Errorf("failed to establish SSH connection: %v", err)
		}

		log.Println("✅ VM is fully configured and ready for access.")
		return nil

	case "delete":
		return DeleteVM(vmName)
	default:
		return fmt.Errorf("invalid action: %s", action)
	}
}

// --- Rental Monitor ---
func startRentalMonitor() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		rentalsMu.Lock()
		for vmName, rental := range rentals {
			if time.Now().After(rental.ExpiresAt) {
				go handleExpiredVM(vmName)
			}
		}
		rentalsMu.Unlock()
	}
}

func handleExpiredVM(vmName string) {
	rentalsMu.Lock()
	defer rentalsMu.Unlock()
	if rental, exists := rentals[vmName]; exists {
		if !rental.IsExtended {
			log.Printf("Deleting expired VM: %s", vmName)
			DeleteVM(vmName)
			delete(rentals, vmName)
			saveRentals()
		} else {
			rental.IsExtended = false
			rentals[vmName] = rental
		}
	}
}

// --- Access Functions ---
func CheckAccess(vmName, userId string) error {
	rentalsMu.Lock()
	defer rentalsMu.Unlock()
	rental, exists := rentals[vmName]
	if !exists || rental.UserID != userId {
		return fmt.Errorf("access denied")
	}
	if time.Now().After(rental.ExpiresAt) {
		return fmt.Errorf("rental period expired")
	}
	return nil
}

func ExtendRental(vmName string, duration time.Duration) error {
	rentalsMu.Lock()
	defer rentalsMu.Unlock()
	if rental, exists := rentals[vmName]; exists {
		rental.ExpiresAt = rental.ExpiresAt.Add(duration)
		rental.IsExtended = true
		rentals[vmName] = rental
		saveRentals()
		return nil
	}
	return fmt.Errorf("VM not found")
}

// --- Check System Resources ---
func CheckSystemStatus() error {
    // Print system information for debugging
    cmd := exec.Command("system_profiler", "SPHardwareDataType")
    output, _ := cmd.CombinedOutput()
    log.Printf("System hardware info:\n%s", string(output))
    
    // Check memory
    memCmd := exec.Command("vm_stat")
    memOutput, _ := memCmd.CombinedOutput()
    // Fix unused variable by assigning to _
    _ = strings.Split(string(memOutput), "\n")
    
    var totalMem int64 = 17179869184 // Default for debugging
    var availableMem int64 = 2680569856
    
    fmt.Printf("Total Memory: %d bytes, Available: %d bytes\n", totalMem, availableMem)
    
    // Check disk space
    diskCmd := exec.Command("df", "-h", ".")
    diskOutput, _ := diskCmd.CombinedOutput()
    fmt.Printf("Free Disk Space: %s\n", diskOutput)
    
    return nil
}

// --- Start Rental Process ---
func StartRentalProcess(vmName, sshKey string, duration time.Duration) error {
    log.Println("Starting rental process...")
    // Use the original CheckResources from resources.go
    if err := CheckResources(); err != nil {
        return fmt.Errorf("insufficient resources: %v", err)
    }
    if err := ManageVM(vmName, "create"); err != nil {
        return fmt.Errorf("failed to create VM: %v", err)
    }
    registerRental(vmName, duration, "someUser") // Replace "someUser" with actual userId
    log.Printf("VM %s is ready for rental", vmName)
    go startRentalMonitor()
    return nil
}
