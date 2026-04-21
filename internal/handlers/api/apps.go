package api

import (
	"errors"
	"fmt"
	"github.com/pruvon/pruvon/internal/docker"
	"github.com/pruvon/pruvon/internal/services/logs"
	"github.com/pruvon/pruvon/internal/system"
	"os"
	"strconv"
	"strings"

	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/handlers/common"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/models"
	appsvc "github.com/pruvon/pruvon/internal/services/apps"

	"github.com/gofiber/fiber/v2"
)

func SetupAppsRoutes(app *fiber.App) {
	app.Get("/api/apps/:name/process", handleAppProcess)
	app.Get("/api/apps/:name/config", handleAppConfig)
	app.Post("/api/apps/:name/config", handleAppConfigSet)
	app.Delete("/api/apps/:name/config/:key", handleAppConfigUnset)
	app.Get("/api/apps/:name/ports", handleAppPorts)
	app.Get("/api/apps/:name/domains", handleAppDomains)
	app.Get("/api/apps/:name/storage", handleAppStorage)
	app.Get("/api/apps/:name/nginx/custom-config-path", handleAppNginxCustomConfigPath)
	app.Post("/api/apps/:name/nginx/custom-config-path", handleAppNginxSetCustomConfigPath)
	app.Post("/api/apps/:name/nginx/custom-config-path/reset", handleAppNginxResetCustomConfigPath)
	app.Get("/api/apps/:name/nginx", handleAppNginx)
	app.Post("/api/apps/:name/nginx/:config", handleAppNginxSet)
	app.Get("/api/apps/:name/summary", handleAppSummary)
	app.Post("/api/apps/:name/containers/:id/kill", handleContainerKill)
	app.Get("/api/apps/:name/cron", handleAppCron)
	app.Post("/api/apps/:name/start", handleAppStart)
	app.Post("/api/apps/:name/stop", handleAppStop)
	app.Post("/api/apps/:name/operations/:action", handleAppActionOperationStart)
	app.Get("/api/apps/:name/operations", handleAppActionOperationStatus)
	app.Post("/api/apps/:name/restart", handleAppRestart)
	app.Post("/api/apps/:name/restart/operations", handleAppRestartOperationStart)
	app.Get("/api/apps/:name/restart/operations", handleAppRestartOperationStatus)
	app.Post("/api/apps/:name/rebuild", handleAppRebuild)
	app.Get("/api/apps/:name/status", handleAppStatus)
	app.Get("/api/apps/:name/stats", handleAppStats)
	app.Post("/api/apps/:name/domains", handleAppDomainAdd)
	app.Post("/api/apps/:name/ssl", handleAppSSLEnable)
	app.Delete("/api/apps/:name/domains/:domain", handleAppDomainRemove)
	app.Get("/api/apps/:name/redirects", handleAppRedirects)
	app.Post("/api/apps/:name/redirects", handleAppRedirectSet)
	app.Delete("/api/apps/:name/redirects", handleAppRedirectUnset)
	app.Post("/api/apps/:name/ports", handleAppPortAdd)
	app.Delete("/api/apps/:name/ports", handleAppPortRemove)
	app.Post("/api/apps/:name/scale", handleAppScale)
	app.Get("/api/apps/:name/app-json", handleAppJson)
	app.Get("/api/apps/:app/procfile", dokku.GetProcfileHandler)
	app.Get("/api/apps/:name/ssl", handleAppSSLInfo)
	app.Post("/api/apps/:name/storage/mount", handleAppStorageMount)
	app.Post("/api/apps/:name/storage/unmount", handleAppStorageUnmount)
	app.Get("/api/apps/check/:name", handleAppCheck)
	app.Get("/api/apps/list", handleAppsList)
	app.Get("/api/apps/list/detailed", handleAppsListDetailed)
	app.Get("/api/apps/:name/resource", handleAppResource)
	app.Post("/api/apps/:name/resource", handleAppResourceSet)
	app.Delete("/api/apps/:name", handleAppDelete)
	app.Get("/api/apps/:name/details", handleAppDetails)
	app.Get("/api/apps/:name/services", handleAppServices)
	app.Get("/api/apps/:name/docker-options", handleAppDockerOptions)
	app.Post("/api/apps/:name/docker-options/:type", handleAppDockerOptionsAdd)
	app.Put("/api/apps/:name/docker-options/:type/:index", handleAppDockerOptionsUpdate)
	app.Delete("/api/apps/:name/docker-options/:type/:index", handleAppDockerOptionsDelete)
	app.Post("/api/apps/:name/procfile-path", handleAppProcfilePath)
	app.Post("/api/apps/:name/appjson-path", handleAppJsonPath)
	app.Get("/api/apps/:name/appjson-path", handleAppJsonPathInfo)
	app.Get("/api/apps/:name/procfile-path", handleAppProcfilePathInfo)
	app.Get("/api/apps/:name/restart-policy", handleAppRestartPolicy)
	app.Post("/api/apps/:name/restart-policy", handleAppSetRestartPolicy)
}

