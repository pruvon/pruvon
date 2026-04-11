package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleAuditOverviewUsesAppScopedDataForGitHubUser(t *testing.T) {
	runner := &dokku.MockCommandRunner{
		OutputMap: map[string]string{
			"dokku plugin:list": "=====> Installed plugins\naudit 0.2.0\n",
			"dokku apps:list":   "=====> My Apps\nallowed-app\nblocked-app\n",
			"dokku audit:timeline allowed-app --limit 24 --format json":           `[{"id":1,"ts":"2026-04-10T12:00:00Z","app":"allowed-app","category":"deploy","action":"finish","status":"success","classification":"source_deploy","actor_label":"sudo-user:emre","message":"deploy finished"}]`,
			"dokku audit:last-deploys --limit 10 --format json --app allowed-app": `[{"id":3,"ts":"2026-04-10T11:50:00Z","app":"allowed-app","category":"deploy","action":"finish","status":"success","classification":"source_deploy","actor_label":"sudo-user:emre","message":"deploy finished"}]`,
		},
		ErrorMap: map[string]error{},
	}

	app := newAuditTestApp(t, githubAuditConfig(config.GitHubUser{
		Username: "octo",
		Apps:     []string{"allowed-app"},
		Routes:   []string{"/api/apps/*"},
	}), runner)

	cookies := loginAuditUser(t, app, "octo", "github")
	resp := performAuditRequest(t, app, http.MethodGet, "/api/audit/overview", cookies)

	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var overview models.AuditOverview
	decodeAuditResponse(t, resp, &overview)

	require.True(t, overview.Enabled)
	assert.Nil(t, overview.Status)
	assert.Nil(t, overview.Doctor)
	assert.Len(t, overview.Recent, 1)
	assert.Len(t, overview.Deploys, 1)
	assert.Equal(t, "allowed-app", overview.Recent[0].App)
	assert.Equal(t, "allowed-app", overview.Deploys[0].App)
}

func TestHandleAuditOverviewIncludesHostHealthForAdmin(t *testing.T) {
	runner := &dokku.MockCommandRunner{
		OutputMap: map[string]string{
			"dokku plugin:list":                                 "=====> Installed plugins\naudit 0.2.0\n",
			"dokku audit:status":                                "plugin version: 0.2.0\ntotal events: 42\npending deploy count: 1\nlast event timestamp: 2026-04-10T12:05:00Z\n",
			"dokku audit:doctor":                                "ok: sqlite3 executable available\n",
			"dokku audit:recent --limit 10 --format json":       `[{"id":1,"ts":"2026-04-10T12:00:00Z","app":"allowed-app","category":"deploy","action":"finish","status":"success","classification":"source_deploy","actor_label":"sudo-user:emre","message":"deploy finished"}]`,
			"dokku audit:last-deploys --limit 10 --format json": `[{"id":3,"ts":"2026-04-10T11:50:00Z","app":"allowed-app","category":"deploy","action":"finish","status":"success","classification":"source_deploy","actor_label":"sudo-user:emre","message":"deploy finished"}]`,
		},
		ErrorMap: map[string]error{},
	}

	app := newAuditTestApp(t, githubAuditConfig(), runner)

	cookies := loginAuditUser(t, app, "admin", "admin")
	resp := performAuditRequest(t, app, http.MethodGet, "/api/audit/overview", cookies)

	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var overview models.AuditOverview
	decodeAuditResponse(t, resp, &overview)

	require.NotNil(t, overview.Status)
	require.NotNil(t, overview.Doctor)
	assert.Equal(t, 42, overview.Status.TotalEvents)
	assert.True(t, overview.Doctor.Healthy)
}

