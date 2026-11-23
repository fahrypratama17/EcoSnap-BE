package entity

import "time"

// HomeMonth is the current WIB calendar month: From inclusive (first day
// 00:00 WIB), To exclusive (the server time the home was sampled at).
type HomeMonth struct {
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	Timezone string    `json:"timezone"`
}

// UserHome is the user beranda: all-time totals, this month's claims, the
// latest ledger entries and one card each for missions, partner, article and
// unread notifications, all read from one snapshot. Missions is null for
// mitra (they use the partner home). Partner and Article are null when none
// is visible. DepositsClaimed and TotalWeightKG are all-time; MonthlyDeposits
// counts claims inside Month.
type UserHome struct {
	UserName        string              `json:"user_name"`
	ServerTime      time.Time           `json:"server_time"`
	Month           HomeMonth           `json:"month"`
	Wallet          Wallet              `json:"wallet"`
	DepositsClaimed int64               `json:"deposits_claimed"`
	MonthlyDeposits int64               `json:"monthly_deposits"`
	TotalWeightKG   string              `json:"total_weight_kg"`
	ActiveVouchers  int64               `json:"active_vouchers"`
	RecentActivity  []PointHistoryEntry `json:"recent_activity"`
	Missions        *MissionList        `json:"missions"`
	Partner         *PartnerDetail      `json:"partner"`
	Article         *ArticleSummary     `json:"article"`
	UnreadCount     int64               `json:"unread_count"`
}

type PartnerDepositCounts struct {
	Created  int64 `json:"created"`
	QRIssued int64 `json:"qr_issued"`
	Claimed  int64 `json:"claimed"`
}

// PartnerHome is the mitra beranda; available for non-verified locations too
// (read-only, D-21) so the app can show the status.
type PartnerHome struct {
	PartnerID       string                  `json:"partner_id"`
	BusinessName    string                  `json:"business_name"`
	PartnerStatus   string                  `json:"partner_status"`
	Deposits        PartnerDepositCounts    `json:"deposits"`
	ClaimedWeightKG string                  `json:"claimed_weight_kg"`
	ClaimedPoints   int64                   `json:"claimed_points"`
	RecentDeposits  []PartnerDepositSummary `json:"recent_deposits"`
}
