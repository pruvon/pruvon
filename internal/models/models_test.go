package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBaseData(t *testing.T) {
	t.Run("Create BaseData struct", func(t *testing.T) {
		base := BaseData{
			HideNavigation: true,
			User:           "testuser",
			Username:       "testusername",
			AuthType:       "jwt",
			FlashMessage:   "Success",
			FlashType:      "success",
		}

		assert.True(t, base.HideNavigation)
		assert.Equal(t, "testuser", base.User)
		assert.Equal(t, "testusername", base.Username)
		assert.Equal(t, "jwt", base.AuthType)
		assert.Equal(t, "Success", base.FlashMessage)
		assert.Equal(t, "success", base.FlashType)
	})

	t.Run("BaseData JSON marshaling", func(t *testing.T) {
		base := BaseData{
			HideNavigation: false,
			User:           "admin",
		}

		jsonData, err := json.Marshal(base)
		assert.NoError(t, err)
		assert.Contains(t, string(jsonData), "hide_navigation")
		assert.Contains(t, string(jsonData), "admin")
	})
}

func TestAppSummary(t *testing.T) {
	t.Run("Create AppSummary", func(t *testing.T) {
		app := AppSummary{
			Name:         "myapp",
			Description:  "My test app",
			Version:      "1.0.0",
			Repository:   "https://github.com/user/myapp",
			Running:      true,
			Deployed:     true,
			CreatedAt:    "2024-01-01",
			LastDeployAt: "2024-01-15",
		}

		assert.Equal(t, "myapp", app.Name)
		assert.Equal(t, "My test app", app.Description)
		assert.Equal(t, "1.0.0", app.Version)
		assert.Equal(t, "https://github.com/user/myapp", app.Repository)
		assert.Equal(t, true, app.Running)
		assert.True(t, app.Deployed)
		assert.Equal(t, "2024-01-01", app.CreatedAt)
		assert.Equal(t, "2024-01-15", app.LastDeployAt)
	})

	t.Run("AppSummary JSON marshaling", func(t *testing.T) {
		app := AppSummary{
			Name:     "testapp",
			Deployed: true,
		}

		jsonData, err := json.Marshal(app)
		assert.NoError(t, err)

		var unmarshaled AppSummary
		err = json.Unmarshal(jsonData, &unmarshaled)
		assert.NoError(t, err)
		assert.Equal(t, "testapp", unmarshaled.Name)
		assert.True(t, unmarshaled.Deployed)
	})
}

func TestEnvVar(t *testing.T) {
	t.Run("Create EnvVar", func(t *testing.T) {
		env := EnvVar{
			Key:   "DATABASE_URL",
			Value: "postgres://localhost/mydb",
		}

		assert.Equal(t, "DATABASE_URL", env.Key)
		assert.Equal(t, "postgres://localhost/mydb", env.Value)
	})
}

func TestEnvVarRequest(t *testing.T) {
	t.Run("Create EnvVarRequest with restart", func(t *testing.T) {
		req := EnvVarRequest{
			Key:     "PORT",
			Value:   "8080",
			Restart: true,
		}

		assert.Equal(t, "PORT", req.Key)
		assert.Equal(t, "8080", req.Value)
		assert.True(t, req.Restart)
	})
}

func TestSystemMetrics(t *testing.T) {
	t.Run("Create SystemMetrics", func(t *testing.T) {
		metrics := SystemMetrics{
			CPUUsage:       45.5,
			CPUInfo:        "4 cores",
			LoadAvg:        "0.5 1.2 1.5",
			RAMUsage:       70.2,
			RAMInfo:        "8GB total",
			SwapUsage:      10.0,
			SwapInfo:       "2GB total",
			DiskUsage:      65.8,
			DiskInfo:       "100GB total",
			ContainerCount: 5,
			ServiceCount:   3,
			AppCount:       10,
			NetBytesRecv:   1024000,
			NetBytesSent:   512000,
		}

		assert.Equal(t, 45.5, metrics.CPUUsage)
		assert.Equal(t, "4 cores", metrics.CPUInfo)
		assert.Equal(t, "0.5 1.2 1.5", metrics.LoadAvg)
		assert.Equal(t, 70.2, metrics.RAMUsage)
		assert.Equal(t, "8GB total", metrics.RAMInfo)
		assert.Equal(t, 10.0, metrics.SwapUsage)
		assert.Equal(t, "2GB total", metrics.SwapInfo)
		assert.Equal(t, 65.8, metrics.DiskUsage)
		assert.Equal(t, "100GB total", metrics.DiskInfo)
		assert.Equal(t, 5, metrics.ContainerCount)
		assert.Equal(t, 3, metrics.ServiceCount)
		assert.Equal(t, 10, metrics.AppCount)
		assert.Equal(t, uint64(1024000), metrics.NetBytesRecv)
		assert.Equal(t, uint64(512000), metrics.NetBytesSent)
	})

	t.Run("SystemMetrics JSON marshaling", func(t *testing.T) {
		metrics := SystemMetrics{
			CPUUsage:       50.0,
			RAMUsage:       75.5,
			ContainerCount: 3,
		}

		jsonData, err := json.Marshal(metrics)
		assert.NoError(t, err)
		assert.Contains(t, string(jsonData), "cpu_usage")
		assert.Contains(t, string(jsonData), "ram_usage")
		assert.Contains(t, string(jsonData), "container_count")
	})
}

