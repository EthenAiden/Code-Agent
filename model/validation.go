package model

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrInvalidConversationID = errors.New("conversation_id must be a valid UUID v4")
	ErrInvalidUserID         = errors.New("user_id must be a valid UUID v4")
	ErrInvalidMessage        = errors.New("message cannot be empty")
	ErrInvalidRole           = errors.New("role must be 'user' or 'assistant'")
	ErrInvalidStatus         = errors.New("status must be 'pending', 'completed', or 'failed'")
	ErrInvalidLimit          = errors.New("limit must be between 1 and 100")
	ErrInvalidOffset         = errors.New("offset must be non-negative")
)

// isValidUUID validates if a string is a valid UUID v4
func isValidUUID(s string) bool {
	// UUID v4 format: xxxxxxxx-xxxx-4xxx-[89ab]xxx-xxxxxxxxxxxx
	// The 13th character indicates version (4 for UUID v4)
	// The 17th character indicates variant (8, 9, a, or b for RFC 4122)
	uuidRegex := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	return uuidRegex.MatchString(s)
}

// ValidateCreateSessionRequest validates CreateSessionRequest
func ValidateCreateSessionRequest(req *CreateSessionRequest) error {
	if req == nil {
		return nil
	}
	return nil
}

// ValidateListSessionsRequest validates ListSessionsRequest
func ValidateListSessionsRequest(req *ListSessionsRequest) error {
	if req == nil {
		return nil
	}
	if req.Limit < 0 {
		return ErrInvalidLimit
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	if req.Offset < 0 {
		return ErrInvalidOffset
	}
	return nil
}

// ValidateChatRequest validates ChatRequest
func ValidateChatRequest(req *ChatRequest) error {
	if req == nil {
		return errors.New("request cannot be nil")
	}
	if strings.TrimSpace(req.Message) == "" {
		return ErrInvalidMessage
	}
	return nil
}

// ValidateConversationID validates conversation_id format
func ValidateConversationID(conversationID string) error {
	if conversationID == "" {
		return errors.New("conversation_id cannot be empty")
	}
	if !isValidUUID(conversationID) {
		return ErrInvalidConversationID
	}
	return nil
}

// ValidateUserID validates user_id format
func ValidateUserID(userID string) error {
	if userID == "" {
		return errors.New("user_id cannot be empty")
	}
	if !isValidUUID(userID) {
		return ErrInvalidUserID
	}
	return nil
}

// ValidateRole validates message role
func ValidateRole(role string) error {
	if role != "user" && role != "assistant" {
		return ErrInvalidRole
	}
	return nil
}

// ValidateStatus validates message status
func ValidateStatus(status string) error {
	if status != "pending" && status != "completed" && status != "failed" {
		return ErrInvalidStatus
	}
	return nil
}

// ValidateMessage validates a Message struct
func ValidateMessage(msg *Message) error {
	if msg == nil {
		return errors.New("message cannot be nil")
	}
	if msg.ConversationID == "" {
		return errors.New("conversation_id cannot be empty")
	}
	if !isValidUUID(msg.ConversationID) {
		return ErrInvalidConversationID
	}
	if err := ValidateRole(msg.Role); err != nil {
		return err
	}
	if msg.Content == "" {
		return errors.New("content cannot be empty")
	}
	if msg.MessageIndex < 0 {
		return errors.New("message_index must be non-negative")
	}
	if err := ValidateStatus(msg.Status); err != nil {
		return err
	}
	return nil
}
