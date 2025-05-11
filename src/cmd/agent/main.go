// cmd/agent/main.go
package main

import (
    "fmt"
    "log"
    "os"
    "strconv"

    "github.com/smeetnagda/vmshare/internal/agent"
)

func main() {
    // Expect exactly two args: <agentID> <dbPath>
    if len(os.Args) != 3 {
        log.Fatalf("Usage: %s <agentID> <dbPath>", os.Args[0])
    }
    agentID, err := strconv.Atoi(os.Args[1])
    if err != nil {
        log.Fatalf("Invalid agentID: %v", err)
    }
    dbPath := os.Args[2]

    fmt.Printf("🔧 Starting agent daemon (ID=%d) polling %s …\n", agentID, dbPath)
    if err := agent.Run(dbPath, agentID); err != nil {
        log.Fatalf("Agent error: %v", err)
    }
}
