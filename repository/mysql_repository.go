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

// InsertSession creates a new project in MySQL
// Note: The table uses 'uuid' column for the external project_id (UUID string)
// and 'id' column as internal auto-increment primary key
func (spr *MySQLSessionPersistenceRepository) InsertSession(ctx context.Context, projectID string, userID string) error {
	return spr.retryWithBackoff(ctx, 3, func() error {
		query := `INSERT INTO projects (uuid, user_id, name, version) VALUES (?, ?, 'New Project', 1)`
		_, err := spr.db.ExecContext(ctx, query, projectID, userID)
		if err != nil {
			return fmt.Errorf("failed to insert project: %w", err)
		}
		return nil
	})
}

// InsertMessage stores a message in MySQL
// Note: messages.project_id is a BIGINT foreign key to projects.id (internal ID),
// but we receive UUID (projects.uuid). We need to join to get the internal ID.
func (spr *MySQLSessionPersistenceRepository) InsertMessage(ctx context.Context, msg model.Message) error {
	return spr.retryWithBackoff(ctx, 3, func() error {
		// Use transaction for atomicity
		tx, err := spr.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback()

		// Get internal project ID from UUID
		var internalProjectID int64
		err = tx.QueryRowContext(ctx, `
			SELECT id FROM projects WHERE uuid = ?`, msg.ConversationID).Scan(&internalProjectID)
		if err != nil {
			return fmt.Errorf("failed to get project internal id: %w", err)
		}

		// Get next message index with lock
		var nextIndex int
		err = tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(message_index), -1) + 1 
			FROM messages 
			WHERE project_id = ? FOR UPDATE`, internalProjectID).Scan(&nextIndex)
		if err != nil {
			return fmt.Errorf("failed to get next message index: %w", err)
		}

		// Insert message
		query := `INSERT INTO messages (project_id, role, content, timestamp, message_index, status) VALUES (?, ?, ?, ?, ?, ?)`
		_, err = tx.ExecContext(ctx, query, internalProjectID, msg.Role, msg.Content, msg.Timestamp, nextIndex, msg.Status)
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

// GetSessionMessages retrieves all messages for a project
// Note: projectID is UUID, need to join with projects table
func (spr *MySQLSessionPersistenceRepository) GetSessionMessages(ctx context.Context, projectID string, userID string) ([]model.Message, error) {
	var messages []model.Message

	err := spr.retryWithBackoff(ctx, 3, func() error {
		query := `
			SELECT m.id, p.uuid, m.role, m.content, m.timestamp, m.message_index, m.status 
			FROM messages m
			INNER JOIN projects p ON m.project_id = p.id
			WHERE p.uuid = ? AND p.user_id = ? AND m.status != 'failed'
			ORDER BY m.message_index ASC`

		rows, err := spr.db.QueryContext(ctx, query, projectID, userID)
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

// ListSessions retrieves project summaries with pagination
func (spr *MySQLSessionPersistenceRepository) ListSessions(ctx context.Context, userID string, limit, offset int) ([]model.SessionSummary, error) {
	var summaries []model.SessionSummary

	err := spr.retryWithBackoff(ctx, 3, func() error {
		// Get paginated projects with summary data
		// Use uuid as the external project_id
		query := `
			SELECT p.uuid, 
				   COALESCE(MAX(m.timestamp), p.created_at) as last_message_timestamp,
				   COUNT(m.id) as message_count
			FROM projects p
			LEFT JOIN messages m ON p.id = m.project_id AND m.status != 'failed'
			WHERE p.user_id = ? AND p.is_deleted = FALSE
			GROUP BY p.id, p.uuid
			ORDER BY p.created_at DESC
			LIMIT ? OFFSET ?`

		rows, err := spr.db.QueryContext(ctx, query, userID, limit, offset)
		if err != nil {
			return fmt.Errorf("failed to query projects: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var summary model.SessionSummary
			err := rows.Scan(&summary.ConversationID, &summary.LastMessageTimestamp, &summary.MessageCount)
			if err != nil {
				return fmt.Errorf("failed to scan project summary: %w", err)
			}
			summaries = append(summaries, summary)
		}

		if err = rows.Err(); err != nil {
			return fmt.Errorf("error iterating projects: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return summaries, nil
}

// DeleteSession removes a project and all its messages
func (spr *MySQLSessionPersistenceRepository) DeleteSession(ctx context.Context, projectID string, userID string) error {
	return spr.retryWithBackoff(ctx, 3, func() error {
		// Delete project (cascade will handle messages)
		result, err := spr.db.ExecContext(ctx, `
			DELETE FROM projects WHERE uuid = ? AND user_id = ?`, projectID, userID)
		if err != nil {
			return fmt.Errorf("failed to delete project: %w", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}

		if rowsAffected == 0 {
			return fmt.Errorf("project not found or not owned by user")
		}

		return nil
	})
}

// GetSessionDetails retrieves project metadata
func (spr *MySQLSessionPersistenceRepository) GetSessionDetails(ctx context.Context, projectID string, userID string) (*model.SessionDetails, error) {
	var details model.SessionDetails
	var lastMessageAt sql.NullTime

	err := spr.retryWithBackoff(ctx, 3, func() error {
		query := `
			SELECT p.uuid,
				   COALESCE((SELECT COUNT(*) FROM messages m WHERE m.project_id = p.id AND m.status != 'failed'), 0) as message_count,
				   p.created_at,
				   (SELECT MAX(m.timestamp) FROM messages m WHERE m.project_id = p.id AND m.status != 'failed') as last_message_at
			FROM projects p
			WHERE p.uuid = ? AND p.user_id = ?`

		err := spr.db.QueryRowContext(ctx, query, projectID, userID).Scan(
			&details.ConversationID,
			&details.MessageCount,
			&details.CreatedAt,
			&lastMessageAt,
		)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("project not found: %w", err)
			}
			return fmt.Errorf("failed to get project details: %w", err)
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

// SetFramework stores the chosen framework for a project.
func (spr *MySQLSessionPersistenceRepository) SetFramework(ctx context.Context, projectID string, userID string, framework string) error {
	return spr.retryWithBackoff(ctx, 3, func() error {
		_, err := spr.db.ExecContext(ctx,
			`UPDATE projects SET description = ? WHERE uuid = ? AND user_id = ?`,
			framework, projectID, userID)
		if err != nil {
			return fmt.Errorf("failed to set framework: %w", err)
		}
		return nil
	})
}

// SessionExists checks if a project exists
func (spr *MySQLSessionPersistenceRepository) SessionExists(ctx context.Context, projectID string, userID string) (bool, error) {
	var exists bool

	err := spr.retryWithBackoff(ctx, 3, func() error {
		err := spr.db.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM projects WHERE uuid = ? AND user_id = ?)`, projectID, userID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check project existence: %w", err)
		}
		return nil
	})

	if err != nil {
		return false, err
	}

	return exists, nil
}

