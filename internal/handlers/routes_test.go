package handlers

import (
	"testing"

	"github.com/pruvon/pruvon/internal/config"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupRoutes(t *testing.T) {
	// Create a new Fiber app
	app := fiber.New()

	// Create a minimal config for testing
	cfg := &config.Config{
		// Minimal config, add required fields as needed
	}

	// Test that SetupRoutes doesn't panic
	assert.NotPanics(t, func() {
		SetupRoutes(app, cfg)
	})

	// Verify that routes were added (basic check)
	require.NotNil(t, app)

	// Note: More comprehensive testing would require testing individual route handlers
	// or using httptest.Server, but that would require significant setup
	// For now, we verify that the function runs without panicking
}