func TestHandleAuditEventDeniedForUnauthorizedGitHubUser(t *testing.T) {
	runner := &dokku.MockCommandRunner{
		OutputMap: map[string]string{
			"dokku plugin:list":                 "=====> Installed plugins\naudit 0.2.0\n",
			"dokku audit:show 99 --format json": `{"id":99,"ts":"2026-04-10T12:00:00Z","app":"blocked-app","category":"deploy","action":"finish","status":"success","classification":"source_deploy","actor_label":"sudo-user:emre","message":"deploy finished"}`,
			"dokku apps:list":                   "=====> My Apps\nallowed-app\nblocked-app\n",
		},
		ErrorMap: map[string]error{},
	}

	app := newAuditTestApp(t, githubAuditConfig(config.GitHubUser{
		Username: "octo",
		Apps:     []string{"allowed-app"},
		Routes:   []string{"/api/apps/*"},
	}), runner)

	cookies := loginAuditUser(t, app, "octo", "github")
	resp := performAuditRequest(t, app, http.MethodGet, "/api/audit/events/99", cookies)

	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)

	var payload map[string]string
	decodeAuditResponse(t, resp, &payload)
	assert.Equal(t, "Access denied", payload["error"])
}

func TestHandleAppAuditDeniedForUnauthorizedGitHubUser(t *testing.T) {
	runner := &dokku.MockCommandRunner{
		OutputMap: map[string]string{
			"dokku apps:list": "=====> My Apps\nallowed-app\nblocked-app\n",
		},
		ErrorMap: map[string]error{},
	}

	app := newAuditTestApp(t, githubAuditConfig(config.GitHubUser{
		Username: "octo",
		Apps:     []string{"allowed-app"},
		Routes:   []string{"/api/apps/*"},
	}), runner)

	cookies := loginAuditUser(t, app, "octo", "github")
	resp := performAuditRequest(t, app, http.MethodGet, "/api/apps/blocked-app/audit", cookies)

	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)

	var payload map[string]string
	decodeAuditResponse(t, resp, &payload)
	assert.Equal(t, "Access denied", payload["error"])
}

func TestHandleAppAuditExportDeniedForUnauthorizedGitHubUser(t *testing.T) {
	runner := &dokku.MockCommandRunner{
		OutputMap: map[string]string{
			"dokku apps:list": "=====> My Apps\nallowed-app\nblocked-app\n",
		},
		ErrorMap: map[string]error{},
	}

	app := newAuditTestApp(t, githubAuditConfig(config.GitHubUser{
		Username: "octo",
		Apps:     []string{"allowed-app"},
		Routes:   []string{"/api/apps/*"},
	}), runner)

	cookies := loginAuditUser(t, app, "octo", "github")
	resp := performAuditRequest(t, app, http.MethodGet, "/api/apps/blocked-app/audit/export", cookies)

	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)

	var payload map[string]string
	decodeAuditResponse(t, resp, &payload)
	assert.Equal(t, "Access denied", payload["error"])
}

func TestHandleAppAuditExportAllowsAuthorizedGitHubUser(t *testing.T) {
	runner := &dokku.MockCommandRunner{
		OutputMap: map[string]string{
			"dokku apps:list":   "=====> My Apps\nallowed-app\nblocked-app\n",
			"dokku plugin:list": "=====> Installed plugins\naudit 0.2.0\n",
			"dokku audit:export --format json --app allowed-app": `[{"id":1}]`,
		},
		ErrorMap: map[string]error{},
	}

	app := newAuditTestApp(t, githubAuditConfig(config.GitHubUser{
		Username: "octo",
		Apps:     []string{"allowed-app"},
	}), runner)

	cookies := loginAuditUser(t, app, "octo", "github")
	resp := performAuditRequest(t, app, http.MethodGet, "/api/apps/allowed-app/audit/export", cookies)

	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `[{"id":1}]`, string(body))
	assert.Equal(t, fiber.MIMEApplicationJSON, resp.Header.Get(fiber.HeaderContentType))
}

func TestHandleAuditExportRequiresAdminEvenWithExplicitRoute(t *testing.T) {
	app := newAuditTestApp(t, githubAuditConfig(config.GitHubUser{
		Username: "octo",
		Routes:   []string{"/api/audit/export"},
	}), &dokku.MockCommandRunner{OutputMap: map[string]string{}, ErrorMap: map[string]error{}})

	cookies := loginAuditUser(t, app, "octo", "github")
	resp := performAuditRequest(t, app, http.MethodGet, "/api/audit/export", cookies)

	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)

	var payload map[string]string
	decodeAuditResponse(t, resp, &payload)
	assert.Equal(t, "Administrator access is required", payload["error"])
}

