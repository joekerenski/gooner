package admin

import (
    "time"
    "database/sql"
	"runtime"
	"context"
	"fmt"

	"gooner/db"
)

func collectBaseMetricsSQLite(pool *db.DBPool, ctx context.Context) (*BaseMetrics, error) {
    rdb, err := pool.GetReadTx(ctx)
    if err != nil {
        return nil, err
    }
    defer rdb.Rollback()

    tables, err := getTablesSQLite(rdb)
    if err != nil {
        return nil, err
    }

    features := extractFeatures(tables)

    return &BaseMetrics{
        Tables:    tables,
        Features:  features,
        Timestamp: time.Now(),
    }, nil
}

func getTablesSQLite(rdb *db.RequestDB) ([]Table, error) {
    query := `
        SELECT name FROM sqlite_master 
        WHERE type='table' AND name NOT LIKE 'sqlite_%'
        ORDER BY name`
    
    rows, err := rdb.Query(query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var tables []Table
    for rows.Next() {
        var tableName string
        if err := rows.Scan(&tableName); err != nil {
            return nil, err
        }

        table, err := getTableDetailsSQLite(rdb, tableName)
        if err != nil {
            return nil, err
        }
        tables = append(tables, *table)
    }

    return tables, nil
}

func getTableDetailsSQLite(rdb *db.RequestDB, tableName string) (*Table, error) {
    // Get row count
    var rowCount int64
    err := rdb.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&rowCount)
    if err != nil {
        return nil, err
    }

    // Get table size (approximate)
    var pageCount, pageSize int64
    err = rdb.QueryRow("PRAGMA page_count").Scan(&pageCount)
    if err != nil {
        return nil, err
    }
    err = rdb.QueryRow("PRAGMA page_size").Scan(&pageSize)
    if err != nil {
        return nil, err
    }
    sizeMB := float64(pageCount*pageSize) / (1024 * 1024)

    // Get field information
    fields, err := getFieldsSQLite(rdb, tableName)
    if err != nil {
        return nil, err
    }

    // Get indexes
    indexes, err := getIndexesSQLite(rdb, tableName)
    if err != nil {
        return nil, err
    }

    return &Table{
        Name:     tableName,
        RowCount: rowCount,
        SizeMB:   sizeMB,
        Fields:   fields,
        Indexes:  indexes,
    }, nil
}

func getFieldsSQLite(rdb *db.RequestDB, tableName string) ([]FieldInfo, error) {
    query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
    rows, err := rdb.Query(query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var fields []FieldInfo
    for rows.Next() {
        var cid int
        var name, dataType string
        var notNull, pk int
        var defaultValue sql.NullString

        err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
        if err != nil {
            return nil, err
        }

        field := FieldInfo{
            Name:         name,
            DataType:     dataType,
            IsNullable:   notNull == 0,
            IsPrimaryKey: pk == 1,
        }

        if defaultValue.Valid {
            field.DefaultValue = &defaultValue.String
        }

        fields = append(fields, field)
    }

    return fields, nil
}

func getIndexesSQLite(rdb *db.RequestDB, tableName string) ([]IndexInfo, error) {
    query := `
        SELECT name, sql FROM sqlite_master 
        WHERE type='index' AND tbl_name=? AND name NOT LIKE 'sqlite_%'`
    
    rows, err := rdb.Query(query, tableName)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var indexes []IndexInfo
    for rows.Next() {
        var name string
        var sql sql.NullString
        
        if err := rows.Scan(&name, &sql); err != nil {
            return nil, err
        }

        index := IndexInfo{
            Name:     name,
            IsUnique: false, // Would need to parse SQL to determine
            SizeMB:   0,     // SQLite doesn't easily provide index size
        }

        indexes = append(indexes, index)
    }

    return indexes, nil
}

func collectRealTimeMetricsSQLite(pool *db.DBPool, ctx context.Context) (*RealTimeMetrics, error) {
    rdb, err := pool.GetReadTx(ctx)
    if err != nil {
        return nil, err
    }
    defer rdb.Rollback()

    // Database health
    var activeConnections int
    activeConnections = pool.ReadDB.Stats().OpenConnections + pool.WriteDB.Stats().OpenConnections

    var dbSizeMB int64
    var pageCount, pageSize int64
    rdb.QueryRow("PRAGMA page_count").Scan(&pageCount)
    rdb.QueryRow("PRAGMA page_size").Scan(&pageSize)
    dbSizeMB = (pageCount * pageSize) / (1024 * 1024)

    dbHealth := DatabaseHealth{
        ActiveConnections: activeConnections,
        DatabaseSizeMB:    dbSizeMB,
    }

    // System health
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
        App:       ApplicationHealth{}, // Populate as needed
    }, nil
}
