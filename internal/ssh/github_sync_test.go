package ssh

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/pruvon/pruvon/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncGitHubKeysWithReader_SkipsUsersWithoutGitHubMetadata(t *testing.T) {
	result, err := SyncGitHubKeysWithReader([]config.User{{
		Username: "admin",
		Role:     config.RoleAdmin,
	}}, "/tmp/authorized_keys", &recordingRunner{}, mockHTTPGetter{}, func(string) ([]SSHKey, error) {
		return nil, nil
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	require.Len(t, result.SkippedUsers, 1)
	assert.Equal(t, GitHubSyncSkippedUser{Username: "admin", Reason: "github_username_missing"}, result.SkippedUsers[0])
}

func TestSyncGitHubKeysWithReader_PartialFailureKeepsOtherUsersSyncing(t *testing.T) {
	runner := &recordingRunner{}
	result, err := SyncGitHubKeysWithReader([]config.User{
		{Username: "broken", GitHub: &config.UserGitHub{Username: "broken-gh"}},
		{Username: "ops", GitHub: &config.UserGitHub{Username: "ops-gh"}},
	}, "/tmp/authorized_keys", runner, mockHTTPGetter{
		responses: map[string]mockHTTPResponse{
			"https://github.com/broken-gh.keys": {err: errors.New("boom")},
			"https://github.com/ops-gh.keys": {
				statusCode: http.StatusOK,
				body:       "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGM0zE8z5l4BzYw4R6Yx2b5A7Q4b2a1n3m4o5p6q7r8 user@example.com\n",
			},
		},
	}, func(string) ([]SSHKey, error) {
		return nil, nil
	})
	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.FailedUsers, 1)
	assert.Equal(t, "broken", result.FailedUsers[0].Username)
	require.Len(t, result.SyncedUsers, 1)
	assert.Equal(t, "ops", result.SyncedUsers[0].Username)
	assert.Equal(t, 1, result.AddedKeys)
	assert.Contains(t, strings.Join(runner.commands, "\n"), "dokku ssh-keys:add pruvon-gh-ops-")
}

func TestSyncGitHubKeysWithReader_AdminUserWithGitHubMetadataSyncs(t *testing.T) {
	runner := &recordingRunner{}
	result, err := SyncGitHubKeysWithReader([]config.User{{
		Username: "admin",
		Role:     config.RoleAdmin,
		GitHub:   &config.UserGitHub{Username: "admin-gh"},
	}}, "/tmp/authorized_keys", runner, mockHTTPGetter{
		responses: map[string]mockHTTPResponse{
			"https://github.com/admin-gh.keys": {
				statusCode: http.StatusOK,
				body:       "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGM0zE8z5l4BzYw4R6Yx2b5A7Q4b2a1n3m4o5p6q7r8 admin@example.com\n",
			},
		},
	}, func(string) ([]SSHKey, error) {
		return nil, nil
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	require.Len(t, result.SyncedUsers, 1)
	assert.Equal(t, "admin", result.SyncedUsers[0].Username)
	assert.Equal(t, "admin-gh", result.SyncedUsers[0].GitHubUsername)
	assert.Equal(t, 1, result.SyncedUsers[0].Added)
	assert.Empty(t, result.SkippedUsers)
	assert.Contains(t, strings.Join(runner.commands, "\n"), "dokku ssh-keys:add pruvon-gh-admin-")
}

func TestSyncGitHubKeysWithReader_DoesNotAddDuplicateKeyMaterial(t *testing.T) {
	runner := &recordingRunner{}
	result, err := SyncGitHubKeysWithReader([]config.User{{
		Username: "ops",
		GitHub:   &config.UserGitHub{Username: "ops-gh"},
	}}, "/tmp/authorized_keys", runner, mockHTTPGetter{
		responses: map[string]mockHTTPResponse{
			"https://github.com/ops-gh.keys": {
				statusCode: http.StatusOK,
				body:       "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGM0zE8z5l4BzYw4R6Yx2b5A7Q4b2a1n3m4o5p6q7r8 user@example.com\n",
			},
		},
	}, func(string) ([]SSHKey, error) {
		return []SSHKey{{
			Name:    "manual-key",
			KeyType: "ssh-ed25519",
			KeyData: "AAAAC3NzaC1lZDI1NTE5AAAAIGM0zE8z5l4BzYw4R6Yx2b5A7Q4b2a1n3m4o5p6q7r8",
		}}, nil
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.AddedKeys)
	assert.Equal(t, 0, result.RemovedKeys)
	for _, command := range runner.commands {
		assert.NotContains(t, command, "dokku ssh-keys:add ")
	}
}

func TestSyncGitHubKeysWithReader_DoesNotReAddDuplicateKeyMaterialAcrossUsersInSameRun(t *testing.T) {
	runner := &recordingRunner{}
	keyLine := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGM0zE8z5l4BzYw4R6Yx2b5A7Q4b2a1n3m4o5p6q7r8 shared@example.com"

	result, err := SyncGitHubKeysWithReader([]config.User{
		{Username: "ops-a", GitHub: &config.UserGitHub{Username: "shared-a"}},
		{Username: "ops-b", GitHub: &config.UserGitHub{Username: "shared-b"}},
	}, "/tmp/authorized_keys", runner, mockHTTPGetter{
		responses: map[string]mockHTTPResponse{
			"https://github.com/shared-a.keys": {statusCode: http.StatusOK, body: keyLine + "\n"},
			"https://github.com/shared-b.keys": {statusCode: http.StatusOK, body: keyLine + "\n"},
		},
	}, func(string) ([]SSHKey, error) {
		return nil, nil
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.AddedKeys)
	assert.Empty(t, result.FailedUsers)
	require.Len(t, runner.commands, 1)
	assert.Contains(t, runner.commands[0], "dokku ssh-keys:add pruvon-gh-ops-a-")
}

func TestSyncGitHubKeysWithReader_TransfersSharedManagedKeyToCanonicalOwnerWhenAnotherConfiguredUserStillNeedsIt(t *testing.T) {
	runner := &recordingRunner{}
	keyData := "AAAAC3NzaC1lZDI1NTE5AAAAIGM0zE8z5l4BzYw4R6Yx2b5A7Q4b2a1n3m4o5p6q7r8"
	keyLine := "ssh-ed25519 " + keyData + " shared@example.com"
	staleKeyName := managedKeyName("ops-a", keyData)
	canonicalKeyName := managedKeyName("ops-b", keyData)

	result, err := SyncGitHubKeysWithReader([]config.User{{
		Username: "ops-b",
		GitHub:   &config.UserGitHub{Username: "shared-b"},
	}}, "/tmp/authorized_keys", runner, mockHTTPGetter{
		responses: map[string]mockHTTPResponse{
			"https://github.com/shared-b.keys": {statusCode: http.StatusOK, body: keyLine + "\n"},
		},
	}, func(string) ([]SSHKey, error) {
		return []SSHKey{{
			Name:    staleKeyName,
			KeyType: "ssh-ed25519",
			KeyData: keyData,
		}}, nil
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.RemovedKeys)
	assert.Equal(t, 1, result.AddedKeys)
	assert.Empty(t, result.FailedUsers)
	assert.Contains(t, runner.commands, "dokku ssh-keys:remove "+staleKeyName)
	assert.Contains(t, strings.Join(runner.commands, "\n"), "dokku ssh-keys:add "+canonicalKeyName)
}

func TestSyncGitHubKeysWithReader_TransfersSharedManagedKeyToCurrentOwner(t *testing.T) {
	runner := &recordingRunner{}
	keyData := "AAAAC3NzaC1lZDI1NTE5AAAAIGM0zE8z5l4BzYw4R6Yx2b5A7Q4b2a1n3m4o5p6q7r8"
	keyLine := "ssh-ed25519 " + keyData + " shared@example.com"
	staleKeyName := managedKeyName("ops-b", keyData)
	currentKeyName := managedKeyName("ops-a", keyData)

	result, err := SyncGitHubKeysWithReader([]config.User{{
		Username: "ops-a",
		GitHub:   &config.UserGitHub{Username: "shared-a"},
	}}, "/tmp/authorized_keys", runner, mockHTTPGetter{
		responses: map[string]mockHTTPResponse{
			"https://github.com/shared-a.keys": {statusCode: http.StatusOK, body: keyLine + "\n"},
		},
	}, func(string) ([]SSHKey, error) {
		return []SSHKey{{
			Name:    staleKeyName,
			KeyType: "ssh-ed25519",
			KeyData: keyData,
		}}, nil
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.RemovedKeys)
	assert.Equal(t, 1, result.AddedKeys)
	require.Len(t, result.SyncedUsers, 1)
	assert.Equal(t, 1, result.SyncedUsers[0].Removed)
	assert.Equal(t, 1, result.SyncedUsers[0].Added)
	assert.Contains(t, runner.commands, "dokku ssh-keys:remove "+staleKeyName)
	assert.Contains(t, strings.Join(runner.commands, "\n"), "dokku ssh-keys:add "+currentKeyName)
}

func TestSyncGitHubKeysWithReader_CleansUpRemovedGitHubMetadataWithHyphenatedUsername(t *testing.T) {
	runner := &recordingRunner{outputs: map[string]string{
		"dokku ssh-keys:remove pruvon-gh-ops-team-482053b4f822": "removed",
	}}
	result, err := SyncGitHubKeysWithReader(nil, "/tmp/authorized_keys", runner, mockHTTPGetter{}, func(string) ([]SSHKey, error) {
		return []SSHKey{{
			Name:    "pruvon-gh-ops-team-482053b4f822",
			KeyType: "ssh-ed25519",
			KeyData: "AAAAC3NzaC1lZDI1NTE5AAAAIGM0zE8z5l4BzYw4R6Yx2b5A7Q4b2a1n3m4o5p6q7r8",
		}}, nil
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.RemovedKeys)
	require.Contains(t, runner.commands, "dokku ssh-keys:remove pruvon-gh-ops-team-482053b4f822")
}

func TestSyncGitHubKeysWithReader_PreservesManualKeysDuringCleanup(t *testing.T) {
	runner := &recordingRunner{}
	result, err := SyncGitHubKeysWithReader(nil, "/tmp/authorized_keys", runner, mockHTTPGetter{}, func(string) ([]SSHKey, error) {
		return []SSHKey{
			{
				Name:    "manual-key",
				KeyType: "ssh-ed25519",
				KeyData: "manualdata",
			},
			{
				Name:    "pruvon-gh-ops-team-482053b4f822",
				KeyType: "ssh-ed25519",
				KeyData: "manageddata",
			},
		}, nil
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.RemovedKeys)
	require.Len(t, runner.commands, 1)
	assert.Equal(t, "dokku ssh-keys:remove pruvon-gh-ops-team-482053b4f822", runner.commands[0])
}

func TestManagedKeyOwner(t *testing.T) {
	owner, ok := managedKeyOwner("pruvon-gh-ops-team-482053b4f822")
	require.True(t, ok)
	assert.Equal(t, "ops-team", owner)
}

func TestManagedKeyName_IsDeterministic(t *testing.T) {
	first := managedKeyName("ops-team", "AAAAC3NzaC1lZDI1NTE5AAAAIGM0zE8z5l4BzYw4R6Yx2b5A7Q4b2a1n3m4o5p6q7r8")
	second := managedKeyName("ops-team", "AAAAC3NzaC1lZDI1NTE5AAAAIGM0zE8z5l4BzYw4R6Yx2b5A7Q4b2a1n3m4o5p6q7r8")
	assert.Equal(t, first, second)
	assert.True(t, strings.HasPrefix(first, "pruvon-gh-ops-team-"))
	assert.Len(t, strings.TrimPrefix(first, "pruvon-gh-ops-team-"), 12)
}

type mockHTTPGetter struct {
	responses map[string]mockHTTPResponse
}

type mockHTTPResponse struct {
	statusCode int
	body       string
	err        error
}

func (m mockHTTPGetter) Get(url string) (*http.Response, error) {
	response, ok := m.responses[url]
	if !ok {
		return nil, errors.New("unexpected url: " + url)
	}
	if response.err != nil {
		return nil, response.err
	}
	return &http.Response{
		StatusCode: response.statusCode,
		Body:       io.NopCloser(strings.NewReader(response.body)),
	}, nil
}

type recordingRunner struct {
	outputs  map[string]string
	commands []string
}

func (r *recordingRunner) RunCommand(command string, args ...string) (string, error) {
	full := command + " " + strings.Join(args, " ")
	r.commands = append(r.commands, full)
	if output, ok := r.outputs[full]; ok {
		return output, nil
	}
	return "", nil
}

func (r *recordingRunner) StartPTY(command string, args ...string) (*os.File, error) {
	return nil, errors.New("not implemented")
}
