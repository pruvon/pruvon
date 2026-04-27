package ws

import (
	"encoding/json"
	"fmt"
	"github.com/pruvon/pruvon/internal/appdeps"
	servicelogs "github.com/pruvon/pruvon/internal/services/logs"
	"os"
	"strings"
	"time"

	"github.com/pruvon/pruvon/internal/exec"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/models"
	"github.com/pruvon/pruvon/internal/services"
	"github.com/pruvon/pruvon/internal/stream"

	"github.com/creack/pty"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Terminal command message format
type TerminalMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

var wsCommandRunner exec.CommandRunner = exec.NewCommandRunner()

func SetupWsRoutes(app *fiber.App, deps *appdeps.Dependencies) {
	if deps != nil && deps.ExecRunner != nil {
		wsCommandRunner = deps.ExecRunner
	}

	// Websocket middleware with auth check
	app.Use("/ws", handleWsAuth(deps))

	app.Get("/ws/apps/:name/logs", websocket.New(handleAppLogs))
	app.Get("/ws/apps/:name/terminal", websocket.New(handleAppTerminal))
	// Import route must come before the general services/:type/:name pattern to avoid conflicts
	app.Get("/ws/services/import/:taskId", websocket.New(handleServiceImportTask))
	app.Get("/ws/services/:type/:name/console", websocket.New(handleServiceConsole))
	app.Get("/ws/apps/:name/nginx-logs/:type", websocket.New(handleAppNginxLogs))
	app.Get("/ws/apps/create", websocket.New(handleAppCreate))
	app.Get("/ws/docker/containers/:id/logs", websocket.New(handleContainerLogs))
	app.Get("/ws/docker/containers/:id/terminal", websocket.New(handleContainerTerminal))
}

func handleWsAuth(deps *appdeps.Dependencies) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			sess, err := middleware.GetStore().Get(c)
			if err != nil {
				return fiber.ErrUnauthorized
			}

			authenticated, ok := sess.Get("authenticated").(bool)
			if !ok || !authenticated {
				return fiber.ErrUnauthorized
			}

			user, ok := sess.Get("user").(string)
			if !ok || user == "" {
				user, ok = sess.Get("username").(string)
				if !ok || user == "" {
					return fiber.ErrUnauthorized
				}
			}

			authType, ok := sess.Get("auth_type").(string)
			if !ok || authType == "" {
				return fiber.ErrUnauthorized
			}

			c.Locals("user", user)
			c.Locals("auth_type", authType)
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	}
}

func handleAppLogs(c *websocket.Conn) {
	appName := c.Params("name")
	requestID := uuid.New().String()
	user := c.Locals("user").(string)
	authType := c.Locals("auth_type").(string)

	// Log app logs viewing session start
	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  requestID,
		IP:         c.IP(),
		User:       user,
		AuthType:   authType,
		Action:     "app_logs_view_start",
		Route:      "/ws/apps/" + appName + "/logs",
		Parameters: json.RawMessage(fmt.Sprintf(`{"app":"%s"}`, appName)),
		StatusCode: 200,
	})

	stream.StreamLogs(c, wsCommandRunner, appName)
}

func handleAppTerminal(c *websocket.Conn) {
	appName := c.Params("name")
	requestID := uuid.New().String()
	user := c.Locals("user").(string)
	authType := c.Locals("auth_type").(string)

	// Log terminal session start
	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  requestID,
		IP:         c.IP(),
		User:       user,
		AuthType:   authType,
		Action:     "terminal_session_start",
		Route:      "/ws/apps/" + appName + "/terminal",
		Parameters: json.RawMessage(fmt.Sprintf(`{"app":"%s"}`, appName)),
		StatusCode: 200,
	})

	// Set the environment variable for the terminal
	os.Setenv("TERM", "xterm-256color")

	// Use commandRunner.StartPTY here
	ptmx, err := wsCommandRunner.StartPTY("dokku", "enter", appName, "web", "bash")
	if err != nil {
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:       time.Now(),
			RequestID:  requestID,
			IP:         c.IP(),
			User:       user,
			AuthType:   authType,
			Action:     "terminal_session_error",
			Route:      "/ws/apps/" + appName + "/terminal",
			Error:      err.Error(),
			Parameters: json.RawMessage(fmt.Sprintf(`{"app":"%s","error":"%s"}`, appName, err.Error())),
			StatusCode: 500,
		})
		_ = c.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
		return
	}
	defer ptmx.Close()

	// Set initial terminal size
	_ = pty.Setsize(ptmx, &pty.Winsize{
		Rows: 30,
		Cols: 100,
	})

	var commandBuffer strings.Builder

	// Handle WebSocket input
	go func() {
		for {
			mt, msg, err := c.ReadMessage()
			if err != nil {
				return
			}

			if mt == websocket.TextMessage {
				// Try to parse as JSON first to check for control messages
				var jsonMsg map[string]interface{}
				if err := json.Unmarshal(msg, &jsonMsg); err == nil {
					// Check if this is a resize message
					if msgType, ok := jsonMsg["type"].(string); ok {
						if msgType == "resize" {
							if data, ok := jsonMsg["data"].(map[string]interface{}); ok {
								if cols, ok := data["cols"].(float64); ok {
									if rows, ok := data["rows"].(float64); ok {
										_ = pty.Setsize(ptmx, &pty.Winsize{
											Rows: uint16(rows),
											Cols: uint16(cols),
										})
										continue
									}
								}
							}
						} else if msgType == "set_env" {
							if data, ok := jsonMsg["data"].(map[string]interface{}); ok {
								if term, ok := data["TERM"].(string); ok {
									os.Setenv("TERM", term)
								}
							}
							continue
						}
					}
				}

				// If not a control message, process as regular input
				// Process each character to detect commands
				for _, ch := range msg {
					switch ch {
					case '\r', '\n':
						// Command completed, log it if not empty
						command := commandBuffer.String()
						if command != "" {
							// Log the command
							_ = servicelogs.LogActivity(models.ActivityLog{
								Time:      time.Now(),
								RequestID: requestID,
								IP:        c.IP(),
								User:      user,
								AuthType:  authType,
								Action:    "terminal_command",
								Route:     "/ws/apps/" + appName + "/terminal",
								Parameters: json.RawMessage(fmt.Sprintf(`{"app":"%s","command":"%s"}`,
									appName, strings.Replace(command, `"`, `\"`, -1))),
								StatusCode: 200,
							})
							// Reset buffer
							commandBuffer.Reset()
						}
					case 127, 8: // Backspace/Delete
						// Remove last character from buffer if not empty
						if commandBuffer.Len() > 0 {
							currentStr := commandBuffer.String()
							commandBuffer.Reset()
							commandBuffer.WriteString(currentStr[:len(currentStr)-1])
						}
					default:
						// Add character to command buffer
						commandBuffer.WriteByte(byte(ch))
					}
				}

				_, _ = ptmx.Write(msg)
			}
		}
	}()

	// Handle terminal output
	buf := make([]byte, 1024)
	for {
		n, err := ptmx.Read(buf)
		if err != nil {
			break
		}
		err = c.WriteMessage(websocket.BinaryMessage, buf[:n])
		if err != nil {
			break
		}
	}

	// Log terminal session end
	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  requestID,
		IP:         c.IP(),
		User:       user,
		AuthType:   authType,
		Action:     "terminal_session_end",
		Route:      "/ws/apps/" + appName + "/terminal",
		Parameters: json.RawMessage(fmt.Sprintf(`{"app":"%s"}`, appName)),
		StatusCode: 200,
	})
}

