package entity

import "time"

// ActivityReport aggregates the caller's own claimed deposits over
// [Period.From, Period.To). A user is measured by deposits they claimed
// (claimed_at); a mitra by deposits of their own location that were claimed
// (claimed_at). Unclaimed deposits never count. Wallet balance and lifetime
// totals are deliberately not part of the report.
type ActivityReport struct {
	Account     ActivityReportAccount    `json:"account"`
	Period      ActivityReportPeriod     `json:"period"`
	GeneratedAt time.Time                `json:"generated_at"`
	Totals      ActivityReportTotals     `json:"totals"`
	Daily       []ActivityReportDay      `json:"daily"`
	Categories  []ActivityReportCategory `json:"categories"`
}

// ActivityReportAccount names the caller only; claimer identities of a
// mitra's deposits never appear.
type ActivityReportAccount struct {
	Role         string  `json:"role"`
	FullName     string  `json:"full_name"`
	BusinessName *string `json:"business_name,omitempty"`
}

// ActivityReportPeriod: From inclusive, To exclusive, both rendered in WIB
// (+07:00). Daily buckets are WIB calendar dates clipped to the period, so the
// first and last day may be partial.
type ActivityReportPeriod struct {
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	Timezone string    `json:"timezone"`
	Days     int       `json:"days"`
}

// Points fields depend on role: users get points_earned (credited to their
// wallet by the claims) and points_spent (redemptions, positive number);
// mitra get points_credited (points their claimed deposits gave to users).
type ActivityReportTotals struct {
	ClaimedDeposits int64  `json:"claimed_deposits"`
	TotalWeightGram int64  `json:"total_weight_grams"`
	TotalWeightKG   string `json:"total_weight_kg"`
	PointsEarned    *int64 `json:"points_earned,omitempty"`
	PointsCredited  *int64 `json:"points_credited,omitempty"`
	PointsSpent     *int64 `json:"points_spent,omitempty"`
}

type ActivityReportDay struct {
	Date            string `json:"date"`
	ClaimedDeposits int64  `json:"claimed_deposits"`
	TotalWeightGram int64  `json:"total_weight_grams"`
	TotalWeightKG   string `json:"total_weight_kg"`
	PointsEarned    *int64 `json:"points_earned,omitempty"`
	PointsCredited  *int64 `json:"points_credited,omitempty"`
	PointsSpent     *int64 `json:"points_spent,omitempty"`
}

// ActivityReportCategory comes from the category name snapshotted on each
// deposit item; DepositCount counts distinct deposits containing the category.
type ActivityReportCategory struct {
	CategoryID    string `json:"category_id"`
	CategoryName  string `json:"category_name"`
	DepositCount  int64  `json:"deposit_count"`
	TotalWeightG  int64  `json:"total_weight_grams"`
	TotalWeightKG string `json:"total_weight_kg"`
}
