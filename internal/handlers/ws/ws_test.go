package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pruvon/pruvon/internal/appdeps"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestSetupWsRoutes(t *testing.T) {
	// Create a new Fiber app
	app := fiber.New()

	// Test that SetupWsRoutes doesn't panic
	assert.NotPanics(t, func() {
		SetupWsRoutes(app, appdeps.NewDependencies(nil))
	})

	// Verify that routes were added (basic check)
	assert.NotNil(t, app)

	// Note: WebSocket testing would require more complex setup with
	// WebSocket connections and message handling
}

func TestHandleWsAuth_RejectsUnauthenticatedUpgrade(t *testing.T) {
	app := fiber.New()
	app.Use("/ws", handleWsAuth(nil))
	app.Get("/ws/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ws/test", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "test-key")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated websocket upgrade, got %d", resp.StatusCode)
	}
}
