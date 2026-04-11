package dokku

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pruvon/pruvon/internal/models"
)

func GetAuditStatus(runner CommandRunner) (models.AuditStatus, error) {
	output, err := runner.RunCommand("dokku", "audit:status")
	if err != nil {
		return models.AuditStatus{}, fmt.Errorf("audit status could not be retrieved: %w", err)
	}

	values := parseAuditKeyValueOutput(output)

	return models.AuditStatus{
		PluginVersion:      values["plugin version"],
		DatabasePath:       values["database path"],
		ApplicationID:      values["application id"],
		SchemaVersion:      values["schema version"],
		JournalMode:        values["journal mode"],
		TotalEvents:        parseAuditInt(values["total events"]),
		PendingDeployCount: parseAuditInt(values["pending deploy count"]),
		LastEventTimestamp: values["last event timestamp"],
		LastMigrationTime:  values["last migration timestamp"],
	}, nil
}

func GetAuditDoctor(runner CommandRunner) (models.AuditDoctor, error) {
	output, err := runner.RunCommand("dokku", "audit:doctor")
	doctor := parseAuditDoctorOutput(output)
	doctor.Healthy = len(doctor.Issues) == 0

	if err != nil && len(doctor.OK) == 0 && len(doctor.Issues) == 0 {
		return models.AuditDoctor{}, fmt.Errorf("audit doctor could not be retrieved: %w", err)
	}

	return doctor, nil
}

func GetAuditRecent(runner CommandRunner, limit int, category string, classification string, status string, since string) ([]models.AuditEvent, error) {
	args := []string{"audit:recent", "--limit", strconv.Itoa(normalizeAuditLimit(limit)), "--format", "json"}
	if category != "" {
		args = append(args, "--category", category)
	}
	if classification != "" {
		args = append(args, "--classification", classification)
	}
	if status != "" {
		args = append(args, "--status", status)
	}
	if since != "" {
		args = append(args, "--since", since)
	}

	return getAuditEvents(runner, args, "recent audit events")
}

func GetAuditTimeline(runner CommandRunner, appName string, limit int, since string, until string, category string) ([]models.AuditEvent, error) {
	args := []string{"audit:timeline", appName, "--limit", strconv.Itoa(normalizeAuditLimit(limit)), "--format", "json"}
	if since != "" {
		args = append(args, "--since", since)
	}
	if until != "" {
		args = append(args, "--until", until)
	}
	if category != "" {
		args = append(args, "--category", category)
	}

	return getAuditEvents(runner, args, "application audit timeline")
}

func GetAuditLastDeploys(runner CommandRunner, limit int, appName string, classification string) ([]models.AuditEvent, error) {
	args := []string{"audit:last-deploys", "--limit", strconv.Itoa(normalizeAuditLimit(limit)), "--format", "json"}
	if appName != "" {
		args = append(args, "--app", appName)
	}
	if classification != "" {
		args = append(args, "--classification", classification)
	}

	return getAuditEvents(runner, args, "audit deploy history")
}

func GetAuditEvent(runner CommandRunner, eventID string) (models.AuditEvent, error) {
	output, err := runner.RunCommand("dokku", "audit:show", eventID, "--format", "json")
	if err != nil {
		return models.AuditEvent{}, fmt.Errorf("audit event could not be retrieved: %w", err)
	}

	var event models.AuditEvent
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &event); err != nil {
		return models.AuditEvent{}, fmt.Errorf("audit event could not be parsed: %w", err)
	}

	return event, nil
}

func ExportAuditEvents(runner CommandRunner, format string, appName string, since string, until string) (string, error) {
	if format == "" {
		format = "json"
	}

	args := []string{"audit:export", "--format", format}
	if appName != "" {
		args = append(args, "--app", appName)
	}
	if since != "" {
		args = append(args, "--since", since)
	}
	if until != "" {
		args = append(args, "--until", until)
	}

	output, err := runner.RunCommand("dokku", args...)
	if err != nil {
		return "", fmt.Errorf("audit export could not be created: %w", err)
	}

	return strings.TrimSpace(output), nil
}

