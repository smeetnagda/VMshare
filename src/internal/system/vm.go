package system

import (
	"fmt"
	"os/exec"
)

// ManageVM creates or deletes a VM using VirtualBox.
func ManageVM(vmName string, action string) error {
	var cmd *exec.Cmd

	switch action {
	case "create":
		cmd = exec.Command("VBoxManage", "createvm", "--name", vmName, "--register")
	case "delete":
		cmd = exec.Command("VBoxManage", "unregistervm", vmName, "--delete")
	default:
		return fmt.Errorf("invalid action: %s", action)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error executing VBoxManage: %s, output: %s", err, output)
	}

	fmt.Printf("VM management action '%s' completed successfully.\n", action)
	return nil
}
