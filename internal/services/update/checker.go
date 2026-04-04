package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	githubAPIBaseURL = "https://api.github.com"
	githubRepoOwner  = "pruvon"
	githubRepoName   = "pruvon"
	githubHTTPClient = &http.Client{Timeout: 5 * time.Second}
)

// UpdateInfo contains information about version updates
type UpdateInfo struct {
	UpdateAvailable bool
	LatestVersion   string
	CurrentVersion  string
}

// CheckForUpdates checks GitHub for the latest version tag
func CheckForUpdates(currentVersion string) (UpdateInfo, error) {
	result := UpdateInfo{
		UpdateAvailable: false,
		LatestVersion:   currentVersion,
		CurrentVersion:  currentVersion,
	}

	url := fmt.Sprintf("%s/repos/%s/%s/tags",
		strings.TrimRight(githubAPIBaseURL, "/"),
		githubRepoOwner,
		githubRepoName,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return result, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Pruvon-App")

	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("GitHub API returned %d status", resp.StatusCode)
	}

	var tags []struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return result, err
	}

	if len(tags) == 0 {
		return result, nil
	}

	// Find latest version by stripping prefixes and comparing
	latestVersionStr := tags[0].Name

	// Standardize version strings (remove v prefix for comparison)
	cleanLatestVersion := strings.TrimPrefix(latestVersionStr, "v")
	cleanCurrentVersion := strings.TrimPrefix(currentVersion, "v")

	// Compare versions using proper numeric comparison
	isNewer := compareVersions(cleanLatestVersion, cleanCurrentVersion)

	if isNewer {
		result.UpdateAvailable = true
		result.LatestVersion = cleanLatestVersion // Store without v prefix
	}

	return result, nil
}

// compareVersions compares two semantic version strings
// Returns true if latest is newer than current
func compareVersions(latest, current string) bool {
	// Split version strings into components
	latestParts := strings.Split(latest, ".")
	currentParts := strings.Split(current, ".")

	// Ensure we have at least 3 parts for each version (major.minor.patch)
	for len(latestParts) < 3 {
		latestParts = append(latestParts, "0")
	}
	for len(currentParts) < 3 {
		currentParts = append(currentParts, "0")
	}

	// Compare major, minor, patch versions numerically
	for i := 0; i < 3; i++ {
		// Convert string to integer for proper numeric comparison
		latestNum, errLatest := strconv.Atoi(latestParts[i])
		currentNum, errCurrent := strconv.Atoi(currentParts[i])

		// If conversion fails, fall back to string comparison
		if errLatest != nil || errCurrent != nil {
			if latestParts[i] > currentParts[i] {
				return true
			} else if latestParts[i] < currentParts[i] {
				return false
			}
			// Equal parts, continue to next component
			continue
		}

		// Numeric comparison
		if latestNum > currentNum {
			return true
		} else if latestNum < currentNum {
			return false
		}
		// Equal, check next component
	}

	// All components are equal
	return false
}
