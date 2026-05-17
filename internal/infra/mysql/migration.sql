CREATE TABLE IF NOT EXISTS notification_tasks (
  id VARCHAR(64) PRIMARY KEY,
  app_id VARCHAR(64) NOT NULL,
  vendor VARCHAR(64) NOT NULL,
  target_url TEXT NOT NULL,
  method VARCHAR(16) NOT NULL,
  headers JSON NOT NULL,
  payload JSON NOT NULL,
  idempotency_key VARCHAR(191) NULL,
  status VARCHAR(32) NOT NULL,
  retry_count INT NOT NULL DEFAULT 0,
  max_retries INT NOT NULL DEFAULT 5,
  next_retry_at DATETIME(6) NULL,
  last_status_code INT NULL,
  last_error TEXT NULL,
  mq_message_id VARCHAR(128) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_app_vendor_idempotency_key (app_id, vendor, idempotency_key),
  KEY idx_status_next_retry (status, next_retry_at),
  KEY idx_app_created_at (app_id, created_at),
  KEY idx_vendor_created_at (vendor, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS outbox_messages (
  id VARCHAR(64) PRIMARY KEY,
  task_id VARCHAR(64) NOT NULL,
  event_type VARCHAR(64) NOT NULL,
  payload JSON NOT NULL,
  status VARCHAR(32) NOT NULL,
  publish_attempts INT NOT NULL DEFAULT 0,
  last_error TEXT NULL,
  created_at DATETIME(6) NOT NULL,
  published_at DATETIME(6) NULL,
  KEY idx_outbox_status_created_at (status, created_at),
  KEY idx_outbox_task_id (task_id),
  CONSTRAINT fk_outbox_task_id FOREIGN KEY (task_id) REFERENCES notification_tasks(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
