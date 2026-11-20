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
