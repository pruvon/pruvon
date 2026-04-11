package api

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/models"
	"github.com/pruvon/pruvon/internal/templates"

	"github.com/gofiber/fiber/v2"
)

var errAuditAdminRequired = errors.New("administrator access is required")
var errAuditAppAccessDenied = errors.New("application access is required")

func SetupAuditRoutes(app *fiber.App) {
	app.Get("/api/audit/overview", handleAuditOverview)
	app.Get("/api/audit/events/:id", handleAuditEvent)
	app.Get("/api/audit/export", handleAuditExport)
	app.Post("/api/audit/backup", handleAuditBackup)
	app.Post("/api/audit/vacuum", handleAuditVacuum)

	app.Get("/api/apps/:name/audit", handleAppAudit)
	app.Get("/api/apps/:name/audit/events/:id", handleAppAuditEvent)
	app.Get("/api/apps/:name/audit/export", handleAppAuditExport)

	app.Get("/api/services/:type/:name/audit", handleServiceAudit)
	app.Get("/api/services/:type/:name/audit/events/:id", handleServiceAuditEvent)
}

func handleAuditOverview(c *fiber.Ctx) error {
	installed, err := dokku.IsPluginInstalledWithRunner(commandRunner, "audit")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Audit plugin status could not be checked: %v", err),
		})
	}

	overview := models.AuditOverview{
		Enabled:         installed,
		PluginInstalled: installed,
		Recent:          []models.AuditEvent{},
		Deploys:         []models.AuditEvent{},
	}

	if !installed {
		return c.JSON(overview)
	}

	allowedApps, isAdmin, err := getAllowedAppMapForCurrentUser(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Allowed app list could not be resolved: %v", err),
		})
	}

	if !isAdmin {
		appNames := sortedAuditAppNames(allowedApps)

		recent, err := getAuditTimelineForApps(appNames, 24, 12)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Recent audit activity could not be retrieved: %v", err),
			})
		}

		deploys, err := getAuditDeploysForApps(appNames, 10, 6)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Recent audit deploys could not be retrieved: %v", err),
			})
		}

		overview.Recent = recent
		overview.Deploys = deploys

		return c.JSON(overview)
	}

	status, err := dokku.GetAuditStatus(commandRunner)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Audit status could not be retrieved: %v", err),
		})
	}

	doctor, err := dokku.GetAuditDoctor(commandRunner)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Audit health could not be retrieved: %v", err),
		})
	}

	recent, err := dokku.GetAuditRecent(commandRunner, 10, "", "", "", "")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Recent audit activity could not be retrieved: %v", err),
		})
	}

	deploys, err := dokku.GetAuditLastDeploys(commandRunner, 10, "", "")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Recent audit deploys could not be retrieved: %v", err),
		})
	}

	overview.Status = &status
	overview.Doctor = &doctor
	overview.Recent = recent
	overview.Deploys = deploys

	return c.JSON(overview)
}

func handleAppAudit(c *fiber.Ctx) error {
	appName := c.Params("name")
	if err := requireAuditAppAccess(c, appName); err != nil {
		if errors.Is(err, errAuditAppAccessDenied) {
			return nil
		}
		return err
	}

	installed, err := dokku.IsPluginInstalledWithRunner(commandRunner, "audit")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Audit plugin status could not be checked: %v", err),
		})
	}

	result := models.AppAuditDetails{
		Enabled:       installed,
		Timeline:      []models.AuditEvent{},
		Deploys:       []models.AuditEvent{},
		DeployFlows:   []models.AuditDeployFlow{},
		ConfigChanges: []models.AuditEvent{},
		DomainChanges: []models.AuditEvent{},
		PortChanges:   []models.AuditEvent{},
		ProblemEvents: []models.AuditEvent{},
	}

	if !installed {
		return c.JSON(result)
	}

	timeline, err := dokku.GetAuditTimeline(commandRunner, appName, 250, "", "", "")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Application audit timeline could not be retrieved: %v", err),
		})
	}

	deploys, err := dokku.GetAuditLastDeploys(commandRunner, 12, appName, "")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Application deploy history could not be retrieved: %v", err),
		})
	}

	result.Timeline = timeline
	result.Deploys = deploys
	result.DeployFlows = dokku.GroupAuditEventsByCorrelation(filterAuditEventsWithCorrelation(timeline))
	result.ConfigChanges = filterAuditEventsByCategory(timeline, "config", 12)
	result.DomainChanges = filterAuditEventsByCategory(timeline, "domains", 12)
	result.PortChanges = filterAuditEventsByCategory(timeline, "ports", 12)
	result.ProblemEvents = filterProblemAuditEvents(timeline, 12)

	return c.JSON(result)
}

func handleAppAuditEvent(c *fiber.Ctx) error {
	appName := c.Params("name")
	eventID := c.Params("id")

	installed, err := dokku.IsPluginInstalledWithRunner(commandRunner, "audit")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Audit plugin status could not be checked: %v", err),
		})
	}
	if !installed {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Audit plugin is not installed",
		})
	}

	event, err := dokku.GetAuditEvent(commandRunner, eventID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Audit event could not be retrieved: %v", err),
		})
	}

	if event.App != appName {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Audit event not found for this application",
		})
	}

	return c.JSON(event)
}

