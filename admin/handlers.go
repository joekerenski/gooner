package admin

import (
    "encoding/json"
    "net/http"
    "gooner/appcontext"
)

type MetricsResponse struct {
    BaseMetrics     *BaseMetrics     `json:"base_metrics"`
    RealTimeMetrics *RealTimeMetrics `json:"realtime_metrics"`
}

func MetricsHandler(ctx *appcontext.AppContext) {
	_, ok := ctx.Context.Value("userID").(string)
    if !ok {
        http.Error(ctx.Writer, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // You might want to check for admin role here
    // role, _ := ctx.Context.Value("role").(string)
    // if role != "admin" {
    //     http.Error(ctx.Writer, "Forbidden", http.StatusForbidden)
    //     return
    // }

    // if ctx.Pool == nil {
    //     http.Error(ctx.Writer, "Database not available", http.StatusInternalServerError)
    //     return
    // }

    baseMetrics, err := CollectBaseMetrics(ctx.Pool, ctx.Context)
    if err != nil {
        ctx.Logger.Printf("Failed to collect base metrics: %v", err)
        http.Error(ctx.Writer, "Failed to collect base metrics", http.StatusInternalServerError)
        return
    }

    realTimeMetrics, err := CollectRealTimeMetrics(ctx.Pool, ctx.Context)
    if err != nil {
        ctx.Logger.Printf("Failed to collect real-time metrics: %v", err)
        http.Error(ctx.Writer, "Failed to collect real-time metrics", http.StatusInternalServerError)
        return
    }

    response := MetricsResponse{
        BaseMetrics:     baseMetrics,
        RealTimeMetrics: realTimeMetrics,
    }

    ctx.Writer.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(ctx.Writer).Encode(response); err != nil {
        ctx.Logger.Printf("Failed to encode metrics response: %v", err)
        http.Error(ctx.Writer, "Failed to encode response", http.StatusInternalServerError)
        return
    }
}
