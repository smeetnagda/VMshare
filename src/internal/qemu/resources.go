package system

import (
	"fmt"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/disk"
)

// CheckResources verifies if the system has enough memory and disk space.
func CheckResources() error {
	v, err := mem.VirtualMemory()
	if err != nil {
		return fmt.Errorf("failed to get memory info: %v", err)
	}

	d, err := disk.Usage("/")
	if err != nil {
		return fmt.Errorf("failed to get disk usage info: %v", err)
	}

	fmt.Printf("Total Memory: %v bytes, Available: %v bytes\n", v.Total, v.Available)
	fmt.Printf("Free Disk Space: %v bytes\n", d.Free)

	// Define minimum thresholds (example: 2GB RAM and 10GB Disk)
	const minRAM = 2 * 1024 * 1024 * 1024
	const minDisk = 10 * 1024 * 1024 * 1024

	if v.Available < minRAM || d.Free < minDisk {
		return fmt.Errorf("insufficient resources")
	}

	return nil
}
