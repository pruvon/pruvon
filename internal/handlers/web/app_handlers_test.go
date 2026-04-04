package web

import "testing"

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
