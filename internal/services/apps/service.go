package apps

import (
	"errors"
	"fmt"
	neturl "net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/pruvon/pruvon/internal/docker"
	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/models"
)

// Service coordinates application-level operations that rely on Dokku commands.
type Service struct {
	runner dokku.CommandRunner
}

var (
	// ErrInvalidOptionType indicates that a docker option type other than build/deploy/run was supplied.
	ErrInvalidOptionType = errors.New("invalid option type")
	// ErrIndexOutOfRange indicates that a provided index does not exist for the requested docker option list.
	ErrIndexOutOfRange = errors.New("index out of range")
	// ErrInvalidRestartPolicy indicates that a restart policy value is not supported by Dokku.
	ErrInvalidRestartPolicy = errors.New("invalid restart policy")
)

// NewService constructs a Service with the provided command runner. A nil runner falls back to dokku.DefaultCommandRunner.
func NewService(runner dokku.CommandRunner) *Service {
	if runner == nil {
		runner = dokku.DefaultCommandRunner
	}
	return &Service{runner: runner}
}

// CommandRunner exposes the underlying command runner, mainly for tests.
func (s *Service) CommandRunner() dokku.CommandRunner {
	return s.runner
}

func (s *Service) runDokku(args ...string) (string, error) {
	return s.runner.RunCommand("dokku", args...)
}

// RunDokkuCommand executes a dokku command and returns its output.
func (s *Service) RunDokkuCommand(args ...string) (string, error) {
	return s.runDokku(args...)
}

// GetProcessReport returns Dokku process metrics for the given app name.
func (s *Service) GetProcessReport(appName string) (*ProcessReport, error) {
	if appName == "" {
		return nil, fmt.Errorf("app name cannot be empty")
	}

	output, err := s.runner.RunCommand("dokku", "ps:report", appName)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch process info: %w", err)
	}

	info := dokku.ParseProcessInfo(output)

	containers, err := dokku.GetAppContainers(s.runner, appName)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch containers: %w", err)
	}

	return &ProcessReport{
		Output:     output,
		Info:       info,
		Containers: containers,
	}, nil
}

// GetConfig returns dokku config output and parsed variables for an app.
func (s *Service) GetConfig(appName string) (*ConfigResult, error) {
	if appName == "" {
		return nil, fmt.Errorf("app name cannot be empty")
	}

	output, err := s.runner.RunCommand("dokku", "config", appName)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch config: %w", err)
	}

	vars := dokku.ParseEnvVars(output)
	return &ConfigResult{
		Output: output,
		Vars:   vars,
	}, nil
}

// SetConfig updates or creates a Dokku config key for an app.
func (s *Service) SetConfig(appName, key, value string, restart bool) error {
	if appName == "" || key == "" {
		return fmt.Errorf("app name and key cannot be empty")
	}

	args := []string{"config:set"}
	if !restart {
		args = append(args, "--no-restart")
	}
	args = append(args, appName, fmt.Sprintf("%s=%s", key, value))

	output, err := s.runner.RunCommand("dokku", args...)
	if err != nil {
		return fmt.Errorf("unable to set config: %w", err)
	}

	if strings.Contains(strings.ToLower(output), "error") {
		return errors.New(output)
	}

	return nil
}

// UnsetConfig removes a Dokku config key for an app.
func (s *Service) UnsetConfig(appName, key string, restart bool) error {
	if appName == "" || key == "" {
		return fmt.Errorf("app name and key cannot be empty")
	}

	args := []string{"config:unset"}
	if !restart {
		args = append(args, "--no-restart")
	}
	args = append(args, appName, key)

	output, err := s.runner.RunCommand("dokku", args...)
	if err != nil {
		return fmt.Errorf("unable to unset config: %w", err)
	}

	if strings.Contains(strings.ToLower(output), "error") {
		return errors.New(output)
	}

	return nil
}

// GetPorts returns Dokku port mappings for an app.
func (s *Service) GetPorts(appName string) (*PortsResult, error) {
	if appName == "" {
		return nil, fmt.Errorf("app name cannot be empty")
	}

	output, err := s.runner.RunCommand("dokku", "ports:report", appName)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch ports: %w", err)
	}

	ports := dokku.ParsePorts(output)
	return &PortsResult{
		Output: output,
		Ports:  ports,
	}, nil
}

// GetDomains returns Dokku domain information for an app.
func (s *Service) GetDomains(appName string) (*DomainsResult, error) {
	if appName == "" {
		return nil, fmt.Errorf("app name cannot be empty")
	}

	output, err := s.runner.RunCommand("dokku", "domains:report", appName)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch domains: %w", err)
	}

	domains := dokku.ParseDomains(output)
	return &DomainsResult{
		Output:  output,
		Domains: domains,
	}, nil
}

// GetStorageInfo returns storage mounts and usage details for an app.
func (s *Service) GetStorageInfo(appName string) (*models.StorageInfo, error) {
	if appName == "" {
		return nil, fmt.Errorf("app name cannot be empty")
	}

	info, err := dokku.ParseStorageInfo(s.runner, appName)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch storage info: %w", err)
	}

	return &info, nil
}