func TestHandleServiceAuditReturnsLinkedAppActivity(t *testing.T) {
	runner := &dokku.MockCommandRunner{
		OutputMap: map[string]string{
			"dokku plugin:list":        "=====> Installed plugins\naudit 0.2.0\n",
			"dokku postgres:links db1": "=====> db1 links\nallowed-app\nlinked-app\n",
			"dokku audit:timeline allowed-app --limit 24 --format json":           `[{"id":11,"ts":"2026-04-10T12:00:00Z","app":"allowed-app","category":"config","action":"set","status":"success","classification":"","actor_label":"sudo-user:emre","message":"config updated"}]`,
			"dokku audit:timeline linked-app --limit 24 --format json":            `[{"id":13,"ts":"2026-04-10T12:02:00Z","app":"linked-app","category":"deploy","action":"finish","status":"success","classification":"source_deploy","actor_label":"sudo-user:emre","message":"deploy finished"}]`,
			"dokku audit:last-deploys --limit 10 --format json --app allowed-app": `[]`,
			"dokku audit:last-deploys --limit 10 --format json --app linked-app":  `[{"id":21,"ts":"2026-04-10T11:50:00Z","app":"linked-app","category":"deploy","action":"finish","status":"success","classification":"source_deploy","actor_label":"sudo-user:emre","message":"deploy finished"}]`,
		},
		ErrorMap: map[string]error{},
	}

	app := newAuditTestApp(t, githubAuditConfig(config.GitHubUser{
		Username: "octo",
		Services: map[string][]string{
			"postgres": {"db1"},
		},
	}), runner)

	cookies := loginAuditUser(t, app, "octo", "github")
	resp := performAuditRequest(t, app, http.MethodGet, "/api/services/postgres/db1/audit", cookies)

	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var details models.ServiceAuditDetails
	decodeAuditResponse(t, resp, &details)

	require.True(t, details.Enabled)
	assert.Equal(t, []string{"allowed-app", "linked-app"}, details.LinkedApps)
	assert.Len(t, details.Recent, 2)
	assert.Len(t, details.Deploys, 1)
	assert.Equal(t, "linked-app", details.Recent[0].App)
	assert.Equal(t, "allowed-app", details.Recent[1].App)
	assert.Equal(t, "linked-app", details.Deploys[0].App)
}

func newAuditTestApp(t *testing.T, cfg *config.Config, runner dokku.CommandRunner) *fiber.App {
	t.Helper()

	originalConfig := config.GetConfig()
	config.UpdateConfig(cfg)
	t.Cleanup(func() {
		config.UpdateConfig(originalConfig)
	})

	originalRunner := commandRunner
	commandRunner = runner
	t.Cleanup(func() {
		commandRunner = originalRunner
	})

	app := fiber.New()
	registerAuditTestLoginRoute(app)

	app.Use(middleware.Auth())
	SetupAuditRoutes(app)

	return app
}

func registerAuditTestLoginRoute(app *fiber.App) {
	app.Get("/__test/login/:user/:type", func(c *fiber.Ctx) error {
		sess, err := middleware.GetStore().Get(c)
		if err != nil {
			return err
		}

		sess.Set("authenticated", true)
		sess.Set("user", c.Params("user"))
		sess.Set("username", c.Params("user"))
		sess.Set("auth_type", c.Params("type"))
		if err := sess.Save(); err != nil {
			return err
		}

		return c.SendStatus(fiber.StatusNoContent)
	})
}

func githubAuditConfig(users ...config.GitHubUser) *config.Config {
	cfg := &config.Config{}
	cfg.GitHub.Users = users
	return cfg
}

func loginAuditUser(t *testing.T, app *fiber.App, user, authType string) []*http.Cookie {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/__test/login/"+user+"/"+authType, nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNoContent, resp.StatusCode)

	return resp.Cookies()
}

func performAuditRequest(t *testing.T, app *fiber.App, method, path string, cookies []*http.Cookie) *http.Response {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err := app.Test(req)
	require.NoError(t, err)

	return resp
}

func decodeAuditResponse(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(body, target), "response body: %s", string(body))
}
