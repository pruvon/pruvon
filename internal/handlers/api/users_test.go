package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

func TestUsersAPI_CreateUpdatePasswordDelete(t *testing.T) {
	app, cfg := newUsersTestApp(t)

	createResp := usersAPIRequest(t, app, http.MethodPost, "/api/settings/users", `{"username":"operator","password":"Secret1!","role":"user","github":{"username":"octocat"}}`)
	if createResp.StatusCode != fiber.StatusOK {
		t.Fatalf("create user returned %d", createResp.StatusCode)
	}
	createdUser := cfg.FindUser("operator")
	if createdUser == nil {
		t.Fatal("expected created user in config")
	}
	if createdUser.Password == "Secret1!" || bcrypt.CompareHashAndPassword([]byte(createdUser.Password), []byte("Secret1!")) != nil {
		t.Fatal("expected stored password to be bcrypt hashed")
	}

	updateResp := usersAPIRequest(t, app, http.MethodPut, "/api/settings/users/operator", `{"username":"ops","role":"user","routes":["/apps/*"],"apps":["foo"],"services":{"postgres":["db1"]},"github":{"username":"octocat"},"disabled":false}`)
	if updateResp.StatusCode != fiber.StatusOK {
		t.Fatalf("update user returned %d", updateResp.StatusCode)
	}
	if cfg.FindUser("operator") != nil {
		t.Fatal("expected old username to be renamed")
	}
	renamedUser := cfg.FindUser("ops")
	if renamedUser == nil || renamedUser.Password == "" {
		t.Fatal("expected renamed user to preserve password hash")
	}

	passwordResp := usersAPIRequest(t, app, http.MethodPut, "/api/settings/users/ops/password", `{"password":"NewSecret1!"}`)
	if passwordResp.StatusCode != fiber.StatusOK {
		t.Fatalf("password update returned %d", passwordResp.StatusCode)
	}
	if bcrypt.CompareHashAndPassword([]byte(cfg.FindUser("ops").Password), []byte("NewSecret1!")) != nil {
		t.Fatal("expected updated password hash to match new password")
	}

	deleteResp := usersAPIRequest(t, app, http.MethodDelete, "/api/settings/users/ops", "")
	if deleteResp.StatusCode != fiber.StatusOK {
		t.Fatalf("delete user returned %d", deleteResp.StatusCode)
	}
	if cfg.FindUser("ops") != nil {
		t.Fatal("expected user to be deleted")
	}
}

func TestUsersAPI_RejectCurrentUserDeleteDisableAndLastAdminRemoval(t *testing.T) {
	app, _ := newUsersTestApp(t)

	deleteResp := usersAPIRequest(t, app, http.MethodDelete, "/api/settings/users/admin", "")
	if deleteResp.StatusCode != fiber.StatusConflict {
		t.Fatalf("self delete returned %d", deleteResp.StatusCode)
	}

	disableResp := usersAPIRequest(t, app, http.MethodPut, "/api/settings/users/admin", `{"username":"admin","role":"admin","routes":[],"apps":[],"services":{},"github":{"username":""},"disabled":true}`)
	if disableResp.StatusCode != fiber.StatusConflict {
		t.Fatalf("self disable returned %d", disableResp.StatusCode)
	}

	demoteResp := usersAPIRequest(t, app, http.MethodPut, "/api/settings/users/admin", `{"username":"admin","role":"user","routes":[],"apps":[],"services":{},"github":{"username":""},"disabled":false}`)
	if demoteResp.StatusCode != fiber.StatusConflict {
		t.Fatalf("last admin demotion returned %d", demoteResp.StatusCode)
	}
}

func TestUsersAPI_RejectsWeakPasswords(t *testing.T) {
	app, _ := newUsersTestApp(t)

	createResp := usersAPIRequest(t, app, http.MethodPost, "/api/settings/users", `{"username":"operator","password":"weakpass","role":"user"}`)
	if createResp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("weak create password returned %d", createResp.StatusCode)
	}
	createBody := mustUsersBody(t, createResp)
	if !strings.Contains(createBody, "at least 8 characters") {
		t.Fatalf("expected strong password error, got %s", createBody)
	}

	passwordResp := usersAPIRequest(t, app, http.MethodPut, "/api/settings/users/admin/password", `{"password":"weakpass"}`)
	if passwordResp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("weak password update returned %d", passwordResp.StatusCode)
	}
	passwordBody := mustUsersBody(t, passwordResp)
	if !strings.Contains(passwordBody, "at least 8 characters") {
		t.Fatalf("expected strong password error, got %s", passwordBody)
	}
}