func CreateAuditBackup(runner CommandRunner) (string, error) {
	output, err := runner.RunCommand("dokku", "audit:backup")
	if err != nil {
		return "", fmt.Errorf("audit backup could not be created: %w", err)
	}

	return strings.TrimSpace(output), nil
}

func VacuumAudit(runner CommandRunner) (string, error) {
	output, err := runner.RunCommand("dokku", "audit:vacuum")
	if err != nil {
		return "", fmt.Errorf("audit vacuum could not be completed: %w", err)
	}

	return strings.TrimSpace(output), nil
}

func FilterAuditEventsByApps(events []models.AuditEvent, allowedApps map[string]bool, limit int) []models.AuditEvent {
	if allowedApps == nil {
		return limitAuditEvents(events, limit)
	}

	filtered := make([]models.AuditEvent, 0, len(events))
	for _, event := range events {
		if !allowedApps[event.App] {
			continue
		}

		filtered = append(filtered, event)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}

	return filtered
}

func GroupAuditEventsByCorrelation(events []models.AuditEvent) []models.AuditDeployFlow {
	groups := make(map[string]*models.AuditDeployFlow)
	order := make([]string, 0)

	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.CorrelationID == "" {
			continue
		}

		flow, ok := groups[event.CorrelationID]
		if !ok {
			flow = &models.AuditDeployFlow{CorrelationID: event.CorrelationID}
			groups[event.CorrelationID] = flow
			order = append(order, event.CorrelationID)
		}

		flow.Events = append(flow.Events, event)
		if event.App != "" {
			flow.App = event.App
		}
		if event.Status != "" {
			flow.Status = event.Status
		}
		if event.Classification != "" {
			flow.Classification = event.Classification
		}
		if event.ActorLabel != "" {
			flow.ActorLabel = event.ActorLabel
		}
		if event.SourceType != "" {
			flow.SourceType = event.SourceType
		}
		if event.ImageTag != "" {
			flow.ImageTag = event.ImageTag
		}
		if event.Revision != "" {
			flow.Revision = event.Revision
		}
		if flow.StartedAt == "" || event.Timestamp < flow.StartedAt {
			flow.StartedAt = event.Timestamp
		}
		if flow.FinishedAt == "" || event.Timestamp > flow.FinishedAt {
			flow.FinishedAt = event.Timestamp
		}
	}

	flows := make([]models.AuditDeployFlow, 0, len(order))
	for i := len(order) - 1; i >= 0; i-- {
		flows = append(flows, *groups[order[i]])
	}

	return flows
}

func limitAuditEvents(events []models.AuditEvent, limit int) []models.AuditEvent {
	if limit <= 0 || len(events) <= limit {
		return events
	}

	return events[:limit]
}

func getAuditEvents(runner CommandRunner, args []string, label string) ([]models.AuditEvent, error) {
	output, err := runner.RunCommand("dokku", args...)
	if err != nil {
		return nil, fmt.Errorf("%s could not be retrieved: %w", label, err)
	}

	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return []models.AuditEvent{}, nil
	}

	var events []models.AuditEvent
	if err := json.Unmarshal([]byte(trimmed), &events); err != nil {
		return nil, fmt.Errorf("%s could not be parsed: %w", label, err)
	}

	return events, nil
}

func parseAuditDoctorOutput(output string) models.AuditDoctor {
	doctor := models.AuditDoctor{
		OK:     []string{},
		Issues: []string{},
	}

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "ok:"):
			doctor.OK = append(doctor.OK, strings.TrimSpace(strings.TrimPrefix(trimmed, "ok:")))
		case strings.HasPrefix(trimmed, "issue:"):
			doctor.Issues = append(doctor.Issues, strings.TrimSpace(strings.TrimPrefix(trimmed, "issue:")))
		}
	}

	return doctor
}

func parseAuditKeyValueOutput(output string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.Contains(trimmed, ":") {
			continue
		}

		parts := strings.SplitN(trimmed, ":", 2)
		values[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
	}

	return values
}

func parseAuditInt(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}

	return parsed
}

func normalizeAuditLimit(limit int) int {
	if limit <= 0 {
		return 20
	}

	return limit
}