func handleAppProcess(c *fiber.Ctx) error {
	appName := c.Params("name")
	report, err := appService.GetProcessReport(appName)
	if err != nil {
		return common.ErrorResponse(c, fiber.StatusInternalServerError, fmt.Sprintf("Operation information could not be retrieved: %v", err))
	}

	return c.JSON(report)
}

func handleAppConfig(c *fiber.Ctx) error {
	appName := c.Params("name")
	result, err := appService.GetConfig(appName)
	if err != nil {
		return common.ErrorResponse(c, fiber.StatusInternalServerError, fmt.Sprintf("Configuration information could not be retrieved: %v", err))
	}

	return c.JSON(result)
}

func handleAppConfigSet(c *fiber.Ctx) error {
	appName := c.Params("name")
	var req struct {
		Key     string `json:"key"`
		Value   string `json:"value"`
		Restart bool   `json:"restart"`
	}
	if err := c.BodyParser(&req); err != nil {
		return err
	}

	if err := appService.SetConfig(appName, req.Key, req.Value, req.Restart); err != nil {
		return common.ErrorResponse(c, fiber.StatusInternalServerError, fmt.Sprintf("Configuration could not be set: %v", err))
	}

	return c.SendStatus(fiber.StatusOK)
}

func handleAppConfigUnset(c *fiber.Ctx) error {
	appName := c.Params("name")
	key := c.Params("key")
	var req struct {
		Restart bool `json:"restart"`
	}
	if err := c.BodyParser(&req); err != nil {
		return err
	}

	if err := appService.UnsetConfig(appName, key, req.Restart); err != nil {
		return common.ErrorResponse(c, fiber.StatusInternalServerError, fmt.Sprintf("Configuration could not be removed: %v", err))
	}
	return c.SendStatus(fiber.StatusOK)
}

func handleAppPorts(c *fiber.Ctx) error {
	appName := c.Params("name")
	result, err := appService.GetPorts(appName)
	if err != nil {
		return common.ErrorResponse(c, fiber.StatusInternalServerError, fmt.Sprintf("Port information could not be retrieved: %v", err))
	}

	return c.JSON(result)
}

func handleAppDomains(c *fiber.Ctx) error {
	appName := c.Params("name")
	result, err := appService.GetDomains(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Domain information could not be retrieved: %v", err),
		})
	}

	return c.JSON(result)
}

func handleAppStorage(c *fiber.Ctx) error {
	appName := c.Params("name")
	info, err := appService.GetStorageInfo(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Storage information could not be retrieved: %v", err),
		})
	}
	return c.JSON(fiber.Map{
		"info": info,
	})
}

func handleAppNginx(c *fiber.Ctx) error {
	appName := c.Params("name")
	result, err := appService.GetNginxConfig(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Nginx configuration information could not be retrieved: %v", err),
		})
	}

	return c.JSON(result)
}

func handleAppNginxSet(c *fiber.Ctx) error {
	appName := c.Params("name")
	config := c.Params("config")
	var req struct {
		Value string `json:"value"`
	}
	if err := c.BodyParser(&req); err != nil {
		return err
	}

	if err := appService.SetNginxConfig(appName, config, req.Value); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Nginx configuration could not be set: %v", err),
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

func handleAppNginxCustomConfigPath(c *fiber.Ctx) error {
	appName := c.Params("name")
	path, err := appService.GetNginxCustomConfigPath(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Custom Nginx config path could not be retrieved: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"path": path,
	})
}