// GetNginxConfig returns parsed nginx configuration for an app.
func (s *Service) GetNginxConfig(appName string) (*NginxResult, error) {
	if appName == "" {
		return nil, fmt.Errorf("app name cannot be empty")
	}

	output, err := s.runner.RunCommand("dokku", "nginx:report", appName)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch nginx info: %w", err)
	}

	configs := dokku.ParseNginxConfig(output)
	return &NginxResult{Configs: configs}, nil
}

// SetNginxConfig updates a specific nginx configuration value and rebuilds the proxy.
func (s *Service) SetNginxConfig(appName, configKey, value string) error {
	if appName == "" || configKey == "" {
		return fmt.Errorf("app name and config key cannot be empty")
	}

	if _, err := s.runner.RunCommand("dokku", "nginx:set", appName, configKey, value); err != nil {
		return fmt.Errorf("unable to set nginx config: %w", err)
	}

	if _, err := s.runner.RunCommand("dokku", "proxy:build-config", appName); err != nil {
		return fmt.Errorf("unable to rebuild proxy config: %w", err)
	}

	return nil
}

// GetNginxCustomConfigPath returns the current custom nginx configuration path for an app.
func (s *Service) GetNginxCustomConfigPath(appName string) (string, error) {
	if appName == "" {
		return "", fmt.Errorf("app name cannot be empty")
	}

	output, err := s.runner.RunCommand("dokku", "nginx:report", appName, "--nginx-computed-nginx-conf-sigil-path")
	if err != nil {
		return "", fmt.Errorf("unable to fetch custom nginx config path: %w", err)
	}

	return strings.TrimSpace(output), nil
}

// SetNginxCustomConfigPath sets the nginx-conf-sigil-path and rebuilds proxy configuration.
func (s *Service) SetNginxCustomConfigPath(appName, path string) error {
	if appName == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	output, err := s.runner.RunCommand("dokku", "nginx:set", appName, "nginx-conf-sigil-path", path)
	if err != nil {
		return fmt.Errorf("failed to set custom nginx config path: %w - %s", err, output)
	}

	if _, err := s.runner.RunCommand("dokku", "proxy:build-config", appName); err != nil {
		return fmt.Errorf("failed to build proxy configuration: %w", err)
	}

	return nil
}

// ResetNginxCustomConfigPath clears the custom nginx-conf-sigil-path value and rebuilds proxy config.
func (s *Service) ResetNginxCustomConfigPath(appName string) error {
	if appName == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	output, err := s.runner.RunCommand("dokku", "nginx:set", appName, "nginx-conf-sigil-path")
	if err != nil {
		return fmt.Errorf("failed to reset custom nginx config path: %w - %s", err, output)
	}

	if _, err := s.runner.RunCommand("dokku", "proxy:build-config", appName); err != nil {
		return fmt.Errorf("failed to build proxy configuration: %w", err)
	}

	return nil
}

// GetAppSummary returns running containers for the given app.
func (s *Service) GetAppSummary(appName string) ([]models.Container, error) {
	if appName == "" {
		return nil, fmt.Errorf("app name cannot be empty")
	}

	containers, err := dokku.GetAppContainers(s.runner, appName)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch containers: %w", err)
	}

	return containers, nil
}

// KillContainer stops a Docker container by ID.
func (s *Service) KillContainer(containerID string) error {
	if strings.TrimSpace(containerID) == "" {
		return fmt.Errorf("container id cannot be empty")
	}

	if _, err := s.runner.RunCommand("docker", "kill", containerID); err != nil {
		return fmt.Errorf("unable to kill container: %w", err)
	}

	return nil
}

// GetCronJobs returns scheduled Dokku cron jobs for the app.
func (s *Service) GetCronJobs(appName string) ([]models.CronJob, error) {
	if appName == "" {
		return nil, fmt.Errorf("app name cannot be empty")
	}

	jobs, err := dokku.GetAppCronJobs(s.runner, appName)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch cron jobs: %w", err)
	}

	return jobs, nil
}

// StartApp starts an app optionally filtered by process type.
func (s *Service) StartApp(appName, processType string) error {
	if appName == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	args := []string{"ps:start", appName}
	if processType != "" {
		args = append(args, processType)
	}

	if _, err := s.runDokku(args...); err != nil {
		return fmt.Errorf("unable to start app: %w", err)
	}
	return nil
}

// StopApp stops an app optionally filtered by process type.
func (s *Service) StopApp(appName, processType string) error {
	if appName == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	args := []string{"ps:stop", appName}
	if processType != "" {
		args = append(args, processType)
	}

	if _, err := s.runDokku(args...); err != nil {
		return fmt.Errorf("unable to stop app: %w", err)
	}
	return nil
}

// RestartApp restarts an app optionally filtered by process type.
func (s *Service) RestartApp(appName, processType string) error {
	if appName == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	args := []string{"ps:restart", appName}
	if processType != "" {
		args = append(args, processType)
	}

	if _, err := s.runDokku(args...); err != nil {
		return fmt.Errorf("unable to restart app: %w", err)
	}
	return nil
}

