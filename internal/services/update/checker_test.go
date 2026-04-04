package update

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		latest   string
		current  string
		expected bool
	}{
		{
			name:     "latest is newer (major version)",
			latest:   "2.0.0",
			current:  "1.9.9",
			expected: true,
		},
		{
			name:     "latest is newer (minor version)",
			latest:   "1.2.0",
			current:  "1.1.9",
			expected: true,
		},
		{
			name:     "latest is newer (patch version)",
			latest:   "1.1.2",
			current:  "1.1.1",
			expected: true,
		},
		{
			name:     "versions are equal",
			latest:   "1.1.1",
			current:  "1.1.1",
			expected: false,
		},
		{
			name:     "current is newer",
			latest:   "1.1.0",
			current:  "1.1.1",
			expected: false,
		},
		{
			name:     "versions with v prefix",
			latest:   "v2.0.0",
			current:  "v1.9.9",
			expected: true, // Note: this tests the trimming logic in CheckForUpdates
		},
		{
			name:     "incomplete versions",
			latest:   "2.0",
			current:  "1.9",
			expected: true,
		},
		{
			name:     "non-numeric versions",
			latest:   "2.0a",
			current:  "1.9b",
			expected: true, // string comparison fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.latest, tt.current)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCheckForUpdates(t *testing.T) {
	originalBaseURL := githubAPIBaseURL
	originalRepoOwner := githubRepoOwner
	originalRepoName := githubRepoName
	originalHTTPClient := githubHTTPClient
	t.Cleanup(func() {
		githubAPIBaseURL = originalBaseURL
		githubRepoOwner = originalRepoOwner
		githubRepoName = originalRepoName
		githubHTTPClient = originalHTTPClient
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf("/repos/%s/%s/tags", githubRepoOwner, githubRepoName)
		if r.URL.Path != expectedPath {
			http.NotFound(w, r)
			return
		}

		if githubRepoName != "pruvon" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"v1.2.3"},{"name":"v1.0.0"}]`))
	}))
	defer server.Close()

	githubAPIBaseURL = server.URL
	githubRepoOwner = "pruvon"
	githubRepoName = "pruvon"
	githubHTTPClient = server.Client()

	t.Run("current version provided", func(t *testing.T) {
		info, err := CheckForUpdates("1.0.0")
		assert.NoError(t, err)
		assert.Equal(t, "1.0.0", info.CurrentVersion)
		assert.Equal(t, "1.2.3", info.LatestVersion)
		assert.True(t, info.UpdateAvailable)
	})

	t.Run("empty version", func(t *testing.T) {
		info, err := CheckForUpdates("")
		assert.NoError(t, err)
		assert.Equal(t, "", info.CurrentVersion)
		assert.True(t, info.UpdateAvailable)
		assert.Equal(t, "1.2.3", info.LatestVersion)
	})

	t.Run("github error status", func(t *testing.T) {
		githubRepoName = "missing"

		info, err := CheckForUpdates("1.0.0")
		assert.Error(t, err)
		assert.Equal(t, "1.0.0", info.CurrentVersion)

		githubRepoName = "pruvon"
	})
}