func handleAppNginxSetCustomConfigPath(c *fiber.Ctx) error {
	appName := c.Params("name")
	var req struct {
		Path string `json:"path"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request format",
		})
	}

	// Check if path starts with a slash
	if strings.HasPrefix(req.Path, "/") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Path cannot start with a slash (/)",
		})
	}

	// Set the custom Nginx config path
	if err := appService.SetNginxCustomConfigPath(appName, req.Path); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Custom Nginx configuration path set successfully",
	})
}

func handleAppNginxResetCustomConfigPath(c *fiber.Ctx) error {
	appName := c.Params("name")

	if err := appService.ResetNginxCustomConfigPath(appName); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Custom Nginx configuration path reset successfully",
	})
}

func handleAppSummary(c *fiber.Ctx) error {
	appName := c.Params("name")
	containers, err := appService.GetAppSummary(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Container information could not be retrieved: %v", err),
		})
	}
	return c.JSON(fiber.Map{
		"containers": containers,
	})
}

func handleContainerKill(c *fiber.Ctx) error {
	containerID := c.Params("id")
	if err := appService.KillContainer(containerID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Konteyner durdurulamadı: %v", err),
		})
	}
	return c.SendStatus(fiber.StatusOK)
}

func handleAppCron(c *fiber.Ctx) error {
	appName := c.Params("name")
	jobs, err := appService.GetCronJobs(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Cron görevleri alınamadı: %v", err),
		})
	}
	return c.JSON(fiber.Map{
		"jobs": jobs,
	})
}

func handleAppStart(c *fiber.Ctx) error {
	appName := c.Params("name")
	processType := c.Query("process_type")

	if err := appService.StartApp(appName, processType); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Uygulama başlatılamadı: %v", err),
		})
	}
	return c.SendStatus(fiber.StatusOK)
}

func handleAppStop(c *fiber.Ctx) error {
	appName := c.Params("name")
	processType := c.Query("process_type")

	if err := appService.StopApp(appName, processType); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Uygulama durdurulamadı: %v", err),
		})
	}
	return c.SendStatus(fiber.StatusOK)
}

func handleAppRestart(c *fiber.Ctx) error {
	appName := c.Params("name")
	processType := c.Query("process_type")

	if err := appService.RestartApp(appName, processType); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Uygulama yeniden başlatılamadı: %v", err),
		})
	}
	return c.SendStatus(fiber.StatusOK)
}

func handleAppRestartOperationStart(c *fiber.Ctx) error {
	return handleTrackedAppActionStart(c, appsvc.ActionRestart)
}

func handleAppActionOperationStart(c *fiber.Ctx) error {
	action := c.Params("action")
	if !isTrackedAppAction(action) {
		return common.ErrorResponse(c, fiber.StatusBadRequest, "Unsupported app action")
	}

	return handleTrackedAppActionStart(c, action)
}

func handleTrackedAppActionStart(c *fiber.Ctx, action string) error {
	appName := c.Params("name")
	processType := c.Query("process_type")

	operation := appsvc.StartAppActionOperation(appService, action, appName, processType)
	statusCode := fiber.StatusAccepted
	if operation.Reused {
		statusCode = fiber.StatusOK
	}

	return c.Status(statusCode).JSON(operation)
}

func handleAppRestartOperationStatus(c *fiber.Ctx) error {
	return handleTrackedAppActionStatus(c)
}

func handleAppActionOperationStatus(c *fiber.Ctx) error {
	return handleTrackedAppActionStatus(c)
}

func handleTrackedAppActionStatus(c *fiber.Ctx) error {
	appName := c.Params("name")
	taskID := c.Query("task_id")

	var (
		operation models.AppRestartOperation
		ok        bool
	)

	if taskID != "" {
		operation, ok = appsvc.GetAppActionOperation(taskID)
	} else {
		operation, ok = appsvc.GetLatestAppActionOperation(appName)
	}

	if !ok || operation.AppName != appName {
		return common.ErrorResponse(c, fiber.StatusNotFound, "No app operation found")
	}

	return c.JSON(operation)
}

func isTrackedAppAction(action string) bool {
	return action == appsvc.ActionRestart || action == appsvc.ActionStop || action == appsvc.ActionRebuild
}

func handleAppRebuild(c *fiber.Ctx) error {
	appName := c.Params("name")
	processType := c.Query("process_type")

	if err := appService.RebuildApp(appName, processType); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Uygulama yeniden oluşturulamadı: %v", err),
		})
	}
	return c.SendStatus(fiber.StatusOK)
}

func handleAppStatus(c *fiber.Ctx) error {
	appName := c.Params("name")
	status, err := appService.GetStatus(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Uygulama durumu alınamadı: %v", err),
		})
	}

	return c.JSON(status)
}

func handleAppStats(c *fiber.Ctx) error {
	appName := c.Params("name")
	stats := appService.GetContainerStats(appName)
	return c.JSON(stats)
}

func handleAppDomainAdd(c *fiber.Ctx) error {
	appName := c.Params("name")
	var req models.DomainRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}

	output, err := appService.AddDomain(appName, req.Domain)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Domain eklenemedi: %v", err),
		})
	}

	if req.EnableSSL {
		sslResult, sslErr := appService.EnableSSL(appName)
		if sslErr != nil {
			return c.JSON(fiber.Map{
				"output": output,
				"ssl": fiber.Map{
					"success": false,
					"error":   sslErr.Error(),
				},
			})
		}
		if !sslResult.Success {
			return c.JSON(fiber.Map{
				"output": output,
				"ssl":    sslResult,
			})
		}
		return c.JSON(fiber.Map{"output": output, "ssl": sslResult})
	}

	return c.JSON(fiber.Map{"output": output})
}

func handleAppSSLEnable(c *fiber.Ctx) error {
	appName := c.Params("name")
	result, err := appService.EnableSSL(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
			"message": "Could not verify if letsencrypt plugin is installed",
		})
	}

	if result.Success {
		return c.JSON(result)
	}

	status := fiber.StatusInternalServerError
	if result.Error == "Letsencrypt plugin is not installed" || result.Error == "No domains configured" {
		status = fiber.StatusBadRequest
	}

	return c.Status(status).JSON(result)
}

func handleAppDomainRemove(c *fiber.Ctx) error {
	appName := c.Params("name")
	domain := c.Params("domain")
	output, err := appService.RemoveDomain(appName, domain)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Domain kaldırılamadı: %v", err),
		})
	}
	return c.JSON(fiber.Map{"output": output})
}

func handleAppRedirects(c *fiber.Ctx) error {
	appName := c.Params("name")
	redirects, installed, err := appService.GetRedirects(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Yönlendirmeler alınamadı: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"redirects":  redirects,
		"has_plugin": installed,
	})
}

func handleAppRedirectSet(c *fiber.Ctx) error {
	appName := c.Params("name")
	var redirect models.Redirect
	if err := c.BodyParser(&redirect); err != nil {
		return err
	}

	if redirect.Code == "" {
		redirect.Code = "301" // Default to 301 if not specified
	}

	output, err := appService.SetRedirect(appName, redirect)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Yönlendirme ayarlanamadı: %v", err),
		})
	}

	success := !strings.Contains(strings.ToLower(output), "error")

	return c.JSON(fiber.Map{
		"success": success,
		"output":  output,
	})
}

func handleAppRedirectUnset(c *fiber.Ctx) error {
	appName := c.Params("name")
	var redirect models.Redirect
	if err := c.BodyParser(&redirect); err != nil {
		return err
	}

	output, err := appService.UnsetRedirect(appName, redirect)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Yönlendirme kaldırılamadı: %v", err),
		})
	}

	return c.JSON(fiber.Map{"output": output})
}

func handleAppPortAdd(c *fiber.Ctx) error {
	appName := c.Params("name")
	var port models.PortMapping
	if err := c.BodyParser(&port); err != nil {
		return err
	}

	if err := appService.AddPortMapping(appName, port); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Port eklenemedi: %v", err),
		})
	}
	return c.SendStatus(fiber.StatusOK)
}

func handleAppPortRemove(c *fiber.Ctx) error {
	appName := c.Params("name")
	var port models.PortMapping
	if err := c.BodyParser(&port); err != nil {
		return err
	}

	if err := appService.RemovePortMapping(appName, port); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Port kaldırılamadı: %v", err),
		})
	}
	return c.SendStatus(fiber.StatusOK)
}

func handleAppScale(c *fiber.Ctx) error {
	appName := c.Params("name")
	var req models.ScaleRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}

	output, err := appService.ScaleApp(appName, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Ölçeklendirme yapılamadı: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"success": !strings.Contains(output, "error"),
		"output":  output,
	})
}

func handleAppJson(c *fiber.Ctx) error {
	appName := c.Params("name")
	content, err := dokku.GetAppJson(commandRunner, appName)
	if err != nil {
		return c.JSON(fiber.Map{
			"content": "",
			"error":   err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"content": content,
	})
}

func handleAppSSLInfo(c *fiber.Ctx) error {
	appName := c.Params("name")
	sslInfo, err := appService.GetSSLInfo(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("SSL bilgisi alınamadı: %v", err),
		})
	}
	return c.JSON(sslInfo)
}

func handleAppStorageMount(c *fiber.Ctx) error {
	appName := c.Params("name")
	var req models.StorageMountRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request data",
		})
	}

	if err := appService.MountStorage(appName, req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

func handleAppStorageUnmount(c *fiber.Ctx) error {
	appName := c.Params("name")
	var req models.StorageMountRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request data",
		})
	}

	if err := appService.UnmountStorage(appName, req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

func handleAppCheck(c *fiber.Ctx) error {
	appName := c.Params("name")
	exists, err := appService.AppExists(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Uygulama listesi alınamadı: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"exists": exists,
	})
}

func handleAppsList(c *fiber.Ctx) error {
	allApps, err := appService.ListApps()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Uygulama listesi alınamadı: %v", err),
		})
	}

	// Get session info
	sess, _ := middleware.GetStore().Get(c)
	authType := sess.Get("auth_type")
	username := sess.Get("username")
	if username == nil {
		username = sess.Get("user")
	}

	// Admin can see all apps
	var allowedApps []string
	if authType == "admin" {
		allowedApps = allApps
	} else if authType == "github" {
		// For GitHub users, filter apps based on their permissions
		cfg := config.GetConfig()
		if cfg == nil {
			return c.Status(500).JSON(fiber.Map{
				"error": "Configuration not available",
			})
		}

		// Find user's permissions
		for _, user := range cfg.GitHub.Users {
			if user.Username == username.(string) {
				// First check direct app permissions (new feature)
				if len(user.Apps) > 0 {
					// Check if user has wildcard access for apps
					for _, app := range user.Apps {
						if app == "*" {
							allowedApps = allApps
							break
						}
					}

					// If no wildcard, add individual apps
					if len(allowedApps) == 0 {
						allowedAppsMap := make(map[string]bool)
						for _, app := range user.Apps {
							if system.Contains(allApps, app) {
								allowedAppsMap[app] = true
							}
						}

						// Convert map to slice
						for app := range allowedAppsMap {
							allowedApps = append(allowedApps, app)
						}
					}
				}

				// If direct app permissions didn't give access, check route-based permissions
				if len(allowedApps) == 0 {
					// If user has wildcard access, show all apps
					for _, route := range user.Routes {
						if route == "*" || route == "/*" || route == "/apps/*" {
							allowedApps = allApps
							break
						}

						// Check app-specific permissions
						if strings.HasPrefix(route, "/apps/") {
							appName := strings.TrimPrefix(route, "/apps/")
							// Handle wildcard for specific app prefix
							if strings.HasSuffix(appName, "*") {
								prefix := strings.TrimSuffix(appName, "*")
								for _, app := range allApps {
									if strings.HasPrefix(app, prefix) {
										allowedApps = append(allowedApps, app)
									}
								}
							} else {
								// Exact app match
								if system.Contains(allApps, appName) {
									allowedApps = append(allowedApps, appName)
								}
							}
						}
					}
				}

				// Deduplicate the app list
				if len(allowedApps) > 0 {
					uniqueApps := make(map[string]bool)
					for _, app := range allowedApps {
						uniqueApps[app] = true
					}

					allowedApps = make([]string, 0, len(uniqueApps))
					for app := range uniqueApps {
						allowedApps = append(allowedApps, app)
					}
				}

				break
			}
		}
	}

	return c.JSON(fiber.Map{
		"apps": allowedApps,
	})
}

func handleAppsListDetailed(c *fiber.Ctx) error {
	allApps, err := dokku.GetDokkuApps(commandRunner)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Uygulama listesi alınamadı: %v", err),
		})
	}

	// Get session info
	sess, _ := middleware.GetStore().Get(c)
	authType := sess.Get("auth_type")
	username := sess.Get("username")
	if username == nil {
		username = sess.Get("user")
	}

	// Get allowed apps first
	var allowedApps []string
	if authType == "admin" {
		allowedApps = allApps
	} else if authType == "github" {
		cfg := config.GetConfig()
		if cfg == nil {
			return c.Status(500).JSON(fiber.Map{
				"error": "Configuration not available",
			})
		}

		for _, user := range cfg.GitHub.Users {
			if user.Username == username.(string) {
				for _, route := range user.Routes {
					if route == "*" || route == "/*" || route == "/apps/*" {
						allowedApps = allApps
						break
					}

					if strings.HasPrefix(route, "/apps/") {
						appName := strings.TrimPrefix(route, "/apps/")
						if strings.HasSuffix(appName, "*") {
							prefix := strings.TrimSuffix(appName, "*")
							for _, app := range allApps {
								if strings.HasPrefix(app, prefix) {
									allowedApps = append(allowedApps, app)
								}
							}
						} else {
							if system.Contains(allApps, appName) {
								allowedApps = append(allowedApps, appName)
							}
						}
					}
				}
				break
			}
		}
	}

	// Now process only the allowed apps
	var detailedApps []map[string]interface{}
	for _, appName := range allowedApps {
		if appName == "" {
			continue
		}

		output, err := commandRunner.RunCommand("dokku", "ps:report", appName)
		if err != nil {
			// Hata durumunda bu uygulamayı atla ve devam et
			continue
		}

		// Default status values
		running := false
		deployed := false
		processes := make(map[string]int)

		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "Deployed:") {
				deployed = strings.TrimSpace(strings.Split(line, ":")[1]) == "true"
			} else if strings.Contains(line, "Running:") {
				runningStatus := strings.TrimSpace(strings.Split(line, ":")[1])
				switch runningStatus {
				case "true":
					running = true
				case "mixed":
					running = true
				}
			} else if strings.HasPrefix(line, "Status ") {
				parts := strings.Fields(line)
				if len(parts) >= 4 && strings.HasSuffix(parts[0], "Status") {
					procType := parts[1]
					if strings.Contains(line, "running") {
						processes[procType]++
					}
				}
			}
		}

		appDetails := map[string]interface{}{
			"name":      appName,
			"running":   running,
			"deployed":  deployed,
			"processes": processes,
		}

		detailedApps = append(detailedApps, appDetails)
	}

	return c.JSON(fiber.Map{
		"apps": detailedApps,
	})
}

func handleAppResource(c *fiber.Ctx) error {
	appName := c.Params("name")
	output, err := commandRunner.RunCommand("dokku", "resource:limit", appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Kaynak limitleri alınamadı: %v", err),
		})
	}

	limits := models.ResourceLimits{}

	// Her bir satırı parse et
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "=====>") || strings.HasPrefix(line, "       ") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "cpu":
			limits.CPU = value
		case "memory":
			limits.Memory = value
		}
	}

	return c.JSON(limits)
}

func handleAppResourceSet(c *fiber.Ctx) error {
	appName := c.Params("name")
	var req struct {
		CPU    string `json:"cpu"`
		Memory string `json:"memory"`
	}

	if err := c.BodyParser(&req); err != nil {
		_ = middleware.SetFlashMessage(c, "Geçersiz istek verisi", "error")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":      "Invalid request data",
			"message":    "Geçersiz istek verisi",
			"flash_type": "error",
		})
	}

	// Get containers for the app
	containers, err := dokku.GetAppContainers(commandRunner, appName)
	if err != nil || len(containers) == 0 {
		errorMsg := fmt.Sprintf("%s uygulaması için çalışan konteyner bulunamadı", appName)
		_ = middleware.SetFlashMessage(c, errorMsg, "error")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":      fmt.Sprintf("No running containers found for app %s: %v", appName, err),
			"message":    errorMsg,
			"flash_type": "error",
		})
	}

	// Count how many containers were updated
	updatedCount := 0

	// Loop through containers and update resource limits for each
	for _, container := range containers {
		// Only update running containers
		if strings.Contains(strings.ToLower(container.Status), "running") {
			err := docker.UpdateContainerResourceLimits(commandRunner, container.ID, req.CPU, req.Memory)
			if err != nil {
				errorMsg := fmt.Sprintf("%s konteynerinin kaynak limitleri güncellenirken hata oluştu", container.ID)
				_ = middleware.SetFlashMessage(c, errorMsg, "error")
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error":      fmt.Sprintf("Failed to update resource limits for container %s: %v", container.ID, err),
					"message":    errorMsg,
					"flash_type": "error",
				})
			}
			updatedCount++
		}
	}

	// Also update Dokku resource limits for future deployments
	args := []string{"resource:limit", appName}
	if req.CPU != "" {
		args = append(args, "--cpu", req.CPU)
	}
	if req.Memory != "" {
		args = append(args, "--memory", req.Memory)
	}

	output, err := commandRunner.RunCommand("dokku", args...)
	if err != nil {
		errorMsg := fmt.Sprintf("Dokku kaynak limitleri ayarlanamadı: %v", err)
		_ = middleware.SetFlashMessage(c, errorMsg, "error")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":      fmt.Sprintf("Dokku kaynak limitleri ayarlanamadı: %v", err),
			"message":    errorMsg,
			"flash_type": "error",
		})
	}

	if strings.Contains(strings.ToLower(output), "error") {
		_ = middleware.SetFlashMessage(c, output, "error")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":      output,
			"message":    output,
			"flash_type": "error",
		})
	}

	// Set success message
	successMsg := fmt.Sprintf("%s uygulaması için kaynak limitleri güncellendi", appName)
	_ = middleware.SetFlashMessage(c, successMsg, "success")

	return c.JSON(fiber.Map{
		"success":            true,
		"message":            successMsg,
		"flash_type":         "success",
		"updated_containers": updatedCount,
	})
}

func handleAppDelete(c *fiber.Ctx) error {
	appName := c.Params("name")
	var req struct {
		DeleteData bool `json:"deleteData"`
		DeleteDb   bool `json:"deleteDb"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request data",
		})
	}

	// Log the app deletion
	_ = logs.LogWithParams(c, "delete_app", fiber.Map{
		"app":         appName,
		"delete_data": req.DeleteData,
		"delete_db":   req.DeleteDb,
	})

	// Get database info before deleting the app
	if req.DeleteDb {
		config, err := commandRunner.RunCommand("dokku", "config", appName)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Uygulama yapılandırması alınamadı: %v", err),
			})
		}

		// Check for DATABASE_URL
		if strings.Contains(config, "DATABASE_URL:") {
			for _, line := range strings.Split(config, "\n") {
				if strings.HasPrefix(line, "DATABASE_URL:") {
					parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
					if len(parts) == 2 {
						dbUrl := strings.TrimSpace(parts[1])
						dbName := ""
						dbType := ""

						// Parse database name from URL host part
						if strings.Contains(dbUrl, "@dokku-postgres-") {
							parts := strings.Split(dbUrl, "@dokku-postgres-")
							if len(parts) > 1 {
								dbName = strings.Split(parts[1], ":")[0]
								dbType = "postgres"
							}
						} else if strings.Contains(dbUrl, "@dokku-mariadb-") {
							parts := strings.Split(dbUrl, "@dokku-mariadb-")
							if len(parts) > 1 {
								dbName = strings.Split(parts[1], ":")[0]
								dbType = "mariadb"
							}
						} else if strings.Contains(dbUrl, "@dokku-mongo-") {
							parts := strings.Split(dbUrl, "@dokku-mongo-")
							if len(parts) > 1 {
								dbName = strings.Split(parts[1], ":")[0]
								dbType = "mongo"
							}
						}

						if dbName != "" && dbType != "" {
							// First unlink the database
							_, _ = commandRunner.RunCommand("dokku", fmt.Sprintf("%s:unlink", dbType), dbName, appName)
							// Then destroy it
							_, _ = commandRunner.RunCommand("dokku", fmt.Sprintf("%s:destroy", dbType), dbName, "-f")
						}
					}
					break
				}
			}
		}

		// Check for REDIS_URL
		if strings.Contains(config, "REDIS_URL:") {
			for _, line := range strings.Split(config, "\n") {
				if strings.HasPrefix(line, "REDIS_URL:") {
					parts := strings.Split(line, "@dokku-redis-")
					if len(parts) > 1 {
						redisName := strings.Split(parts[1], ":")[0]
						// First unlink redis
						_, _ = commandRunner.RunCommand("dokku", "redis:unlink", redisName, appName)
						// Then destroy it
						_, _ = commandRunner.RunCommand("dokku", "redis:destroy", redisName, "-f")
					}
					break
				}
			}
		}

		// Ayrıca <app>-db formatındaki veritabanlarını da kontrol et ve sil
		dbName := fmt.Sprintf("%s-db", appName)

		// Postgres veritabanını kontrol et ve sil
		_, err = commandRunner.RunCommand("dokku", "postgres:info", dbName)
		if err == nil {
			// Veritabanı mevcut, önce unlink yap sonra sil
			_, _ = commandRunner.RunCommand("dokku", "postgres:unlink", dbName, appName)
			_, _ = commandRunner.RunCommand("dokku", "postgres:destroy", dbName, "-f")
		}

		// MariaDB veritabanını kontrol et ve sil
		_, err = commandRunner.RunCommand("dokku", "mariadb:info", dbName)
		if err == nil {
			// Veritabanı mevcut, önce unlink yap sonra sil
			_, _ = commandRunner.RunCommand("dokku", "mariadb:unlink", dbName, appName)
			_, _ = commandRunner.RunCommand("dokku", "mariadb:destroy", dbName, "-f")
		}

		// MongoDB veritabanını kontrol et ve sil
		_, err = commandRunner.RunCommand("dokku", "mongo:info", dbName)
		if err == nil {
			// Veritabanı mevcut, önce unlink yap sonra sil
			_, _ = commandRunner.RunCommand("dokku", "mongo:unlink", dbName, appName)
			_, _ = commandRunner.RunCommand("dokku", "mongo:destroy", dbName, "-f")
		}

		// Redis veritabanını kontrol et ve sil
		_, err = commandRunner.RunCommand("dokku", "redis:info", dbName)
		if err == nil {
			// Redis mevcut, önce unlink yap sonra sil
			_, _ = commandRunner.RunCommand("dokku", "redis:unlink", dbName, appName)
			_, _ = commandRunner.RunCommand("dokku", "redis:destroy", dbName, "-f")
		}
	}

	// Delete application
	output, err := commandRunner.RunCommand("dokku", "apps:destroy", appName, "--force")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Uygulama silinemedi: %v", err),
		})
	}

	if strings.Contains(strings.ToLower(output), "error") {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": output,
		})
	}

	// Delete storage data if requested
	if req.DeleteData {
		storagePath := fmt.Sprintf("/var/lib/dokku/data/storage/%s", appName)
		if err := os.RemoveAll(storagePath); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to delete storage data: %s", err),
			})
		}
	}

	// Flash mesajı ekle
	_ = middleware.SetFlashMessage(c, fmt.Sprintf("Uygulama '%s' başarıyla silindi", appName), "success")

	// Flash mesajını API yanıtında da döndür
	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Uygulama '%s' başarıyla silindi", appName),
		"type":    "success",
	})
}