func handleAuditEvent(c *fiber.Ctx) error {
	eventID := c.Params("id")
	installed, err := dokku.IsPluginInstalledWithRunner(commandRunner, "audit")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Audit plugin status could not be checked: %v", err),
		})
	}
	if !installed {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Audit plugin is not installed",
		})
	}

	event, err := dokku.GetAuditEvent(commandRunner, eventID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Audit event could not be retrieved: %v", err),
		})
	}

	_, authType := getSessionIdentity(c)
	if authType != "admin" {
		if event.App == "" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Access denied",
			})
		}

		allowedApps, _, err := getAllowedAppMapForCurrentUser(c)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Allowed app list could not be resolved: %v", err),
			})
		}

		if !allowedApps[event.App] {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Access denied",
			})
		}
	}

	return c.JSON(event)
}

func handleAppAuditExport(c *fiber.Ctx) error {
	if err := requireAuditAppAccess(c, c.Params("name")); err != nil {
		if errors.Is(err, errAuditAppAccessDenied) {
			return nil
		}
		return err
	}

	return writeAuditExportResponse(c, c.Params("name"))
}

func handleServiceAudit(c *fiber.Ctx) error {
	svcType := c.Params("type")
	svcName := c.Params("name")

	installed, err := dokku.IsPluginInstalledWithRunner(commandRunner, "audit")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Audit plugin status could not be checked: %v", err),
		})
	}

	result := models.ServiceAuditDetails{
		Enabled:    installed,
		LinkedApps: []string{},
		Recent:     []models.AuditEvent{},
		Deploys:    []models.AuditEvent{},
	}

	if !installed {
		return c.JSON(result)
	}

	linkedApps, err := dokku.GetLinkedApps(commandRunner, svcType, svcName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Linked applications could not be retrieved: %v", err),
		})
	}

	result.LinkedApps = linkedApps
	if len(linkedApps) == 0 {
		return c.JSON(result)
	}

	recent, err := getAuditTimelineForApps(linkedApps, 24, 12)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service audit activity could not be retrieved: %v", err),
		})
	}

	deploys, err := getAuditDeploysForApps(linkedApps, 10, 8)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service deploy activity could not be retrieved: %v", err),
		})
	}

	result.Recent = recent
	result.Deploys = deploys

	return c.JSON(result)
}

func handleServiceAuditEvent(c *fiber.Ctx) error {
	svcType := c.Params("type")
	svcName := c.Params("name")
	eventID := c.Params("id")

	installed, err := dokku.IsPluginInstalledWithRunner(commandRunner, "audit")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Audit plugin status could not be checked: %v", err),
		})
	}
	if !installed {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Audit plugin is not installed",
		})
	}

	linkedApps, err := dokku.GetLinkedApps(commandRunner, svcType, svcName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Linked applications could not be retrieved: %v", err),
		})
	}

	event, err := dokku.GetAuditEvent(commandRunner, eventID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Audit event could not be retrieved: %v", err),
		})
	}

	allowed := false
	for _, appName := range linkedApps {
		if event.App == appName {
			allowed = true
			break
		}
	}

	if !allowed {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Audit event not found for this service",
		})
	}

	return c.JSON(event)
}

func handleAuditExport(c *fiber.Ctx) error {
	if err := requireAuditAdmin(c); err != nil {
		if errors.Is(err, errAuditAdminRequired) {
			return nil
		}
		return err
	}

	return writeAuditExportResponse(c, c.Query("app"))
}

func handleAuditBackup(c *fiber.Ctx) error {
	if err := requireAuditAdmin(c); err != nil {
		if errors.Is(err, errAuditAdminRequired) {
			return nil
		}
		return err
	}

	path, err := dokku.CreateAuditBackup(commandRunner)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Audit backup could not be created: %v", err),
		})
	}

	return c.JSON(fiber.Map{"path": path})
}

func handleAuditVacuum(c *fiber.Ctx) error {
	if err := requireAuditAdmin(c); err != nil {
		if errors.Is(err, errAuditAdminRequired) {
			return nil
		}
		return err
	}

	message, err := dokku.VacuumAudit(commandRunner)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Audit vacuum could not be completed: %v", err),
		})
	}

	return c.JSON(fiber.Map{"message": message})
}

func writeAuditExportResponse(c *fiber.Ctx, appName string) error {
	installed, err := dokku.IsPluginInstalledWithRunner(commandRunner, "audit")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Audit plugin status could not be checked: %v", err),
		})
	}
	if !installed {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Audit plugin is not installed",
		})
	}

	format := strings.ToLower(strings.TrimSpace(c.Query("format", "json")))
	if format != "json" && format != "jsonl" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Unsupported export format",
		})
	}

	content, err := dokku.ExportAuditEvents(commandRunner, format, appName, c.Query("since"), c.Query("until"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Audit export could not be created: %v", err),
		})
	}

	if format == "jsonl" {
		c.Set(fiber.HeaderContentType, "application/x-ndjson")
	} else {
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	}

	return c.SendString(content)
}

