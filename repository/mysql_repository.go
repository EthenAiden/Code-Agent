package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethen-aiden/code-agent/model"
	"golang.org/x/exp/rand"
)

// MySQLSessionPersistenceRepository implements SessionPersistenceRepository interface
type MySQLSessionPersistenceRepository struct {
	db *sql.DB
}

// NewMySQLSessionPersistenceRepository creates a new MySQL repository instance
func NewMySQLSessionPersistenceRepository(db *sql.DB) *MySQLSessionPersistenceRepository {
	return &MySQLSessionPersistenceRepository{db: db}
}

// initSchema creates the database schema (sessions and messages tables)
func (spr *MySQLSessionPersistenceRepository) initSchema(ctx context.Context) error {
	// Create sessions table
	sessionsTable := `
	CREATE TABLE IF NOT EXISTS sessions (
		id CHAR(36) PRIMARY KEY COMMENT 'UUID v4 conversation_id',
		user_id CHAR(36) NOT NULL COMMENT 'UUID v4 user identifier for session isolation',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		version INT DEFAULT 1 COMMENT 'Version field for optimistic locking',
		INDEX idx_user_id (user_id),
		INDEX idx_created_at (created_at),
		UNIQUE KEY uk_user_session (user_id, id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`

	// Create messages table
	messagesTable := `
	CREATE TABLE IF NOT EXISTS messages (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		conversation_id CHAR(36) NOT NULL,
		role ENUM('user', 'assistant') NOT NULL,
		content TEXT NOT NULL,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		message_index INT NOT NULL,
		status ENUM('pending', 'completed', 'failed') DEFAULT 'pending' COMMENT 'Message status for streaming reliability',
		INDEX idx_conversation_index (conversation_id, message_index),
		INDEX idx_timestamp (timestamp),
		INDEX idx_status (status),
		FOREIGN KEY (conversation_id) REFERENCES sessions(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`

	// Execute schema creation
	_, err := spr.db.ExecContext(ctx, sessionsTable)
	if err != nil {
		return fmt.Errorf("failed to create sessions table: %w", err)
	}

	_, err = spr.db.ExecContext(ctx, messagesTable)
	if err != nil {
		return fmt.Errorf("failed to create messages table: %w", err)
	}

	return nil
}

// retryWithBackoff executes a function with retry and exponential backoff
func (spr *MySQLSessionPersistenceRepository) retryWithBackoff(ctx context.Context, maxRetries int, fn func() error) error {
	var err error
	for attempt := 0; attempt < maxRetries; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}

		// Check if it's a database connection error
		if isConnectionError(err) {
			if attempt < maxRetries-1 {
				// Exponential backoff: 1s, 2s, 4s
				backoff := time.Duration(1<<attempt) * time.Second
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff):
					continue
				}
			}
		}
	}
	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries, err)
}

// isConnectionError checks if an error is a database connection error
func isConnectionError(err error) bool {
	errStr := err.Error()
	return errors.Is(err, sql.ErrConnDone) ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "dial") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "reset by peer")
}

// RepositoryError represents a repository error with error code
type RepositoryError struct {
	ErrorCode string
	Err       error
}