func handleAppNginxLogs(c *websocket.Conn) {
	appName := c.Params("name")
	logType := c.Params("type")
	requestID := uuid.New().String()
	user := c.Locals("user").(string)
	authType := c.Locals("auth_type").(string)

	// Log app nginx logs viewing session start
	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  requestID,
		IP:         c.IP(),
		User:       user,
		AuthType:   authType,
		Action:     "app_nginx_logs_view_start",
		Route:      "/ws/apps/" + appName + "/nginx-logs/" + logType,
		Parameters: json.RawMessage(fmt.Sprintf(`{"app":"%s","type":"%s"}`, appName, logType)),
		StatusCode: 200,
	})

	stream.StreamNginxLogs(c, appName, logType)
}

// Handle application creation via WebSocket
func handleAppCreate(c *websocket.Conn) {
	requestID := uuid.New().String()
	user := c.Locals("user").(string)
	authType := c.Locals("auth_type").(string)

	// Get form data from WebSocket
	_, msg, err := c.ReadMessage()
	if err != nil {
		errorMsg, _ := json.Marshal(models.StepResult{
			Message:  "WebSocket connection error",
			Progress: 0,
			Error:    err.Error(),
		})
		_ = c.WriteMessage(websocket.TextMessage, errorMsg)
		return
	}

	// Parse form data
	var formData struct {
		Name     string `json:"name"`
		Image    string `json:"image"`
		Services []struct {
			Name         string `json:"name"`
			ServiceName  string `json:"serviceName"`
			Image        string `json:"image"`
			ImageVersion string `json:"imageVersion"`
		} `json:"services"`
		Domain string `json:"domain"`
		SSL    bool   `json:"ssl"`
		Env    []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"env"`
		Port struct {
			Container string `json:"container"`
			Host      string `json:"host"`
		} `json:"port"`
		Mounts []struct {
			Source      string `json:"source"`
			Destination string `json:"destination"`
		} `json:"mounts"`
	}

	if err := json.Unmarshal(msg, &formData); err != nil {
		errorMsg, _ := json.Marshal(models.StepResult{
			Message:  "Failed to parse form data",
			Progress: 0,
			Error:    err.Error(),
		})
		_ = c.WriteMessage(websocket.TextMessage, errorMsg)
		return
	}

	// Get the app name
	appName := formData.Name
	if appName == "" {
		errorMsg, _ := json.Marshal(models.StepResult{
			Message:  "Application name not specified",
			Progress: 0,
			Error:    "Application name cannot be empty",
		})
		_ = c.WriteMessage(websocket.TextMessage, errorMsg)
		return
	}

	// Report progress
	progressMsg, _ := json.Marshal(models.StepResult{
		Message:  "Creating application...",
		Progress: 10,
	})
	_ = c.WriteMessage(websocket.TextMessage, progressMsg)

	// Log app creation start
	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  requestID,
		IP:         c.IP(),
		User:       user,
		AuthType:   authType,
		Action:     "app_create_start",
		Route:      "/ws/apps/create",
		Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s"}`, appName)),
		StatusCode: 200,
	})

	// Run the app creation command
	output, err := wsCommandRunner.RunCommand("dokku", "apps:create", appName)
	if err != nil {
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:       time.Now(),
			RequestID:  requestID,
			IP:         c.IP(),
			User:       user,
			AuthType:   authType,
			Action:     "app_create_error",
			Route:      "/ws/apps/create",
			Error:      err.Error(),
			Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","error":"%s"}`, appName, err.Error())),
			StatusCode: 500,
		})

		// Send error message in JSON format
		errorMsg, _ := json.Marshal(models.StepResult{
			Message:  "Application creation error",
			Progress: 0,
			Error:    err.Error(),
		})
		_ = c.WriteMessage(websocket.TextMessage, errorMsg)
		return
	}

	// Database creation
	if len(formData.Services) > 0 {
		for _, service := range formData.Services {
			progressMsg, _ := json.Marshal(models.StepResult{
				Message:  fmt.Sprintf("Creating %s service...", service.Name),
				Progress: 30,
			})
			_ = c.WriteMessage(websocket.TextMessage, progressMsg)

			// Return error if service name is empty
			if service.Name == "" {
				_ = servicelogs.LogActivity(models.ActivityLog{
					Time:       time.Now(),
					RequestID:  requestID,
					IP:         c.IP(),
					User:       user,
					AuthType:   authType,
					Action:     "service_create_error",
					Route:      "/ws/apps/create",
					Error:      "Service type not specified",
					Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","error":"Service type not specified"}`, appName)),
					StatusCode: 500,
				})

				errorMsg, _ := json.Marshal(models.StepResult{
					Message:  "Service type not specified",
					Progress: 30,
					Error:    "Service type not specified",
				})
				_ = c.WriteMessage(websocket.TextMessage, errorMsg)
				continue
			}

			// Use the service name specified by the user, error if empty
			if service.ServiceName == "" {
				_ = servicelogs.LogActivity(models.ActivityLog{
					Time:       time.Now(),
					RequestID:  requestID,
					IP:         c.IP(),
					User:       user,
					AuthType:   authType,
					Action:     "service_create_error",
					Route:      "/ws/apps/create",
					Error:      "Service name not specified",
					Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","error":"Service name not specified"}`, appName)),
					StatusCode: 500,
				})

				errorMsg, _ := json.Marshal(models.StepResult{
					Message:  "Service name not specified",
					Progress: 30,
					Error:    "Service name not specified",
				})
				_ = c.WriteMessage(websocket.TextMessage, errorMsg)
				continue
			}

			// Use the service name directly, without adding -db
			serviceName := service.ServiceName

			var createArgs []string

			// Build base command: dokku <service>:create <service-name>
			if service.Image != "" && service.ImageVersion != "" {
				// If image and version are specified: dokku postgres:create <service-name> --image <image> --image-version <version>
				createArgs = []string{fmt.Sprintf("%s:create", service.Name), serviceName, "--image", service.Image, "--image-version", service.ImageVersion}
			} else if service.Image != "" {
				// If only image is specified: dokku postgres:create <service-name> --image <image>
				createArgs = []string{fmt.Sprintf("%s:create", service.Name), serviceName, "--image", service.Image}
			} else {
				// If none specified: dokku postgres:create <service-name>
				createArgs = []string{fmt.Sprintf("%s:create", service.Name), serviceName}
			}

			_, err := wsCommandRunner.RunCommand("dokku", createArgs...)
			if err != nil {
				_ = servicelogs.LogActivity(models.ActivityLog{
					Time:       time.Now(),
					RequestID:  requestID,
					IP:         c.IP(),
					User:       user,
					AuthType:   authType,
					Action:     "service_create_error",
					Route:      "/ws/apps/create",
					Error:      err.Error(),
					Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","service_type":"%s","error":"%s"}`, appName, service.Name, err.Error())),
					StatusCode: 500,
				})

				// Send error message in JSON format
				errorMsg, _ := json.Marshal(models.StepResult{
					Message:  fmt.Sprintf("Error creating %s service", service.Name),
					Progress: 30,
					Error:    err.Error(),
				})
				_ = c.WriteMessage(websocket.TextMessage, errorMsg)
				// Continue on error, just log and notify
			} else {
				// Link the service to the app
				progressMsg, _ := json.Marshal(models.StepResult{
					Message:  fmt.Sprintf("Linking %s service to application...", service.Name),
					Progress: 40,
				})
				_ = c.WriteMessage(websocket.TextMessage, progressMsg)

				_, err := wsCommandRunner.RunCommand("dokku", fmt.Sprintf("%s:link", service.Name), serviceName, appName)
				if err != nil {
					_ = servicelogs.LogActivity(models.ActivityLog{
						Time:       time.Now(),
						RequestID:  requestID,
						IP:         c.IP(),
						User:       user,
						AuthType:   authType,
						Action:     "service_link_error",
						Route:      "/ws/apps/create",
						Error:      err.Error(),
						Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","service_type":"%s","error":"%s"}`, appName, service.Name, err.Error())),
						StatusCode: 500,
					})

					// Send error message in JSON format
					errorMsg, _ := json.Marshal(models.StepResult{
						Message:  fmt.Sprintf("Error linking %s service", service.Name),
						Progress: 40,
						Error:    err.Error(),
					})
					_ = c.WriteMessage(websocket.TextMessage, errorMsg)
					// Continue on error, just log and notify
				}
			}
		}
	}

	// Environment variables
	if len(formData.Env) > 0 {
		progressMsg, _ := json.Marshal(models.StepResult{
			Message:  "Setting environment variables...",
			Progress: 70,
		})
		_ = c.WriteMessage(websocket.TextMessage, progressMsg)

		for _, env := range formData.Env {
			// Skip empty keys or values
			if env.Key == "" || env.Value == "" {
				continue
			}

			_, err := wsCommandRunner.RunCommand("dokku", "config:set", "--no-restart", appName, fmt.Sprintf("%s=%s", env.Key, env.Value))
			if err != nil {
				_ = servicelogs.LogActivity(models.ActivityLog{
					Time:       time.Now(),
					RequestID:  requestID,
					IP:         c.IP(),
					User:       user,
					AuthType:   authType,
					Action:     "env_set_error",
					Route:      "/ws/apps/create",
					Error:      err.Error(),
					Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","key":"%s","error":"%s"}`, appName, env.Key, err.Error())),
					StatusCode: 500,
				})

				// Send error message in JSON format
				errorMsg, _ := json.Marshal(models.StepResult{
					Message:  fmt.Sprintf("Error setting environment variable: %s", env.Key),
					Progress: 70,
					Error:    err.Error(),
				})
				_ = c.WriteMessage(websocket.TextMessage, errorMsg)
				// Continue on error, just log and notify
			}
		}
	}

	// Set domain
	if formData.Domain != "" {
		progressMsg, _ := json.Marshal(models.StepResult{
			Message:  "Setting up domain...",
			Progress: 80,
		})
		_ = c.WriteMessage(websocket.TextMessage, progressMsg)

		// Calculate and remove the default domain
		vhostContent, err := wsCommandRunner.RunCommand("cat", "/home/dokku/VHOST")
		if err != nil {
			_ = servicelogs.LogActivity(models.ActivityLog{
				Time:       time.Now(),
				RequestID:  requestID,
				IP:         c.IP(),
				User:       user,
				AuthType:   authType,
				Action:     "vhost_read_error",
				Route:      "/ws/apps/create",
				Error:      err.Error(),
				Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","error":"%s"}`, appName, err.Error())),
				StatusCode: 500,
			})
			// If VHOST cannot be read, skip default domain removal
		} else {
			// Trim whitespace from VHOST content
			vhostDomain := strings.TrimSpace(vhostContent)
			if vhostDomain != "" {
				// Calculate default domain: <app>.<VHOST>
				defaultDomain := fmt.Sprintf("%s.%s", appName, vhostDomain)

				// First remove the default domain
				_, err := wsCommandRunner.RunCommand("dokku", "domains:remove", appName, defaultDomain)
				if err != nil {
					_ = servicelogs.LogActivity(models.ActivityLog{
						Time:       time.Now(),
						RequestID:  requestID,
						IP:         c.IP(),
						User:       user,
						AuthType:   authType,
						Action:     "domain_remove_error",
						Route:      "/ws/apps/create",
						Error:      err.Error(),
						Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","domain":"%s","error":"%s"}`, appName, defaultDomain, err.Error())),
						StatusCode: 500,
					})
					// Continue on error, just log and notify
				}
			}
		}

		// Add the domain specified by the user
		_, err = wsCommandRunner.RunCommand("dokku", "domains:add", appName, formData.Domain)
		if err != nil {
			_ = servicelogs.LogActivity(models.ActivityLog{
				Time:       time.Now(),
				RequestID:  requestID,
				IP:         c.IP(),
				User:       user,
				AuthType:   authType,
				Action:     "domain_set_error",
				Route:      "/ws/apps/create",
				Error:      err.Error(),
				Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","domain":"%s","error":"%s"}`, appName, formData.Domain, err.Error())),
				StatusCode: 500,
			})

			// Send error message in JSON format
			errorMsg, _ := json.Marshal(models.StepResult{
				Message:  "Error setting domain",
				Progress: 80,
				Error:    err.Error(),
			})
			_ = c.WriteMessage(websocket.TextMessage, errorMsg)
			// Continue on error, just log and notify
		}
	}

	// Port configuration
	if formData.Port.Host != "" && formData.Port.Container != "" {
		progressMsg, _ := json.Marshal(models.StepResult{
			Message:  "Configuring ports...",
			Progress: 85,
		})
		_ = c.WriteMessage(websocket.TextMessage, progressMsg)

		portMapping := fmt.Sprintf("http:%s:%s", formData.Port.Host, formData.Port.Container)
		_, err := wsCommandRunner.RunCommand("dokku", "ports:add", appName, portMapping)
		if err != nil {
			_ = servicelogs.LogActivity(models.ActivityLog{
				Time:       time.Now(),
				RequestID:  requestID,
				IP:         c.IP(),
				User:       user,
				AuthType:   authType,
				Action:     "port_add_error",
				Route:      "/ws/apps/create",
				Error:      err.Error(),
				Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","port":"%s","error":"%s"}`, appName, portMapping, err.Error())),
				StatusCode: 500,
			})

			// Send error message in JSON format
			errorMsg, _ := json.Marshal(models.StepResult{
				Message:  "Error configuring port",
				Progress: 85,
				Error:    err.Error(),
			})
			_ = c.WriteMessage(websocket.TextMessage, errorMsg)
			// Continue on error, just log and notify
		}
	}

	// Enable SSL - AFTER port configuration
	if formData.Domain != "" && formData.SSL {
		progressMsg, _ := json.Marshal(models.StepResult{
			Message:  "Enabling SSL...",
			Progress: 90,
		})
		_ = c.WriteMessage(websocket.TextMessage, progressMsg)

		// Directly try to enable Let's Encrypt
		_, err := wsCommandRunner.RunCommand("dokku", "letsencrypt:enable", appName)
		if err != nil {
			_ = servicelogs.LogActivity(models.ActivityLog{
				Time:       time.Now(),
				RequestID:  requestID,
				IP:         c.IP(),
				User:       user,
				AuthType:   authType,
				Action:     "ssl_enable_error",
				Route:      "/ws/apps/create",
				Error:      err.Error(),
				Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","error":"%s"}`, appName, err.Error())),
				StatusCode: 500,
			})

			// Let's Encrypt failing is not critical - app can still be used,
			// and user can enable it later manually
			warningMsg, _ := json.Marshal(models.StepResult{
				Message:  "SSL enabling error. Check your domain settings and manually run 'sudo -n -u dokku dokku letsencrypt:enable " + appName + "' command later.",
				Progress: 90,
				Error:    err.Error(),
			})
			_ = c.WriteMessage(websocket.TextMessage, warningMsg)
			// Continue despite error
		} else {
			successMsg, _ := json.Marshal(models.StepResult{
				Message:  "SSL successfully enabled",
				Progress: 92,
			})
			_ = c.WriteMessage(websocket.TextMessage, successMsg)
		}
	}

	// Storage mount operation
	if len(formData.Mounts) > 0 {
		progressMsg, _ := json.Marshal(models.StepResult{
			Message:  "Setting up storage mounts...",
			Progress: 95,
		})
		_ = c.WriteMessage(websocket.TextMessage, progressMsg)

		// First create the base storage directory
		baseStorageDir := fmt.Sprintf("/var/lib/dokku/data/storage/%s", appName)
		err := os.MkdirAll(baseStorageDir, 0755)
		if err != nil {
			_ = servicelogs.LogActivity(models.ActivityLog{
				Time:       time.Now(),
				RequestID:  requestID,
				IP:         c.IP(),
				User:       user,
				AuthType:   authType,
				Action:     "storage_dir_create_error",
				Route:      "/ws/apps/create",
				Error:      err.Error(),
				Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","dir":"%s","error":"%s"}`, appName, baseStorageDir, err.Error())),
				StatusCode: 500,
			})

			errorMsg, _ := json.Marshal(models.StepResult{
				Message:  "Error creating storage directory",
				Progress: 95,
				Error:    err.Error(),
			})
			_ = c.WriteMessage(websocket.TextMessage, errorMsg)
		} else {
			// Set permissions
			_, err = wsCommandRunner.RunCommand("chown", "-R", "dokku:dokku", baseStorageDir)
			if err != nil {
				_ = servicelogs.LogActivity(models.ActivityLog{
					Time:       time.Now(),
					RequestID:  requestID,
					IP:         c.IP(),
					User:       user,
					AuthType:   authType,
					Action:     "storage_permissions_error",
					Route:      "/ws/apps/create",
					Error:      err.Error(),
					Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","dir":"%s","error":"%s"}`, appName, baseStorageDir, err.Error())),
					StatusCode: 500,
				})
			}

			// Process each mount point
			for _, mount := range formData.Mounts {
				// Skip empty source or destination directories
				if mount.Source == "" || mount.Destination == "" {
					continue
				}

				// Create source directory
				sourceDir := mount.Source
				if !strings.HasPrefix(sourceDir, "/") {
					// If not a full path, create the full path
					sourceDir = fmt.Sprintf("/var/lib/dokku/data/storage/%s/%s", appName, sourceDir)
				}

				err := os.MkdirAll(sourceDir, 0755)
				if err != nil {
					_ = servicelogs.LogActivity(models.ActivityLog{
						Time:       time.Now(),
						RequestID:  requestID,
						IP:         c.IP(),
						User:       user,
						AuthType:   authType,
						Action:     "storage_source_dir_error",
						Route:      "/ws/apps/create",
						Error:      err.Error(),
						Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","source":"%s","error":"%s"}`, appName, sourceDir, err.Error())),
						StatusCode: 500,
					})

					continue // Skip this mount point and continue with others
				}

				// Set permissions
				_, err = wsCommandRunner.RunCommand("chown", "-R", "dokku:dokku", sourceDir)
				if err != nil {
					_ = servicelogs.LogActivity(models.ActivityLog{
						Time:       time.Now(),
						RequestID:  requestID,
						IP:         c.IP(),
						User:       user,
						AuthType:   authType,
						Action:     "storage_source_permissions_error",
						Route:      "/ws/apps/create",
						Error:      err.Error(),
						Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","source":"%s","error":"%s"}`, appName, sourceDir, err.Error())),
						StatusCode: 500,
					})
				}

				// Perform mount: dokku storage:mount app source:destination
				mountArg := fmt.Sprintf("%s:%s", sourceDir, mount.Destination)
				_, err = wsCommandRunner.RunCommand("dokku", "storage:mount", appName, mountArg)
				if err != nil {
					_ = servicelogs.LogActivity(models.ActivityLog{
						Time:       time.Now(),
						RequestID:  requestID,
						IP:         c.IP(),
						User:       user,
						AuthType:   authType,
						Action:     "storage_mount_error",
						Route:      "/ws/apps/create",
						Error:      err.Error(),
						Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","mount":"%s","error":"%s"}`, appName, mountArg, err.Error())),
						StatusCode: 500,
					})

					errorMsg, _ := json.Marshal(models.StepResult{
						Message:  fmt.Sprintf("Error mounting storage: %s", mountArg),
						Progress: 95,
						Error:    err.Error(),
					})
					_ = c.WriteMessage(websocket.TextMessage, errorMsg)
				}
			}
		}
	}

	// Check if we have an image from template - moved to end of process
	if formData.Image != "" {
		progressMsg, _ := json.Marshal(models.StepResult{
			Message:  fmt.Sprintf("Creating application from image: %s", formData.Image),
			Progress: 98,
		})
		_ = c.WriteMessage(websocket.TextMessage, progressMsg)

		// Use git:from-image to set up the app from the image
		_, err := wsCommandRunner.RunCommand("dokku", "git:from-image", appName, formData.Image)
		if err != nil {
			_ = servicelogs.LogActivity(models.ActivityLog{
				Time:       time.Now(),
				RequestID:  requestID,
				IP:         c.IP(),
				User:       user,
				AuthType:   authType,
				Action:     "app_from_image_error",
				Route:      "/ws/apps/create",
				Error:      err.Error(),
				Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","image":"%s","error":"%s"}`, appName, formData.Image, err.Error())),
				StatusCode: 500,
			})

			// Send error message in JSON format
			errorMsg, _ := json.Marshal(models.StepResult{
				Message:  "Error creating application from image",
				Progress: 98,
				Error:    err.Error(),
			})
			_ = c.WriteMessage(websocket.TextMessage, errorMsg)
			// Continue despite error
		} else {
			progressMsg, _ := json.Marshal(models.StepResult{
				Message:  "Application successfully created from image",
				Progress: 99,
			})
			_ = c.WriteMessage(websocket.TextMessage, progressMsg)
		}
	}

	// Send success response in JSON format
	successMsg, _ := json.Marshal(models.StepResult{
		Message:  "Application Created Successfully",
		Progress: 100,
	})
	_ = c.WriteMessage(websocket.TextMessage, successMsg)

	// Log app creation success
	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  requestID,
		IP:         c.IP(),
		User:       user,
		AuthType:   authType,
		Action:     "app_create_success",
		Route:      "/ws/apps/create",
		Parameters: json.RawMessage(fmt.Sprintf(`{"name":"%s","output":"%s"}`, appName, strings.ReplaceAll(output, "\"", "\\\""))),
		StatusCode: 200,
	})
}

