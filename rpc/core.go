package rpc

import (
	"rpc"
	"fmt"
)

type RPCRequest struct {
    JobID       int                    `json:"job_id"`
    Service     string                 `json:"service"`
    Method      string                 `json:"method"`
    Payload     []byte                 `json:"payload"`
    CallbackURL string                 `json:"callback_url"`
    Timeout     time.Duration          `json:"timeout"`
    MaxRetries  int                    `json:"max_retries"`
}

type RPCResponse struct {
    JobID   int    `json:"job_id"`
    Success bool   `json:"success"`
    Result  []byte `json:"result,omitempty"`
    Error   string `json:"error,omitempty"`
    Stage   string `json:"stage"`             // "queued", "calling", "completed", "failed"
}

func CreateRPCJob(pool *db.DBPool, ctx context.Context, service, method string, payload []byte) (int, error) {
    rpcPayload := RPCRequest{
        Service:     service,
        Method:      method,
        Payload:     payload,
        CallbackURL: fmt.Sprintf("%s/api/rpc/callback", config.BaseURL),
        Timeout:     30 * time.Second,
        MaxRetries:  3,
    }

    jobPayload, _ := json.Marshal(rpcPayload)
    jobID, err := db.CreateJob(pool, ctx, "rpc_call", 1, jobPayload)
    if err != nil {
        return 0, fmt.Errorf("failed to create RPC job: %w", err)
    }

    broadcastRPCStatus(jobID, "queued", "", "")
    return jobID, nil
}

func ProcessRPCJob(jobID int, payload []byte) error {
    var rpcReq RPCRequest
    if err := json.Unmarshal(payload, &rpcReq); err != nil {
        HandleRPCError(jobID, "job_processing", "validation", err.Error(), false)
        return err
    }

    rpcReq.JobID = jobID

    broadcastRPCStatus(jobID, "calling", rpcReq.Service, rpcReq.Method)

    ctx, cancel := context.WithTimeout(context.Background(), rpcReq.Timeout)
    defer cancel()

    if err := callExternalService(ctx, rpcReq); err != nil {
        errorType := determineErrorType(err)
        retryable := errorType == "network" || errorType == "timeout"
        HandleRPCError(jobID, "service_call", errorType, err.Error(), retryable)
        return err
    }

    broadcastRPCStatus(jobID, "processing", rpcReq.Service, rpcReq.Method)
    return nil
}

func callExternalService(ctx context.Context, req RPCRequest) error {
    switch req.Service {
    case "python":
        return callPythonService(ctx, req)
    case "rust":
        return callRustService(ctx, req)
    case "c":
        return callCService(ctx, req)
    default:
        return fmt.Errorf("unknown service: %s", req.Service)
    }
}

func RPCCallbackHandler(ctx *appcontext.AppContext) {
    var response RPCResponse
    if err := json.NewDecoder(ctx.Request.Body).Decode(&response); err != nil {
        HandleRPCError(0, "callback", "validation", err.Error(), false)
        http.Error(ctx.Writer, "Invalid callback data", http.StatusBadRequest)
        return
    }

    if response.Success {
        // Store results and mark job complete
        _, err := ctx.Pool.PgxPool.Exec(ctx.Context,
            "UPDATE job_queue SET status = 'completed' WHERE id = $1", response.JobID)
        if err != nil {
            HandleRPCError(response.JobID, "callback", "database", err.Error(), true)
            return
        }

        // Store results in history table
        _, err = ctx.Pool.PgxPool.Exec(ctx.Context,
            "UPDATE job_history SET result = $1 WHERE id = $2", 
            response.Result, response.JobID)

        // Success notification
        broadcastRPCStatus(response.JobID, "completed", "", "")
    } else {
        HandleRPCError(response.JobID, "service_execution", "service", response.Error, false)
    }

    ctx.Writer.WriteHeader(http.StatusOK)
}

func broadcastRPCStatus(jobID int, status, service, method string) {
    notification := map[string]any{
        "type":    "rpc_status",
        "job_id":  jobID,
        "status":  status,
        "service": service,
        "method":  method,
        "timestamp": time.Now().Unix(),
    }

    data, _ := json.Marshal(notification)
    wsHub.broadcast <- data
}

func broadcastRPCError(rpcError RPCError) {
    notification := map[string]any{
        "type":  "rpc_error",
        "error": rpcError,
    }

    data, _ := json.Marshal(notification)
    wsHub.broadcast <- data
}
