-- Job queue table for active jobs
CREATE TABLE IF NOT EXISTS job_queue (
    id SERIAL PRIMARY KEY,
    type TEXT NOT NULL,
    priority INTEGER DEFAULT 0,  -- Higher = more important
    payload BYTEA,               -- MessagePack data for RPC
    status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    claimed_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    timeout_seconds INTEGER DEFAULT 300,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    worker_id TEXT,
    scheduled_for TIMESTAMPTZ DEFAULT NOW()  -- For delayed jobs
);

-- Job history table for completed jobs (audit trail and results)
CREATE TABLE IF NOT EXISTS job_history (
    id INTEGER PRIMARY KEY,      -- Same ID as job_queue
    type TEXT NOT NULL,
    priority INTEGER,
    payload BYTEA,
    result BYTEA,               -- MessagePack results from RPC
    error TEXT,                 -- Error details if failed
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ DEFAULT NOW(),
    worker_id TEXT,
    execution_time_ms INTEGER   -- For performance monitoring
);

-- Critical index for efficient priority-based job selection
CREATE INDEX IF NOT EXISTS idx_job_priority_queue ON job_queue (status, priority DESC, created_at);

-- Index for worker queries and cleanup
CREATE INDEX IF NOT EXISTS idx_job_queue_worker ON job_queue (worker_id, status);

-- Index for scheduled jobs
CREATE INDEX IF NOT EXISTS idx_job_queue_scheduled ON job_queue (scheduled_for, status);

-- Index for job type filtering
CREATE INDEX IF NOT EXISTS idx_job_queue_type ON job_queue (type, status);

-- History table indexes for monitoring and retrieval
CREATE INDEX IF NOT EXISTS idx_job_history_type ON job_history (type, completed_at DESC);
CREATE INDEX IF NOT EXISTS idx_job_history_status ON job_history (status, completed_at DESC);
CREATE INDEX IF NOT EXISTS idx_job_history_performance ON job_history (execution_time_ms DESC);

-- Function to automatically move completed jobs to history
CREATE OR REPLACE FUNCTION move_job_to_history()
RETURNS TRIGGER AS $$
BEGIN
    -- Only move if status changed to completed, failed, or cancelled
    IF NEW.status IN ('completed', 'failed', 'cancelled') AND OLD.status NOT IN ('completed', 'failed', 'cancelled') THEN
        INSERT INTO job_history (
            id, type, priority, payload, result, error, status,
            created_at, started_at, completed_at, worker_id,
            execution_time_ms
        ) VALUES (
            NEW.id, NEW.type, NEW.priority, NEW.payload, 
            NULL, -- result will be updated separately
            CASE WHEN NEW.status = 'failed' THEN 'Job failed' ELSE NULL END,
            NEW.status, NEW.created_at, NEW.started_at, NOW(), NEW.worker_id,
            CASE 
                WHEN NEW.started_at IS NOT NULL THEN 
                    EXTRACT(EPOCH FROM (NOW() - NEW.started_at))::INTEGER * 1000
                ELSE NULL 
            END
        );
        
        -- Delete from active queue
        DELETE FROM job_queue WHERE id = NEW.id;
        
        -- Prevent the original update since we're deleting the row
        RETURN NULL;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically archive completed jobs
CREATE TRIGGER trigger_move_job_to_history
    AFTER UPDATE OF status ON job_queue
    FOR EACH ROW
    EXECUTE FUNCTION move_job_to_history();

-- Function for atomic job claiming (FOR UPDATE SKIP LOCKED)
CREATE OR REPLACE FUNCTION claim_next_job(worker_id_param TEXT)
RETURNS TABLE(
    job_id INTEGER,
    job_type TEXT,
    job_payload BYTEA,
    job_timeout INTEGER
) AS $$
DECLARE
    claimed_job_id INTEGER;
BEGIN
    -- Atomically claim the highest priority job
    UPDATE job_queue 
    SET 
        status = 'running',
        claimed_at = NOW(),
        started_at = NOW(),
        worker_id = worker_id_param
    WHERE id = (
        SELECT id FROM job_queue 
        WHERE status = 'pending' 
        AND scheduled_for <= NOW()
        ORDER BY priority DESC, created_at ASC
        LIMIT 1
        FOR UPDATE SKIP LOCKED
    )
    RETURNING id INTO claimed_job_id;
    
    -- Return job details if we claimed one
    IF claimed_job_id IS NOT NULL THEN
        RETURN QUERY
        SELECT 
            jq.id,
            jq.type,
            jq.payload,
            jq.timeout_seconds
        FROM job_queue jq
        WHERE jq.id = claimed_job_id;
    END IF;
END;
$$ LANGUAGE plpgsql;
