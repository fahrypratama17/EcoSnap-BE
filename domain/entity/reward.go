package entity

import "time"

type Reward struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Terms       string `json:"terms"`
	CostPoints  int64  `json:"cost_points"`
	Stock       int64  `json:"stock"`
	Available   bool   `json:"available"`
	IsDemo      bool   `json:"is_demo"`
}

// Voucher is a redemption seen by its owner. Status is issued, used, or
// expired; expired is derived from expires_at at read time.
type Voucher struct {
	ID         string     `json:"id"`
	RewardID   string     `json:"reward_id"`
	RewardName string     `json:"reward_name"`
	CostPoints int64      `json:"cost_points"`
	Code       string     `json:"code"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	UsedAt     *time.Time `json:"used_at"`
}

type Redemption struct {
	Voucher       Voucher `json:"voucher"`
	WalletBalance int64   `json:"wallet_balance"`
}