// RebuildApp rebuilds an app optionally filtered by process type.
func (s *Service) RebuildApp(appName, processType string) error {
	if appName == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	args := []string{"ps:rebuild", appName}
	if processType != "" {
		args = append(args, processType)
	}

	if _, err := s.runDokku(args...); err != nil {
		return fmt.Errorf("unable to rebuild app: %w", err)
	}
	return nil
}

// GetStatus returns aggregated Dokku ps:report data as AppStatus.
func (s *Service) GetStatus(appName string) (*models.AppStatus, error) {
	if appName == "" {
		return nil, fmt.Errorf("app name cannot be empty")
	}

	output, err := s.runDokku("ps:report", appName)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch app status: %w", err)
	}

	status := &models.AppStatus{
		Processes: make(map[string]int),
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Deployed:") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				status.Deployed = strings.TrimSpace(parts[1]) == "true"
			}
		} else if strings.Contains(line, "Running:") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				switch strings.TrimSpace(parts[1]) {
				case "true":
					status.Running = true
				case "mixed":
					status.Running = "mixed"
				default:
					status.Running = false
				}
			}
		} else if strings.HasPrefix(line, "Status ") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				procType := parts[1]
				if _, exists := status.Processes[procType]; !exists {
					status.Processes[procType] = 0
				}
				if strings.Contains(line, "running") {
					status.Processes[procType]++
				}
			}
		}
	}

	return status, nil
}

// GetContainerStats returns docker stats for an app.
func (s *Service) GetContainerStats(appName string) models.ContainerStats {
	return docker.GetContainerStats(s.runner, appName)
}

// AddDomain adds a domain to an app.
func (s *Service) AddDomain(appName, domain string) (string, error) {
	if appName == "" || domain == "" {
		return "", fmt.Errorf("app name and domain cannot be empty")
	}

	output, err := s.runDokku("domains:add", appName, domain)
	if err != nil {
		return "", fmt.Errorf("unable to add domain: %w", err)
	}

	return output, nil
}

// EnableSSL attempts to enable Let's Encrypt for an app and returns a structured result.
func (s *Service) EnableSSL(appName string) (*SSLEnableResult, error) {
	if appName == "" {
		return nil, fmt.Errorf("app name cannot be empty")
	}

	installed, err := dokku.IsLetsencryptInstalled(s.runner)
	if err != nil {
		return nil, fmt.Errorf("plugin check failed: %w", err)
	}

	if !installed {
		return &SSLEnableResult{
			Success: false,
			Error:   "Letsencrypt plugin is not installed",
			Message: "The letsencrypt plugin is not installed. Please install it first with 'sudo -n -u dokku dokku plugin:install https://github.com/dokku/dokku-letsencrypt.git'",
		}, nil
	}

	domains, err := dokku.GetDomains(s.runner, appName)
	if err != nil {
		return nil, fmt.Errorf("domain check failed: %w", err)
	}

	if len(domains) == 0 {
		return &SSLEnableResult{
			Success: false,
			Error:   "No domains configured",
			Message: "You need to configure at least one domain for this app before enabling SSL",
		}, nil
	}

	output, err := s.runDokku("letsencrypt:enable", appName)
	if err != nil {
		result := &SSLEnableResult{
			Success: false,
			Output:  output,
		}

		switch {
		case strings.Contains(output, "App must be deployed"):
			result.Error = "App must be deployed before enabling SSL"
			result.Message = "The application must be deployed before enabling SSL certificates"
		case strings.Contains(output, "no cert to delete"):
			result.Error = "No existing certificate to update"
			result.Message = "There is no existing certificate to update"
		case strings.Contains(output, "app does not exist"):
			result.Error = "Application does not exist"
			result.Message = "The application does not exist"
		case strings.Contains(output, "forbidden by policy") || strings.Contains(output, "rejectedIdentifier"):
			result.Error = "Domain not allowed for Let's Encrypt"
			result.Message = "Your domain is not allowed for Let's Encrypt certificates. Example.com and similar reserved domains cannot be used. Please configure a real domain."
		case strings.Contains(output, "not be verified") || strings.Contains(output, "challenge failed"):
			result.Error = "Domain verification failed"
			result.Message = "Let's Encrypt could not verify domain ownership. Make sure your DNS is configured correctly and the app is accessible via all domains."
		case strings.Contains(output, "rate limit"):
			result.Error = "Let's Encrypt rate limit exceeded"
			result.Message = "Let's Encrypt rate limit exceeded. Please wait before trying again."
		default:
			result.Error = fmt.Sprintf("SSL could not be enabled: %v", err)
			result.Message = "SSL certificates could not be enabled"
		}

		return result, nil
	}

	return &SSLEnableResult{
		Success: true,
		Output:  output,
		Message: "SSL certificates have been successfully enabled",
	}, nil
}

// RemoveDomain removes a domain from an app.
func (s *Service) RemoveDomain(appName, domain string) (string, error) {
	if appName == "" || domain == "" {
		return "", fmt.Errorf("app name and domain cannot be empty")
	}

	output, err := s.runDokku("domains:remove", appName, domain)
	if err != nil {
		return "", fmt.Errorf("unable to remove domain: %w", err)
	}

	return output, nil
}

