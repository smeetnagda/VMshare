
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
	"bytes" 

	"vm-agent/internal/multipass"
)

func main() {
    if len(os.Args) != 4 {
        log.Fatalf("Usage: %s <vmName> <sshPubKeyPath> <durationMinutes>", os.Args[0])
    }
    vmName := os.Args[1]
    keyPath := os.Args[2]
    mins, err := strconv.Atoi(os.Args[3])
    if err != nil {
        log.Fatalf("Invalid duration: %v", err)
    }

    // Load the public key
    data, err := os.ReadFile(keyPath)
    if err != nil {
        log.Fatalf("Read SSH key: %v", err)
    }
    sshKey := string(bytes.TrimSpace(data))

    fmt.Println("🔧 Starting rental process...")
    if err := system.StartVM(vmName, sshKey, time.Duration(mins)*time.Minute); err != nil {
        log.Fatalf("Error: %v", err)
    }
}