func handleAppDetails(c *fiber.Ctx) error {
	appName := c.Params("name")
	output, err := commandRunner.RunCommand("dokku", "ps:report", appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Uygulama detayları alınamadı: %v", err),
		})
	}

	// Default status values
	var running interface{} = false // interface{} olarak tanımla
	deployed := false
	processes := make(map[string]int)

	// Parse the output manually
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Deployed:") {
			deployed = strings.TrimSpace(strings.Split(line, ":")[1]) == "true"
		} else if strings.Contains(line, "Running:") {
			runningStatus := strings.TrimSpace(strings.Split(line, ":")[1])
			switch runningStatus {
			case "true":
				running = true
			case "mixed":
				running = "mixed"
			default:
				running = false
			}
		} else if strings.HasPrefix(line, "Status ") {
			parts := strings.Fields(line)
			if len(parts) >= 4 && strings.HasSuffix(parts[0], "Status") {
				procType := parts[1]
				if strings.Contains(line, "running") {
					processes[procType]++
				}
			}
		}
	}

	// Get app creation and last deploy times
	createdAt := dokku.GetAppCreatedAt(commandRunner, appName)
	lastDeployAt := dokku.GetLastDeployTime(commandRunner, appName)

	return c.JSON(fiber.Map{
		"running":      running,
		"deployed":     deployed,
		"processes":    processes,
		"createdAt":    createdAt,
		"lastDeployAt": lastDeployAt,
	})
}

