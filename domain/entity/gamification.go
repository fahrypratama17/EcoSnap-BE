package entity

import "time"

// Mission statuses.
const (
	MissionInProgress = "in_progress"
	MissionClaimable  = "claimable"
	MissionClaimed    = "claimed"
)

// Mission is one weekly mission for the caller. Progress is measured in Unit
// (deposit count or grams) over the current Asia/Jakarta week.
type Mission struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Metric      string     `json:"metric"`
	Unit        string     `json:"unit"`
	Target      int64      `json:"target"`
	Progress    int64      `json:"progress"`
	RewardXP    int64      `json:"reward_xp"`
	Status      string     `json:"status"`
	ClaimedAt   *time.Time `json:"claimed_at"`
}

// MissionList also carries both XP totals: this week's claimed XP and the
// all-time claimed XP (sum of persisted claims).
type MissionList struct {
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	ServerTime  time.Time `json:"server_time"`
	WeeklyXP    int64     `json:"weekly_xp"`
	TotalXP     int64     `json:"total_xp"`
	Missions    []Mission `json:"missions"`
}

type MissionClaimResult struct {
	Mission     Mission   `json:"mission"`
	XPAwarded   int64     `json:"xp_awarded"`
	WeeklyXP    int64     `json:"weekly_xp"`
	TotalXP     int64     `json:"total_xp"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	ServerTime  time.Time `json:"server_time"`
}

// Badge is permanent: Earned and UnlockedAt derive from immutable deposit
// claims (the claim that first crossed the threshold).
type Badge struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Metric      string     `json:"metric"`
	Target      int64      `json:"target"`
	Progress    int64      `json:"progress"`
	Earned      bool       `json:"earned"`
	UnlockedAt  *time.Time `json:"unlocked_at"`
}

type BadgeList struct {
	ServerTime         time.Time `json:"server_time"`
	ClaimedDeposits    int64     `json:"claimed_deposits"`
	ClaimedWeightGrams int64     `json:"claimed_weight_grams"`
	Badges             []Badge   `json:"badges"`
}

type LeaderboardEntry struct {
	Rank        int64   `json:"rank"`
	DisplayName string  `json:"display_name"`
	PhotoPath   *string `json:"photo_path"`
	XP          int64   `json:"xp"`
}

type LeaderboardMe struct {
	Rank int64 `json:"rank"`
	XP   int64 `json:"xp"`
}

type Leaderboard struct {
	PeriodStart time.Time          `json:"period_start"`
	PeriodEnd   time.Time          `json:"period_end"`
	ServerTime  time.Time          `json:"server_time"`
	Limit       int                `json:"limit"`
	Entries     []LeaderboardEntry `json:"entries"`
	Me          *LeaderboardMe     `json:"me"`
}

type LeaderboardVisibility struct {
	Visible bool `json:"visible"`
}
