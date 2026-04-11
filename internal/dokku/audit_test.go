package dokku

import (
	"errors"
	"os"
	"testing"

	"github.com/pruvon/pruvon/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type outputAndErrorRunner struct {
	output string
	err    error
}

func (r *outputAndErrorRunner) RunCommand(command string, args ...string) (string, error) {
	return r.output, r.err
}

func (r *outputAndErrorRunner) StartPTY(command string, args ...string) (*os.File, error) {
	return nil, errors.New("not implemented")
}

func TestGetAuditStatus(t *testing.T) {
	mockRunner := &MockCommandRunner{
		OutputMap: map[string]string{
			"dokku audit:status": "plugin version: 0.2.0\ndatabase path: /var/lib/dokku/data/dokku-audit/audit.db\napplication id: 1145132356\nschema version: 3\njournal mode: wal\ntotal events: 42\npending deploy count: 1\nlast event timestamp: 2026-04-10T10:11:12Z\nlast migration timestamp: 2026-04-09T09:00:00Z\n",
		},
		ErrorMap: map[string]error{},
	}

	status, err := GetAuditStatus(mockRunner)
	require.NoError(t, err)
	assert.Equal(t, "0.2.0", status.PluginVersion)
	assert.Equal(t, "/var/lib/dokku/data/dokku-audit/audit.db", status.DatabasePath)
	assert.Equal(t, "1145132356", status.ApplicationID)
	assert.Equal(t, "3", status.SchemaVersion)
	assert.Equal(t, "wal", status.JournalMode)
	assert.Equal(t, 42, status.TotalEvents)
	assert.Equal(t, 1, status.PendingDeployCount)
	assert.Equal(t, "2026-04-10T10:11:12Z", status.LastEventTimestamp)
	assert.Equal(t, "2026-04-09T09:00:00Z", status.LastMigrationTime)
}

func TestGetAuditDoctorParsesIssueOutput(t *testing.T) {
	mockRunner := &outputAndErrorRunner{
		output: "ok: sqlite3 executable available\nissue: stale pending deploy rows detected: 1\n",
		err:    errors.New("doctor reported issues"),
	}

	doctor, err := GetAuditDoctor(mockRunner)
	require.NoError(t, err)
	assert.False(t, doctor.Healthy)
	assert.Equal(t, []string{"sqlite3 executable available"}, doctor.OK)
	assert.Equal(t, []string{"stale pending deploy rows detected: 1"}, doctor.Issues)
}

func TestGetAuditEvent(t *testing.T) {
	mockRunner := &MockCommandRunner{
		OutputMap: map[string]string{
			"dokku audit:show 21 --format json": `{"id":21,"ts":"2026-04-10T12:00:00Z","app":"demo","category":"config","action":"set","status":"success","classification":"","source_trigger":"manual","source_type":"","image_tag":"","rev":"","actor_type":"user","actor_name":"emre","actor_label":"sudo-user:emre","correlation_id":"","message":"config keys set: DATABASE_URL","meta":{"triggered_by_command":"dokku config:set demo DATABASE_URL=[REDACTED]"},"created_at":"2026-04-10T12:00:00Z"}`,
		},
		ErrorMap: map[string]error{},
	}

	event, err := GetAuditEvent(mockRunner, "21")
	require.NoError(t, err)
	assert.Equal(t, 21, event.ID)
	assert.Equal(t, "demo", event.App)
	assert.Equal(t, "config", event.Category)
}

func TestGetAuditRecent(t *testing.T) {
	mockRunner := &MockCommandRunner{
		OutputMap: map[string]string{
			"dokku audit:recent --limit 5 --format json --category deploy --classification source_deploy --status success --since 2026-04-01T00:00:00Z": `[{"id":1,"ts":"2026-04-10T12:00:00Z","app":"demo","category":"deploy","action":"finish","status":"success","classification":"source_deploy","source_trigger":"manual","source_type":"git","image_tag":"dokku/demo:latest","rev":"abc123","actor_type":"user","actor_name":"emre","actor_label":"sudo-user:emre","correlation_id":"corr-1","message":"source deploy finished","meta":{"triggered_by_command":"dokku ps:rebuild demo"},"created_at":"2026-04-10T12:00:00Z"}]`,
		},
		ErrorMap: map[string]error{},
	}

	events, err := GetAuditRecent(mockRunner, 5, "deploy", "source_deploy", "success", "2026-04-01T00:00:00Z")
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, 1, events[0].ID)
	assert.Equal(t, "demo", events[0].App)
	assert.Equal(t, "corr-1", events[0].CorrelationID)
	assert.Equal(t, "dokku ps:rebuild demo", events[0].Meta["triggered_by_command"])
}

func TestFilterAuditEventsByApps(t *testing.T) {
	events := []models.AuditEvent{
		{ID: 1, App: "alpha"},
		{ID: 2, App: "beta"},
		{ID: 3, App: "alpha"},
	}

	filtered := FilterAuditEventsByApps(events, map[string]bool{"alpha": true}, 10)
	require.Len(t, filtered, 2)
	assert.Equal(t, 1, filtered[0].ID)
	assert.Equal(t, 3, filtered[1].ID)
}

func TestGroupAuditEventsByCorrelation(t *testing.T) {
	events := []models.AuditEvent{
		{ID: 4, Timestamp: "2026-04-10T12:03:00Z", App: "demo", Category: "deploy", Action: "finish", Status: "success", Classification: "source_deploy", CorrelationID: "corr-2", ActorLabel: "sudo-user:emre"},
		{ID: 3, Timestamp: "2026-04-10T12:02:00Z", App: "demo", Category: "deploy", Action: "post-extract", CorrelationID: "corr-2"},
		{ID: 2, Timestamp: "2026-04-10T11:01:00Z", App: "demo", Category: "deploy", Action: "finish", Status: "success", Classification: "release_only", CorrelationID: "corr-1"},
		{ID: 1, Timestamp: "2026-04-10T11:00:00Z", App: "demo", Category: "deploy", Action: "receive-app", CorrelationID: "corr-1"},
	}

	flows := GroupAuditEventsByCorrelation(events)
	require.Len(t, flows, 2)
	assert.Equal(t, "corr-2", flows[0].CorrelationID)
	assert.Equal(t, "source_deploy", flows[0].Classification)
	assert.Equal(t, "2026-04-10T12:02:00Z", flows[0].StartedAt)
	assert.Equal(t, "2026-04-10T12:03:00Z", flows[0].FinishedAt)
	require.Len(t, flows[0].Events, 2)
	assert.Equal(t, []int{3, 4}, []int{flows[0].Events[0].ID, flows[0].Events[1].ID})
	assert.Equal(t, "corr-1", flows[1].CorrelationID)
}