// GetRedirects retrieves redirect configuration for an app.
func (s *Service) GetRedirects(appName string) ([]models.Redirect, bool, error) {
	if appName == "" {
		return nil, false, fmt.Errorf("app name cannot be empty")
	}

	redirects, err := dokku.GetRedirects(s.runner, appName)
	if err != nil {
		return nil, false, fmt.Errorf("unable to fetch redirects: %w", err)
	}

	installed, err := dokku.IsRedirectPluginInstalled(s.runner)
	if err != nil {
		return nil, false, fmt.Errorf("unable to check redirect plugin: %w", err)
	}

	return redirects, installed, nil
}

// SetRedirect configures a redirect for an app.
func (s *Service) SetRedirect(appName string, redirect models.Redirect) (string, error) {
	if appName == "" || redirect.Source == "" || redirect.Destination == "" {
		return "", fmt.Errorf("app name, source, and destination cannot be empty")
	}

	code := redirect.Code
	if code == "" {
		code = "301"
	}

	output, err := s.runDokku("redirect:set", appName, redirect.Source, redirect.Destination, code)
	if err != nil {
		return "", fmt.Errorf("unable to set redirect: %w", err)
	}

	return output, nil
}

// UnsetRedirect removes a redirect for an app.
func (s *Service) UnsetRedirect(appName string, redirect models.Redirect) (string, error) {
	if appName == "" || redirect.Source == "" {
		return "", fmt.Errorf("app name and source cannot be empty")
	}

	output, err := s.runDokku("redirect:unset", appName, redirect.Source)
	if err != nil {
		return "", fmt.Errorf("unable to unset redirect: %w", err)
	}

	return output, nil
}

// AddPortMapping adds a port mapping to an app.
func (s *Service) AddPortMapping(appName string, port models.PortMapping) error {
	if appName == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	mapping := fmt.Sprintf("%s:%s:%s", port.Protocol, port.Host, port.Container)
	if _, err := s.runDokku("ports:add", appName, mapping); err != nil {
		return fmt.Errorf("unable to add port: %w", err)
	}

	return nil
}

// RemovePortMapping removes a port mapping from an app.
func (s *Service) RemovePortMapping(appName string, port models.PortMapping) error {
	if appName == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	mapping := fmt.Sprintf("%s:%s:%s", port.Protocol, port.Host, port.Container)
	if _, err := s.runDokku("ports:remove", appName, mapping); err != nil {
		return fmt.Errorf("unable to remove port: %w", err)
	}

	return nil
}

// ScaleApp scales processes for an app.
func (s *Service) ScaleApp(appName string, req models.ScaleRequest) (string, error) {
	if appName == "" {
		return "", fmt.Errorf("app name cannot be empty")
	}

	args := []string{"ps:scale", appName}
	if strings.TrimSpace(req.Scale) != "" {
		args = append(args, strings.Split(req.Scale, " ")...)
	}
	if req.SkipDeploy {
		args = append(args, "--skip-deploy")
	}

	output, err := s.runDokku(args...)
	if err != nil {
		return "", fmt.Errorf("unable to scale app: %w", err)
	}

	return output, nil
}

// GetSSLInfo returns ssl information for an app.
func (s *Service) GetSSLInfo(appName string) (*models.SSLInfo, error) {
	if appName == "" {
		return nil, fmt.Errorf("app name cannot be empty")
	}

	info, err := dokku.GetSSLInfo(s.runner, appName)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch ssl info: %w", err)
	}

	return &info, nil
}

// GetAppServices returns the list of services linked to an application along with metadata.
func (s *Service) GetAppServices(appName string) ([]models.AppServiceInfo, error) {
	if appName == "" {
		return nil, fmt.Errorf("app name cannot be empty")
	}

	serviceTypes, err := dokku.GetAvailableServicePluginList(s.runner)
	if err != nil {
		return nil, fmt.Errorf("unable to determine available service plugins: %w", err)
	}

	if len(serviceTypes) == 0 {
		return []models.AppServiceInfo{}, nil
	}

	typeNames := defaultServiceTypeNames()
	results := make(map[string]*models.AppServiceInfo)

	for _, serviceType := range serviceTypes {
		serviceNames, err := s.getAppLinkedServices(appName, serviceType)
		if err != nil {
			continue
		}

		for _, serviceName := range serviceNames {
			info, err := s.buildAppServiceInfo(appName, serviceType, serviceName, typeNames)
			if err != nil || info == nil {
				continue
			}
			key := generateDatabaseKey(info.URL, serviceType)
			results[key] = info
		}
	}

	if len(results) == 0 {
		fallback, err := s.discoverServicesFromConfig(appName, serviceTypes, typeNames)
		if err == nil {
			for key, info := range fallback {
				results[key] = info
			}
		}
	}

	services := make([]models.AppServiceInfo, 0, len(results))
	for _, info := range results {
		services = append(services, *info)
	}

	sort.Slice(services, func(i, j int) bool {
		if services[i].Type == services[j].Type {
			return services[i].Name < services[j].Name
		}
		return services[i].Type < services[j].Type
	})

	return services, nil
}

