package system

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pruvon/pruvon/internal/exec"
	"github.com/pruvon/pruvon/internal/models"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/docker"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

// readFileFunc, os.ReadFile fonksiyonunu mock'lamak için değişken
var readFileFunc = os.ReadFile

var systemCommandRunner = exec.NewCommandRunner()
var publicIPHTTPClient = &http.Client{Timeout: 2 * time.Second}
var publicIPLookupURL = "https://api.ipify.org"
var dockerStatFunc = docker.GetDockerStat
var nowFunc = time.Now

var (
	resourceMetricsMu          sync.RWMutex
	resourceMetricsCache       models.SystemMetrics
	resourceMetricsCheckedAt   time.Time
	resourceMetricsInitialized bool

	countMetricsMu          sync.RWMutex
	countMetricsCache       models.SystemMetrics
	countMetricsCheckedAt   time.Time
	countMetricsInitialized bool

	serverInfoMu          sync.RWMutex
	serverInfoCache       models.ServerInfo
	serverInfoCheckedAt   time.Time
	serverInfoInitialized bool

	publicIPMu         sync.RWMutex
	publicIPCache      string
	publicIPCheckedAt  time.Time
	publicIPRefreshing bool
)

var (
	resourceMetricsTTL = 2 * time.Second
	countMetricsTTL    = 15 * time.Second
	serverInfoTTL      = 30 * time.Second
	publicIPTTL        = 30 * time.Minute
)

var serviceTypesForMetrics = []string{"postgres", "mariadb", "mongo", "redis"}

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
	resourceMetrics := GetResourceMetrics()
	countMetrics := GetCountMetrics()

	resourceMetrics.DokkuVersion = countMetrics.DokkuVersion
	resourceMetrics.ContainerCount = countMetrics.ContainerCount
	resourceMetrics.ServiceCount = countMetrics.ServiceCount
	resourceMetrics.AppCount = countMetrics.AppCount

	return resourceMetrics
}

// GetServerInfo returns server information
func GetServerInfo() models.ServerInfo {
	if info, ok := getCachedServerInfo(); ok {
		return info
	}

	info := loadServerInfo()
	setCachedServerInfo(info)
	return info
}

func GetResourceMetrics() models.SystemMetrics {
	if metrics, ok := getCachedResourceMetrics(); ok {
		return metrics
	}

	metrics := loadResourceMetrics()
	setCachedResourceMetrics(metrics)
	return metrics
}

func GetCountMetrics() models.SystemMetrics {
	if metrics, ok := getCachedCountMetrics(); ok {
		return metrics
	}

	metrics := loadCountMetrics()
	setCachedCountMetrics(metrics)
	return metrics
}

func loadResourceMetrics() models.SystemMetrics {
	metrics := models.SystemMetrics{}

	if cpuPercent, err := cpu.Percent(0, false); err == nil && len(cpuPercent) > 0 {
		metrics.CPUUsage = math.Round(cpuPercent[0])
	}

	if cpuInfo, err := cpu.Info(); err == nil {
		var totalCores int32
		for _, currentCPU := range cpuInfo {
			totalCores += currentCPU.Cores
		}
		metrics.CPUInfo = fmt.Sprintf("%d cores", totalCores)
	}

	if loadAvg, err := load.Avg(); err == nil {
		metrics.LoadAvg = fmt.Sprintf("%.2f %.2f %.2f", loadAvg.Load1, loadAvg.Load5, loadAvg.Load15)
	}

	if memInfo, err := mem.VirtualMemory(); err == nil {
		metrics.RAMUsage = math.Round(memInfo.UsedPercent)
		metrics.RAMInfo = fmt.Sprintf("%.1f GB / %.1f GB", float64(memInfo.Used)/1024/1024/1024, float64(memInfo.Total)/1024/1024/1024)
	}

	if swapInfo, err := mem.SwapMemory(); err == nil {
		metrics.SwapUsage = math.Round(swapInfo.UsedPercent)
		metrics.SwapInfo = fmt.Sprintf("%.1f GB / %.1f GB", float64(swapInfo.Used)/1024/1024/1024, float64(swapInfo.Total)/1024/1024/1024)
	}

	if diskInfo, err := disk.Usage("/"); err == nil {
		metrics.DiskUsage = math.Round(diskInfo.UsedPercent)
		metrics.DiskInfo = fmt.Sprintf("%.1f GB / %.1f GB", float64(diskInfo.Used)/1024/1024/1024, float64(diskInfo.Total)/1024/1024/1024)
	}

	return metrics
}

