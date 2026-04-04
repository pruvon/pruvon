package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/templates"
	"io"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// HandleLogin handles the login page
func HandleLogin(c *fiber.Ctx) error {
	// Login sayfası için templates dizinindeki login.html şablonunu kullan
	tmpl, err := templates.GetTemplate("login.html")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template load error: "+err.Error())
	}

	// Get config for GitHub credentials
	cfg := GetConfig()

	// Şablona gönderilecek verileri hazırla
	data := fiber.Map{
		"Error":              c.Query("error"),
		"HideNavigation":     true,
		"User":               nil,
		"AuthType":           nil,
		"GitHubClientID":     cfg.GitHub.ClientID,
		"GitHubClientSecret": cfg.GitHub.ClientSecret,
	}

	// Şablonu çalıştır
	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "base.html", data); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template execute error: "+err.Error())
	}

	// Yanıtı döndür
	return c.Type("html").SendString(out.String())
}

// HandleLoginAPI handles the login API endpoint
func HandleLoginAPI(c *fiber.Ctx) error {
	// Get username and password from form data
	username := c.FormValue("username")
	password := c.FormValue("password")

	if username == "" || password == "" {
		// Try to parse from JSON if form values not found
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&req); err == nil {
			username = req.Username
			password = req.Password
		}
	}

	// Validate credentials
	if username != GetConfig().Admin.Username || !middleware.ComparePasswords(GetConfig().Admin.Password, password) {
		// Log failed login attempt
		if strings.Contains(c.Get("Accept"), "application/json") {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
		}
		return c.Redirect("/login?error=Invalid+credentials")
	}

	sess, err := middleware.GetStore().Get(c)
	if err != nil {
		if strings.Contains(c.Get("Accept"), "application/json") {
			return c.Status(500).JSON(fiber.Map{"error": "Session error"})
		}
		return c.Redirect("/login?error=Session+error")
	}

	sess.Set("authenticated", true)
	sess.Set("user", username)
	sess.Set("username", username)
	sess.Set("auth_type", "admin")
	if err := sess.Save(); err != nil {
		if strings.Contains(c.Get("Accept"), "application/json") {
			return c.Status(500).JSON(fiber.Map{"error": "Could not save session"})
		}
		return c.Redirect("/login?error=Could+not+save+session")
	}

	// Respond based on request type
	if strings.Contains(c.Get("Accept"), "application/json") {
		return c.JSON(fiber.Map{"success": true})
	}
	return c.Redirect("/")
}

// HandleLogout handles the logout endpoint
func HandleLogout(c *fiber.Ctx) error {
	sess, err := middleware.GetStore().Get(c)
	if err == nil {
		// Log logout only if there was an active session
		_ = sess.Destroy()
	}
	return c.Redirect("/login")
}

// HandleGithubAuth handles the GitHub authentication endpoint
func HandleGithubAuth(c *fiber.Ctx) error {
	state := uuid.NewString()
	sess, _ := middleware.GetStore().Get(c)
	sess.Set("oauth_state", state)
	_ = sess.Save()

	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&state=%s",
		GetConfig().GitHub.ClientID,
		state,
	)
	return c.Redirect(authURL)
}

// HandleGithubCallback handles the GitHub callback endpoint
func HandleGithubCallback(c *fiber.Ctx) error {
	sess, err := middleware.GetStore().Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to load session")
	}

	state, ok := sess.Get("oauth_state").(string)
	if !ok || state == "" || state != c.Query("state") {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid state")
	}

	// Exchange code for access token
	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing code")
	}

	req, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(
		fmt.Sprintf("client_id=%s&client_secret=%s&code=%s",
			GetConfig().GitHub.ClientID,
			GetConfig().GitHub.ClientSecret,
			code,
		),
	))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to get access token")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.Status(fiber.StatusBadGateway).SendString("GitHub token request failed: " + readGitHubError(resp))
	}

	var tokenData struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenData); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to parse token response: " + err.Error())
	}
	if tokenData.AccessToken == "" {
		return c.Status(fiber.StatusBadGateway).SendString("GitHub token request failed: missing access token")
	}

	// Get user info from GitHub
	userReq, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	userReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
	userReq.Header.Set("Accept", "application/json")

	userResp, err := http.DefaultClient.Do(userReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to get user info")
	}
	defer userResp.Body.Close()
	if userResp.StatusCode != http.StatusOK {
		return c.Status(fiber.StatusBadGateway).SendString("GitHub user request failed: " + readGitHubError(userResp))
	}

	var userData struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(userResp.Body).Decode(&userData); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to parse user info")
	}

	// Check if user is allowed
	allowed := false
	cfg := GetConfig()
	if cfg != nil && cfg.GitHub.Users != nil {
		for _, user := range cfg.GitHub.Users {
			if user.Username == userData.Login {
				allowed = true
				break
			}
		}
	}

	if !allowed {
		_ = middleware.SetFlashMessage(c, "GitHub user not authorized. Please contact administrator.", "error")
		return c.Redirect("/login")
	}

	// Set session
	sess.Delete("oauth_state")
	sess.Set("authenticated", true)
	sess.Set("username", userData.Login)
	sess.Set("user", userData.Login) // Add this line - set both username and user
	sess.Set("auth_type", "github")
	_ = sess.Save()

	return c.Redirect("/")
}

func readGitHubError(resp *http.Response) string {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return resp.Status
	}

	message := strings.TrimSpace(string(body))
	if message == "" {
		return resp.Status
	}

	return message
}
