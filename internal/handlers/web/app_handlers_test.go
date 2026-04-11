package web

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/pruvon/pruvon/internal/templates"
)

func TestParseAppMetadata(t *testing.T) {
	tests := []struct {
		name          string
		appJSON       string
		description   string
		version       string
		repositoryURL string
	}{
		{
			name:          "string repository",
			appJSON:       `{"description":"Demo app","version":"1.2.3","repository":"https://example.com/repo.git"}`,
			description:   "Demo app",
			version:       "1.2.3",
			repositoryURL: "https://example.com/repo.git",
		},
		{
			name:          "object repository",
			appJSON:       `{"description":"Demo app","version":"1.2.3","repository":{"url":"https://example.com/repo.git"}}`,
			description:   "Demo app",
			version:       "1.2.3",
			repositoryURL: "https://example.com/repo.git",
		},
		{
			name:          "invalid json",
			appJSON:       `{"description":`,
			description:   "",
			version:       "",
			repositoryURL: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			description, version, repositoryURL := parseAppMetadata(tt.appJSON)
			if description != tt.description || version != tt.version || repositoryURL != tt.repositoryURL {
				t.Fatalf(
					"parseAppMetadata() = (%q, %q, %q), want (%q, %q, %q)",
					description,
					version,
					repositoryURL,
					tt.description,
					tt.version,
					tt.repositoryURL,
				)
			}
		})
	}
}

func TestHandleDashboardRendersWithoutDokkuCalls(t *testing.T) {
	if err := templates.Initialize(); err != nil {
		t.Fatalf("templates.Initialize failed: %v", err)
	}

	app := fiber.New()
	app.Get("/", HandleDashboard)

	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("App test failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}