func loadCountMetrics() models.SystemMetrics {
	metrics := models.SystemMetrics{
		DokkuVersion: "Unknown",
	}

	if containers, err := dockerStatFunc(); err == nil {
		metrics.ContainerCount = len(containers)
	}

	serviceCount := 0
	for _, serviceType := range serviceTypesForMetrics {
		output, err := systemCommandRunner.RunCommand("dokku", serviceType+":list")
		if err != nil {
			continue
		}
		serviceCount += countDokkuEntries(output)
	}
	metrics.ServiceCount = serviceCount

	if output, err := systemCommandRunner.RunCommand("dokku", "--quiet", "apps:list"); err == nil {
		metrics.AppCount = countDokkuEntries(output)
	}

	if output, err := systemCommandRunner.RunCommand("dokku", "--version"); err == nil {
		metrics.DokkuVersion = parseDokkuVersionOutput(output)
	}

	return metrics
}

func loadServerInfo() models.ServerInfo {
	info := models.ServerInfo{}

	if hostInfo, err := host.Info(); err == nil {
		info.Hostname = hostInfo.Hostname
		info.OS = hostInfo.Platform + " " + hostInfo.PlatformVersion
		info.Kernel = hostInfo.KernelVersion

		uptime := time.Duration(hostInfo.Uptime) * time.Second
		days := int(uptime.Hours() / 24)
		hours := int(uptime.Hours()) % 24
		minutes := int(uptime.Minutes()) % 60
		info.Uptime = fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}

	if cpuInfo, err := cpu.Info(); err == nil {
		var totalCores int32
		for _, c := range cpuInfo {
			totalCores += c.Cores
		}
		info.CPUCores = fmt.Sprintf("%d", totalCores)
	}

	if memInfo, err := mem.VirtualMemory(); err == nil {
		info.MemoryTotal = fmt.Sprintf("%.1f GB", float64(memInfo.Total)/1024/1024/1024)
		info.MemoryUsed = fmt.Sprintf("%.1f GB", float64(memInfo.Used)/1024/1024/1024)
	}

	if diskInfo, err := disk.Usage("/"); err == nil {
		info.DiskUsage = fmt.Sprintf("Total: %.1f GB, Used: %.1f GB, Free: %.1f GB (%.1f%%)",
			float64(diskInfo.Total)/1024/1024/1024,
			float64(diskInfo.Used)/1024/1024/1024,
			float64(diskInfo.Free)/1024/1024/1024,
			diskInfo.UsedPercent,
		)
	}

	info.PublicIP = getCachedPublicIP()

	return info
}

func getCachedResourceMetrics() (models.SystemMetrics, bool) {
	now := nowFunc()
	resourceMetricsMu.RLock()
	defer resourceMetricsMu.RUnlock()
	if !resourceMetricsInitialized || now.Sub(resourceMetricsCheckedAt) >= resourceMetricsTTL {
		return models.SystemMetrics{}, false
	}
	return resourceMetricsCache, true
}

func setCachedResourceMetrics(metrics models.SystemMetrics) {
	resourceMetricsMu.Lock()
	defer resourceMetricsMu.Unlock()
	resourceMetricsCache = metrics
	resourceMetricsCheckedAt = nowFunc()
	resourceMetricsInitialized = true
}

