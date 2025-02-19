package system

import "fmt"

// CheckResources verifies if system has enough memory and disk space.
func CheckResources(requiredMemory int64, requiredDiskSpace int64) bool {
	totalMem, availableMem := GetMemoryStats()
	freeDisk := GetDiskStats()

	fmt.Printf("Total Memory: %d bytes, Available: %d bytes\n", totalMem, availableMem)
	fmt.Printf("Free Disk Space: %d bytes\n", freeDisk)

	if availableMem < uint64(requiredMemory) {
		fmt.Println("Not enough memory available!")
		return false
	}

	if freeDisk < uint64(requiredDiskSpace) {
		fmt.Println("Not enough disk space available!")
		return false
	}

	fmt.Println("Sufficient resources available. Proceeding with VM creation.")
	return true
}
