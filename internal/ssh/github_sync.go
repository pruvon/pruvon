package ssh

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/pruvon/pruvon/internal/config"
	execpkg "github.com/pruvon/pruvon/internal/exec"
)

const managedGitHubKeyPrefix = "pruvon-gh-"

type GitHubSyncUserSummary struct {
	Username       string `json:"username"`
	GitHubUsername string `json:"github_username"`
	Added          int    `json:"added"`
	Removed        int    `json:"removed"`
}

type GitHubSyncSkippedUser struct {
	Username string `json:"username"`
	Reason   string `json:"reason"`
}

type GitHubSyncFailedUser struct {
	Username       string `json:"username"`
	GitHubUsername string `json:"github_username"`
	Error          string `json:"error"`
}

type GitHubSyncResult struct {
	Success      bool                    `json:"success"`
	SyncedUsers  []GitHubSyncUserSummary `json:"synced_users"`
	SkippedUsers []GitHubSyncSkippedUser `json:"skipped_users"`
	FailedUsers  []GitHubSyncFailedUser  `json:"failed_users"`
	AddedKeys    int                     `json:"added_keys"`
	RemovedKeys  int                     `json:"removed_keys"`
}

type HTTPGetter interface {
	Get(url string) (*http.Response, error)
}

type AuthorizedKeysReader func(path string) ([]SSHKey, error)

func SyncGitHubKeys(users []config.User, authorizedKeysPath string, runner execpkg.CommandRunner, httpClient HTTPGetter) (GitHubSyncResult, error) {
	return SyncGitHubKeysWithReader(users, authorizedKeysPath, runner, httpClient, ReadAuthorizedKeys)
}

