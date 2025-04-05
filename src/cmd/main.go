package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
	"vm-agent/internal/system"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: main <VM_NAME> <SSH_KEY_PATH> <DURATION_IN_MINUTES>")
		os.Exit(1)
	}

	vmName := os.Args[1]
	sshKeyPath := os.Args[2]
	durationMinutes, err := strconv.Atoi(os.Args[3])
	if err != nil {
		log.Fatalf("Invalid duration: %v", err)
	}

	// Read SSH Key
	sshKey, err := os.ReadFile(sshKeyPath)
	if err != nil {
		log.Fatalf("Failed to read SSH key: %v", err)
	}

	duration := time.Duration(durationMinutes) * time.Minute

	log.Println("Starting rental process...")
	if err := system.StartRentalProcess(vmName, string(sshKey), duration); err != nil {
		log.Fatalf("Error starting rental: %v", err)
	}

	log.Println("VM rental setup complete!")
}
