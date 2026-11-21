package entity

// UserHome is the user beranda: all-time totals plus the latest ledger
// entries. Missions/badges join once Q-07 is decided.
type UserHome struct {
	Wallet          Wallet              `json:"wallet"`
	DepositsClaimed int64               `json:"deposits_claimed"`
	TotalWeightKG   string              `json:"total_weight_kg"`
	ActiveVouchers  int64               `json:"active_vouchers"`
	RecentActivity  []PointHistoryEntry `json:"recent_activity"`
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
