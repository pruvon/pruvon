package update

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	originalWebBaseURL := githubWebBaseURL
	originalRepoOwner := githubRepoOwner
	originalRepoName := githubRepoName
	originalHTTPClient := githubHTTPClient
	originalTTL := updateCheckTTL
	originalCachedLatestRelease := cachedLatestRelease
	originalLatestReleaseCheckedAt := latestReleaseCheckedAt
	t.Cleanup(func() {
		githubAPIBaseURL = originalBaseURL
		githubWebBaseURL = originalWebBaseURL
		githubRepoOwner = originalRepoOwner
		githubRepoName = originalRepoName
		githubHTTPClient = originalHTTPClient
		updateCheckTTL = originalTTL
		cachedLatestRelease = originalCachedLatestRelease
		latestReleaseCheckedAt = originalLatestReleaseCheckedAt
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectPath := fmt.Sprintf("/%s/%s/releases/latest", githubRepoOwner, githubRepoName)
		redirectTarget := fmt.Sprintf("/%s/%s/releases/tag/v1.2.3", githubRepoOwner, githubRepoName)
		apiPath := fmt.Sprintf("/repos/%s/%s/releases/latest", githubRepoOwner, githubRepoName)

		switch r.URL.Path {
		case redirectPath:
			if githubRepoName != "pruvon" {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, redirectTarget, http.StatusFound)
		case redirectTarget:
			w.WriteHeader(http.StatusOK)
		case apiPath:
			if githubRepoName != "pruvon" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v1.2.3"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	githubAPIBaseURL = server.URL
	githubWebBaseURL = server.URL
	githubRepoOwner = "pruvon"
	githubRepoName = "pruvon"
	githubHTTPClient = server.Client()
	updateCheckTTL = 0
	cachedLatestRelease = ""
	latestReleaseCheckedAt = time.Time{}

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

	t.Run("version with v prefix", func(t *testing.T) {
		info, err := CheckForUpdates("v1.2.2")
		assert.NoError(t, err)
		assert.Equal(t, "1.2.2", info.CurrentVersion)
		assert.Equal(t, "1.2.3", info.LatestVersion)
		assert.True(t, info.UpdateAvailable)
	})

	t.Run("github error status", func(t *testing.T) {
		cachedLatestRelease = ""
		latestReleaseCheckedAt = time.Time{}
		githubRepoName = "missing"

		info, err := CheckForUpdates("1.0.0")
		assert.Error(t, err)
		assert.Equal(t, "1.0.0", info.CurrentVersion)

		githubRepoName = "pruvon"
	})

	t.Run("uses stale cached version on fetch error", func(t *testing.T) {
		cachedLatestRelease = "1.2.3"
		latestReleaseCheckedAt = time.Now().Add(-updateCheckTTL - time.Minute)
		githubRepoName = "missing"

		info, err := CheckForUpdates("1.2.2")
		assert.NoError(t, err)
		assert.Equal(t, "1.2.3", info.LatestVersion)
		assert.True(t, info.UpdateAvailable)

		githubRepoName = "pruvon"
	})
}
