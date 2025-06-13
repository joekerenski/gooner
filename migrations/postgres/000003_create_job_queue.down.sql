-- Drop trigger and function
DROP TRIGGER IF EXISTS trigger_move_job_to_history ON job_queue;
DROP FUNCTION IF EXISTS move_job_to_history();
DROP FUNCTION IF EXISTS claim_next_job(TEXT);

-- Drop indexes
DROP INDEX IF EXISTS idx_job_history_performance;
DROP INDEX IF EXISTS idx_job_history_status;
DROP INDEX IF EXISTS idx_job_history_type;
DROP INDEX IF EXISTS idx_job_queue_type;
DROP INDEX IF EXISTS idx_job_queue_scheduled;
DROP INDEX IF EXISTS idx_job_queue_worker;
DROP INDEX IF EXISTS idx_job_priority_queue;

-- Drop tables
DROP TABLE IF EXISTS job_history;
DROP TABLE IF EXISTS job_queue;
