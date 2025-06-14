package admin

import (
    "time"
)

type BaseMetrics struct {
    Tables    []Table         `json:"tables"`
    Schemas   []SchemaInfo    `json:"schemas,omitempty"` // PostgreSQL only
    Features  []FeatureInfo   `json:"features"`
    Timestamp time.Time       `json:"timestamp"`
}

type Table struct {
    Name              string            `json:"name"`
    Schema            string            `json:"schema,omitempty"` // PostgreSQL only
    RowCount          int64             `json:"row_count"`
    SizeMB            float64           `json:"size_mb"`
    Fields            []FieldInfo       `json:"fields"`
    Indexes           []IndexInfo       `json:"indexes"`
    Constraints       []ConstraintInfo  `json:"constraints"`
    Triggers          []TriggerInfo     `json:"triggers,omitempty"`
    Permissions       []PermissionInfo  `json:"permissions,omitempty"` // PostgreSQL only
    LastModified      *time.Time        `json:"last_modified,omitempty"`
    AutoVacuumEnabled bool              `json:"auto_vacuum_enabled,omitempty"` // SQLite only
}

type FieldInfo struct {
    Name         string  `json:"name"`
    DataType     string  `json:"data_type"`
    IsNullable   bool    `json:"is_nullable"`
    DefaultValue *string `json:"default_value,omitempty"`
    IsPrimaryKey bool    `json:"is_primary_key"`
    IsForeignKey bool    `json:"is_foreign_key"`
    References   *string `json:"references,omitempty"` // "table.column" format
    MaxLength    *int    `json:"max_length,omitempty"`
    IsUnique     bool    `json:"is_unique"`
}

type IndexInfo struct {
    Name      string   `json:"name"`
    Columns   []string `json:"columns"`
    IsUnique  bool     `json:"is_unique"`
    IsPrimary bool     `json:"is_primary"`
    SizeMB    float64  `json:"size_mb"`
}

type ConstraintInfo struct {
    Name       string `json:"name"`
    Type       string `json:"type"` // PRIMARY, FOREIGN, UNIQUE, CHECK
    Definition string `json:"definition"`
}

type TriggerInfo struct {
    Name      string `json:"name"`
    Event     string `json:"event"`     // INSERT, UPDATE, DELETE
    Timing    string `json:"timing"`    // BEFORE, AFTER
    Statement string `json:"statement"`
}

type PermissionInfo struct {
    Role       string   `json:"role"`
    Privileges []string `json:"privileges"` // SELECT, INSERT, UPDATE, DELETE
}

type SchemaInfo struct {
    Name   string `json:"name"`
    Owner  string `json:"owner,omitempty"`
    Tables int    `json:"table_count"`
}

type FeatureInfo struct {
    Name         string    `json:"name"`
    TablePrefix  string    `json:"table_prefix"`
    TableCount   int       `json:"table_count"`
    IsActive     bool      `json:"is_active"`
    LastActivity *time.Time `json:"last_activity,omitempty"`
}

type RealTimeMetrics struct {
    Timestamp time.Time         `json:"timestamp"`
    Database  DatabaseHealth    `json:"database"`
    Jobs      JobSystemHealth   `json:"jobs,omitempty"`      // PostgreSQL only
    System    SystemHealth      `json:"system"`
    App       ApplicationHealth `json:"app"`
}

type DatabaseHealth struct {
    ActiveConnections   int     `json:"active_connections"`
    AvgQueryLatencyMs   float64 `json:"avg_query_latency_ms"`
    CurrentTransactions int     `json:"current_transactions"`
    LockWaits          int     `json:"lock_waits,omitempty"`    // PostgreSQL only
    DatabaseSizeMB     int64   `json:"database_size_mb"`
    CacheHitRatio      float64 `json:"cache_hit_ratio,omitempty"` // PostgreSQL only
    WALSizeMB          int64   `json:"wal_size_mb,omitempty"`     // PostgreSQL only
    VacuumRunning      bool    `json:"vacuum_running,omitempty"`  // PostgreSQL only
}

type JobSystemHealth struct {
    PendingJobs           int     `json:"pending_jobs"`
    RunningJobs           int     `json:"running_jobs"`
    FailedJobsLast10Min   int     `json:"failed_jobs_recent"`
    AvgExecutionLatencyMs float64 `json:"job_execution_latency_ms"`
    ActiveWorkers         int     `json:"active_workers"`
    QueueDepth            int     `json:"queue_depth"`
}

type SystemHealth struct {
    MemoryUsageMB        int64   `json:"memory_usage_mb"`
    CPUUsagePercent      float64 `json:"cpu_usage_percent"`
    WebSocketConnections int     `json:"websocket_connections"`
    GoroutineCount       int     `json:"goroutine_count"`
    HeapSizeMB           int64   `json:"heap_size_mb"`
}

type ApplicationHealth struct {
    ActiveSessions    int     `json:"active_sessions"`
    APIRequestsPerMin int     `json:"api_request_rate_per_min"`
    ErrorsPerMin      int     `json:"error_rate_per_min"`
    UptimeSeconds     int64   `json:"uptime_seconds"`
    AvgResponseTimeMs float64 `json:"avg_response_time_ms"`
}
