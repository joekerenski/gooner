package rpc

type RPCError struct {
    JobID     int       `json:"job_id"`
    Stage     string    `json:"stage"`        // "job_creation", "service_call", "callback", "timeout"
    ErrorType string    `json:"error_type"`   // "network", "timeout", "service", "validation"
    Message   string    `json:"message"`
    Timestamp time.Time `json:"timestamp"`
    Retryable bool      `json:"retryable"`
}

func HandleRPCError(jobID int, stage, errorType, message string, retryable bool) {
    rpcError := RPCError{
        JobID:     jobID,
        Stage:     stage,
        ErrorType: errorType,
        Message:   message,
        Timestamp: time.Now(),
        Retryable: retryable,
    }

    logRPCError(rpcError)                    // Structured logging
    storeRPCError(rpcError)                  // Database for audit
    broadcastRPCError(rpcError)              // WebSocket for real-time UI

    if retryable {
        scheduleRPCRetry(jobID)
    } else {
        markJobFailed(jobID, message)
    }
}