func (s *Service) getAppLinkedServices(appName, serviceType string) ([]string, error) {
	cmd := fmt.Sprintf("%s:app-links", serviceType)
	output, err := s.runDokku(cmd, appName)
	if (err != nil || strings.TrimSpace(output) == "") && s.runner == dokku.DefaultCommandRunner {
		altOutput, altErr := s.runner.RunCommand("dokku", cmd, appName)
		if altErr == nil && strings.TrimSpace(altOutput) != "" {
			output = altOutput
			err = nil
		} else if err == nil {
			err = altErr
		}
	}
	if err != nil {
		return nil, err
	}

	var serviceNames []string
	for _, line := range strings.Split(output, "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			serviceNames = append(serviceNames, name)
		}
	}

	return serviceNames, nil
}

func (s *Service) buildAppServiceInfo(appName, serviceType, serviceName string, typeNames map[string]string) (*models.AppServiceInfo, error) {
	infoCmd := fmt.Sprintf("%s:info", serviceType)
	output, err := s.runDokku(infoCmd, serviceName)
	if err != nil {
		return nil, err
	}

	serviceURL := extractServiceURL(output)
	if serviceURL == "" {
		configOutput, configErr := s.runDokku("config", appName)
		if configErr == nil {
			envVars := dokku.ParseEnvVars(configOutput)
			for _, envVar := range envVars {
				if isDbConnectionEnvVar(envVar.Key, serviceType) && envVar.Value != "" {
					needle := fmt.Sprintf("dokku-%s-%s", serviceType, serviceName)
					if strings.Contains(envVar.Value, needle) {
						serviceURL = envVar.Value
						break
					}
				}
			}
		}
	}

	if serviceURL == "" || !isValidDatabaseURL(serviceURL, serviceType) {
		return nil, nil
	}

	serviceConfig := make(map[string]string)
	if cfg, cfgErr := dokku.GetServiceConfig(s.runner, serviceType, serviceName); cfgErr == nil && cfg != nil {
		serviceConfig = cfg
	}

	displayType := typeNames[serviceType]
	if displayType == "" && serviceType != "" {
		displayType = strings.ToUpper(serviceType[:1]) + serviceType[1:]
	}

	return &models.AppServiceInfo{
		Name:         serviceName,
		Type:         displayType,
		Provider:     "dokku",
		URL:          serviceURL,
		Image:        serviceConfig["image"],
		ImageVersion: serviceConfig["image_version"],
	}, nil
}

func (s *Service) discoverServicesFromConfig(appName string, serviceTypes []string, typeNames map[string]string) (map[string]*models.AppServiceInfo, error) {
	configOutput, err := s.runDokku("config", appName)
	if err != nil {
		return nil, err
	}

	envVars := dokku.ParseEnvVars(configOutput)
	serviceMap := s.collectAllServiceNames(serviceTypes)
	results := make(map[string]*models.AppServiceInfo)

	for _, envVar := range envVars {
		value := strings.TrimSpace(envVar.Value)
		if value == "" || !isDbConnectionVar(envVar.Key) || !isValidDatabaseURL(value, "") {
			continue
		}

		svcType, defaultName := detectServiceTypeFromURL(value)
		if svcType == "" {
			continue
		}

		serviceName := inferServiceName(value, svcType, envVar.Key, appName, serviceMap[svcType])
		displayType := typeNames[svcType]
		if displayType == "" {
			displayType = defaultName
		}

		key := generateDatabaseKey(value, svcType)
		results[key] = &models.AppServiceInfo{
			Name:     serviceName,
			Type:     displayType,
			Provider: "dokku",
			URL:      value,
		}
	}

	return results, nil
}

func (s *Service) collectAllServiceNames(serviceTypes []string) map[string][]string {
	serviceMap := make(map[string][]string)
	for _, serviceType := range serviceTypes {
		output, err := s.runDokku(fmt.Sprintf("%s:list", serviceType))
		if err != nil {
			continue
		}

		var names []string
		for _, line := range strings.Split(output, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "===") {
				continue
			}
			names = append(names, trimmed)
		}

		if len(names) > 0 {
			serviceMap[serviceType] = names
		}
	}

	return serviceMap
}

func defaultServiceTypeNames() map[string]string {
	return map[string]string{
		"postgres":      "PostgreSQL",
		"mariadb":       "MariaDB",
		"mongo":         "MongoDB",
		"mongodb":       "MongoDB",
		"redis":         "Redis",
		"elasticsearch": "Elasticsearch",
		"rabbitmq":      "RabbitMQ",
		"memcached":     "Memcached",
		"clickhouse":    "ClickHouse",
		"nats":          "NATS",
		"solr":          "Solr",
		"rethinkdb":     "RethinkDB",
		"couchdb":       "CouchDB",
		"mysql":         "MySQL",
	}
}

