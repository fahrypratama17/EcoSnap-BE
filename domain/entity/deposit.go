package entity

import "time"

// DepositCreateRequest carries only measured materials. Partner, tariffs, and
// totals are derived by the server from the authenticated mitra.
type DepositCreateRequest struct {
	Items []PointEstimateInput `json:"items"`
}

type Deposit struct {
	ID            string              `json:"id"`
	PartnerID     string              `json:"partner_id"`
	Status        string              `json:"status"`
	TotalWeightKG string              `json:"total_weight_kg"`
	TotalPoints   int64               `json:"total_points"`
	UsesDemoRates bool                `json:"uses_demo_rates"`
	RatesAsOf     time.Time           `json:"rates_as_of"`
	CreatedAt     time.Time           `json:"created_at"`
	Items         []PointEstimateItem `json:"items"`
}

// DepositQR is returned once to the issuing mitra; the token is not stored in
// plaintext and cannot be fetched again (reissue instead).
type DepositQR struct {
	DepositID string    `json:"deposit_id"`
	Token     string    `json:"token"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Warning   string    `json:"warning"`
}

type ClaimTokenRequest struct {
	Token string `json:"token"`
}

type DepositClaimPreview struct {
	DepositID     string              `json:"deposit_id"`
	PartnerID     string              `json:"partner_id"`
	PartnerName   string              `json:"partner_name"`
	TotalWeightKG string              `json:"total_weight_kg"`
	TotalPoints   int64               `json:"total_points"`
	UsesDemoRates bool                `json:"uses_demo_rates"`
	ExpiresAt     time.Time           `json:"expires_at"`
	Items         []PointEstimateItem `json:"items"`
}

type DepositClaim struct {
	DepositID     string    `json:"deposit_id"`
	PartnerID     string    `json:"partner_id"`
	Points        int64     `json:"points"`
	ClaimedAt     time.Time `json:"claimed_at"`
	WalletBalance int64     `json:"wallet_balance"`
}