func handleAppServices(c *fiber.Ctx) error {
	appName := c.Params("name")

	services, err := appService.GetAppServices(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Services could not be retrieved: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"services": services,
	})
}

func handleAppDockerOptions(c *fiber.Ctx) error {
	appName := c.Params("name")
	options, err := appService.GetDockerOptions(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Docker options could not be retrieved: %v", err),
		})
	}
	return c.JSON(options)
}

func handleAppDockerOptionsAdd(c *fiber.Ctx) error {
	appName := c.Params("name")
	optionType := c.Params("type")

	var req struct {
		Option string `json:"option"`
	}
	if err := c.BodyParser(&req); err != nil {
		return err
	}

	if err := appService.AddDockerOption(appName, optionType, req.Option); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Docker option could not be added: %v", err),
		})
	}
	return c.SendStatus(fiber.StatusOK)
}

func handleAppDockerOptionsUpdate(c *fiber.Ctx) error {
	appName := c.Params("name")
	optionType := c.Params("type")
	index := c.Params("index")

	var req struct {
		Option string `json:"option"`
	}
	if err := c.BodyParser(&req); err != nil {
		return err
	}

	indexInt, err := strconv.Atoi(index)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid index value",
		})
	}

	if indexInt < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Index cannot be negative",
		})
	}

	if err := appService.UpdateDockerOption(appName, optionType, indexInt, req.Option); err != nil {
		status := fiber.StatusInternalServerError
		switch {
		case errors.Is(err, appsvc.ErrInvalidOptionType):
			status = fiber.StatusBadRequest
		case errors.Is(err, appsvc.ErrIndexOutOfRange):
			status = fiber.StatusBadRequest
		}
		return c.Status(status).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to update Docker option: %v", err),
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

func handleAppDockerOptionsDelete(c *fiber.Ctx) error {
	appName := c.Params("name")
	optionType := c.Params("type")
	index := c.Params("index")

	indexInt, err := strconv.Atoi(index)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid index value",
		})
	}

	if indexInt < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Index cannot be negative",
		})
	}

	if err := appService.DeleteDockerOption(appName, optionType, indexInt); err != nil {
		status := fiber.StatusInternalServerError
		if errors.Is(err, appsvc.ErrInvalidOptionType) || errors.Is(err, appsvc.ErrIndexOutOfRange) {
			status = fiber.StatusBadRequest
		}
		return c.Status(status).JSON(fiber.Map{
			"error": fmt.Sprintf("Docker option could not be removed: %v", err),
		})
	}
	return c.SendStatus(fiber.StatusOK)
}

