package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

var (
	githubAPIBaseURL = "https://api.github.com"
	githubWebBaseURL = "https://github.com"
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
	currentVersion = normalizeVersion(currentVersion)

	result := UpdateInfo{
		UpdateAvailable: false,
		LatestVersion:   currentVersion,
		CurrentVersion:  currentVersion,
	}

	latestVersion, err := latestReleaseVersion()
	if err != nil {
		return result, err
	}

	result.LatestVersion = latestVersion

	// Compare versions using proper numeric comparison
	isNewer := compareVersions(latestVersion, currentVersion)

	if isNewer {
		result.UpdateAvailable = true
	}

	return result, nil
}

func latestReleaseVersion() (string, error) {
	latestVersion, err := latestReleaseVersionFromRedirect()
	if err == nil {
		return latestVersion, nil
	}

	latestVersion, err = latestReleaseVersionFromAPI()
	if err == nil {
		return latestVersion, nil
	}

	return "", err
}

func latestReleaseVersionFromRedirect() (string, error) {
	url := fmt.Sprintf("%s/%s/%s/releases/latest",
		strings.TrimRight(githubWebBaseURL, "/"),
		githubRepoOwner,
		githubRepoName,
	)

	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Pruvon-App")

	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.Request == nil || resp.Request.URL == nil {
		return "", fmt.Errorf("could not resolve latest release redirect")
	}

	resolvedVersion := normalizeVersion(path.Base(resp.Request.URL.Path))
	if resolvedVersion == "" || resolvedVersion == "latest" {
		return "", fmt.Errorf("could not resolve latest release version from redirect")
	}

	return resolvedVersion, nil
}

func latestReleaseVersionFromAPI() (string, error) {

	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest",
		strings.TrimRight(githubAPIBaseURL, "/"),
		githubRepoOwner,
		githubRepoName,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Pruvon-App")

	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("GitHub API returned %d status", resp.StatusCode)
		return "", err
	}

	var release struct {
		TagName string `json:"tag_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	latestVersion := normalizeVersion(release.TagName)
	return latestVersion, nil
}
func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")
	return version
}

// compareVersions compares two semantic version strings
// Returns true if latest is newer than current
func compareVersions(latest, current string) bool {
	latest = normalizeVersion(latest)
	current = normalizeVersion(current)

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
