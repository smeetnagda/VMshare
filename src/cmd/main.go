package main

import (
	"fmt"
	"log"
	"vm-agent/internal/system"
)

func main() {
	requiredMemory := int64(1024 * 1024 * 1024) // 1GB in bytes
	requiredDiskSpace := int64(10 * 1024 * 1024 * 1024) // 10GB in bytes

	// Step 1: Check Resources
	fmt.Println("Checking system resources before VM creation...")
	if !system.CheckResources(requiredMemory, requiredDiskSpace) {
		log.Fatal("Insufficient resources. Cannot proceed with VM creation.")
	}

	// Step 2: Create VM
	vmName := "TestVM"
	fmt.Println("Attempting to create VM...")
	if err := system.ManageVM(vmName, "create"); err != nil {
		log.Fatalf("Failed to create VM: %v", err)
	}
	fmt.Println("VM created successfully!")

	// Step 3: Delete VM
	fmt.Println("Attempting to delete VM...")
	if err := system.ManageVM(vmName, "delete"); err != nil {
		log.Fatalf("Failed to delete VM: %v", err)
	}
	fmt.Println("VM deleted successfully!")
}