// handleContainerLogs streams logs from a Docker container
func handleContainerLogs(c *websocket.Conn) {
	containerId := c.Params("id")
	requestID := uuid.New().String()
	user := c.Locals("user").(string)
	authType := c.Locals("auth_type").(string)

	// Log container logs viewing session start
	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  requestID,
		IP:         c.IP(),
		User:       user,
		AuthType:   authType,
		Action:     "container_logs_view",
		Route:      "/ws/docker/containers/" + containerId + "/logs",
		Parameters: json.RawMessage(fmt.Sprintf(`{"container_id":"%s"}`, containerId)),
		StatusCode: 200,
	})

	// Start docker logs process with follow option
	ptmx, err := wsCommandRunner.StartPTY("docker", "logs", "--follow", containerId)
	if err != nil {
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:       time.Now(),
			RequestID:  requestID,
			IP:         c.IP(),
			User:       user,
			AuthType:   authType,
			Action:     "container_logs_error",
			Route:      "/ws/docker/containers/" + containerId + "/logs",
			Error:      err.Error(),
			Parameters: json.RawMessage(fmt.Sprintf(`{"container_id":"%s","error":"%s"}`, containerId, err.Error())),
			StatusCode: 500,
		})
		_ = c.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
		return
	}
	defer ptmx.Close()

	// Create buffer for output
	buf := make([]byte, 1024)

	// Stream logs to the client
	go func() {
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				_ = c.WriteMessage(websocket.TextMessage, []byte("\nLog streaming ended."))
				break
			}
			err = c.WriteMessage(websocket.BinaryMessage, buf[:n])
			if err != nil {
				break
			}
		}
	}()

	// Keep the connection open
	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			break
		}
	}
}

