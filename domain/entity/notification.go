package entity

import "time"

// NotificationTarget names the record a notification refers to. ID is the
// deposit or voucher UUID, or the mission id; the client resolves it through
// its own scoped endpoints.
type NotificationTarget struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// Notification is one in-app inbox entry. SeenAt is null until the owner
// first marks it read.
type Notification struct {
	ID        string             `json:"id"`
	Type      string             `json:"type"`
	Title     string             `json:"title"`
	Body      string             `json:"body"`
	Target    NotificationTarget `json:"target"`
	CreatedAt time.Time          `json:"created_at"`
	SeenAt    *time.Time         `json:"seen_at"`
}
