package stream

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/pruvon/pruvon/internal/models"

	"github.com/creack/pty"
	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"
)

// CommandRunner defines an interface for running commands
type CommandRunner interface {
	RunCommand(command string, args ...string) (string, error)
	StartPTY(command string, args ...string) (*os.File, error)
}

// ActivityLogger defines an interface for logging activity
type ActivityLogger interface {
	LogActivity(log models.ActivityLog)
}

// streamCommandRunner is the command runner used for stream operations
var streamCommandRunner CommandRunner

// activityLogger is the logger used for stream operations
var activityLogger ActivityLogger

// SetStreamCommandRunner sets the command runner used for stream operations
func SetStreamCommandRunner(runner CommandRunner) {
	streamCommandRunner = runner
}

// SetActivityLogger sets the activity logger used for stream operations
func SetActivityLogger(logger ActivityLogger) {
	activityLogger = logger
}

// StreamLogs streams application logs to a WebSocket connection.
func StreamLogs(c *websocket.Conn, runner CommandRunner, appName string) {
	// Log application logs viewing activity
	requestID := uuid.New().String()
	// Get session info from locals
	user := c.Locals("user").(string)
	authType := c.Locals("auth_type").(string)

	// Log logs viewing start
	if activityLogger != nil {
		activityLogger.LogActivity(models.ActivityLog{
			Time:       time.Now(),
			RequestID:  requestID,
			IP:         c.IP(),
			User:       user,
			AuthType:   authType,
			Action:     "app_logs_view_start",
			Route:      "/ws/apps/" + appName + "/logs",
			Parameters: json.RawMessage(fmt.Sprintf(`{"app": "%s"}`, appName)),
			StatusCode: 200,
		})
	}

	ptmx, err := runner.StartPTY("dokku", "logs", "-t", appName)
	if err != nil {
		_ = c.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
		return
	}
	defer ptmx.Close()

	reader := bufio.NewReader(ptmx)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				_ = c.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
			}
			break
		}

		if err := c.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
			break
		}
	}

	// Log logs viewing end
	if activityLogger != nil {
		activityLogger.LogActivity(models.ActivityLog{
			Time:       time.Now(),
			RequestID:  requestID,
			IP:         c.IP(),
			User:       user,
			AuthType:   authType,
			Action:     "app_logs_view_end",
			Route:      "/ws/apps/" + appName + "/logs",
			Parameters: json.RawMessage(fmt.Sprintf(`{"app": "%s"}`, appName)),
			StatusCode: 200,
		})
	}

}

// StreamTerminal streams a terminal session to a WebSocket connection
func StreamTerminal(c *websocket.Conn, appName string) {
	// Log terminal session start
	requestID := uuid.New().String()
	// Get session info from locals
	user := c.Locals("user").(string)
	authType := c.Locals("auth_type").(string)

	// Terminal session started
	if activityLogger != nil {
		activityLogger.LogActivity(models.ActivityLog{
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
	}

	// Use commandRunner.StartPTY
	if streamCommandRunner == nil {
		_ = c.WriteMessage(websocket.TextMessage, []byte("Error: command runner not initialized"))
		return
	}

	ptmx, err := streamCommandRunner.StartPTY("dokku", "enter", appName, "web", "bash")
	if err != nil {
		if activityLogger != nil {
			activityLogger.LogActivity(models.ActivityLog{
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
		}
		_ = c.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
		return
	}
	defer ptmx.Close()

	// Set terminal size
	_ = pty.Setsize(ptmx, &pty.Winsize{
		Rows: 24,
		Cols: 80,
	})

	// Send data from WebSocket to terminal
	go func() {
		for {
			mt, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			if mt == websocket.TextMessage {
				_, _ = ptmx.Write(msg)
			}
		}
	}()

	// Send data from terminal to WebSocket
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
}

// StreamNginxLogs streams Nginx logs to a WebSocket connection
func StreamNginxLogs(c *websocket.Conn, appName string, logType string) {
	// Log Nginx logs viewing activity
	requestID := uuid.New().String()
	// Get session info from locals
	user := c.Locals("user").(string)
	authType := c.Locals("auth_type").(string)

	// Log Nginx logs viewing start
	if activityLogger != nil {
		activityLogger.LogActivity(models.ActivityLog{
			Time:       time.Now(),
			RequestID:  requestID,
			IP:         c.IP(),
			User:       user,
			AuthType:   authType,
			Action:     "nginx_logs_view_start",
			Route:      "/ws/apps/" + appName + "/nginx-logs/" + logType,
			Parameters: json.RawMessage(fmt.Sprintf(`{"app": "%s", "log_type": "%s"}`, appName, logType)),
			StatusCode: 200,
		})
	}

	// Determine log path
	var logPath string
	switch logType {
	case "access":
		logPath = fmt.Sprintf("/var/log/nginx/%s-access.log", appName)
	case "error":
		logPath = fmt.Sprintf("/var/log/nginx/%s-error.log", appName)
	default:
		return
	}

	// We need to use exec.Command directly here because we need access to StdoutPipe
	cmd := exec.Command("tail", "-f", logPath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = c.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
		return
	}

	if err := cmd.Start(); err != nil {
		_ = c.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
		return
	}

	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				_ = c.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
			}
			break
		}

		if err := c.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
			break
		}
	}

	// Log nginx logs viewing end
	if activityLogger != nil {
		activityLogger.LogActivity(models.ActivityLog{
			Time:       time.Now(),
			RequestID:  requestID,
			IP:         c.IP(),
			User:       user,
			AuthType:   authType,
			Action:     "nginx_logs_view_end",
			Route:      "/ws/apps/" + appName + "/nginx-logs/" + logType,
			Parameters: json.RawMessage(fmt.Sprintf(`{"app": "%s", "log_type": "%s"}`, appName, logType)),
			StatusCode: 200,
		})
	}

	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}