func getCachedCountMetrics() (models.SystemMetrics, bool) {
	now := nowFunc()
	countMetricsMu.RLock()
	defer countMetricsMu.RUnlock()
	if !countMetricsInitialized || now.Sub(countMetricsCheckedAt) >= countMetricsTTL {
		return models.SystemMetrics{}, false
	}
	return countMetricsCache, true
}

func setCachedCountMetrics(metrics models.SystemMetrics) {
	countMetricsMu.Lock()
	defer countMetricsMu.Unlock()
	countMetricsCache = metrics
	countMetricsCheckedAt = nowFunc()
	countMetricsInitialized = true
}

func getCachedServerInfo() (models.ServerInfo, bool) {
	now := nowFunc()
	serverInfoMu.RLock()
	defer serverInfoMu.RUnlock()
	if !serverInfoInitialized || now.Sub(serverInfoCheckedAt) >= serverInfoTTL {
		return models.ServerInfo{}, false
	}
	return serverInfoCache, true
}

func setCachedServerInfo(info models.ServerInfo) {
	serverInfoMu.Lock()
	defer serverInfoMu.Unlock()
	serverInfoCache = info
	serverInfoCheckedAt = nowFunc()
	serverInfoInitialized = true
}

func getCachedPublicIP() string {
	now := nowFunc()
	publicIPMu.RLock()
	value := publicIPCache
	checkedAt := publicIPCheckedAt
	refreshing := publicIPRefreshing
	publicIPMu.RUnlock()

	if value != "" && now.Sub(checkedAt) < publicIPTTL {
		return value
	}

	if !refreshing {
		refreshPublicIPAsync()
	}

	return value
}

func refreshPublicIPAsync() {
	publicIPMu.Lock()
	if publicIPRefreshing {
		publicIPMu.Unlock()
		return
	}
	publicIPRefreshing = true
	publicIPMu.Unlock()

	go func() {
		publicIPMu.RLock()
		ip := publicIPCache
		publicIPMu.RUnlock()

		refreshSucceeded := false
		resp, err := publicIPHTTPClient.Get(publicIPLookupURL)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
				if body, readErr := io.ReadAll(resp.Body); readErr == nil {
					trimmedIP := strings.TrimSpace(string(body))
					if trimmedIP != "" {
						ip = trimmedIP
						refreshSucceeded = true
					}
				}
			}
		}

		publicIPMu.Lock()
		if refreshSucceeded {
			publicIPCache = ip
			publicIPCheckedAt = nowFunc()
		}
		publicIPRefreshing = false
		publicIPMu.Unlock()
	}()
}

func countDokkuEntries(output string) int {
	trimmedOutput := strings.TrimSpace(output)
	if trimmedOutput == "" {
		return 0
	}

	count := 0
	for _, line := range strings.Split(trimmedOutput, "\n") {
		if line == "" || strings.Contains(line, "=====") {
			continue
		}
		count++
	}

	return count
}

func parseDokkuVersionOutput(output string) string {
	parts := strings.Fields(output)
	if len(parts) >= 3 {
		return parts[2]
	}

	trimmedOutput := strings.TrimSpace(output)
	if trimmedOutput == "" {
		return "Unknown"
	}

	return trimmedOutput
}

func resetSystemCaches() {
	resourceMetricsMu.Lock()
	resourceMetricsCache = models.SystemMetrics{}
	resourceMetricsCheckedAt = time.Time{}
	resourceMetricsInitialized = false
	resourceMetricsMu.Unlock()

	countMetricsMu.Lock()
	countMetricsCache = models.SystemMetrics{}
	countMetricsCheckedAt = time.Time{}
	countMetricsInitialized = false
	countMetricsMu.Unlock()

	serverInfoMu.Lock()
	serverInfoCache = models.ServerInfo{}
	serverInfoCheckedAt = time.Time{}
	serverInfoInitialized = false
	serverInfoMu.Unlock()

	publicIPMu.Lock()
	publicIPCache = ""
	publicIPCheckedAt = time.Time{}
	publicIPRefreshing = false
	publicIPMu.Unlock()
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