// Handler for setting the Procfile path
func handleAppProcfilePath(c *fiber.Ctx) error {
	appName := c.Params("name")

	var req struct {
		Path string `json:"path"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request data",
		})
	}

	// Validate path if not empty (empty = reset to default)
	if req.Path != "" && strings.HasPrefix(req.Path, "/") {
		errorMsg := "Path must be relative and should not start with a slash (/)"
		_ = middleware.SetFlashMessage(c, errorMsg, "error")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success":    false,
			"error":      errorMsg,
			"message":    errorMsg,
			"flash_type": "error",
		})
	}

	output, err := appService.SetProcfilePath(appName, req.Path)
	if err != nil {
		message := err.Error()
		if output != "" {
			message = output
		}
		_ = middleware.SetFlashMessage(c, message, "error")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success":    false,
			"error":      message,
			"message":    message,
			"flash_type": "error",
		})
	}

	successMsg := fmt.Sprintf("Procfile path for %s has been updated", appName)
	_ = middleware.SetFlashMessage(c, successMsg, "success")

	return c.JSON(fiber.Map{
		"success":    true,
		"message":    successMsg,
		"flash_type": "success",
	})
}

// Handler for setting the app.json path
func handleAppJsonPath(c *fiber.Ctx) error {
	appName := c.Params("name")

	var req struct {
		Path string `json:"path"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request data",
		})
	}

	// Validate path if not empty (empty = reset to default)
	if req.Path != "" && strings.HasPrefix(req.Path, "/") {
		errorMsg := "Path must be relative and should not start with a slash (/)"
		_ = middleware.SetFlashMessage(c, errorMsg, "error")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success":    false,
			"error":      errorMsg,
			"message":    errorMsg,
			"flash_type": "error",
		})
	}

	output, err := appService.SetAppJSONPath(appName, req.Path)
	if err != nil {
		message := err.Error()
		if output != "" {
			message = output
		}
		_ = middleware.SetFlashMessage(c, message, "error")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success":    false,
			"error":      message,
			"message":    message,
			"flash_type": "error",
		})
	}

	successMsg := fmt.Sprintf("app.json path for %s has been updated", appName)
	_ = middleware.SetFlashMessage(c, successMsg, "success")

	return c.JSON(fiber.Map{
		"success":    true,
		"message":    successMsg,
		"flash_type": "success",
	})
}