func extractServiceURL(report string) string {
	for _, line := range strings.Split(report, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "DSN:") || strings.Contains(line, "URL:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func detectServiceTypeFromURL(url string) (string, string) {
	switch {
	case strings.HasPrefix(url, "postgres://"):
		return "postgres", "PostgreSQL"
	case strings.HasPrefix(url, "mysql://"):
		return "mariadb", "MariaDB"
	case strings.HasPrefix(url, "mongodb://"):
		return "mongo", "MongoDB"
	case strings.HasPrefix(url, "redis://"):
		return "redis", "Redis"
	default:
		return "", ""
	}
}

func inferServiceName(url, serviceType, envKey, appName string, candidates []string) string {
	needle := fmt.Sprintf("dokku-%s-", serviceType)
	for _, candidate := range candidates {
		if strings.Contains(url, needle+candidate) {
			return candidate
		}
	}

	parts := strings.Split(url, "@"+needle)
	if len(parts) > 1 {
		suffixParts := strings.Split(parts[1], ":")
		if len(suffixParts) > 0 && suffixParts[0] != "" {
			return suffixParts[0]
		}
	}

	if strings.EqualFold(envKey, "DATABASE_URL") {
		return appName + "-db"
	}

	cleaned := strings.ToLower(strings.Replace(envKey, "_URL", "", 1))
	cleaned = strings.ReplaceAll(cleaned, "_", "-")
	if cleaned != "" {
		return cleaned
	}

	return fmt.Sprintf("%s-service", serviceType)
}

// generateDatabaseKey creates a unique key for a database based on its URL to prevent duplicate entries.
func generateDatabaseKey(url string, dbType string) string {
	if dbType == "" {
		dbType, _ = detectServiceTypeFromURL(url)
	}

	cleanURL := url
	if parsedURL, err := neturl.Parse(url); err == nil {
		hostname := parsedURL.Hostname()
		if hostname != "" {
			return fmt.Sprintf("%s:%s", dbType, hostname)
		}
	}

	return fmt.Sprintf("%s:%s", dbType, cleanURL)
}

func isValidDatabaseURL(url string, dbType string) bool {
	errorIndicators := []string{
		"illegal", "Already linked", "already linked",
		"error", "Error", "ERROR",
		"!", "usage:", "Usage:", "USAGE:",
		"/var/lib/dokku/plugins",
		"help", "Help", "HELP",
	}

	for _, indicator := range errorIndicators {
		if strings.Contains(url, indicator) {
			return false
		}
	}

	if dbType == "" {
		return strings.HasPrefix(url, "postgres://") ||
			strings.HasPrefix(url, "mysql://") ||
			strings.HasPrefix(url, "mongodb://") ||
			strings.HasPrefix(url, "redis://")
	}

	switch dbType {
	case "postgres":
		return strings.HasPrefix(url, "postgres://")
	case "mariadb":
		return strings.HasPrefix(url, "mysql://")
	case "mongo":
		return strings.HasPrefix(url, "mongodb://")
	case "redis":
		return strings.HasPrefix(url, "redis://")
	default:
		return false
	}
}

func isDbConnectionEnvVar(key string, dbType string) bool {
	key = strings.ToUpper(key)

	switch dbType {
	case "postgres":
		return key == "DATABASE_URL" || key == "POSTGRES_URL" || strings.HasSuffix(key, "_DATABASE_URL") || strings.HasSuffix(key, "_POSTGRES_URL")
	case "mariadb":
		return key == "DATABASE_URL" || key == "MYSQL_URL" || key == "MARIADB_URL" || strings.HasSuffix(key, "_DATABASE_URL") || strings.HasSuffix(key, "_MYSQL_URL") || strings.HasSuffix(key, "_MARIADB_URL")
	case "mongo":
		return key == "MONGODB_URL" || key == "MONGO_URL" || strings.HasSuffix(key, "_MONGODB_URL") || strings.HasSuffix(key, "_MONGO_URL")
	case "redis":
		return key == "REDIS_URL" || strings.HasSuffix(key, "_REDIS_URL")
	default:
		return false
	}
}

func isDbConnectionVar(key string) bool {
	dbEnvVars := []string{
		"DATABASE_URL", "DB_URL", "REDIS_URL",
		"MONGODB_URL", "MYSQL_URL", "POSTGRES_URL",
		"MARIADB_URL", "MONGO_URL", "MEMCACHED_URL",
	}

	key = strings.ToUpper(key)
	for _, dbVar := range dbEnvVars {
		if key == dbVar || strings.HasSuffix(key, "_"+dbVar) {
			return true
		}
	}

	return strings.Contains(key, "DATABASE") ||
		strings.Contains(key, "DB_URL") ||
		strings.Contains(key, "_URL")
}

func isDockerOptionType(optionType string) bool {
	switch optionType {
	case "build", "deploy", "run":
		return true
	default:
		return false
	}
}

func isValidRestartPolicy(policy string) bool {
	switch policy {
	case "no", "always", "unless-stopped", "on-failure":
		return true
	}

	if strings.HasPrefix(policy, "on-failure:") {
		num := strings.TrimPrefix(policy, "on-failure:")
		if num == "" {
			return false
		}
		if _, err := strconv.Atoi(num); err == nil {
			return true
		}
	}

	return false
}

func selectDockerOption(options dokku.OptionsReport, optionType string, index int) (string, error) {
	var list []string
	switch optionType {
	case "build":
		list = options.Build
	case "deploy":
		list = options.Deploy
	case "run":
		list = options.Run
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidOptionType, optionType)
	}

	if index < 0 || index >= len(list) {
		return "", fmt.Errorf("%w (%d)", ErrIndexOutOfRange, index)
	}

	return list[index], nil
}