// handleContainerTerminal provides interactive terminal access to a Docker container
func handleContainerTerminal(c *websocket.Conn) {
	containerId := c.Params("id")
	requestID := uuid.New().String()
	user := c.Locals("user").(string)
	authType := c.Locals("auth_type").(string)

	// Log container terminal session start
	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  requestID,
		IP:         c.IP(),
		User:       user,
		AuthType:   authType,
		Action:     "container_terminal_session_start",
		Route:      "/ws/docker/containers/" + containerId + "/terminal",
		Parameters: json.RawMessage(fmt.Sprintf(`{"container_id":"%s"}`, containerId)),
		StatusCode: 200,
	})

	// Set the environment variable for the terminal
	os.Setenv("TERM", "xterm-256color")

	// Start an interactive shell in the container
	ptmx, err := wsCommandRunner.StartPTY("docker", "exec", "-i", containerId, "sh")
	if err != nil {
		// Try bash if sh fails
		ptmx, err = wsCommandRunner.StartPTY("docker", "exec", "-i", containerId, "bash")
		if err != nil {
			_ = servicelogs.LogActivity(models.ActivityLog{
				Time:       time.Now(),
				RequestID:  requestID,
				IP:         c.IP(),
				User:       user,
				AuthType:   authType,
				Action:     "container_terminal_session_error",
				Route:      "/ws/docker/containers/" + containerId + "/terminal",
				Error:      err.Error(),
				Parameters: json.RawMessage(fmt.Sprintf(`{"container_id":"%s","error":"%s"}`, containerId, err.Error())),
				StatusCode: 500,
			})
			_ = c.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()+"\nCannot connect to container terminal. Make sure the container is running and has a shell."))
			return
		}
	}
	defer ptmx.Close()

	// Set initial terminal size
	_ = pty.Setsize(ptmx, &pty.Winsize{
		Rows: 30,
		Cols: 100,
	})

	var commandBuffer strings.Builder

	// Handle WebSocket input
	go func() {
		for {
			mt, msg, err := c.ReadMessage()
			if err != nil {
				return
			}

			if mt == websocket.TextMessage {
				// Try to parse as JSON first to check for control messages
				var jsonMsg map[string]interface{}
				if err := json.Unmarshal(msg, &jsonMsg); err == nil {
					// Check if this is a resize message
					if msgType, ok := jsonMsg["type"].(string); ok {
						if msgType == "resize" {
							if data, ok := jsonMsg["data"].(map[string]interface{}); ok {
								if cols, ok := data["cols"].(float64); ok {
									if rows, ok := data["rows"].(float64); ok {
										_ = pty.Setsize(ptmx, &pty.Winsize{
											Rows: uint16(rows),
											Cols: uint16(cols),
										})
										continue
									}
								}
							}
						} else if msgType == "set_env" {
							if data, ok := jsonMsg["data"].(map[string]interface{}); ok {
								if term, ok := data["TERM"].(string); ok {
									os.Setenv("TERM", term)
								}
							}
							continue
						}
					}
				}

				// If not a control message, process as regular input
				// Process each character to detect commands
				for _, ch := range msg {
					switch ch {
					case '\r', '\n':
						// Command completed, log it if not empty
						command := commandBuffer.String()
						if command != "" {
							// Log the command
							_ = servicelogs.LogActivity(models.ActivityLog{
								Time:      time.Now(),
								RequestID: requestID,
								IP:        c.IP(),
								User:      user,
								AuthType:  authType,
								Action:    "container_terminal_command",
								Route:     "/ws/docker/containers/" + containerId + "/terminal",
								Parameters: json.RawMessage(fmt.Sprintf(`{"container_id":"%s","command":"%s"}`,
									containerId, strings.Replace(command, `"`, `\"`, -1))),
								StatusCode: 200,
							})
							// Reset buffer
							commandBuffer.Reset()
						}
					case 127, 8: // Backspace/Delete
						// Remove last character from buffer if not empty
						if commandBuffer.Len() > 0 {
							currentStr := commandBuffer.String()
							commandBuffer.Reset()
							commandBuffer.WriteString(currentStr[:len(currentStr)-1])
						}
					default:
						// Add character to command buffer
						commandBuffer.WriteByte(byte(ch))
					}
				}

				_, _ = ptmx.Write(msg)
			}
		}
	}()

	// Read output from the PTY and send to WebSocket
	buf := make([]byte, 1024)
	for {
		n, err := ptmx.Read(buf)
		if err != nil {
			_ = c.WriteMessage(websocket.TextMessage, []byte("\r\nTerminal session ended."))
			break
		}
		err = c.WriteMessage(websocket.BinaryMessage, buf[:n])
		if err != nil {
			break
		}
	}

	// Log session end
	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  requestID,
		IP:         c.IP(),
		User:       user,
		AuthType:   authType,
		Action:     "container_terminal_session_end",
		Route:      "/ws/docker/containers/" + containerId + "/terminal",
		Parameters: json.RawMessage(fmt.Sprintf(`{"container_id":"%s"}`, containerId)),
		StatusCode: 200,
	})
}