// Error returns the error message
func (e *RepositoryError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error
func (e *RepositoryError) Unwrap() error {
	return e.Err
}

// NewRepositoryError creates a new repository error
func NewRepositoryError(errorCode string, err error) *RepositoryError {
	return &RepositoryError{
		ErrorCode: errorCode,
		Err:       err,
	}
}

// InsertSession creates a new session in MySQL
func (spr *MySQLSessionPersistenceRepository) InsertSession(ctx context.Context, conversationID string, userID string) error {
	return spr.retryWithBackoff(ctx, 3, func() error {
		query := `INSERT INTO sessions (id, user_id, version) VALUES (?, ?, 1)`
		_, err := spr.db.ExecContext(ctx, query, conversationID, userID)
		if err != nil {
			return fmt.Errorf("failed to insert session: %w", err)
		}
		return nil
	})
}

// InsertMessage stores a message in MySQL
func (spr *MySQLSessionPersistenceRepository) InsertMessage(ctx context.Context, msg model.Message) error {
	return spr.retryWithBackoff(ctx, 3, func() error {
		// Use transaction for atomicity
		tx, err := spr.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback()

		// Get next message index with lock
		var nextIndex int
		err = tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(message_index), -1) + 1 
			FROM messages 
			WHERE conversation_id = ? FOR UPDATE`, msg.ConversationID).Scan(&nextIndex)
		if err != nil {
			return fmt.Errorf("failed to get next message index: %w", err)
		}

		// Insert message
		query := `INSERT INTO messages (conversation_id, role, content, timestamp, message_index, status) VALUES (?, ?, ?, ?, ?, ?)`
		_, err = tx.ExecContext(ctx, query, msg.ConversationID, msg.Role, msg.Content, msg.Timestamp, nextIndex, msg.Status)
		if err != nil {
			return fmt.Errorf("failed to insert message: %w", err)
		}

		// Commit transaction
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		return nil
	})
}

// GetSessionMessages retrieves all messages for a session
func (spr *MySQLSessionPersistenceRepository) GetSessionMessages(ctx context.Context, conversationID string, userID string) ([]model.Message, error) {
	var messages []model.Message

	err := spr.retryWithBackoff(ctx, 3, func() error {
		query := `
			SELECT id, conversation_id, role, content, timestamp, message_index, status 
			FROM messages 
			WHERE conversation_id = ? AND status != 'failed'
			ORDER BY message_index ASC`

		rows, err := spr.db.QueryContext(ctx, query, conversationID)
		if err != nil {
			return fmt.Errorf("failed to query messages: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var msg model.Message
			err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &msg.Timestamp, &msg.MessageIndex, &msg.Status)
			if err != nil {
				return fmt.Errorf("failed to scan message: %w", err)
			}
			messages = append(messages, msg)
		}

		if err = rows.Err(); err != nil {
			return fmt.Errorf("error iterating messages: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return messages, nil
}

// ListSessions retrieves session summaries with pagination
func (spr *MySQLSessionPersistenceRepository) ListSessions(ctx context.Context, userID string, limit, offset int) ([]model.SessionSummary, error) {
	var summaries []model.SessionSummary

	err := spr.retryWithBackoff(ctx, 3, func() error {
		// Get total count
		var total int
		err := spr.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM sessions WHERE user_id = ?`, userID).Scan(&total)
		if err != nil {
			return fmt.Errorf("failed to get total count: %w", err)
		}

		// Get paginated sessions with summary data
		query := `
			SELECT s.id, 
				   COALESCE(MAX(m.timestamp), s.created_at) as last_message_timestamp,
				   COUNT(m.id) as message_count
			FROM sessions s
			LEFT JOIN messages m ON s.id = m.conversation_id AND m.status != 'failed'
			WHERE s.user_id = ?
			GROUP BY s.id
			ORDER BY s.created_at DESC
			LIMIT ? OFFSET ?`

		rows, err := spr.db.QueryContext(ctx, query, userID, limit, offset)
		if err != nil {
			return fmt.Errorf("failed to query sessions: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var summary model.SessionSummary
			err := rows.Scan(&summary.ConversationID, &summary.LastMessageTimestamp, &summary.MessageCount)
			if err != nil {
				return fmt.Errorf("failed to scan session summary: %w", err)
			}
			summaries = append(summaries, summary)
		}

		if err = rows.Err(); err != nil {
			return fmt.Errorf("error iterating sessions: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return summaries, nil
}

// DeleteSession removes a session and all its messages
func (spr *MySQLSessionPersistenceRepository) DeleteSession(ctx context.Context, conversationID string, userID string) error {
	return spr.retryWithBackoff(ctx, 3, func() error {
		// Delete messages first (foreign key will handle cascade)
		_, err := spr.db.ExecContext(ctx, `
			DELETE m FROM messages m
			INNER JOIN sessions s ON m.conversation_id = s.id
			WHERE s.id = ? AND s.user_id = ?`, conversationID, userID)
		if err != nil {
			return fmt.Errorf("failed to delete messages: %w", err)
		}

		// Delete session
		_, err = spr.db.ExecContext(ctx, `
			DELETE FROM sessions WHERE id = ? AND user_id = ?`, conversationID, userID)
		if err != nil {
			return fmt.Errorf("failed to delete session: %w", err)
		}

		return nil
	})
}

// GetSessionDetails retrieves session metadata
func (spr *MySQLSessionPersistenceRepository) GetSessionDetails(ctx context.Context, conversationID string, userID string) (*model.SessionDetails, error) {
	var details model.SessionDetails
	var lastMessageAt sql.NullTime

	err := spr.retryWithBackoff(ctx, 3, func() error {
		query := `
			SELECT s.id, 
				   COALESCE((SELECT COUNT(*) FROM messages m WHERE m.conversation_id = s.id AND m.status != 'failed'), 0) as message_count,
				   s.created_at,
				   (SELECT MAX(m.timestamp) FROM messages m WHERE m.conversation_id = s.id AND m.status != 'failed') as last_message_at
			FROM sessions s
			WHERE s.id = ? AND s.user_id = ?`

		err := spr.db.QueryRowContext(ctx, query, conversationID, userID).Scan(
			&details.ConversationID,
			&details.MessageCount,
			&details.CreatedAt,
			&lastMessageAt,
		)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("session not found: %w", err)
			}
			return fmt.Errorf("failed to get session details: %w", err)
		}

		// Convert sql.NullTime to *time.Time
		if lastMessageAt.Valid {
			details.LastMessageAt = &lastMessageAt.Time
		} else {
			details.LastMessageAt = nil
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &details, nil
}

// SessionExists checks if a session exists
func (spr *MySQLSessionPersistenceRepository) SessionExists(ctx context.Context, conversationID string, userID string) (bool, error) {
	var exists bool

	err := spr.retryWithBackoff(ctx, 3, func() error {
		err := spr.db.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM sessions WHERE id = ? AND user_id = ?)`, conversationID, userID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check session existence: %w", err)
		}
		return nil
	})

	if err != nil {
		return false, err
	}

	return exists, nil
}

// GetSessionVersion retrieves the current version for optimistic locking
func (spr *MySQLSessionPersistenceRepository) GetSessionVersion(ctx context.Context, conversationID string, userID string) (int, error) {
	var version int

	err := spr.retryWithBackoff(ctx, 3, func() error {
		err := spr.db.QueryRowContext(ctx, `
			SELECT version FROM sessions WHERE id = ? AND user_id = ?`, conversationID, userID).Scan(&version)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("session not found: %w", err)
			}
			return fmt.Errorf("failed to get session version: %w", err)
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return version, nil
}

// UpdateSessionVersion updates the session version for optimistic locking
func (spr *MySQLSessionPersistenceRepository) UpdateSessionVersion(ctx context.Context, conversationID string, userID string, expectedVersion int) error {
	return spr.retryWithBackoff(ctx, 3, func() error {
		result, err := spr.db.ExecContext(ctx, `
			UPDATE sessions SET version = version + 1 WHERE id = ? AND user_id = ? AND version = ?`,
			conversationID, userID, expectedVersion)
		if err != nil {
			return fmt.Errorf("failed to update session version: %w", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}

		if rowsAffected == 0 {
			return fmt.Errorf("concurrent update detected: version mismatch")
		}

		return nil
	})
}

// GetNextMessageIndex gets the next message index for a session
func (spr *MySQLSessionPersistenceRepository) GetNextMessageIndex(ctx context.Context, conversationID string, userID string) (int, error) {
	var nextIndex int

	err := spr.retryWithBackoff(ctx, 3, func() error {
		err := spr.db.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(message_index), -1) + 1 
			FROM messages 
			WHERE conversation_id = ?`, conversationID).Scan(&nextIndex)
		if err != nil {
			return fmt.Errorf("failed to get next message index: %w", err)
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return nextIndex, nil
}

// CleanupFailedMessages removes failed messages from a session
func (spr *MySQLSessionPersistenceRepository) CleanupFailedMessages(ctx context.Context, conversationID string, userID string) error {
	return spr.retryWithBackoff(ctx, 3, func() error {
		_, err := spr.db.ExecContext(ctx, `
			DELETE FROM messages 
			WHERE conversation_id = ? AND status = 'failed'`, conversationID)
		if err != nil {
			return fmt.Errorf("failed to cleanup failed messages: %w", err)
		}
		return nil
	})
}

// UpdateMessageStatus updates the status of a message
func (spr *MySQLSessionPersistenceRepository) UpdateMessageStatus(ctx context.Context, conversationID string, messageIndex int, status string) error {
	return spr.retryWithBackoff(ctx, 3, func() error {
		// Get message ID first
		var messageID int
		err := spr.db.QueryRowContext(ctx, `
			SELECT id FROM messages WHERE conversation_id = ? AND message_index = ?`,
			conversationID, messageIndex).Scan(&messageID)
		if err != nil {
			return fmt.Errorf("failed to get message: %w", err)
		}

		// Update status
		_, err = spr.db.ExecContext(ctx, `
			UPDATE messages SET status = ? WHERE id = ?`,
			status, messageID)
		if err != nil {
			return fmt.Errorf("failed to update message status: %w", err)
		}

		return nil
	})
}

// GenerateRandomTTL generates a random TTL with ±10% variation
func GenerateRandomTTL(baseTTL time.Duration) time.Duration {
	// Use crypto/rand for better randomness
	rand.Seed(uint64(time.Now().UnixNano()))
	variation := float64(baseTTL) * 0.1 // 10% variation
	minTTL := float64(baseTTL) - variation
	maxTTL := float64(baseTTL) + variation

	// Generate random value in range
	randValue := rand.Float64()*(maxTTL-minTTL) + minTTL
	return time.Duration(randValue)
}

// MarshalMessages converts messages to JSON for caching
func MarshalMessages(messages []model.Message) ([]byte, error) {
	return json.Marshal(messages)
}

// UnmarshalMessages parses JSON into messages slice
func UnmarshalMessages(data []byte) ([]model.Message, error) {
	var messages []model.Message
	err := json.Unmarshal(data, &messages)
	return messages, err
}