func TestUsersAPI_GetUsersDoesNotLeakPasswordHashes(t *testing.T) {
	app, _ := newUsersTestApp(t)

	resp := usersAPIRequest(t, app, http.MethodGet, "/api/settings/users", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("get users returned %d", resp.StatusCode)
	}
	body, _ := ioReadAll(resp.Body)
	if strings.Contains(string(body), "$2a$") || strings.Contains(string(body), "\"password\"") {
		t.Fatalf("expected password hashes to be hidden, got: %s", string(body))
	}

	var data struct {
		Users []struct {
			Username    string `json:"username"`
			HasPassword bool   `json:"has_password"`
		} `json:"users"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("failed to decode get users response: %v", err)
	}
	if len(data.Users) != 1 || !data.Users[0].HasPassword {
		t.Fatalf("unexpected get users response: %+v", data.Users)
	}
}

func TestUsersAPI_RejectScopedUserAccess(t *testing.T) {
	app, _ := newUsersScopedTestApp(t, config.User{
		Username: "operator",
		Role:     config.RoleUser,
		Routes:   []string{"/api/settings/users", "/api/settings/user-options"},
	})

	for _, path := range []string{"/api/settings/users", "/api/settings/user-options"} {
		resp := usersAPIRequest(t, app, http.MethodGet, path, "")
		if resp.StatusCode != fiber.StatusForbidden {
			t.Fatalf("GET %s returned %d", path, resp.StatusCode)
		}
	}
}

func TestUsersAPI_FailedSaveDoesNotMutateRuntimeConfig(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		app, cfg, cfgPath := newUsersTestAppWithConfigPath(t)
		makeConfigSaveFail(t, cfgPath)

		resp := usersAPIRequest(t, app, http.MethodPost, "/api/settings/users", `{"username":"operator","password":"Secret1!","role":"user"}`)
		if resp.StatusCode != fiber.StatusInternalServerError {
			t.Fatalf("create user returned %d", resp.StatusCode)
		}
		if cfg.FindUser("operator") != nil {
			t.Fatal("expected create failure to leave runtime config unchanged")
		}
	})

	t.Run("update", func(t *testing.T) {
		app, cfg, cfgPath := newUsersTestAppWithConfigPath(t, config.User{
			Username: "operator",
			Password: mustHashPassword(t, "secret"),
			Role:     config.RoleUser,
		})
		makeConfigSaveFail(t, cfgPath)

		resp := usersAPIRequest(t, app, http.MethodPut, "/api/settings/users/operator", `{"username":"ops","role":"user","routes":["/apps/*"],"apps":["foo"],"services":{},"github":{"username":"octocat"},"disabled":false}`)
		if resp.StatusCode != fiber.StatusInternalServerError {
			t.Fatalf("update user returned %d", resp.StatusCode)
		}
		if cfg.FindUser("ops") != nil {
			t.Fatal("expected update failure to avoid renaming runtime user")
		}
		user := cfg.FindUser("operator")
		if user == nil {
			t.Fatal("expected original user to remain after failed update")
		}
		if len(user.Apps) != 0 || user.GitHub != nil {
			t.Fatalf("expected original user scopes to remain unchanged, got %#v", user)
		}
	})

	t.Run("password", func(t *testing.T) {
		app, cfg, cfgPath := newUsersTestAppWithConfigPath(t, config.User{
			Username: "operator",
			Password: mustHashPassword(t, "secret"),
			Role:     config.RoleUser,
		})
		makeConfigSaveFail(t, cfgPath)
		originalHash := cfg.FindUser("operator").Password

		resp := usersAPIRequest(t, app, http.MethodPut, "/api/settings/users/operator/password", `{"password":"NewSecret1!"}`)
		if resp.StatusCode != fiber.StatusInternalServerError {
			t.Fatalf("password update returned %d", resp.StatusCode)
		}
		if cfg.FindUser("operator").Password != originalHash {
			t.Fatal("expected failed password save to keep original runtime hash")
		}
	})

	t.Run("delete", func(t *testing.T) {
		app, cfg, cfgPath := newUsersTestAppWithConfigPath(t, config.User{
			Username: "operator",
			Password: mustHashPassword(t, "secret"),
			Role:     config.RoleUser,
		})
		makeConfigSaveFail(t, cfgPath)

		resp := usersAPIRequest(t, app, http.MethodDelete, "/api/settings/users/operator", "")
		if resp.StatusCode != fiber.StatusInternalServerError {
			t.Fatalf("delete user returned %d", resp.StatusCode)
		}
		if cfg.FindUser("operator") == nil {
			t.Fatal("expected delete failure to leave runtime user in place")
		}
	})
}

func newUsersTestApp(t *testing.T) (*fiber.App, *config.Config) {
	t.Helper()
	app, cfg, _ := newUsersScopedTestAppWithConfigPath(t, config.User{
		Username: "admin",
		Role:     config.RoleAdmin,
	})
	return app, cfg
}

func newUsersScopedTestApp(t *testing.T, sessionUser config.User) (*fiber.App, *config.Config) {
	t.Helper()
	app, cfg, _ := newUsersScopedTestAppWithConfigPath(t, sessionUser)
	return app, cfg
}

func newUsersTestAppWithConfigPath(t *testing.T, extraUsers ...config.User) (*fiber.App, *config.Config, string) {
	t.Helper()
	return newUsersScopedTestAppWithConfigPath(t, config.User{
		Username: "admin",
		Role:     config.RoleAdmin,
	}, extraUsers...)
}

func newUsersScopedTestAppWithConfigPath(t *testing.T, sessionUser config.User, extraUsers ...config.User) (*fiber.App, *config.Config, string) {
	t.Helper()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "pruvon.yml")
	cfg := &config.Config{
		Users: []config.User{
			{
				Username: "admin",
				Password: mustHashPassword(t, "secret"),
				Role:     config.RoleAdmin,
			},
		},
	}
	if sessionUser.Username != "" && sessionUser.Username != "admin" {
		cfg.Users = append(cfg.Users, sessionUser)
	}
	for _, extraUser := range extraUsers {
		if extraUser.Username == "" || cfg.FindUser(extraUser.Username) != nil {
			continue
		}
		cfg.Users = append(cfg.Users, extraUser)
	}
	config.UpdateConfig(cfg)
	if err := config.WriteConfigFile(cfgPath, cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	if _, err := config.LoadConfig(cfgPath); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	middleware.GetStore()
	app := fiber.New()
	app.Use(config.ConfigMiddleware(config.GetConfig()))
	app.Use(func(c *fiber.Ctx) error {
		sess, err := middleware.GetStore().Get(c)
		if err != nil {
			return err
		}
		role := sessionUser.Role
		if role == "" {
			role = config.RoleAdmin
		}
		username := sessionUser.Username
		if username == "" {
			username = "admin"
		}
		sess.Set("authenticated", true)
		sess.Set("username", username)
		sess.Set("user", username)
		sess.Set("role", role)
		sess.Set("auth_type", role)
		if err := sess.Save(); err != nil {
			return err
		}
		return c.Next()
	})
	SetupUsersRoutes(app)
	return app, config.GetConfig(), cfgPath
}

func usersAPIRequest(t *testing.T, app *fiber.App, method, path, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if method != http.MethodGet && method != http.MethodDelete {
		req.Header.Set("Content-Type", "application/json")
	}
	// Fiber defaults app.Test to 1s, which is too tight under -race when bcrypt runs in the handler.
	resp, err := app.Test(req, 10_000)
	if err != nil {
		t.Fatalf("request %s %s failed: %v", method, path, err)
	}
	return resp
}

func ioReadAll(body io.ReadCloser) ([]byte, error) {
	defer body.Close()
	return io.ReadAll(body)
}

func mustUsersBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := ioReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	return string(body)
}

func mustHashPassword(t *testing.T, password string) string {
	t.Helper()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(hashedPassword)
}

func makeConfigSaveFail(t *testing.T, cfgPath string) {
	t.Helper()
	dir := filepath.Dir(cfgPath)
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatalf("failed to chmod config directory: %v", err)
	}
	if err := os.Chmod(cfgPath, 0400); err != nil {
		t.Fatalf("failed to chmod config file: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0700)
		_ = os.Chmod(cfgPath, 0600)
	})
}