func TestActivityLog(t *testing.T) {
	t.Run("Create ActivityLog", func(t *testing.T) {
		now := time.Now()
		params := json.RawMessage(`{"app": "myapp"}`)

		log := ActivityLog{
			Time:       now,
			RequestID:  "req-123",
			IP:         "192.168.1.1",
			User:       "admin",
			AuthType:   "jwt",
			Action:     "deploy",
			Method:     "POST",
			Route:      "/api/apps/myapp/deploy",
			Parameters: params,
			StatusCode: 200,
		}

		assert.Equal(t, now, log.Time)
		assert.Equal(t, "req-123", log.RequestID)
		assert.Equal(t, "192.168.1.1", log.IP)
		assert.Equal(t, "admin", log.User)
		assert.Equal(t, "jwt", log.AuthType)
		assert.Equal(t, "deploy", log.Action)
		assert.Equal(t, "POST", log.Method)
		assert.Equal(t, "/api/apps/myapp/deploy", log.Route)
		assert.Equal(t, params, log.Parameters)
		assert.Equal(t, 200, log.StatusCode)
	})

	t.Run("ActivityLog with error", func(t *testing.T) {
		now := time.Now()
		log := ActivityLog{
			Time:       now,
			RequestID:  "req-456",
			StatusCode: 500,
			Error:      "Internal server error",
		}

		assert.Equal(t, now, log.Time)
		assert.Equal(t, "req-456", log.RequestID)
		assert.Equal(t, 500, log.StatusCode)
		assert.Equal(t, "Internal server error", log.Error)
	})
}

func TestSSHKeyRequest(t *testing.T) {
	t.Run("Create SSHKeyRequest", func(t *testing.T) {
		req := SSHKeyRequest{
			Name: "my-laptop",
			Key:  "ssh-rsa AAAAB3NzaC1...",
		}

		assert.Equal(t, "my-laptop", req.Name)
		assert.Contains(t, req.Key, "ssh-rsa")
	})
}

func TestPortMapping(t *testing.T) {
	t.Run("Create PortMapping", func(t *testing.T) {
		port := PortMapping{
			Protocol:  "http",
			Host:      "80",
			Container: "5000",
		}

		assert.Equal(t, "http", port.Protocol)
		assert.Equal(t, "80", port.Host)
		assert.Equal(t, "5000", port.Container)
	})
}

func TestStorageMount(t *testing.T) {
	t.Run("Create StorageMount", func(t *testing.T) {
		mount := StorageMount{
			Source:      "/var/lib/data",
			Destination: "/app/data",
			Size:        "10GB",
		}

		assert.Equal(t, "/var/lib/data", mount.Source)
		assert.Equal(t, "/app/data", mount.Destination)
		assert.Equal(t, "10GB", mount.Size)
	})
}

func TestServiceInfo(t *testing.T) {
	t.Run("Create ServiceInfo", func(t *testing.T) {
		service := ServiceInfo{
			Name:         "postgres",
			Image:        "postgres",
			ImageVersion: "14",
		}

		assert.Equal(t, "postgres", service.Name)
		assert.Equal(t, "postgres", service.Image)
		assert.Equal(t, "14", service.ImageVersion)
	})
}

func TestCreateAppRequest(t *testing.T) {
	t.Run("Create complete app request", func(t *testing.T) {
		req := CreateAppRequest{
			Name: "myapp",
			Services: []ServiceInfo{
				{
					Name:         "postgres",
					Image:        "postgres",
					ImageVersion: "14",
				},
			},
			Env: []EnvVar{
				{Key: "PORT", Value: "5000"},
			},
			Domain: "myapp.example.com",
			SSL:    true,
			Port: PortMapping{
				Protocol:  "http",
				Host:      "80",
				Container: "5000",
			},
			Mounts: []StorageMount{
				{
					Source:      "/data",
					Destination: "/app/data",
				},
			},
		}

		assert.Equal(t, "myapp", req.Name)
		assert.Len(t, req.Services, 1)
		assert.Equal(t, "postgres", req.Services[0].Name)
		assert.Len(t, req.Env, 1)
		assert.Equal(t, "PORT", req.Env[0].Key)
		assert.True(t, req.SSL)
		assert.Equal(t, "myapp.example.com", req.Domain)
		assert.Len(t, req.Mounts, 1)
		assert.Equal(t, "/data", req.Mounts[0].Source)
	})
}

