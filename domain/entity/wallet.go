package entity

import "time"

type Wallet struct {
	Balance     int64     `json:"balance"`
	TotalEarned int64     `json:"total_earned"`
	TotalSpent  int64     `json:"total_spent"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PointHistoryDeposit struct {
	ID            string `json:"id"`
	PartnerID     string `json:"partner_id"`
	PartnerName   string `json:"partner_name"`
	TotalWeightKG string `json:"total_weight_kg"`
}

// PointHistoryEntry is one ledger row. Amount is signed: credits positive,
// later debits (rewards, appeal corrections) negative.
type PointHistoryEntry struct {
	ID        string               `json:"id"`
	Amount    int64                `json:"amount"`
	Source    string               `json:"source"`
	CreatedAt time.Time            `json:"created_at"`
	Deposit   *PointHistoryDeposit `json:"deposit,omitempty"`
	// Redemption is set for source=redemption (amount negative).
	Redemption *PointHistoryRedemption `json:"redemption,omitempty"`
}

type PointHistoryRedemption struct {
	ID         string `json:"id"`
	RewardName string `json:"reward_name"`
}

// PartnerDepositSummary omits the claimer identity: a mitra sees whether its
// deposit was claimed, not by whom.
type PartnerDepositSummary struct {
	ID            string     `json:"id"`
	Status        string     `json:"status"`
	TotalWeightKG string     `json:"total_weight_kg"`
	TotalPoints   int64      `json:"total_points"`
	UsesDemoRates bool       `json:"uses_demo_rates"`
	CreatedAt     time.Time  `json:"created_at"`
	ClaimedAt     *time.Time `json:"claimed_at"`
}
