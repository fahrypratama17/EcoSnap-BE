package entity

import "time"

// FAQ is a reviewed help entry. IsDemo marks content written for the demo
// build, not final legal or operational policy.
type FAQ struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	IsDemo   bool   `json:"is_demo"`
}

// SupportTicket is what the owner of the ticket sees. StaffResponse and
// RespondedAt are nil until the repo owner replies.
type SupportTicket struct {
	ID            string     `json:"id"`
	Kind          string     `json:"kind"`
	Subject       string     `json:"subject"`
	Message       string     `json:"message"`
	Status        string     `json:"status"`
	StaffResponse *string    `json:"staff_response"`
	RespondedAt   *time.Time `json:"responded_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// SupportTicketOwnerView adds the author for the owner CLI. It is never
// served over HTTP.
type SupportTicketOwnerView struct {
	SupportTicket
	UserUID string `json:"user_uid"`
}
