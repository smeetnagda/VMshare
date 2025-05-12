package multipass

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// StartVM launches a Multipass VM named vmName, injects sshKey,
// waits for it to boot, prints the SSH command, and deletes it after duration.
func StartVM(vmName, sshKey string, duration time.Duration) error {
    // 1) Prepare workspace
    workDir := filepath.Join(os.TempDir(), "vmrentals", vmName)
    os.RemoveAll(workDir)
    if err := os.MkdirAll(workDir, 0755); err != nil {
        return fmt.Errorf("mkdir workspace: %v", err)
    }

    // 2) Write cloud-init user-data
    cloudInit := fmt.Sprintf(`#cloud-config
ssh_authorized_keys:
  - %s
users:
  - name: ubuntu
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
`, sshKey)
    yamlPath := filepath.Join(workDir, "cloud-init.yaml")
    if err := ioutil.WriteFile(yamlPath, []byte(cloudInit), 0644); err != nil {
        return fmt.Errorf("write cloud-init: %v", err)
    }

    // 3) Launch via CLI – jammy is Ubuntu 22.04 ARM64 on M-series
    launch := exec.Command(
        "multipass", "launch",
        "--name", vmName,
        "--cloud-init", yamlPath,
        "--cpus", "2",
        "--memory", "2G",
        "jammy",
    )
    launch.Stdout = os.Stdout
    launch.Stderr = os.Stderr
    if err := launch.Run(); err != nil {
        return fmt.Errorf("launch failed: %v", err)
    }

    // 4) Poll `multipass info` for IPv4
    var ip string
    for i := 0; i < 30; i++ {
        out, _ := exec.Command("multipass", "info", vmName).Output()
        for _, line := range strings.Split(string(out), "\n") {
            line = strings.TrimSpace(line)
            if strings.HasPrefix(line, "IPv4:") {
                parts := strings.Fields(line)
                if len(parts) >= 2 {
                    ip = parts[1]
                    break
                }
            }
        }
        if ip != "" {
            break
        }
        time.Sleep(2 * time.Second)
    }
    if ip == "" {
        return fmt.Errorf("could not determine VM IP after waiting")
    }

    // 5) Report SSH access
    fmt.Printf("✅ VM %q running; SSH with:\n    ssh -p 22 ubuntu@%s\n", vmName, ip)

    // 6) Schedule automatic deletion after duration
    go func() {
        time.Sleep(duration)
        exec.Command("multipass", "delete", "--purge", vmName).Run()
    }()

    return nil
}

// DeleteVM stops and purges the given VM immediately.
func DeleteVM(vmName string) error {
    cmd := exec.Command("multipass", "delete", "--purge", vmName)
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("delete failed: %v, output: %s", err, string(output))
    }
    return nil
}
