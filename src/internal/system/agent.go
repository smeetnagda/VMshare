package system

import (
	"fmt"
	"log"
	"time"
)

// StartRentalProcess checks resources, then creates a VM with SSH access.
func StartRentalProcess(vmName, sshKey string, duration time.Duration) error {
	log.Println("Starting rental process...")

	// Check system resources
	if err := CheckResources(); err != nil {
		return fmt.Errorf("insufficient resources: %v", err)
	}

	// Create and configure VM
	if err := ManageVM(vmName, "create"); err != nil {
		return fmt.Errorf("failed to create VM: %v", err)
	}

	// Inject SSH key for renter access
	if err := InjectSSHKey(vmName, sshKey); err != nil {
		return fmt.Errorf("failed to inject SSH key: %v", err)
	}

	log.Println("VM is now available for the renter.")

	// Schedule deletion after rental period
	go func() {
		time.Sleep(duration)
		_ = ManageVM(vmName, "delete")
	}()

	return nil
}
