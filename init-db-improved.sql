-- Improved Database Schema with Better Primary Key Strategy
-- 使用自增 ID 作为主键，UUID 作为业务标识

CREATE DATABASE IF NOT EXISTS agent_service CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE agent_service;

-- ============================================
-- Projects Table (优化主键策略)
-- ============================================
CREATE TABLE IF NOT EXISTS projects (
    -- 使用自增 ID 作为主键（性能优化）
    id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT 'Internal auto-increment ID for performance',
    
    -- UUID 作为业务标识（对外暴露）
    uuid CHAR(36) NOT NULL UNIQUE COMMENT 'UUID v4 for external API (project_id)',
    
    user_id CHAR(36) NOT NULL COMMENT 'UUID v4 user identifier',
    name VARCHAR(255) NOT NULL DEFAULT 'New Project' COMMENT 'Project name',
    description TEXT COMMENT 'Project description',
    icon VARCHAR(10) DEFAULT '💬' COMMENT 'Project icon (emoji)',
    thumbnail VARCHAR(100) DEFAULT 'bg-gradient-to-br from-gray-100 to-gray-200' COMMENT 'Thumbnail CSS class',
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'Project creation time',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last update time',
    version INT DEFAULT 1 COMMENT 'Optimistic locking version',
    
    is_deleted BOOLEAN DEFAULT FALSE COMMENT 'Soft delete flag',
    deleted_at TIMESTAMP NULL COMMENT 'Deletion timestamp',
    
    -- Indexes for performance
    INDEX idx_uuid (uuid),                              -- UUID 查询
    INDEX idx_user_id (user_id),                        -- 用户查询
    INDEX idx_created_at (created_at DESC),             -- 时间排序
    INDEX idx_updated_at (updated_at DESC),             -- 更新时间
    INDEX idx_user_created (user_id, created_at DESC),  -- 复合查询
    INDEX idx_is_deleted (is_deleted),                  -- 软删除过滤
    
    -- Unique constraint for user + uuid
    UNIQUE KEY uk_user_uuid (user_id, uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci 
COMMENT='Project storage with optimized primary key';

-- ============================================
-- Messages Table (使用 project_id 外键)
-- ============================================
CREATE TABLE IF NOT EXISTS messages (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    
    -- 使用 BIGINT 外键指向 projects.id（性能优化）
    project_id BIGINT NOT NULL COMMENT 'Internal project ID (FK to projects.id)',
    
    role ENUM('user', 'assistant', 'system') NOT NULL COMMENT 'Message sender role',
    content TEXT NOT NULL COMMENT 'Message content',
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'Message creation time',
    message_index INT NOT NULL COMMENT 'Sequential message index within conversation',
    status ENUM('pending', 'completed', 'failed') DEFAULT 'pending' COMMENT 'Message processing status',
    
    token_count INT DEFAULT 0 COMMENT 'Token count for cost tracking',
    model VARCHAR(100) COMMENT 'AI model used for assistant messages',
    error_message TEXT COMMENT 'Error details if status is failed',
    metadata JSON COMMENT 'Additional metadata (e.g., attachments, citations)',
    
    -- Indexes for performance (BIGINT 索引更快)
    INDEX idx_project (project_id),
    INDEX idx_project_index (project_id, message_index),
    INDEX idx_project_timestamp (project_id, timestamp DESC),
    INDEX idx_status (status),
    INDEX idx_timestamp (timestamp DESC),
    INDEX idx_role (role),
    
    -- Foreign key with cascade delete (BIGINT 外键性能更好)
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    
    -- Unique constraint to prevent duplicate message_index
    UNIQUE KEY uk_project_index (project_id, message_index)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci 
COMMENT='Chat messages with optimized foreign key';

-- ============================================
-- Session Statistics Table
-- ============================================
CREATE TABLE IF NOT EXISTS session_statistics (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    
    -- 使用 BIGINT 外键（性能优化）
    project_id BIGINT NOT NULL COMMENT 'Internal project ID (FK to projects.id)',
    
    user_id CHAR(36) NOT NULL COMMENT 'User ID',
    total_messages INT DEFAULT 0 COMMENT 'Total message count',
    total_tokens INT DEFAULT 0 COMMENT 'Total token usage',
    last_activity_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last activity timestamp',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    -- Indexes
    INDEX idx_project (project_id),
    INDEX idx_user (user_id),
    INDEX idx_last_activity (last_activity_at DESC),
    
    -- Foreign key
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    
    -- Unique constraint
    UNIQUE KEY uk_project_stats (project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci 
COMMENT='Session statistics for analytics';


-- ============================================
-- Triggers for automatic statistics updates
-- ============================================
DELIMITER //

-- Trigger to update session statistics on message insert
CREATE TRIGGER after_message_insert
AFTER INSERT ON messages
FOR EACH ROW
BEGIN
    INSERT INTO session_statistics (project_id, user_id, total_messages, total_tokens, last_activity_at)
    SELECT NEW.project_id, p.user_id, 1, COALESCE(NEW.token_count, 0), NEW.timestamp
    FROM projects p WHERE p.id = NEW.project_id
    ON DUPLICATE KEY UPDATE
        total_messages = total_messages + 1,
        total_tokens = total_tokens + COALESCE(NEW.token_count, 0),
        last_activity_at = NEW.timestamp;


-- Trigger to update project updated_at on message insert
CREATE TRIGGER after_message_insert_update_project
AFTER INSERT ON messages
FOR EACH ROW
BEGIN
    UPDATE projects SET updated_at = NEW.timestamp WHERE id = NEW.project_id;
END//

DELIMITER ;

-- ============================================
-- Useful Views (使用 UUID 对外暴露)
-- ============================================

-- View for active projects with message counts
CREATE OR REPLACE VIEW v_active_projects AS
SELECT 
    p.uuid AS project_id,              -- 对外使用 UUID
    p.user_id,
    p.name,
    p.description,
    p.icon,
    p.thumbnail,
    p.created_at,
    p.updated_at,
    COALESCE(s.total_messages, 0) AS message_count,
    COALESCE(s.total_tokens, 0) AS token_count,
    COALESCE(s.last_activity_at, p.created_at) AS last_activity_at
FROM projects p
LEFT JOIN session_statistics s ON p.id = s.project_id
WHERE p.is_deleted = FALSE
ORDER BY p.updated_at DESC;

-- View for recent messages (使用 UUID)
CREATE OR REPLACE VIEW v_recent_messages AS
SELECT 
    m.id,
    p.uuid AS project_id,              -- 对外使用 UUID
    p.name AS project_name,
    p.user_id,
    m.role,
    LEFT(m.content, 100) AS content_preview,
    m.timestamp,
    m.status,
    m.token_count
FROM messages m
INNER JOIN projects p ON m.project_id = p.id
WHERE p.is_deleted = FALSE AND m.status != 'failed'
ORDER BY m.timestamp DESC
LIMIT 100;

-- ============================================
-- Helper Functions
-- ============================================
DELIMITER //

-- Function to get project internal ID from UUID
CREATE FUNCTION get_project_id(project_uuid CHAR(36))
RETURNS BIGINT
DETERMINISTIC
READS SQL DATA
BEGIN
    DECLARE project_internal_id BIGINT;
    SELECT id INTO project_internal_id FROM projects WHERE uuid = project_uuid LIMIT 1;
    RETURN project_internal_id;
END//

-- Function to get project UUID from internal ID
CREATE FUNCTION get_project_uuid(project_internal_id BIGINT)
RETURNS CHAR(36)
DETERMINISTIC
READS SQL DATA
BEGIN
    DECLARE project_external_uuid CHAR(36);
    SELECT uuid INTO project_external_uuid FROM projects WHERE id = project_internal_id LIMIT 1;
    RETURN project_external_uuid;
END//

DELIMITER ;

-- ============================================
-- Performance Optimization Procedures
-- ============================================
DELIMITER //

-- Procedure to cleanup old failed messages
CREATE PROCEDURE cleanup_failed_messages(IN days_old INT)
BEGIN
    DELETE FROM messages 
    WHERE status = 'failed' 
    AND timestamp < DATE_SUB(NOW(), INTERVAL days_old DAY);
    
    SELECT ROW_COUNT() AS deleted_count;
END//

-- Procedure to soft delete old projects
CREATE PROCEDURE archive_old_projects(IN days_inactive INT)
BEGIN
    UPDATE projects p
    LEFT JOIN session_statistics s ON p.id = s.project_id
    SET p.is_deleted = TRUE, p.deleted_at = NOW()
    WHERE p.is_deleted = FALSE
    AND COALESCE(s.last_activity_at, p.created_at) < DATE_SUB(NOW(), INTERVAL days_inactive DAY);
    
    SELECT ROW_COUNT() AS archived_count;
END//

-- Procedure to permanently delete soft-deleted projects
CREATE PROCEDURE purge_deleted_projects(IN days_deleted INT)
BEGIN
    DELETE FROM projects
    WHERE is_deleted = TRUE
    AND deleted_at < DATE_SUB(NOW(), INTERVAL days_deleted DAY);
    
    SELECT ROW_COUNT() AS purged_count;
END//

DELIMITER ;

-- ============================================
-- Performance Analysis
-- ============================================

-- Compare index sizes
SELECT 
    'UUID Primary Key' AS type,
    36 AS bytes_per_key,
    36 * 1000000 AS bytes_for_1m_rows,
    ROUND(36 * 1000000 / 1024 / 1024, 2) AS mb_for_1m_rows
UNION ALL
SELECT 
    'BIGINT Primary Key' AS type,
    8 AS bytes_per_key,
    8 * 1000000 AS bytes_for_1m_rows,
    ROUND(8 * 1000000 / 1024 / 1024, 2) AS mb_for_1m_rows;

-- Check table sizes
SELECT 
    table_name,
    ROUND(((data_length + index_length) / 1024 / 1024), 2) AS size_mb,
    table_rows
FROM information_schema.TABLES
WHERE table_schema = 'agent_service'
ORDER BY (data_length + index_length) DESC;

SELECT 'Database schema with improved primary key strategy created successfully!' AS message;