// Handler for getting the app.json path
func handleAppJsonPathInfo(c *fiber.Ctx) error {
	appName := c.Params("name")

	pathValue, err := appService.GetAppJSONPath(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("app.json path could not be retrieved: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"path":    pathValue,
	})
}

// Handler for getting the Procfile path
func handleAppProcfilePathInfo(c *fiber.Ctx) error {
	appName := c.Params("name")

	pathValue, err := appService.GetProcfilePath(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Procfile path could not be retrieved: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"path":    pathValue,
	})
}

// handleAppRestartPolicy retrieves the current restart policy for an app
func handleAppRestartPolicy(c *fiber.Ctx) error {
	appName := c.Params("name")
	policy, err := appService.GetRestartPolicy(appName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   fmt.Sprintf("Failed to retrieve restart policy: %v", err),
			"success": false,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"policy":  policy,
	})
}

// handleAppSetRestartPolicy sets a new restart policy for an app
func handleAppSetRestartPolicy(c *fiber.Ctx) error {
	appName := c.Params("name")
	var req struct {
		Policy string `json:"policy"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   fmt.Sprintf("Invalid request: %v", err),
			"success": false,
		})
	}
	if err := appService.SetRestartPolicy(appName, req.Policy); err != nil {
		status := fiber.StatusInternalServerError
		if errors.Is(err, appsvc.ErrInvalidRestartPolicy) {
			status = fiber.StatusBadRequest
		}
		return c.Status(status).JSON(fiber.Map{
			"error":   fmt.Sprintf("Failed to set restart policy: %v", err),
			"success": false,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Restart policy for %s has been set to %s", appName, req.Policy),
	})
}

// generateDatabaseKey creates a unique key for a database based on its URL
// to prevent duplicate entries in the results
