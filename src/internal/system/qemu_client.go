package system

import (
    "fmt"
    "io/ioutil"
    "math/rand"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "time"
)

// StartVM launches a QEMU VM with cloud-init, user SSH key injected,
// forwards guest:22 → random host port, and returns that host port.
func StartVM(vmName, sshKey string, duration time.Duration) (int, error) {
    workDir := filepath.Join(os.TempDir(), "vmrentals", vmName)
    os.RemoveAll(workDir)
    if err := os.MkdirAll(workDir, 0755); err != nil {
        return 0, fmt.Errorf("mkdir workspace: %v", err)
    }

    // --- write user-data + meta-data ---
    userData := fmt.Sprintf(`#cloud-config
ssh_authorized_keys:
  - %s
users:
  - name: ubuntu
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
`, sshKey)
    if err := ioutil.WriteFile(filepath.Join(workDir, "user-data"), []byte(userData), 0644); err != nil {
        return 0, fmt.Errorf("write user-data: %v", err)
    }
    metaData := fmt.Sprintf("instance-id: %s\n", vmName)
    if err := ioutil.WriteFile(filepath.Join(workDir, "meta-data"), []byte(metaData), 0644); err != nil {
        return 0, fmt.Errorf("write meta-data: %v", err)
    }

    // --- build seed ISO ---
    isoPath := filepath.Join(workDir, "seed.iso")
    var isoCmd *exec.Cmd
    if runtime.GOOS == "darwin" {
        // macOS path: hdiutil
        isoCmd = exec.Command("hdiutil", "makehybrid",
            "-o", isoPath,
            workDir,
            "-udf", "-joliet", "-iso")
    } else {
        // Linux fallback: genisoimage or mkisofs
        if _, err := exec.LookPath("genisoimage"); err == nil {
            isoCmd = exec.Command("genisoimage",
                "-output", isoPath,
                "-volid", "cidata",
                "-joliet", "-rock",
                "-graft-points",
                "user-data="+filepath.Join(workDir, "user-data"),
                "meta-data="+filepath.Join(workDir, "meta-data"))
        } else {
            isoCmd = exec.Command("mkisofs",
                "-output", isoPath,
                "-volid", "cidata",
                "-joliet", "-rock",
                "-graft-points",
                "user-data="+filepath.Join(workDir, "user-data"),
                "meta-data="+filepath.Join(workDir, "meta-data"))
        }
    }
    isoCmd.Stdout = os.Stdout
    isoCmd.Stderr = os.Stderr
    if err := isoCmd.Run(); err != nil {
        return 0, fmt.Errorf("build seed ISO: %v", err)
    }

    // --- backing disk ---
    baseImg := "/Users/smeetnagda/qemu-images/ubuntu-24.04-server-arm64.img" // adjust path
    qcow := filepath.Join(workDir, vmName+".qcow2")
    imgCmd := exec.Command("qemu-img", "create",
        "-f", "qcow2",
        "-b", baseImg,
        "-F", "raw",
        qcow)
    if out, err := imgCmd.CombinedOutput(); err != nil {
        return 0, fmt.Errorf("qemu-img create: %v, output: %s", err, out)
    }

    // --- pick a host port and launch QEMU ---
    rand.Seed(time.Now().UnixNano())
    hostPort := 20000 + rand.Intn(10000)

    qemuArgs := []string{
        "-machine", "virt,accel=hvf",
        "-cpu", "cortex-a72",
        "-m", "2048",
        "-smp", "2",
        "-drive", "file=" + qcow + ",if=virtio,format=qcow2",
        "-drive", "file=" + isoPath + ",if=virtio,media=cdrom,readonly=on",
        "-nic", fmt.Sprintf("user,hostfwd=tcp::%d-:22", hostPort),
        "-nographic",
    }
    cmd := exec.Command("qemu-system-aarch64", qemuArgs...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    if err := cmd.Start(); err != nil {
        return 0, fmt.Errorf("start QEMU: %v", err)
    }

    // cleanup after duration
    go func() {
        time.Sleep(duration)
        cmd.Process.Kill()
    }()

    return hostPort, nil
}
