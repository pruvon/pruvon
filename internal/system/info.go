package system

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pruvon/pruvon/internal/exec"
	"github.com/pruvon/pruvon/internal/models"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/docker"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// readFileFunc, os.ReadFile fonksiyonunu mock'lamak için değişken
var readFileFunc = os.ReadFile

// ReadCPUStats reads CPU statistics from /proc/stat
func ReadCPUStats() (models.CPUStats, error) {
	contents, err := readFileFunc("/proc/stat")
	if err != nil {
		return models.CPUStats{}, err
	}

	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)[1:] // Skip "cpu" label
			stats := models.CPUStats{}
			if len(fields) >= 8 {
				stats.User, _ = strconv.ParseUint(fields[0], 10, 64)
				stats.Nice, _ = strconv.ParseUint(fields[1], 10, 64)
				stats.System, _ = strconv.ParseUint(fields[2], 10, 64)
				stats.Idle, _ = strconv.ParseUint(fields[3], 10, 64)
				stats.Iowait, _ = strconv.ParseUint(fields[4], 10, 64)
				stats.Irq, _ = strconv.ParseUint(fields[5], 10, 64)
				stats.Softirq, _ = strconv.ParseUint(fields[6], 10, 64)
				stats.Steal, _ = strconv.ParseUint(fields[7], 10, 64)
			}
			return stats, nil
		}
	}
	return models.CPUStats{}, fmt.Errorf("CPU stats not found")
}

// CalculateCPUUsage calculates CPU usage percentage
func CalculateCPUUsage() float64 {
	stat1, err := ReadCPUStats()
	if err != nil {
		return 0
	}

	// Kısa bir bekleme süresi
	time.Sleep(200 * time.Millisecond)

	stat2, err := ReadCPUStats()
	if err != nil {
		return 0
	}

	// CPU zamanlarındaki değişimi hesapla
	idle := float64(stat2.Idle - stat1.Idle + stat2.Iowait - stat1.Iowait)
	total := float64(
		(stat2.User + stat2.Nice + stat2.System + stat2.Idle + stat2.Iowait +
			stat2.Irq + stat2.Softirq + stat2.Steal) -
			(stat1.User + stat1.Nice + stat1.System + stat1.Idle + stat1.Iowait +
				stat1.Irq + stat1.Softirq + stat1.Steal))

	if total == 0 {
		return 0
	}

	return math.Round((1.0 - idle/total) * 100.0)
}

// formatBytesPerSecond formats bytes/sec to human-readable format (KB/s, MB/s, etc.)
func formatBytesPerSecond(bytesPerSec float64) string {
	const unit = 1024.0
	if bytesPerSec < unit {
		return fmt.Sprintf("%.1f B/s", bytesPerSec)
	}
	div, exp := float64(unit), 0
	for n := bytesPerSec / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB/s", bytesPerSec/div, "KMGTPE"[exp])
}