// GetDockerOptions returns docker options for an app.
func (s *Service) GetDockerOptions(appName string) (dokku.OptionsReport, error) {
	if appName == "" {
		return dokku.OptionsReport{}, fmt.Errorf("app name cannot be empty")
	}

	options, err := dokku.GetDockerOptions(s.runner, appName)
	if err != nil {
		return dokku.OptionsReport{}, fmt.Errorf("unable to fetch docker options: %w", err)
	}

	return options, nil
}

// AddDockerOption adds a docker option for the given phase.
func (s *Service) AddDockerOption(appName, optionType, option string) error {
	if appName == "" || optionType == "" || strings.TrimSpace(option) == "" {
		return fmt.Errorf("app name, option type, and option cannot be empty")
	}
	if !isDockerOptionType(optionType) {
		return fmt.Errorf("%w: %s", ErrInvalidOptionType, optionType)
	}

	if err := dokku.AddDockerOption(s.runner, appName, optionType, option); err != nil {
		return fmt.Errorf("unable to add docker option: %w", err)
	}

	return nil
}

// UpdateDockerOption updates a docker option at the provided index.
func (s *Service) UpdateDockerOption(appName, optionType string, index int, newOption string) error {
	if appName == "" || optionType == "" || strings.TrimSpace(newOption) == "" {
		return fmt.Errorf("app name, option type, and option cannot be empty")
	}
	if index < 0 {
		return fmt.Errorf("index cannot be negative")
	}
	if !isDockerOptionType(optionType) {
		return fmt.Errorf("%w: %s", ErrInvalidOptionType, optionType)
	}

	options, err := dokku.GetDockerOptions(s.runner, appName)
	if err != nil {
		return fmt.Errorf("unable to fetch docker options: %w", err)
	}

	current, err := selectDockerOption(options, optionType, index)
	if err != nil {
		return err
	}

	if err := dokku.UpdateDockerOption(s.runner, appName, optionType, current, newOption); err != nil {
		return fmt.Errorf("unable to update docker option: %w", err)
	}

	return nil
}

// DeleteDockerOption removes a docker option at the provided index.
func (s *Service) DeleteDockerOption(appName, optionType string, index int) error {
	if appName == "" || optionType == "" {
		return fmt.Errorf("app name and option type cannot be empty")
	}
	if index < 0 {
		return fmt.Errorf("index cannot be negative")
	}
	if !isDockerOptionType(optionType) {
		return fmt.Errorf("%w: %s", ErrInvalidOptionType, optionType)
	}

	options, err := dokku.GetDockerOptions(s.runner, appName)
	if err != nil {
		return fmt.Errorf("unable to fetch docker options: %w", err)
	}

	current, err := selectDockerOption(options, optionType, index)
	if err != nil {
		return err
	}

	if _, err := s.runDokku("docker-options:remove", appName, optionType, current); err != nil {
		return fmt.Errorf("unable to remove docker option: %w", err)
	}

	return nil
}

// SetProcfilePath configures the Procfile path for an app. An empty path resets to default.
func (s *Service) SetProcfilePath(appName, path string) (string, error) {
	if appName == "" {
		return "", fmt.Errorf("app name cannot be empty")
	}

	args := []string{"ps:set", appName, "procfile-path"}
	if strings.TrimSpace(path) != "" {
		args = append(args, path)
	}

	output, err := s.runDokku(args...)
	if err != nil {
		return "", fmt.Errorf("unable to set procfile path: %w", err)
	}

	if strings.Contains(strings.ToLower(output), "error") {
		return output, errors.New(output)
	}

	return output, nil
}

// GetProcfilePath returns the configured Procfile path for an app.
func (s *Service) GetProcfilePath(appName string) (string, error) {
	if appName == "" {
		return "", fmt.Errorf("app name cannot be empty")
	}

	output, err := s.runDokku("ps:report", appName)
	if err != nil {
		return "", fmt.Errorf("unable to fetch procfile path: %w", err)
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if strings.Contains(lower, "procfile path") || strings.Contains(lower, "procfile-path") || strings.Contains(lower, "ps-procfile-path") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				if value != "" {
					return value, nil
				}
			}
		}
	}

	return "Procfile", nil
}

// SetAppJSONPath configures the app.json path for an app. An empty path resets to default.
func (s *Service) SetAppJSONPath(appName, path string) (string, error) {
	if appName == "" {
		return "", fmt.Errorf("app name cannot be empty")
	}

	args := []string{"app-json:set", appName, "appjson-path"}
	if strings.TrimSpace(path) != "" {
		args = append(args, path)
	}

	output, err := s.runDokku(args...)
	if err != nil {
		return "", fmt.Errorf("unable to set app.json path: %w", err)
	}

	if strings.Contains(strings.ToLower(output), "error") {
		return output, errors.New(output)
	}

	return output, nil
}

