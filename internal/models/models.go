package models

import (
	"encoding/json"
	"time"
)

type BaseData struct {
	HideNavigation bool        `json:"hide_navigation"`
	User           interface{} `json:"user"`
	Username       interface{} `json:"username"`
	AuthType       interface{} `json:"auth_type"`
	FlashMessage   interface{} `json:"flash_message"`
	FlashType      interface{} `json:"flash_type"`
}

type AppData struct {
	BaseData
	Apps []string
}

// AppSummary contains application summary information
type AppSummary struct {
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Version      string      `json:"version"`
	Repository   string      `json:"repository"`
	Running      interface{} `json:"running"`
	Deployed     bool        `json:"deployed"`
	CreatedAt    string      `json:"created_at"`
	LastDeployAt string      `json:"last_deploy_at"`
}

type ProcessReport struct {
	Name    string
	Running string
	Status  string
	Ports   string
}

type AppDetailData struct {
	BaseData
	Name          string
	Description   string
	Version       string
	RepositoryURL string
	CreatedAt     string
	LastDeployAt  string
}

type SSHKey struct {
	Fingerprint string `json:"fingerprint"`
	Name        string `json:"name"`
	Allowed     string `json:"SSHCOMMAND_ALLOWED_KEYS"`
}

type SSHKeyData struct {
	BaseData
	Keys []SSHKey
}

type SSHKeyRequest struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type DomainInfo struct {
	Domains []string
}

type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type EnvVarRequest struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Restart bool   `json:"restart"`
}

type PortMapping struct {
	Protocol  string `json:"protocol"`
	Host      string `json:"host"`
	Container string `json:"container"`
}

type ProcessInfo struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type StorageMount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Size        string `json:"size"`
}

type StorageInfo struct {
	Mounts    []StorageMount `json:"mounts"`
	DiskUsage string         `json:"disk_usage"`
}

type Plugin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Database struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Version string `json:"version"`
}

type Container struct {
	ID          string `json:"id"`
	Image       string `json:"image"`
	Command     string `json:"command"`
	Status      string `json:"status"`
	Ports       string `json:"ports"`
	PortDetails string `json:"port_details"`
	Name        string `json:"name"`
}

type NginxConfig struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type CronJob struct {
	ID       string `json:"id"`
	App      string `json:"app"`
	Command  string `json:"command"`
	Schedule string `json:"schedule"`
}

type SystemMetrics struct {
	CPUUsage       float64 `json:"cpu_usage"`
	CPUInfo        string  `json:"cpu_info"`
	LoadAvg        string  `json:"load_avg"` // Newly added
	RAMUsage       float64 `json:"ram_usage"`
	RAMInfo        string  `json:"ram_info"`
	SwapUsage      float64 `json:"swap_usage"` // Newly added
	SwapInfo       string  `json:"swap_info"`  // Newly added
	DiskUsage      float64 `json:"disk_usage"`
	DiskInfo       string  `json:"disk_info"`
	DokkuVersion   string  `json:"dokku_version"`
	ContainerCount int     `json:"container_count"`
	ServiceCount   int     `json:"service_count"`
	AppCount       int     `json:"app_count"`
	// Network metrics
	NetBytesRecv     uint64  `json:"net_bytes_recv"`      // Current bytes received
	NetBytesSent     uint64  `json:"net_bytes_sent"`      // Current bytes sent
	NetBytesRecvRate float64 `json:"net_bytes_recv_rate"` // Current receive rate in bytes/sec
	NetBytesSentRate float64 `json:"net_bytes_sent_rate"` // Current send rate in bytes/sec
	NetInfo          string  `json:"net_info"`            // Human-readable network information
	NetInterfaceName string  `json:"net_interface_name"`  // Name of the primary network interface
}

type CPUStats struct {
	User    uint64
	Nice    uint64
	System  uint64
	Idle    uint64
	Iowait  uint64
	Irq     uint64
	Softirq uint64
	Steal   uint64
}

type BackupFile struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Database string `json:"database"`
}

type AppStatus struct {
	Deployed  bool           `json:"deployed"`
	Running   interface{}    `json:"running"` // Changed from bool to interface{} to support "mixed" string // Changed to interface{}
	Processes map[string]int `json:"processes"`
}