// GetSessionVersion retrieves the current version for optimistic locking
func (spr *MySQLSessionPersistenceRepository) GetSessionVersion(ctx context.Context, projectID string, userID string) (int, error) {
	var version int

	err := spr.retryWithBackoff(ctx, 3, func() error {
		err := spr.db.QueryRowContext(ctx, `
			SELECT version FROM projects WHERE uuid = ? AND user_id = ?`, projectID, userID).Scan(&version)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("project not found: %w", err)
			}
			return fmt.Errorf("failed to get project version: %w", err)
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return version, nil
}

// UpdateSessionVersion updates the project version for optimistic locking
func (spr *MySQLSessionPersistenceRepository) UpdateSessionVersion(ctx context.Context, projectID string, userID string, expectedVersion int) error {
	return spr.retryWithBackoff(ctx, 3, func() error {
		result, err := spr.db.ExecContext(ctx, `
			UPDATE projects SET version = version + 1 WHERE uuid = ? AND user_id = ? AND version = ?`,
			projectID, userID, expectedVersion)
		if err != nil {
			return fmt.Errorf("failed to update project version: %w", err)
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

// GetNextMessageIndex gets the next message index for a project
func (spr *MySQLSessionPersistenceRepository) GetNextMessageIndex(ctx context.Context, projectID string, userID string) (int, error) {
	var nextIndex int

	err := spr.retryWithBackoff(ctx, 3, func() error {
		err := spr.db.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(m.message_index), -1) + 1 
			FROM messages m
			INNER JOIN projects p ON m.project_id = p.id
			WHERE p.uuid = ?`, projectID).Scan(&nextIndex)
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

// CleanupFailedMessages removes failed messages from a project
func (spr *MySQLSessionPersistenceRepository) CleanupFailedMessages(ctx context.Context, projectID string, userID string) error {
	return spr.retryWithBackoff(ctx, 3, func() error {
		_, err := spr.db.ExecContext(ctx, `
			DELETE m FROM messages m
			INNER JOIN projects p ON m.project_id = p.id
			WHERE p.uuid = ? AND m.status = 'failed'`, projectID)
		if err != nil {
			return fmt.Errorf("failed to cleanup failed messages: %w", err)
		}
		return nil
	})
}

// UpdateMessageStatus updates the status of a message
func (spr *MySQLSessionPersistenceRepository) UpdateMessageStatus(ctx context.Context, projectID string, messageIndex int, status string) error {
	return spr.retryWithBackoff(ctx, 3, func() error {
		// Update message status by joining with projects table
		result, err := spr.db.ExecContext(ctx, `
			UPDATE messages m
			INNER JOIN projects p ON m.project_id = p.id
			SET m.status = ?
			WHERE p.uuid = ? AND m.message_index = ?`,
			status, projectID, messageIndex)
		if err != nil {
			return fmt.Errorf("failed to update message status: %w", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}

		if rowsAffected == 0 {
			return fmt.Errorf("message not found")
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