// GetAppJSONPath returns the configured app.json path for an app.
func (s *Service) GetAppJSONPath(appName string) (string, error) {
	if appName == "" {
		return "", fmt.Errorf("app name cannot be empty")
	}

	output, err := s.runDokku("app-json:report", appName, "--app-json-selected")
	if err != nil {
		return "", fmt.Errorf("unable to fetch app.json path: %w", err)
	}

	value := strings.TrimSpace(output)
	if parts := strings.SplitN(value, ":", 2); len(parts) == 2 {
		value = strings.TrimSpace(parts[1])
	}

	if value == "" {
		value = "app.json"
	}

	return value, nil
}

// GetRestartPolicy retrieves the restart policy configured for an app.
func (s *Service) GetRestartPolicy(appName string) (string, error) {
	if appName == "" {
		return "", fmt.Errorf("app name cannot be empty")
	}

	output, err := s.runDokku("ps:report", appName, "--ps-restart-policy")
	if err != nil {
		return "", fmt.Errorf("unable to retrieve restart policy: %w", err)
	}

	output = strings.TrimSpace(output)
	if strings.Contains(output, "Restart policy:") {
		parts := strings.Split(output, "Restart policy:")
		if len(parts) > 1 {
			return strings.TrimSpace(parts[1]), nil
		}
	}

	return output, nil
}

// SetRestartPolicy updates the restart policy for an app after validating the input.
func (s *Service) SetRestartPolicy(appName, policy string) error {
	if appName == "" {
		return fmt.Errorf("app name cannot be empty")
	}
	policy = strings.TrimSpace(policy)
	if policy == "" {
		return fmt.Errorf("%w: value cannot be empty", ErrInvalidRestartPolicy)
	}

	if !isValidRestartPolicy(policy) {
		return fmt.Errorf("%w: %s", ErrInvalidRestartPolicy, policy)
	}

	if _, err := s.runDokku("ps:set", appName, "restart-policy", policy); err != nil {
		return fmt.Errorf("unable to set restart policy: %w", err)
	}

	return nil
}

// MountStorage configures a storage mount for an app.
func (s *Service) MountStorage(appName string, req models.StorageMountRequest) error {
	if appName == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	if req.Source == "" || req.Destination == "" {
		return fmt.Errorf("source and destination cannot be empty")
	}

	fullSourcePath := fmt.Sprintf("/var/lib/dokku/data/storage/%s/%s", appName, req.Source)
	baseDir := fmt.Sprintf("/var/lib/dokku/data/storage/%s", appName)

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}

	if err := os.MkdirAll(fullSourcePath, 0755); err != nil {
		return fmt.Errorf("failed to create source directory: %w", err)
	}

	if _, err := s.runner.RunCommand("chown", "-R", "dokku:dokku", fullSourcePath); err != nil {
		return fmt.Errorf("failed to set directory ownership: %w", err)
	}

	chmod := req.Chmod
	if chmod == "" {
		chmod = "755"
	}
	if _, err := s.runner.RunCommand("chmod", "-R", chmod, fullSourcePath); err != nil {
		return fmt.Errorf("failed to set directory permissions: %w", err)
	}

	mountArg := fmt.Sprintf("%s:%s", fullSourcePath, req.Destination)
	output, err := s.runDokku("storage:mount", appName, mountArg)
	if err != nil {
		return fmt.Errorf("unable to mount storage: %w", err)
	}

	if strings.Contains(strings.ToLower(output), "error") {
		return errors.New(output)
	}

	return nil
}

// UnmountStorage removes a storage mount from an app.
func (s *Service) UnmountStorage(appName string, req models.StorageMountRequest) error {
	if appName == "" {
		return fmt.Errorf("app name cannot be empty")
	}
	if req.Source == "" || req.Destination == "" {
		return fmt.Errorf("source and destination cannot be empty")
	}

	mountArg := fmt.Sprintf("%s:%s", req.Source, req.Destination)
	output, err := s.runDokku("storage:unmount", appName, mountArg)
	if err != nil {
		return fmt.Errorf("unable to unmount storage: %w", err)
	}

	if strings.Contains(strings.ToLower(output), "error") {
		return errors.New(output)
	}

	return nil
}

// AppExists checks if an app exists in Dokku.
func (s *Service) AppExists(appName string) (bool, error) {
	if appName == "" {
		return false, fmt.Errorf("app name cannot be empty")
	}

	apps, err := dokku.GetDokkuApps(s.runner)
	if err != nil {
		return false, fmt.Errorf("unable to list apps: %w", err)
	}

	for _, app := range apps {
		if app == appName {
			return true, nil
		}
	}

	return false, nil
}

// ListApps returns all dokku apps.
func (s *Service) ListApps() ([]string, error) {
	apps, err := dokku.GetDokkuApps(s.runner)
	if err != nil {
		return nil, fmt.Errorf("unable to list apps: %w", err)
	}
	return apps, nil
}
