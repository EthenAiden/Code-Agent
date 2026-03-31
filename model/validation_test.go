package model

import (
	"testing"
	"time"
)

func TestIsValidUUID(t *testing.T) {
	validUUIDs := []string{
		"123e4567-e89b-12d3-a456-426614174000",
		"123e4567-e89b-12d3-a456-426614174001",
		"00000000-0000-4000-8000-000000000000",
		"ffffffff-ffff-4fff-bfff-ffffffffffff",
		"123e4567-e89b-4567-a456-426614174000",
	}

	for _, uuid := range validUUIDs {
		if !isValidUUID(uuid) {
			t.Errorf("Expected %s to be valid UUID v4", uuid)
		}
	}

	invalidUUIDs := []string{
		"",
		"invalid",
		"123e4567-e89b-12d3-a456-42661417400",
		"123e4567-e89b-12d3-a456-4266141740000",
		"123e4567-e89b-12d3-a456-42661417400g",
		"123e4567-e89b-12d3-a456-42661417400z",
		"123e4567-e89b-12d3-a456-42661417400!",
		"123e4567-e89b-12d3-a456-42661417400@",
		"123e4567-e89b-12d3-a456-42661417400#",
		"123e4567-e89b-12d3-a456-42661417400$",
	}

	for _, uuid := range invalidUUIDs {
		if isValidUUID(uuid) {
			t.Errorf("Expected %s to be invalid UUID v4", uuid)
		}
	}
}

func TestValidateConversationID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid UUID", "123e4567-e89b-12d3-a456-426614174000", false},
		{"empty string", "", true},
		{"invalid format", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConversationID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConversationID(%s) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestValidateUserID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid UUID", "123e4567-e89b-12d3-a456-426614174000", false},
		{"empty string", "", true},
		{"invalid format", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUserID(%s) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRole(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		wantErr bool
	}{
		{"user role", "user", false},
		{"assistant role", "assistant", false},
		{"invalid role", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRole(tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRole(%s) error = %v, wantErr %v", tt.role, err, tt.wantErr)
			}
		})
	}
}

func TestValidateStatus(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		wantErr bool
	}{
		{"pending status", "pending", false},
		{"completed status", "completed", false},
		{"failed status", "failed", false},
		{"invalid status", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatus(tt.status)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStatus(%s) error = %v, wantErr %v", tt.status, err, tt.wantErr)
			}
		})
	}
}

func TestValidateMessage(t *testing.T) {
	now := time.Now()
	validMsg := Message{
		ID:             1,
		ConversationID: "123e4567-e89b-12d3-a456-426614174000",
		Role:           "user",
		Content:        "Hello",
		Timestamp:      now,
		MessageIndex:   0,
		Status:         "pending",
	}

	err := ValidateMessage(&validMsg)
	if err != nil {
		t.Errorf("ValidateMessage(valid) error = %v, want nil", err)
	}

	invalidMsgs := []Message{
		{ConversationID: "", Role: "user", Content: "Hello", Timestamp: now, MessageIndex: 0, Status: "pending"},
		{ConversationID: "123e4567-e89b-12d3-a456-426614174000", Role: "invalid", Content: "Hello", Timestamp: now, MessageIndex: 0, Status: "pending"},
		{ConversationID: "123e4567-e89b-12d3-a456-426614174000", Role: "user", Content: "", Timestamp: now, MessageIndex: 0, Status: "pending"},
		{ConversationID: "123e4567-e89b-12d3-a456-426614174000", Role: "user", Content: "Hello", Timestamp: now, MessageIndex: -1, Status: "pending"},
		{ConversationID: "123e4567-e89b-12d3-a456-426614174000", Role: "user", Content: "Hello", Timestamp: now, MessageIndex: 0, Status: "invalid"},
	}

	for _, msg := range invalidMsgs {
		err := ValidateMessage(&msg)
		if err == nil {
			t.Errorf("ValidateMessage(invalid) should return error, got nil")
		}
	}
}
