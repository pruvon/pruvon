package apps

import "github.com/pruvon/pruvon/internal/models"

// ProcessReport summarizes Dokku process information for an app.
type ProcessReport struct {
	Output     string               `json:"output"`
	Info       []models.ProcessInfo `json:"info"`
	Containers []models.Container   `json:"containers"`
}

// ConfigResult represents the output of dokku config commands.
type ConfigResult struct {
	Output string          `json:"output"`
	Vars   []models.EnvVar `json:"vars"`
}

// PortsResult wraps Dokku ports report output.
type PortsResult struct {
	Output string               `json:"output"`
	Ports  []models.PortMapping `json:"ports"`
}

// DomainsResult wraps Dokku domain report output.
type DomainsResult struct {
	Output  string   `json:"output"`
	Domains []string `json:"domains"`
}

// NginxResult wraps Dokku nginx report output.
type NginxResult struct {
	Configs []models.NginxConfig `json:"configs"`
}

// SSLEnableResult reports the outcome of enabling Let's Encrypt.
type SSLEnableResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
	Output  string `json:"output,omitempty"`
}