// handleServiceImportTask handles the WebSocket connection for a service import task
func handleServiceImportTask(c *websocket.Conn) {
	taskId := c.Params("taskId")
	requestID := uuid.New().String() // New request ID for this specific WS interaction

	// --- Retrieve Task Details ---
	services.ImportTasksMutex.Lock()
	taskDetails, ok := services.ImportTasks[taskId]
	if ok {
		// Remove task once retrieved to prevent reuse
		delete(services.ImportTasks, taskId)
	}
	services.ImportTasksMutex.Unlock()

	if !ok {
		// Task not found or already processed
		errorMsg := map[string]string{"error": "Invalid or expired import task ID."}
		_ = c.WriteJSON(errorMsg)
		_ = c.Close()
		return
	}

	// Ensure temporary file is cleaned up when we're done
	defer os.Remove(taskDetails.TempFilePath)

	// Log the execution start
	logRoutePath := fmt.Sprintf("/ws/services/import/%s", taskId)
	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  requestID,
		IP:         c.IP(),
		User:       taskDetails.User,
		AuthType:   taskDetails.AuthType,
		Action:     "service_import_execute",
		Route:      logRoutePath,
		Parameters: json.RawMessage(fmt.Sprintf(`{"taskId":"%s", "type":"%s", "name":"%s"}`, taskId, taskDetails.SvcType, taskDetails.SvcName)),
		StatusCode: 200, // Indicate the process is starting
	})

	// Check if the file is gzipped
	isGzipped := strings.HasSuffix(strings.ToLower(taskDetails.OriginalFilename), ".gz")

	var importScript string

	if isGzipped {
		// Send message about decompression
		_ = c.WriteJSON(map[string]string{"output": "Detected compressed .gz file. Will decompress before import...\n"})

		// Use positional parameters so the shell never interpolates user-controlled values.
		importScript = `zcat -- "$1" | dokku "$2:import" "$3"`
	} else {
		// Standard import for uncompressed files
		importScript = `cat -- "$1" | dokku "$2:import" "$3"`
	}

	// --- Execute Import Command ---
	ptmx, err := wsCommandRunner.StartPTY(
		"bash",
		"-c",
		importScript,
		"--",
		taskDetails.TempFilePath,
		taskDetails.SvcType,
		taskDetails.SvcName,
	)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to start import command: %v", err)
		// Log error
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:       time.Now(),
			RequestID:  requestID,
			IP:         c.IP(),
			User:       taskDetails.User,
			AuthType:   taskDetails.AuthType,
			Action:     "service_import_error",
			Route:      logRoutePath,
			Error:      errMsg,
			Parameters: json.RawMessage(fmt.Sprintf(`{"taskId":"%s", "type":"%s", "name":"%s"}`, taskId, taskDetails.SvcType, taskDetails.SvcName)),
			StatusCode: 500,
		})
		// Send error to client
		_ = c.WriteJSON(map[string]string{"error": errMsg})
		return
	}
	defer ptmx.Close()

	// --- Stream Output ---
	buf := make([]byte, 1024)
	for {
		n, err := ptmx.Read(buf)
		if err != nil {
			// Command finished or error occurred during read
			_ = c.WriteJSON(map[string]string{"output": "\n--- Import process finished ---"})
			break
		}
		if n > 0 {
			// Send output chunk to client
			err = c.WriteJSON(map[string]string{"output": string(buf[:n])})
			if err != nil {
				// WebSocket write error, stop streaming
				break
			}
		}
	}

	// Log completion
	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  requestID,
		IP:         c.IP(),
		User:       taskDetails.User,
		AuthType:   taskDetails.AuthType,
		Action:     "service_import_complete",
		Route:      logRoutePath,
		Parameters: json.RawMessage(fmt.Sprintf(`{"taskId":"%s", "type":"%s", "name":"%s"}`, taskId, taskDetails.SvcType, taskDetails.SvcName)),
		StatusCode: 200,
	})
}