// GetSystemMetrics returns system metrics
func GetSystemMetrics() models.SystemMetrics {
	metrics := models.SystemMetrics{}

	// CPU Kullanımı
	if cpuPercent, err := cpu.Percent(time.Second, false); err == nil && len(cpuPercent) > 0 {
		metrics.CPUUsage = math.Round(cpuPercent[0])
	}

	// CPU bilgileri - Tüm CPU'ların toplam core sayısı
	if cpuInfo, err := cpu.Info(); err == nil {
		var totalCores int32
		for _, cpu := range cpuInfo {
			totalCores += cpu.Cores
		}
		metrics.CPUInfo = fmt.Sprintf("%d cores", totalCores)
	}

	// Load Average bilgileri
	if loadAvg, err := load.Avg(); err == nil {
		metrics.LoadAvg = fmt.Sprintf("%.2f %.2f %.2f", loadAvg.Load1, loadAvg.Load5, loadAvg.Load15)
	}

	// RAM Kullanımı
	if memInfo, err := mem.VirtualMemory(); err == nil {
		metrics.RAMUsage = math.Round(memInfo.UsedPercent)
		metrics.RAMInfo = fmt.Sprintf("%.1f GB / %.1f GB", float64(memInfo.Used)/1024/1024/1024, float64(memInfo.Total)/1024/1024/1024)
	}

	// Swap Kullanımı
	if swapInfo, err := mem.SwapMemory(); err == nil {
		metrics.SwapUsage = math.Round(swapInfo.UsedPercent)
		metrics.SwapInfo = fmt.Sprintf("%.1f GB / %.1f GB", float64(swapInfo.Used)/1024/1024/1024, float64(swapInfo.Total)/1024/1024/1024)
	}

	// Disk Kullanımı
	if diskInfo, err := disk.Usage("/"); err == nil {
		metrics.DiskUsage = math.Round(diskInfo.UsedPercent)
		metrics.DiskInfo = fmt.Sprintf("%.1f GB / %.1f GB", float64(diskInfo.Used)/1024/1024/1024, float64(diskInfo.Total)/1024/1024/1024)
	}

	// Network Usage - Get current network stats
	if netStats1, err := net.IOCounters(false); err == nil && len(netStats1) > 0 {
		// Aggregate stats from all interfaces if not pernic
		totalSent := netStats1[0].BytesSent
		totalRecv := netStats1[0].BytesRecv

		// Short delay to calculate rate
		time.Sleep(500 * time.Millisecond)

		// Get updated stats
		if netStats2, err := net.IOCounters(false); err == nil && len(netStats2) > 0 {
			// Calculate the rates in bytes/sec
			sentDiff := netStats2[0].BytesSent - totalSent
			recvDiff := netStats2[0].BytesRecv - totalRecv
			elapsedSecs := 0.5 // 500ms = 0.5s

			// Set the metrics
			metrics.NetBytesSent = netStats2[0].BytesSent
			metrics.NetBytesRecv = netStats2[0].BytesRecv
			metrics.NetBytesSentRate = float64(sentDiff) / elapsedSecs
			metrics.NetBytesRecvRate = float64(recvDiff) / elapsedSecs

			// Get details for the main interface
			if netStatsPerNic, err := net.IOCounters(true); err == nil && len(netStatsPerNic) > 0 {
				// Find the most active interface (excluding loopback)
				var mainInterface *net.IOCountersStat
				maxBytes := uint64(0)

				for i := range netStatsPerNic {
					if netStatsPerNic[i].Name != "lo" && netStatsPerNic[i].Name != "lo0" {
						totalBytes := netStatsPerNic[i].BytesSent + netStatsPerNic[i].BytesRecv
						if totalBytes > maxBytes {
							maxBytes = totalBytes
							mainInterface = &netStatsPerNic[i]
						}
					}
				}

				if mainInterface != nil {
					metrics.NetInterfaceName = mainInterface.Name
					// Format human-readable network info
					metrics.NetInfo = fmt.Sprintf(
						"↓ %s  ↑ %s",
						formatBytesPerSecond(metrics.NetBytesRecvRate),
						formatBytesPerSecond(metrics.NetBytesSentRate),
					)
				}
			}
		}
	}

	// Container Sayısı
	if containers, err := docker.GetDockerStat(); err == nil {
		metrics.ContainerCount = len(containers)
	}

	// Service Sayısı
	serviceCount := 0
	for _, serviceType := range []string{"postgres", "mariadb", "mongo", "redis"} {
		output, err := exec.NewCommandRunner().RunCommand("dokku", serviceType+":list")
		if err != nil {
			continue
		}
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if line != "" && !strings.Contains(line, "=====") {
				serviceCount++
			}
		}
	}
	metrics.ServiceCount = serviceCount

	// Uygulama Sayısı - Dokku özel olduğu için aynı kalıyor
	output, err := exec.NewCommandRunner().RunCommand("dokku", "--quiet", "apps:list")
	if err != nil {
		metrics.AppCount = 0
	} else {
		// Handle empty output case
		trimmedOutput := strings.TrimSpace(output)
		if trimmedOutput == "" {
			metrics.AppCount = 0
		} else {
			metrics.AppCount = len(strings.Split(trimmedOutput, "\n"))
		}
	}

	return metrics
}

// GetServerInfo returns server information
func GetServerInfo() models.ServerInfo {
	info := models.ServerInfo{}

	// Host bilgileri
	if hostInfo, err := host.Info(); err == nil {
		info.Hostname = hostInfo.Hostname
		info.OS = hostInfo.Platform + " " + hostInfo.PlatformVersion
		info.Kernel = hostInfo.KernelVersion

		// Uptime hesaplama
		uptime := time.Duration(hostInfo.Uptime) * time.Second
		days := int(uptime.Hours() / 24)
		hours := int(uptime.Hours()) % 24
		minutes := int(uptime.Minutes()) % 60
		info.Uptime = fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}

	// CPU bilgileri
	if cpuInfo, err := cpu.Info(); err == nil {
		var totalCores int32
		for _, c := range cpuInfo {
			totalCores += c.Cores
		}
		info.CPUCores = fmt.Sprintf("%d", totalCores)
	}

	// Memory bilgileri
	if memInfo, err := mem.VirtualMemory(); err == nil {
		info.MemoryTotal = fmt.Sprintf("%.1f GB", float64(memInfo.Total)/1024/1024/1024)
		info.MemoryUsed = fmt.Sprintf("%.1f GB", float64(memInfo.Used)/1024/1024/1024)
	}

	// Disk bilgileri
	if diskInfo, err := disk.Usage("/"); err == nil {
		info.DiskUsage = fmt.Sprintf("Total: %.1f GB, Used: %.1f GB, Free: %.1f GB (%.1f%%)",
			float64(diskInfo.Total)/1024/1024/1024,
			float64(diskInfo.Used)/1024/1024/1024,
			float64(diskInfo.Free)/1024/1024/1024,
			diskInfo.UsedPercent,
		)
	}

	// Public IP - mevcut yöntem iyi çalıştığı için aynı kalabilir
	resp, err := http.Get("https://api.ipify.org")
	if err == nil {
		defer resp.Body.Close()
		if ip, err := io.ReadAll(resp.Body); err == nil {
			info.PublicIP = string(ip)
		}
	}

	return info
}

// Contains checks if a string slice contains a specific string
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
