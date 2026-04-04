package docker

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/pruvon/pruvon/internal/exec"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

// UpdateContainerResourceLimits updates the resource limits for a container
// using the docker update command with the given CPU and memory limits.
// It returns an error if the operation fails.
func UpdateContainerResourceLimits(c exec.CommandRunner, containerId, cpus, memory string) error {
	// Validate CPU and memory limits
	if cpus == "" && memory == "" {
		return fmt.Errorf("at least one resource limit (CPU or memory) must be provided")
	}

	// Prepare docker update command args
	args := []string{"update"}

	// Add CPU limit if provided
	if cpus != "" {
		formattedCPU, err := validateCPULimit(cpus)
		if err != nil {
			return err
		}
		args = append(args, "--cpus", formattedCPU)
	}

	// Add memory limit if provided
	if memory != "" {
		formattedMemory, err := validateMemoryLimit(memory)
		if err != nil {
			return err
		}
		args = append(args, "--memory", formattedMemory)
		// Always set memory-swap to -1 to disable swap limit
		args = append(args, "--memory-swap", "-1")
	}

	// Add container ID at the end
	args = append(args, containerId)

	// Execute docker update command
	_, err := c.RunCommand("docker", args...)
	if err != nil {
		return fmt.Errorf("failed to update container resource limits: %v", err)
	}

	return nil
}

// validateCPULimit validates the CPU limit and ensures it doesn't exceed the system's total CPU count
func validateCPULimit(cpus string) (string, error) {
	// Check if the cpus value is a valid number
	cpuValue, err := strconv.ParseFloat(cpus, 64)
	if err != nil {
		return "", fmt.Errorf("invalid CPU value: %s, must be a number", cpus)
	}

	// CPU value must be positive
	if cpuValue <= 0 {
		return "", fmt.Errorf("CPU value must be greater than 0")
	}

	// Get total CPU cores
	cpuInfo, err := cpu.Info()
	if err != nil {
		return "", fmt.Errorf("failed to get CPU information: %v", err)
	}

	var totalCores int32
	for _, c := range cpuInfo {
		totalCores += c.Cores
	}

	// Ensure CPU limit doesn't exceed total CPU cores
	if cpuValue > float64(totalCores) {
		return "", fmt.Errorf("CPU value %v exceeds system total cores %v", cpuValue, totalCores)
	}

	// Return formatted CPU value
	return fmt.Sprintf("%.2f", cpuValue), nil
}

// validateMemoryLimit validates the memory limit format and ensures it doesn't exceed the system's total memory
func validateMemoryLimit(memory string) (string, error) {
	// Regular expression to validate memory format (e.g., 300M, 1G)
	memRegex := regexp.MustCompile(`^(\d+)([MG])$`)
	matches := memRegex.FindStringSubmatch(memory)

	if len(matches) != 3 {
		return "", fmt.Errorf("invalid memory format: %s, must be like 300M or 1G", memory)
	}

	// Extract value and unit
	valueStr := matches[1]
	unit := matches[2]

	// Parse value
	value, err := strconv.ParseUint(valueStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid memory value: %s", valueStr)
	}

	if value <= 0 {
		return "", fmt.Errorf("memory value must be greater than 0")
	}

	// Convert to bytes based on unit
	var memoryBytes uint64
	switch unit {
	case "M":
		memoryBytes = value * 1024 * 1024 // MB to bytes
	case "G":
		memoryBytes = value * 1024 * 1024 * 1024 // GB to bytes
	}

	// Get total system memory
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return "", fmt.Errorf("failed to get memory information: %v", err)
	}

	// Ensure memory limit doesn't exceed total system memory
	if memoryBytes > memInfo.Total {
		// Using formatBytes from system.go
		totalMemFormatted := fmt.Sprintf("%.1f %cB",
			float64(memInfo.Total)/float64(1024*1024*1024), 'G')
		return "", fmt.Errorf("memory limit %s exceeds system total memory %s",
			memory, totalMemFormatted)
	}

	// Return the original memory string since Docker accepts this format
	return memory, nil
}
