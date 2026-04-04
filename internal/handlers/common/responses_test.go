package common

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func setupTestApp() *fiber.App {
	return fiber.New()
}

func TestErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
		details    []interface{}
		wantStatus int
		wantBody   map[string]interface{}
	}{
		{
			name:       "Simple error without details",
			statusCode: fiber.StatusBadRequest,
			message:    "Invalid request",
			details:    nil,
			wantStatus: fiber.StatusBadRequest,
			wantBody: map[string]interface{}{
				"error": "Invalid request",
			},
		},
		{
			name:       "Error with single detail",
			statusCode: fiber.StatusNotFound,
			message:    "Resource not found",
			details:    []interface{}{"User ID 123 does not exist"},
			wantStatus: fiber.StatusNotFound,
			wantBody: map[string]interface{}{
				"error":   "Resource not found",
				"details": "User ID 123 does not exist",
			},
		},
		{
			name:       "Error with multiple details",
			statusCode: fiber.StatusInternalServerError,
			message:    "Server error",
			details:    []interface{}{"Database connection failed", "Retry after 5 seconds"},
			wantStatus: fiber.StatusInternalServerError,
			wantBody: map[string]interface{}{
				"error": "Server error",
				"details": []interface{}{
					"Database connection failed",
					"Retry after 5 seconds",
				},
			},
		},
		{
			name:       "Unauthorized error",
			statusCode: fiber.StatusUnauthorized,
			message:    "Authentication required",
			details:    nil,
			wantStatus: fiber.StatusUnauthorized,
			wantBody: map[string]interface{}{
				"error": "Authentication required",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp()

			app.Get("/test", func(c *fiber.Ctx) error {
				return ErrorResponse(c, tt.statusCode, tt.message, tt.details...)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := app.Test(req)
			assert.NoError(t, err)
			defer resp.Body.Close()

			// Check status code
			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			// Check response body
			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)

			var gotBody map[string]interface{}
			err = json.Unmarshal(body, &gotBody)
			assert.NoError(t, err)

			// Compare error message
			assert.Equal(t, tt.wantBody["error"], gotBody["error"])

			// Compare details if present
			if tt.wantBody["details"] != nil {
				assert.Equal(t, tt.wantBody["details"], gotBody["details"])
			} else {
				assert.Nil(t, gotBody["details"])
			}
		})
	}
}

func TestSuccessResponse(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		wantBody map[string]interface{}
	}{
		{
			name: "Success with string data",
			data: "Operation completed successfully",
			wantBody: map[string]interface{}{
				"data": "Operation completed successfully",
			},
		},
		{
			name: "Success with map data",
			data: map[string]interface{}{
				"id":   123,
				"name": "Test User",
			},
			wantBody: map[string]interface{}{
				"data": map[string]interface{}{
					"id":   float64(123), // JSON unmarshals numbers as float64
					"name": "Test User",
				},
			},
		},
		{
			name: "Success with array data",
			data: []string{"item1", "item2", "item3"},
			wantBody: map[string]interface{}{
				"data": []interface{}{"item1", "item2", "item3"},
			},
		},
		{
			name: "Success with nil data",
			data: nil,
			wantBody: map[string]interface{}{
				"data": nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp()

			app.Get("/test", func(c *fiber.Ctx) error {
				return SuccessResponse(c, tt.data)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := app.Test(req)
			assert.NoError(t, err)
			defer resp.Body.Close()

			// Check status code
			assert.Equal(t, fiber.StatusOK, resp.StatusCode)

			// Check response body
			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)

			var gotBody map[string]interface{}
			err = json.Unmarshal(body, &gotBody)
			assert.NoError(t, err)

			assert.Equal(t, tt.wantBody["data"], gotBody["data"])
		})
	}
}

func TestValidationErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		errors     map[string]string
		wantStatus int
		wantBody   map[string]interface{}
	}{
		{
			name: "Single validation error",
			errors: map[string]string{
				"email": "Email is required",
			},
			wantStatus: fiber.StatusUnprocessableEntity,
			wantBody: map[string]interface{}{
				"error": "validation_error",
				"details": map[string]interface{}{
					"email": "Email is required",
				},
			},
		},
		{
			name: "Multiple validation errors",
			errors: map[string]string{
				"email":    "Email is required",
				"password": "Password must be at least 8 characters",
				"username": "Username already exists",
			},
			wantStatus: fiber.StatusUnprocessableEntity,
			wantBody: map[string]interface{}{
				"error": "validation_error",
				"details": map[string]interface{}{
					"email":    "Email is required",
					"password": "Password must be at least 8 characters",
					"username": "Username already exists",
				},
			},
		},
		{
			name:       "Empty validation errors",
			errors:     map[string]string{},
			wantStatus: fiber.StatusUnprocessableEntity,
			wantBody: map[string]interface{}{
				"error":   "validation_error",
				"details": map[string]interface{}{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp()

			app.Get("/test", func(c *fiber.Ctx) error {
				return ValidationErrorResponse(c, tt.errors)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := app.Test(req)
			assert.NoError(t, err)
			defer resp.Body.Close()

			// Check status code
			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			// Check response body
			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)

			var gotBody map[string]interface{}
			err = json.Unmarshal(body, &gotBody)
			assert.NoError(t, err)

			assert.Equal(t, tt.wantBody["error"], gotBody["error"])
			assert.Equal(t, tt.wantBody["details"], gotBody["details"])
		})
	}
}

func BenchmarkErrorResponse(b *testing.B) {
	app := setupTestApp()
	app.Get("/test", func(c *fiber.Ctx) error {
		return ErrorResponse(c, fiber.StatusBadRequest, "Test error", "Test detail")
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		_, _ = app.Test(req)
	}
}

func BenchmarkSuccessResponse(b *testing.B) {
	app := setupTestApp()
	app.Get("/test", func(c *fiber.Ctx) error {
		return SuccessResponse(c, map[string]string{"status": "ok"})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		_, _ = app.Test(req)
	}
}

func BenchmarkValidationErrorResponse(b *testing.B) {
	app := setupTestApp()
	app.Get("/test", func(c *fiber.Ctx) error {
		return ValidationErrorResponse(c, map[string]string{
			"field1": "error1",
			"field2": "error2",
		})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		_, _ = app.Test(req)
	}
}
