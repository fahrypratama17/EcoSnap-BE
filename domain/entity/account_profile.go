package entity

import "time"

type AccountProfile struct {
	UID         string                `json:"uid"`
	Role        string                `json:"role"`
	FullName    string                `json:"full_name"`
	DisplayName string                `json:"display_name"`
	Email       string                `json:"email"`
	Phone       string                `json:"phone"`
	Status      string                `json:"status"`
	CreatedAt   time.Time             `json:"created_at"`
	Wallet      WalletSummary         `json:"wallet"`
	Settings    AccountSettings       `json:"settings"`
	Business    *BusinessRegistration `json:"business,omitempty"`
}
type WalletSummary struct {
	Balance     int64 `json:"balance"`
	TotalEarned int64 `json:"total_earned"`
	TotalSpent  int64 `json:"total_spent"`
}
type AccountSettings struct {
	NotificationEnabled bool `json:"notification_enabled"`
	ShowOnLeaderboard   bool `json:"show_on_leaderboard"`
	OnboardingCompleted bool `json:"onboarding_completed"`
}
type BusinessRegistration struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Status   string `json:"status"`
}
