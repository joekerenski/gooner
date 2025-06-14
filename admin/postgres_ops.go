package admin

import (
	"context"
	"runtime"
	"time"
	"fmt"

	"gooner/db"
)

func collectBaseMetricsPostgres(pool *db.DBPool, ctx context.Context) (*BaseMetrics, error) {
    tables, err := getTablesPostgres(pool, ctx)
    if err != nil {
        return nil, err
    }

    schemas, err := getSchemasPostgres(pool, ctx)
    if err != nil {
        return nil, err
    }

    features := extractFeatures(tables)

    return &BaseMetrics{
        Tables:    tables,
        Schemas:   schemas,
        Features:  features,
        Timestamp: time.Now(),
    }, nil
}

func getTablesPostgres(pool *db.DBPool, ctx context.Context) ([]Table, error) {
    query := `
        SELECT schemaname, tablename, 
               pg_total_relation_size(schemaname||'.'||tablename) as size_bytes
        FROM pg_tables 
        WHERE schemaname NOT IN ('information_schema', 'pg_catalog')
        ORDER BY schemaname, tablename`

    rows, err := pool.PgxPool.Query(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var tables []Table
    for rows.Next() {
        var schema, tableName string
        var sizeBytes int64
        
        if err := rows.Scan(&schema, &tableName, &sizeBytes); err != nil {
            return nil, err
        }

        table, err := getTableDetailsPostgres(pool, ctx, schema, tableName, sizeBytes)
        if err != nil {
            return nil, err
        }
        tables = append(tables, *table)
    }

    return tables, nil
}

func collectRealTimeMetricsPostgres(pool *db.DBPool, ctx context.Context) (*RealTimeMetrics, error) {
    var activeConns, maxConns int
    var dbSizeMB int64

    err := pool.PgxPool.QueryRow(ctx, `
        SELECT numbackends, setting::int as max_connections,
               pg_database_size(current_database())/1024/1024 as size_mb
        FROM pg_stat_database d, pg_settings s 
        WHERE d.datname = current_database() AND s.name = 'max_connections'`).
        Scan(&activeConns, &maxConns, &dbSizeMB)
    if err != nil {
        return nil, err
    }

    dbHealth := DatabaseHealth{
        ActiveConnections: activeConns,
        DatabaseSizeMB:    dbSizeMB,
    }

    // System metrics
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    systemHealth := SystemHealth{
        MemoryUsageMB:   int64(m.Alloc / 1024 / 1024),
        GoroutineCount:  runtime.NumGoroutine(),
        HeapSizeMB:      int64(m.HeapAlloc / 1024 / 1024),
    }

    return &RealTimeMetrics{
        Timestamp: time.Now(),
        Database:  dbHealth,
        System:    systemHealth,
        App:       ApplicationHealth{},
    }, nil
}

func getSchemasPostgres(pool *db.DBPool, ctx context.Context) ([]SchemaInfo, error) {
    query := `
        SELECT nspname, pg_catalog.pg_get_userbyid(nspowner) as owner, 
               (SELECT count(*) FROM pg_tables WHERE schemaname = nspname) as table_count
        FROM pg_namespace
        WHERE nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
        ORDER BY nspname
    `

    rows, err := pool.PgxPool.Query(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var schemas []SchemaInfo
    for rows.Next() {
        var schema SchemaInfo
        if err := rows.Scan(&schema.Name, &schema.Owner, &schema.Tables); err != nil {
            return nil, err
        }
        schemas = append(schemas, schema)
    }

    return schemas, nil
}

func getTableDetailsPostgres(pool *db.DBPool, ctx context.Context, schema, tableName string, sizeBytes int64) (*Table, error) {
    fullTableName := fmt.Sprintf("%s.%s", schema, tableName)
    
    // Get row count
    var rowCount int64
    err := pool.PgxPool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", fullTableName)).Scan(&rowCount)
    if err != nil {
        return nil, err
    }

    sizeMB := float64(sizeBytes) / (1024 * 1024)

    // Get field information
    fields, err := getFieldsPostgres(pool, ctx, schema, tableName)
    if err != nil {
        return nil, err
    }

    // Get indexes
    indexes, err := getIndexesPostgres(pool, ctx, schema, tableName)
    if err != nil {
        return nil, err
    }

    return &Table{
        Name:     tableName,
        Schema:   schema,
        RowCount: rowCount,
        SizeMB:   sizeMB,
        Fields:   fields,
        Indexes:  indexes,
    }, nil
}

func getFieldsPostgres(pool *db.DBPool, ctx context.Context, schema, tableName string) ([]FieldInfo, error) {
    query := `
		SELECT c.column_name, c.data_type, c.is_nullable, c.column_default,
			   CASE WHEN pk.pk_column_name IS NOT NULL THEN true ELSE false END as is_primary_key
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT ku.column_name AS pk_column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage ku ON tc.constraint_name = ku.constraint_name
			WHERE tc.constraint_type = 'PRIMARY KEY' AND tc.table_schema = $1 AND tc.table_name = $2
		) pk ON c.column_name = pk.pk_column_name
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position    
		`

    rows, err := pool.PgxPool.Query(ctx, query, schema, tableName)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var fields []FieldInfo
    for rows.Next() {
        var name, dataType, isNullable string
        var defaultValue *string
        var isPrimaryKey bool

        err := rows.Scan(&name, &dataType, &isNullable, &defaultValue, &isPrimaryKey)
        if err != nil {
            return nil, err
        }

        field := FieldInfo{
            Name:         name,
            DataType:     dataType,
            IsNullable:   isNullable == "YES",
            IsPrimaryKey: isPrimaryKey,
            DefaultValue: defaultValue,
        }

        fields = append(fields, field)
    }

    return fields, nil
}

func getIndexesPostgres(pool *db.DBPool, ctx context.Context, schema, tableName string) ([]IndexInfo, error) {
    query := `
        SELECT i.relname as index_name, 
               array_agg(a.attname ORDER BY c.ordinality) as columns,
               ix.indisunique,
               ix.indisprimary,
               pg_relation_size(i.oid)/1024/1024 as size_mb
        FROM pg_class t
        JOIN pg_index ix ON t.oid = ix.indrelid
        JOIN pg_class i ON i.oid = ix.indexrelid
        JOIN pg_namespace n ON t.relnamespace = n.oid
        JOIN unnest(ix.indkey) WITH ORDINALITY c(colnum, ordinality) ON true
        JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = c.colnum
        WHERE n.nspname = $1 AND t.relname = $2
        GROUP BY i.relname, ix.indisunique, ix.indisprimary, i.oid
        ORDER BY i.relname
    `

    rows, err := pool.PgxPool.Query(ctx, query, schema, tableName)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var indexes []IndexInfo
    for rows.Next() {
        var name string
        var columns []string
        var isUnique, isPrimary bool
        var sizeMB float64

        err := rows.Scan(&name, &columns, &isUnique, &isPrimary, &sizeMB)
        if err != nil {
            return nil, err
        }

        index := IndexInfo{
            Name:      name,
            Columns:   columns,
            IsUnique:  isUnique,
            IsPrimary: isPrimary,
            SizeMB:    sizeMB,
        }

        indexes = append(indexes, index)
    }

    return indexes, nil
}