func requireAuditAdmin(c *fiber.Ctx) error {
	_, authType := getSessionIdentity(c)
	if authType == "admin" {
		return nil
	}

	if err := c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"error": "Administrator access is required",
	}); err != nil {
		return err
	}

	return errAuditAdminRequired
}

func requireAuditAppAccess(c *fiber.Ctx, appName string) error {
	_, authType := getSessionIdentity(c)
	if authType == "admin" {
		return nil
	}

	allowedApps, _, err := getAllowedAppMapForCurrentUser(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Allowed app list could not be resolved: %v", err),
		})
	}

	if allowedApps[appName] {
		return nil
	}

	if err := c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"error": "Access denied",
	}); err != nil {
		return err
	}

	return errAuditAppAccessDenied
}

func getAllowedAppMapForCurrentUser(c *fiber.Ctx) (map[string]bool, bool, error) {
	username, authType := getSessionIdentity(c)
	if authType == "admin" {
		return nil, true, nil
	}

	allApps, err := dokku.GetDokkuApps(commandRunner)
	if err != nil {
		return nil, false, err
	}

	allowedList := templates.GetUserAllowedApps(username, authType, allApps)
	allowed := make(map[string]bool, len(allowedList))
	for _, appName := range allowedList {
		allowed[appName] = true
	}

	return allowed, false, nil
}

func getSessionIdentity(c *fiber.Ctx) (string, string) {
	sess, _ := middleware.GetStore().Get(c)
	username, _ := sess.Get("username").(string)
	if username == "" {
		username, _ = sess.Get("user").(string)
	}
	authType, _ := sess.Get("auth_type").(string)
	return username, authType
}

func getAuditTimelineForApps(appNames []string, perAppLimit int, finalLimit int) ([]models.AuditEvent, error) {
	return collectAuditEventsForApps(appNames, finalLimit, func(appName string) ([]models.AuditEvent, error) {
		return dokku.GetAuditTimeline(commandRunner, appName, perAppLimit, "", "", "")
	})
}

func getAuditDeploysForApps(appNames []string, perAppLimit int, finalLimit int) ([]models.AuditEvent, error) {
	return collectAuditEventsForApps(appNames, finalLimit, func(appName string) ([]models.AuditEvent, error) {
		return dokku.GetAuditLastDeploys(commandRunner, perAppLimit, appName, "")
	})
}

func collectAuditEventsForApps(appNames []string, finalLimit int, fetch func(appName string) ([]models.AuditEvent, error)) ([]models.AuditEvent, error) {
	if len(appNames) == 0 {
		return []models.AuditEvent{}, nil
	}

	events := make([]models.AuditEvent, 0)
	seen := make(map[int]bool)
	for _, appName := range appNames {
		appEvents, err := fetch(appName)
		if err != nil {
			return nil, err
		}

		for _, event := range appEvents {
			if seen[event.ID] {
				continue
			}
			seen[event.ID] = true
			events = append(events, event)
		}
	}

	sort.Slice(events, func(i, j int) bool {
		left := auditEventSortKey(events[i])
		right := auditEventSortKey(events[j])
		if left == right {
			return events[i].ID > events[j].ID
		}
		return left > right
	})

	if finalLimit > 0 && len(events) > finalLimit {
		return events[:finalLimit], nil
	}

	return events, nil
}

func sortedAuditAppNames(allowedApps map[string]bool) []string {
	appNames := make([]string, 0, len(allowedApps))
	for appName := range allowedApps {
		appNames = append(appNames, appName)
	}
	sort.Strings(appNames)
	return appNames
}

func auditEventSortKey(event models.AuditEvent) string {
	if event.Timestamp != "" {
		return event.Timestamp
	}
	return event.CreatedAt
}

func filterAuditEventsByCategory(events []models.AuditEvent, category string, limit int) []models.AuditEvent {
	filtered := make([]models.AuditEvent, 0)
	for _, event := range events {
		if event.Category != category {
			continue
		}

		filtered = append(filtered, event)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}

	return filtered
}

func filterProblemAuditEvents(events []models.AuditEvent, limit int) []models.AuditEvent {
	filtered := make([]models.AuditEvent, 0)
	for _, event := range events {
		normalizedStatus := strings.ToLower(strings.TrimSpace(event.Status))
		if normalizedStatus == "" || normalizedStatus == "success" || normalizedStatus == "pending" {
			continue
		}

		filtered = append(filtered, event)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}

	return filtered
}

func filterAuditEventsWithCorrelation(events []models.AuditEvent) []models.AuditEvent {
	filtered := make([]models.AuditEvent, 0)
	for _, event := range events {
		if event.CorrelationID == "" {
			continue
		}
		filtered = append(filtered, event)
	}

	return filtered
}
