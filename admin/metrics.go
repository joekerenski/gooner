package admin

import (
    "context"
    "fmt"
	"strings"

	"gooner/db"
)

func CollectBaseMetrics(pool *db.DBPool, ctx context.Context) (*BaseMetrics, error) {
    switch pool.Type {
    case "postgres":
        return collectBaseMetricsPostgres(pool, ctx)
    case "sqlite3":
        return collectBaseMetricsSQLite(pool, ctx)
    default:
        return nil, fmt.Errorf("unsupported database type: %s", pool.Type)
    }
}

func CollectRealTimeMetrics(pool *db.DBPool, ctx context.Context) (*RealTimeMetrics, error) {
    switch pool.Type {
    case "postgres":
        return collectRealTimeMetricsPostgres(pool, ctx)
    case "sqlite3":
        return collectRealTimeMetricsSQLite(pool, ctx)
    default:
        return nil, fmt.Errorf("unsupported database type: %s", pool.Type)
    }
}

func extractFeatures(tables []Table) []FeatureInfo {
    featureMap := make(map[string]*FeatureInfo)

    for _, table := range tables {
        parts := strings.Split(table.Name, "_")
        if len(parts) > 1 {
            prefix := parts[0] + "_"
            
            if feature, exists := featureMap[prefix]; exists {
                feature.TableCount++
            } else {
                featureMap[prefix] = &FeatureInfo{
                    Name:        parts[0],
                    TablePrefix: prefix,
                    TableCount:  1,
                    IsActive:    true,
                }
            }
        }
    }
    
    var features []FeatureInfo
    for _, feature := range featureMap {
        features = append(features, *feature)
    }
    
    return features
}