type AppRestartOperation struct {
	ID          string     `json:"id"`
	AppName     string     `json:"app_name"`
	Action      string     `json:"action"`
	ProcessType string     `json:"process_type,omitempty"`
	Reused      bool       `json:"reused,omitempty"`
	State       string     `json:"state"`
	Message     string     `json:"message"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

type ServerInfo struct {
	Hostname    string `json:"hostname"`
	OS          string `json:"os"`
	Kernel      string `json:"kernel"`
	Uptime      string `json:"uptime"`
	CPUCores    string `json:"cpu_cores"`
	MemoryTotal string `json:"memory_total"`
	MemoryUsed  string `json:"memory_used"`
	DiskUsage   string `json:"disk_usage"`
	PublicIP    string `json:"public_ip"`
}

type DomainRequest struct {
	Domain string `json:"domain"`
}

type DockerStats struct {
	Version           string `json:"version"`
	RunningContainers int    `json:"running_containers"`
	TotalContainers   int    `json:"total_containers"`
	TotalImages       int    `json:"total_images"`
}

type DockerInfo struct {
	ID                string `json:"ID"`
	Containers        int    `json:"Containers"`
	ContainersRunning int    `json:"ContainersRunning"`
	ContainersPaused  int    `json:"ContainersPaused"`
	ContainersStopped int    `json:"ContainersStopped"`
	Images            int    `json:"Images"`
	NCPU              int    `json:"NCPU"`
	MemTotal          int64  `json:"MemTotal"`
	DockerRootDir     string `json:"DockerRootDir"`
	ServerVersion     string `json:"ServerVersion"`
	OperatingSystem   string `json:"OperatingSystem"`
	KernelVersion     string `json:"KernelVersion"`
	Architecture      string `json:"Architecture"`
}

type AvailablePlugin struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	IsInstalled bool   `json:"is_installed"`
}

type Redirect struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Code        string `json:"code"`
}

type ScaleRequest struct {
	Scale      string `json:"scale"`
	SkipDeploy bool   `json:"skipDeploy"`
}

type SSLInfo struct {
	Active     bool   `json:"active"`
	Autorenew  bool   `json:"autorenew"`
	Email      string `json:"email"`
	Expiration int64  `json:"expiration"`
}

type StorageMountRequest struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Chmod       string `json:"chmod"`
}

type CreateAppRequest struct {
	Name     string         `json:"name"`
	Services []ServiceInfo  `json:"services"`
	Env      []EnvVar       `json:"env"`
	Domain   string         `json:"domain"`
	SSL      bool           `json:"ssl"`
	Port     PortMapping    `json:"port"`
	Mounts   []StorageMount `json:"mounts"`
}

// ServiceInfo describes a service to be created and linked to an app
type ServiceInfo struct {
	Name         string `json:"name"`
	Image        string `json:"image,omitempty"`
	ImageVersion string `json:"imageVersion,omitempty"`
}

type StepResult struct {
	Message  string `json:"message"`
	Progress int    `json:"progress"`
	Error    string `json:"error,omitempty"`
}

type ContainerStats struct {
	CPUUsage      float64 `json:"cpu_usage"`
	MemoryUsage   float64 `json:"memory_usage"`
	DiskUsage     float64 `json:"disk_usage"`      // Percentage usage
	DiskUsageText string  `json:"disk_usage_text"` // E.g: "1.2GB/10GB"
	IsDeployed    bool    `json:"is_deployed"`
}

// ServiceInstanceInfo represents details about a service
type ServiceInstanceInfo struct {
	Name           string            `json:"name"`
	Service        string            `json:"service"`
	Type           string            `json:"type"`
	Version        string            `json:"version"`
	Status         string            `json:"status"`
	URL            string            `json:"url"`
	InstanceName   string            `json:"instance_name,omitempty"`
	ContainerID    string            `json:"container_id,omitempty"`
	Details        map[string]string `json:"details,omitempty"`
	ResourceLimits *ResourceLimits   `json:"resource_limits,omitempty"`
	LinkInfo       *LinkInfo         `json:"link_info,omitempty"`
}

// LinkInfo represents information about linked applications
type LinkInfo struct {
	LinkedApps []string `json:"linked_apps"`
}

// AppServiceInfo contains information about services linked to an app
type AppServiceInfo struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Provider     string `json:"provider"`
	URL          string `json:"url"`
	Image        string `json:"image,omitempty"`
	ImageVersion string `json:"image_version,omitempty"`
}

type ActivityLog struct {
	Time       time.Time       `json:"time"`
	RequestID  string          `json:"request_id"`
	IP         string          `json:"ip"`
	User       string          `json:"user"`
	AuthType   string          `json:"auth_type"`
	Action     string          `json:"action"`
	Method     string          `json:"method"` // Add HTTP method field
	Route      string          `json:"route"`
	Parameters json.RawMessage `json:"parameters,omitempty"`
	StatusCode int             `json:"status_code"`
	Error      string          `json:"error,omitempty"`
}

type LogSearchParams struct {
	Username string `json:"username"`
	Query    string `json:"query"`
	Page     int    `json:"page"`
	PerPage  int    `json:"per_page"`
}

type LogSearchResult struct {
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	TotalPages int           `json:"total_pages"`
	TotalLogs  int           `json:"total_logs"`
	Logs       []ActivityLog `json:"logs"`
}

type ResourceLimits struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

type Service struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Version string `json:"version"`
}
