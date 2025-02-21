package system

import (
	"fmt"
	"time"
)

// StartRentalProcess checks resources and then creates a VM.
func StartRentalProcess(vmName string, sshKey string, duration time.Duration) error {
	// Ensure the CheckResources function is implemented
	if err := CheckResources(); err != nil {
		return fmt.Errorf("insufficient resources: %v", err)
	}

	// Proceed with VM creation
	return CreateVM(vmName, sshKey, duration)
}