func handleServiceConsole(c *websocket.Conn) {
	svcType := c.Params("type")
	svcName := c.Params("name")
	requestID := uuid.New().String()
	user := c.Locals("user").(string)
	authType := c.Locals("auth_type").(string)

	// Log service console session start
	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  requestID,
		IP:         c.IP(),
		User:       user,
		AuthType:   authType,
		Action:     "service_console_start",
		Route:      "/ws/services/" + svcType + "/" + svcName + "/console",
		Parameters: json.RawMessage(fmt.Sprintf(`{"type":"%s","name":"%s"}`, svcType, svcName)),
		StatusCode: 200,
	})

	// Set the environment variable for the terminal (system level only)
	os.Setenv("TERM", "xterm-256color")

	// Select the appropriate command based on the service type
	var command string
	var args []string

	switch svcType {
	case "mariadb":
		command = "dokku"
		args = []string{"mariadb:connect", svcName}
	case "postgres":
		command = "dokku"
		args = []string{"postgres:connect", svcName}
	case "mongo":
		command = "dokku"
		args = []string{"mongo:connect", svcName}
	case "redis":
		command = "dokku"
		args = []string{"redis:connect", svcName}
	default:
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:       time.Now(),
			RequestID:  requestID,
			IP:         c.IP(),
			User:       user,
			AuthType:   authType,
			Action:     "service_console_error",
			Route:      "/ws/services/" + svcType + "/" + svcName + "/console",
			Error:      "Unsupported service type",
			Parameters: json.RawMessage(fmt.Sprintf(`{"type":"%s","name":"%s","error":"Unsupported service type"}`, svcType, svcName)),
			StatusCode: 400,
		})
		_ = c.WriteMessage(websocket.TextMessage, []byte("Error: Unsupported service type"))
		return
	}

	ptmx, err := wsCommandRunner.StartPTY(command, args...)
	if err != nil {
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:       time.Now(),
			RequestID:  requestID,
			IP:         c.IP(),
			User:       user,
			AuthType:   authType,
			Action:     "service_console_error",
			Route:      "/ws/services/" + svcType + "/" + svcName + "/console",
			Error:      err.Error(),
			Parameters: json.RawMessage(fmt.Sprintf(`{"type":"%s","name":"%s","error":"%s"}`, svcType, svcName, err.Error())),
			StatusCode: 500,
		})
		_ = c.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
		return
	}
	defer ptmx.Close()

	// Set terminal size
	_ = pty.Setsize(ptmx, &pty.Winsize{
		Rows: 30,
		Cols: 100,
	})

	// Handle WebSocket input with command logging
	go func() {
		var currentCommand strings.Builder
		for {
			mt, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			if mt == websocket.TextMessage {
				// Process command characters
				for _, ch := range msg {
					if ch == '\r' || ch == '\n' {
						// Log completed command if not empty
						if cmd := currentCommand.String(); cmd != "" {
							_ = servicelogs.LogActivity(models.ActivityLog{
								Time:       time.Now(),
								RequestID:  requestID,
								IP:         c.IP(),
								User:       user,
								AuthType:   authType,
								Action:     "service_command",
								Route:      "/ws/services/" + svcType + "/" + svcName + "/console",
								Parameters: json.RawMessage(fmt.Sprintf(`{"command":"%s","svc_type":"%s","svc_name":"%s"}`, cmd, svcType, svcName)),
								StatusCode: 200,
							})
							currentCommand.Reset()
						}
					} else {
						currentCommand.WriteByte(byte(ch))
					}
				}
				_, _ = ptmx.Write(msg)
			}
		}
	}()

	// Handle console output
	buf := make([]byte, 1024)
	for {
		n, err := ptmx.Read(buf)
		if err != nil {
			break
		}
		err = c.WriteMessage(websocket.BinaryMessage, buf[:n])
		if err != nil {
			break
		}
	}

	// Log the process finished successfully
	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  requestID,
		IP:         c.IP(),
		User:       user,
		AuthType:   authType,
		Action:     "service_console_end",
		Route:      "/ws/services/" + svcType + "/" + svcName + "/console",
		Parameters: json.RawMessage(fmt.Sprintf(`{"type":"%s","name":"%s"}`, svcType, svcName)),
		StatusCode: 200,
	})
}
