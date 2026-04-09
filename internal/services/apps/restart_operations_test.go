package apps

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sequenceResult struct {
	output string
	err    error
}

type sequenceCommandRunner struct {
	mu      sync.Mutex
	results map[string][]sequenceResult
	calls   map[string]int
}

func (r *sequenceCommandRunner) RunCommand(command string, args ...string) (string, error) {
	key := command + " " + strings.Join(args, " ")

	r.mu.Lock()
	defer r.mu.Unlock()

	results, ok := r.results[key]
	if !ok || len(results) == 0 {
		return "", errors.New("command not mocked: " + key)
	}

	index := r.calls[key]
	if index >= len(results) {
		index = len(results) - 1
	}
	r.calls[key]++

	return results[index].output, results[index].err
}

func (r *sequenceCommandRunner) StartPTY(command string, args ...string) (*os.File, error) {
	return nil, errors.New("PTY not supported in mock")
}

func TestIsRestartVerified(t *testing.T) {
	t.Run("returns false for nil status", func(t *testing.T) {
		assert.False(t, isRestartVerified(nil, ""))
	})

	t.Run("returns false when app is not deployed", func(t *testing.T) {
		status := &models.AppStatus{Deployed: false, Running: true}
		assert.False(t, isRestartVerified(status, ""))
	})

	t.Run("accepts running app for full restart", func(t *testing.T) {
		status := &models.AppStatus{Deployed: true, Running: true}
		assert.True(t, isRestartVerified(status, ""))
	})

	t.Run("accepts mixed running status for full restart", func(t *testing.T) {
		status := &models.AppStatus{Deployed: true, Running: "mixed"}
		assert.True(t, isRestartVerified(status, ""))
	})

	t.Run("requires process count for targeted restart", func(t *testing.T) {
		status := &models.AppStatus{
			Deployed:  true,
			Running:   true,
			Processes: map[string]int{"web": 1, "worker": 0},
		}

		assert.True(t, isRestartVerified(status, "web"))
		assert.False(t, isRestartVerified(status, "worker"))
	})
}

func TestRestartOperationStoreLatestAndCleanup(t *testing.T) {
	store := newRestartOperationStore()
	now := time.Date(2026, time.April, 9, 12, 0, 0, 0, time.UTC)
	finishedAt := now
	store.now = func() time.Time { return now }

	operation := models.AppRestartOperation{
		ID:         "op-1",
		AppName:    "demo-app",
		Action:     "restart",
		State:      RestartOperationStateSucceeded,
		Message:    "done",
		StartedAt:  now,
		FinishedAt: &finishedAt,
	}

	store.operations[operation.ID] = operation
	store.latestByApp[operation.AppName] = operation.ID

	latest, ok := store.latest("demo-app")
	require.True(t, ok)
	assert.Equal(t, operation.ID, latest.ID)

	now = now.Add(restartOperationTTL + time.Second)
	_, ok = store.latest("demo-app")
	assert.False(t, ok)
	assert.Empty(t, store.operations)
	assert.Empty(t, store.latestByApp)
}

func TestRestartOperationStoreStartReusesActiveOperation(t *testing.T) {
	store := newRestartOperationStore()
	now := time.Date(2026, time.April, 9, 12, 0, 0, 0, time.UTC)

	operation := models.AppRestartOperation{
		ID:        "op-active",
		AppName:   "demo-app",
		Action:    "restart",
		State:     RestartOperationStateRestarting,
		Message:   "Dokku is restarting the application.",
		StartedAt: now,
	}

	store.operations[operation.ID] = operation
	store.latestByApp[operation.AppName] = operation.ID

	reused := store.start(nil, operation.AppName, "")
	assert.Equal(t, operation.ID, reused.ID)
	assert.True(t, reused.Reused)
	assert.False(t, store.operations[operation.ID].Reused)
}

func TestWaitForRestartVerificationFailsFastWhenAppIsDeleted(t *testing.T) {
	runner := &sequenceCommandRunner{
		results: map[string][]sequenceResult{
			"dokku ps:report demo-app": {
				{err: errors.New("ps report failed")},
			},
			"dokku apps:list": {
				{output: "=====> My Apps\nother-app"},
			},
		},
		calls: make(map[string]int),
	}

	service := NewService(runner)
	store := newRestartOperationStore()
	store.verificationTimeout = 10 * time.Second
	store.verificationInterval = time.Millisecond

	err := store.waitForRestartVerification(service, "demo-app", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no longer exists")
}

func TestWaitForRestartVerificationSucceedsAfterTransientStatusError(t *testing.T) {
	runner := &sequenceCommandRunner{
		results: map[string][]sequenceResult{
			"dokku ps:report demo-app": {
				{err: errors.New("temporary failure")},
				{output: "Deployed: true\nRunning: true\nStatus web 1 running"},
			},
			"dokku apps:list": {
				{output: "=====> My Apps\ndemo-app"},
			},
		},
		calls: make(map[string]int),
	}

	service := NewService(runner)
	store := newRestartOperationStore()
	store.verificationTimeout = 10 * time.Second
	store.verificationInterval = time.Millisecond

	err := store.waitForRestartVerification(service, "demo-app", "")
	assert.NoError(t, err)
}

var _ dokku.CommandRunner = (*sequenceCommandRunner)(nil)
