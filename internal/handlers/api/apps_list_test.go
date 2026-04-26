package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/middleware"
	appsvc "github.com/pruvon/pruvon/internal/services/apps"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleAppsListCombinesAppsAndRouteDerivedApps(t *testing.T) {
	runner := &dokku.MockCommandRunner{
		OutputMap: map[string]string{
			"dokku apps:list": "=====> My Apps\nfoo\nbar\nbaz\n",
		},
		ErrorMap: map[string]error{},
	}

	app := newAppsListTestApp(t, &config.Config{Users: []config.User{{
		Username: "octo",
		Role:     config.RoleUser,
		Apps:     []string{"foo"},
		Routes:   []string{"/api/apps/list", "/apps/bar/*"},
	}}}, runner)

	resp := performAppsListRequest(t, app, "/api/apps/list", loginAppsUser(t, app, "octo", config.RoleUser))
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var payload struct {
		Apps []string `json:"apps"`
	}
	decodeAppsListResponse(t, resp, &payload)
	assert.ElementsMatch(t, []string{"foo", "bar"}, payload.Apps)
}

func TestHandleAppsListDetailedIncludesAppsGrantedWithoutRoutes(t *testing.T) {
	runner := &dokku.MockCommandRunner{
		OutputMap: map[string]string{
			"dokku apps:list":     "=====> My Apps\nfoo\nbar\n",
			"dokku ps:report foo": "Deployed: true\nRunning: true\nStatus web 1 running\n",
			"dokku ps:report bar": "Deployed: true\nRunning: true\nStatus web 1 running\n",
		},
		ErrorMap: map[string]error{},
	}

	app := newAppsListTestApp(t, &config.Config{Users: []config.User{{
		Username: "octo",
		Role:     config.RoleUser,
		Apps:     []string{"foo"},
		Routes:   []string{"/api/apps/list/detailed"},
	}}}, runner)

	resp := performAppsListRequest(t, app, "/api/apps/list/detailed", loginAppsUser(t, app, "octo", config.RoleUser))
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var payload struct {
		Apps []map[string]interface{} `json:"apps"`
	}
	decodeAppsListResponse(t, resp, &payload)
	require.Len(t, payload.Apps, 1)
	assert.Equal(t, "foo", payload.Apps[0]["name"])
}

func TestHandleAppsListAllowsScopedServiceUserAndReturnsEmptyFilteredSet(t *testing.T) {
	runner := &dokku.MockCommandRunner{
		OutputMap: map[string]string{
			"dokku apps:list": "=====> My Apps\nfoo\nbar\n",
		},
		ErrorMap: map[string]error{},
	}

	app := newAppsListTestApp(t, &config.Config{Users: []config.User{{
		Username: "octo",
		Role:     config.RoleUser,
		Services: map[string][]string{
			"postgres": {"db1"},
		},
	}}}, runner)

	resp := performAppsListRequest(t, app, "/api/apps/list", loginAppsUser(t, app, "octo", config.RoleUser))
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var payload struct {
		Apps []string `json:"apps"`
	}
	decodeAppsListResponse(t, resp, &payload)
	assert.Empty(t, payload.Apps)
}

func newAppsListTestApp(t *testing.T, cfg *config.Config, runner dokku.CommandRunner) *fiber.App {
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

	originalAppService := appService
	appService = appsvc.NewService(runner)
	t.Cleanup(func() {
		appService = originalAppService
	})

	app := fiber.New()
	registerAppsListTestLoginRoute(app)
	app.Use(config.ConfigMiddleware(cfg))
	app.Use(middleware.Auth())
	SetupAppsRoutes(app)

	return app
}

func registerAppsListTestLoginRoute(app *fiber.App) {
	app.Get("/__test/login/:user/:role", func(c *fiber.Ctx) error {
		sess, err := middleware.GetStore().Get(c)
		if err != nil {
			return err
		}

		sess.Set("authenticated", true)
		sess.Set("user", c.Params("user"))
		sess.Set("username", c.Params("user"))
		sess.Set("role", c.Params("role"))
		sess.Set("auth_type", c.Params("role"))
		return sess.Save()
	})
}

func loginAppsUser(t *testing.T, app *fiber.App, user, role string) []*http.Cookie {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/__test/login/"+user+"/"+role, nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	return resp.Cookies()
}

func performAppsListRequest(t *testing.T, app *fiber.App, path string, cookies []*http.Cookie) *http.Response {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func decodeAppsListResponse(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(target))
}
