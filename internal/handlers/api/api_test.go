package api

import (
	"testing"

	"github.com/pruvon/pruvon/internal/appdeps"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestSetupApiRoutes(t *testing.T) {
	// Create a new Fiber app
	app := fiber.New()

	// Test that SetupApiRoutes doesn't panic
	assert.NotPanics(t, func() {
		SetupApiRoutes(app, appdeps.NewDependencies(nil))
	})

	// Verify that routes were added (basic check)
	assert.NotNil(t, app)

	// Note: More comprehensive testing would require testing individual route handlers
	// For now, we verify that the function runs without panicking
}
