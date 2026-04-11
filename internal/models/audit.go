package models

type AuditStatus struct {
	PluginVersion      string `json:"plugin_version"`
	DatabasePath       string `json:"database_path"`
	ApplicationID      string `json:"application_id"`
	SchemaVersion      string `json:"schema_version"`
	JournalMode        string `json:"journal_mode"`
	TotalEvents        int    `json:"total_events"`
	PendingDeployCount int    `json:"pending_deploy_count"`
	LastEventTimestamp string `json:"last_event_timestamp"`
	LastMigrationTime  string `json:"last_migration_timestamp"`
}

type AuditDoctor struct {
	OK      []string `json:"ok"`
	Issues  []string `json:"issues"`
	Healthy bool     `json:"healthy"`
}

type AuditEvent struct {
	ID             int                    `json:"id"`
	Timestamp      string                 `json:"ts"`
	App            string                 `json:"app"`
	Category       string                 `json:"category"`
	Action         string                 `json:"action"`
	Status         string                 `json:"status"`
	Classification string                 `json:"classification"`
	SourceTrigger  string                 `json:"source_trigger"`
	SourceType     string                 `json:"source_type"`
	ImageTag       string                 `json:"image_tag"`
	Revision       string                 `json:"rev"`
	ActorType      string                 `json:"actor_type"`
	ActorName      string                 `json:"actor_name"`
	ActorLabel     string                 `json:"actor_label"`
	CorrelationID  string                 `json:"correlation_id"`
	Message        string                 `json:"message"`
	Meta           map[string]interface{} `json:"meta"`
	CreatedAt      string                 `json:"created_at"`
}

type AuditDeployFlow struct {
	CorrelationID  string       `json:"correlation_id"`
	App            string       `json:"app"`
	Status         string       `json:"status"`
	Classification string       `json:"classification"`
	ActorLabel     string       `json:"actor_label"`
	StartedAt      string       `json:"started_at"`
	FinishedAt     string       `json:"finished_at"`
	SourceType     string       `json:"source_type"`
	ImageTag       string       `json:"image_tag"`
	Revision       string       `json:"revision"`
	Events         []AuditEvent `json:"events"`
}

type AuditOverview struct {
	Enabled         bool         `json:"enabled"`
	PluginInstalled bool         `json:"plugin_installed"`
	Status          *AuditStatus `json:"status,omitempty"`
	Doctor          *AuditDoctor `json:"doctor,omitempty"`
	Recent          []AuditEvent `json:"recent"`
	Deploys         []AuditEvent `json:"deploys"`
}

type AppAuditDetails struct {
	Enabled       bool              `json:"enabled"`
	Timeline      []AuditEvent      `json:"timeline"`
	Deploys       []AuditEvent      `json:"deploys"`
	DeployFlows   []AuditDeployFlow `json:"deploy_flows"`
	ConfigChanges []AuditEvent      `json:"config_changes"`
	DomainChanges []AuditEvent      `json:"domain_changes"`
	PortChanges   []AuditEvent      `json:"port_changes"`
	ProblemEvents []AuditEvent      `json:"problem_events"`
}

type ServiceAuditDetails struct {
	Enabled    bool         `json:"enabled"`
	LinkedApps []string     `json:"linked_apps"`
	Recent     []AuditEvent `json:"recent"`
	Deploys    []AuditEvent `json:"deploys"`
}
