package models

import "time"

type Hotel struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`    // Foreign key to the user who owns or created the hotel record.
	Data      string    `json:"data"`       // JSON or string-encoded hotel information.
	CreatedAt time.Time `json:"created_at"` // Timestamp indicating when the hotel record was created.
}
