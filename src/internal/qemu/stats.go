package system

import (
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"log"
)

// GetMemoryStats fetches total system memory.
func GetMemoryStats() (uint64, uint64) {
	vmStats, err := mem.VirtualMemory()
	if err != nil {
		log.Fatalf("Failed to get memory stats: %v", err)
	}
	return vmStats.Total, vmStats.Available
}

// GetDiskStats fetches available disk space.
func GetDiskStats() uint64 {
	diskStats, err := disk.Usage("/")
	if err != nil {
		log.Fatalf("Failed to get disk stats: %v", err)
	}
	return diskStats.Free
}