func TestLogSearchParams(t *testing.T) {
	t.Run("Create LogSearchParams", func(t *testing.T) {
		params := LogSearchParams{
			Username: "admin",
			Query:    "deploy",
			Page:     1,
			PerPage:  50,
		}

		assert.Equal(t, "admin", params.Username)
		assert.Equal(t, "deploy", params.Query)
		assert.Equal(t, 1, params.Page)
		assert.Equal(t, 50, params.PerPage)
	})
}

func TestLogSearchResult(t *testing.T) {
	t.Run("Create LogSearchResult", func(t *testing.T) {
		result := LogSearchResult{
			Page:       1,
			PerPage:    10,
			TotalPages: 5,
			TotalLogs:  50,
			Logs: []ActivityLog{
				{
					RequestID: "req-1",
					User:      "admin",
				},
			},
		}

		assert.Equal(t, 1, result.Page)
		assert.Equal(t, 10, result.PerPage)
		assert.Equal(t, 5, result.TotalPages)
		assert.Equal(t, 50, result.TotalLogs)
		assert.Len(t, result.Logs, 1)
	})
}

func TestContainerStats(t *testing.T) {
	t.Run("Create ContainerStats", func(t *testing.T) {
		stats := ContainerStats{
			CPUUsage:      45.5,
			MemoryUsage:   70.2,
			DiskUsage:     60.0,
			DiskUsageText: "6GB/10GB",
			IsDeployed:    true,
		}

		assert.Equal(t, 45.5, stats.CPUUsage)
		assert.Equal(t, 70.2, stats.MemoryUsage)
		assert.Equal(t, 60.0, stats.DiskUsage)
		assert.Equal(t, "6GB/10GB", stats.DiskUsageText)
		assert.True(t, stats.IsDeployed)
	})
}

func TestSSLInfo(t *testing.T) {
	t.Run("Create SSLInfo", func(t *testing.T) {
		ssl := SSLInfo{
			Active:     true,
			Autorenew:  true,
			Email:      "admin@example.com",
			Expiration: 1735689600,
		}

		assert.True(t, ssl.Active)
		assert.True(t, ssl.Autorenew)
		assert.Equal(t, "admin@example.com", ssl.Email)
		assert.Greater(t, ssl.Expiration, int64(0))
	})
}

func TestDockerStats(t *testing.T) {
	t.Run("Create DockerStats", func(t *testing.T) {
		stats := DockerStats{
			Version:           "20.10.7",
			RunningContainers: 5,
			TotalContainers:   10,
			TotalImages:       15,
		}

		assert.Equal(t, "20.10.7", stats.Version)
		assert.Equal(t, 5, stats.RunningContainers)
		assert.Equal(t, 10, stats.TotalContainers)
		assert.Equal(t, 15, stats.TotalImages)
	})
}

func TestAppStatus(t *testing.T) {
	t.Run("AppStatus with boolean running", func(t *testing.T) {
		status := AppStatus{
			Deployed: true,
			Running:  true,
			Processes: map[string]int{
				"web":    2,
				"worker": 1,
			},
		}

		assert.True(t, status.Deployed)
		assert.Equal(t, true, status.Running)
		assert.Equal(t, 2, status.Processes["web"])
		assert.Equal(t, 1, status.Processes["worker"])
	})

	t.Run("AppStatus with string running", func(t *testing.T) {
		status := AppStatus{
			Deployed: true,
			Running:  "mixed",
			Processes: map[string]int{
				"web": 1,
			},
		}

		assert.True(t, status.Deployed)
		assert.Equal(t, "mixed", status.Running)
		assert.Equal(t, 1, status.Processes["web"])
	})
}

func BenchmarkAppSummaryJSON(b *testing.B) {
	app := AppSummary{
		Name:        "testapp",
		Description: "Test application",
		Version:     "1.0.0",
		Deployed:    true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(app)
	}
}

func BenchmarkSystemMetricsJSON(b *testing.B) {
	metrics := SystemMetrics{
		CPUUsage:       50.0,
		RAMUsage:       75.0,
		DiskUsage:      60.0,
		ContainerCount: 5,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(metrics)
	}
}
