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

    src := "internal/multipass/cloud-init-ssh-forward.yaml"
    yamlPath := filepath.Join(workDir, "cloud-init.yaml")
    data, err := ioutil.ReadFile(src)
    if err != nil {
        return fmt.Errorf("read cloud-init template: %v", err)
    }
    if err := ioutil.WriteFile(yamlPath, data, 0644); err != nil {
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

func FetchIP(vmName string) (string, error) {
    // Try up to 30 times, 2s apart, to give the VM time to boot and get an IP.
    for i := 0; i < 30; i++ {
        out, err := exec.Command("multipass", "info", vmName).CombinedOutput()
        if err != nil {
            // if the command itself failed, return immediately
            return "", fmt.Errorf("multipass info failed: %v, output: %s", err, string(out))
        }
        lines := strings.Split(string(out), "\n")
        for _, line := range lines {
            line = strings.TrimSpace(line)
            if strings.HasPrefix(line, "IPv4:") {
                parts := strings.Fields(line)
                if len(parts) >= 2 {
                    return parts[1], nil
                }
            }
        }
        time.Sleep(2 * time.Second)
    }
    return "", fmt.Errorf("could not determine IP for VM %q after waiting", vmName)
}