func SyncGitHubKeysWithReader(users []config.User, authorizedKeysPath string, runner execpkg.CommandRunner, httpClient HTTPGetter, readAuthorizedKeys AuthorizedKeysReader) (GitHubSyncResult, error) {
	result := GitHubSyncResult{Success: true}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	type syncTarget struct {
		Username       string
		GitHubUsername string
		TargetKeyData  map[string]string
	}

	existingKeys, err := readAuthorizedKeys(authorizedKeysPath)
	if err != nil {
		return result, err
	}

	existingByKeyData := make(map[string]SSHKey, len(existingKeys))
	managedKeysByUser := make(map[string]map[string]SSHKey)
	managedUsers := make(map[string]bool)
	failedUsers := make(map[string]bool)
	desiredKeyData := make(map[string]bool)
	canonicalOwners := make(map[string]string)
	syncTargets := make([]syncTarget, 0, len(users))
	for _, existingKey := range existingKeys {
		existingByKeyData[existingKey.KeyData] = existingKey
		if localUsername, ok := managedKeyOwner(existingKey.Name); ok {
			managedUsers[localUsername] = true
			if managedKeysByUser[localUsername] == nil {
				managedKeysByUser[localUsername] = make(map[string]SSHKey)
			}
			managedKeysByUser[localUsername][existingKey.KeyData] = existingKey
		}
	}

	configuredGitHubUsers := make(map[string]bool)
	for _, user := range users {
		if user.Disabled {
			result.SkippedUsers = append(result.SkippedUsers, GitHubSyncSkippedUser{Username: user.Username, Reason: "user_disabled"})
			continue
		}
		if user.GitHub == nil || strings.TrimSpace(user.GitHub.Username) == "" {
			result.SkippedUsers = append(result.SkippedUsers, GitHubSyncSkippedUser{Username: user.Username, Reason: "github_username_missing"})
			continue
		}

		configuredGitHubUsers[user.Username] = true
		githubUsername := strings.TrimSpace(user.GitHub.Username)
		keys, fetchErr := fetchGitHubPublicKeys(httpClient, githubUsername)
		if fetchErr != nil {
			failedUsers[user.Username] = true
			result.Success = false
			result.FailedUsers = append(result.FailedUsers, GitHubSyncFailedUser{
				Username:       user.Username,
				GitHubUsername: githubUsername,
				Error:          fetchErr.Error(),
			})
			continue
		}

		targetKeyData := make(map[string]string)
		for _, key := range keys {
			trimmed := strings.TrimSpace(key)
			if !IsValidSSHKey(trimmed) {
				continue
			}
			parts := strings.Fields(trimmed)
			if len(parts) < 2 {
				continue
			}
			targetKeyData[parts[1]] = trimmed
			desiredKeyData[parts[1]] = true
			if owner, exists := canonicalOwners[parts[1]]; !exists || user.Username < owner {
				canonicalOwners[parts[1]] = user.Username
			}
		}

		syncTargets = append(syncTargets, syncTarget{
			Username:       user.Username,
			GitHubUsername: githubUsername,
			TargetKeyData:  targetKeyData,
		})
	}

	sort.Slice(syncTargets, func(i, j int) bool {
		return syncTargets[i].Username < syncTargets[j].Username
	})

	for _, target := range syncTargets {
		summary := GitHubSyncUserSummary{
			Username:       target.Username,
			GitHubUsername: target.GitHubUsername,
		}
		canonicalTargetKeyData := make(map[string]string)
		for keyData, keyLine := range target.TargetKeyData {
			if canonicalOwners[keyData] == target.Username {
				canonicalTargetKeyData[keyData] = keyLine
			}
		}

		for keyData, keyLine := range canonicalTargetKeyData {
			if existingKey, exists := existingByKeyData[keyData]; exists {
				if localUsername, ok := managedKeyOwner(existingKey.Name); ok {
					if localUsername == target.Username {
						if managedKeysByUser[target.Username] == nil {
							managedKeysByUser[target.Username] = make(map[string]SSHKey)
						}
						managedKeysByUser[target.Username][keyData] = existingKey
						continue
					}
					if failedUsers[localUsername] {
						continue
					}
					if _, err := runner.RunCommand("dokku", "ssh-keys:remove", existingKey.Name); err != nil {
						result.Success = false
						result.FailedUsers = append(result.FailedUsers, GitHubSyncFailedUser{
							Username:       target.Username,
							GitHubUsername: target.GitHubUsername,
							Error:          err.Error(),
						})
						continue
					}
					if managedKeysByUser[localUsername] != nil {
						delete(managedKeysByUser[localUsername], keyData)
						if len(managedKeysByUser[localUsername]) == 0 {
							delete(managedKeysByUser, localUsername)
						}
					}
					delete(existingByKeyData, keyData)
					summary.Removed++
					result.RemovedKeys++
				} else {
					continue
				}
			}

			if _, exists := existingByKeyData[keyData]; exists {
				continue
			}
			keyName := managedKeyName(target.Username, keyData)
			if err := addSSHKey(runner, keyName, keyLine); err != nil {
				result.Success = false
				result.FailedUsers = append(result.FailedUsers, GitHubSyncFailedUser{
					Username:       target.Username,
					GitHubUsername: target.GitHubUsername,
					Error:          err.Error(),
				})
				continue
			}
			existingKey := SSHKey{
				Name:    keyName,
				KeyType: strings.Fields(keyLine)[0],
				KeyData: keyData,
			}
			existingByKeyData[keyData] = existingKey
			if managedKeysByUser[target.Username] == nil {
				managedKeysByUser[target.Username] = make(map[string]SSHKey)
			}
			managedKeysByUser[target.Username][keyData] = existingKey
			managedUsers[target.Username] = true
			summary.Added++
			result.AddedKeys++
		}

		for keyData, existingKey := range managedKeysByUser[target.Username] {
			if _, keep := canonicalTargetKeyData[keyData]; keep {
				continue
			}
			if _, err := runner.RunCommand("dokku", "ssh-keys:remove", existingKey.Name); err != nil {
				result.Success = false
				result.FailedUsers = append(result.FailedUsers, GitHubSyncFailedUser{
					Username:       target.Username,
					GitHubUsername: target.GitHubUsername,
					Error:          err.Error(),
				})
				continue
			}
			delete(existingByKeyData, keyData)
			delete(managedKeysByUser[target.Username], keyData)
			summary.Removed++
			result.RemovedKeys++
		}

		result.SyncedUsers = append(result.SyncedUsers, summary)
	}

	for username := range managedUsers {
		if configuredGitHubUsers[username] {
			continue
		}
		for _, existingKey := range managedKeysByUser[username] {
			if desiredKeyData[existingKey.KeyData] {
				continue
			}
			if _, err := runner.RunCommand("dokku", "ssh-keys:remove", existingKey.Name); err != nil {
				result.Success = false
				result.FailedUsers = append(result.FailedUsers, GitHubSyncFailedUser{
					Username: username,
					Error:    err.Error(),
				})
				continue
			}
			result.RemovedKeys++
		}
	}

	sort.Slice(result.SyncedUsers, func(i, j int) bool {
		return result.SyncedUsers[i].Username < result.SyncedUsers[j].Username
	})
	sort.Slice(result.SkippedUsers, func(i, j int) bool {
		return result.SkippedUsers[i].Username < result.SkippedUsers[j].Username
	})
	sort.Slice(result.FailedUsers, func(i, j int) bool {
		return result.FailedUsers[i].Username < result.FailedUsers[j].Username
	})

	return result, nil
}

func fetchGitHubPublicKeys(httpClient HTTPGetter, username string) ([]string, error) {
	resp, err := httpClient.Get(fmt.Sprintf("https://github.com/%s.keys", username))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%d from GitHub", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(body), "\n")
	keys := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	return keys, nil
}

func managedKeyName(localUsername, keyData string) string {
	hash := sha256.Sum256([]byte(keyData))
	shortHash := hex.EncodeToString(hash[:])[:12]
	return fmt.Sprintf("%s%s-%s", managedGitHubKeyPrefix, localUsername, shortHash)
}

func managedKeyOwner(name string) (string, bool) {
	if !strings.HasPrefix(name, managedGitHubKeyPrefix) {
		return "", false
	}
	trimmed := strings.TrimPrefix(name, managedGitHubKeyPrefix)
	idx := strings.LastIndex(trimmed, "-")
	if idx <= 0 || idx == len(trimmed)-1 {
		return "", false
	}
	return trimmed[:idx], true
}

func addSSHKey(runner execpkg.CommandRunner, keyName, keyLine string) error {
	tmpfile, err := os.CreateTemp("", "ssh-key-*.pub")
	if err != nil {
		return err
	}
	tmpPath := tmpfile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpfile.WriteString(keyLine); err != nil {
		_ = tmpfile.Close()
		return err
	}
	if err := tmpfile.Close(); err != nil {
		return err
	}
	_, err = runner.RunCommand("dokku", "ssh-keys:add", keyName, tmpPath)
	return err
}
