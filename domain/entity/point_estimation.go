package entity

import "time"

type PointEstimateInput struct {
	CategoryID string `json:"category_id"`
	WeightKG   string `json:"weight_kg"`
}
type PointEstimateRequest struct {
	PartnerID *string              `json:"partner_id,omitempty"`
	Items     []PointEstimateInput `json:"items"`
}
type PointEstimateItem struct {
	CategoryID   string `json:"category_id"`
	CategoryName string `json:"category_name"`
	WeightKG     string `json:"weight_kg"`
	RateID       string `json:"rate_id"`
	PointsPerKG  string `json:"points_per_kg"`
	ExactPoints  string `json:"exact_points"`
	IsDemo       bool   `json:"is_demo"`
}
type PointEstimate struct {
	PartnerID       *string             `json:"partner_id,omitempty"`
	AsOf            time.Time           `json:"as_of"`
	TotalWeightKG   string              `json:"total_weight_kg"`
	EstimatedPoints int64               `json:"estimated_points"`
	UsesDemoRates   bool                `json:"uses_demo_rates"`
	Items           []PointEstimateItem `json:"items"`
}
