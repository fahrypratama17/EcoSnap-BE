package service

import (
	"context"
	"errors"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
)

var (
	ErrMissionNotFound          = errors.New("misi tidak ditemukan")
	ErrMissionExpired           = errors.New("periode misi sudah berakhir; muat ulang misi minggu ini")
	ErrInvalidMissionClaim      = errors.New("body klaim harus berisi period_start RFC3339")
	ErrMissionIncomplete        = errors.New("misi belum selesai")
	ErrGamificationForbidden    = errors.New("hanya akun pengguna yang dapat memakai misi, lencana, dan papan peringkat")
	ErrInvalidGamificationQuery = errors.New("parameter limit harus bilangan bulat 1 sampai 100")
)

// Weekly mission IDs are stable API values.
const (
	MissionFirstDeposit   = "first_deposit"
	MissionThreeKilograms = "three_kilograms"

	MetricClaimedDeposits   = "claimed_deposit_count"
	MetricClaimedWeightGram = "claimed_weight_grams"

	// Badge thresholds are all-time over claimed deposits.
	BadgeDepositsSmall = 1
	BadgeDepositsLarge = 10
	BadgeWeightGrams   = 10000

	LeaderboardDefaultLimit = 20
	LeaderboardMaxLimit     = 100
)

type MissionDefinition struct {
	ID, Title, Description, Metric, Unit string
	Target, RewardXP                     int64
}

// Missions is the fixed weekly catalogue (D24/D25). Games are deferred.
var Missions = []MissionDefinition{
	{MissionFirstDeposit, "Setoran Pertama Minggu Ini", "Selesaikan klaim setoran pertamamu minggu ini.", MetricClaimedDeposits, "deposit", 1, 50},
	{MissionThreeKilograms, "Tiga Kilogram Minggu Ini", "Kumpulkan total 3 kg sampah dari setoran yang sudah diklaim minggu ini.", MetricClaimedWeightGram, "gram", 3000, 100},
}

// MissionSnapshot is a consistent view of one user's current week. Claimed
// maps mission ID to claim time for the current week only.
type MissionSnapshot struct {
	PeriodStart, PeriodEnd, Now time.Time
	Deposits, WeightGrams       int64
	Claimed                     map[string]time.Time
	WeeklyXP, TotalXP           int64
}

// BadgeSnapshot holds all-time claimed totals and the claim time that first
// crossed each threshold (nil until crossed).
type BadgeSnapshot struct {
	Now                        time.Time
	Deposits, WeightGrams      int64
	SmallAt, LargeAt, WeightAt *time.Time
}

type LeaderboardSnapshot struct {
	PeriodStart, PeriodEnd, Now time.Time
	Entries                     []entity.LeaderboardEntry
	Me                          *entity.LeaderboardMe
}

type GamificationStore interface {
	Missions(ctx context.Context, uid string) (MissionSnapshot, error)
	ClaimMission(ctx context.Context, uid string, def MissionDefinition, periodStart time.Time) (MissionSnapshot, bool, error)
	Badges(ctx context.Context, uid string) (BadgeSnapshot, error)
	Leaderboard(ctx context.Context, uid string, limit int) (LeaderboardSnapshot, error)
	SetLeaderboardVisibility(ctx context.Context, uid string, visible bool) error
}

type GamificationService struct{ store GamificationStore }

func NewGamificationService(store GamificationStore) *GamificationService {
	return &GamificationService{store: store}
}

func missionView(def MissionDefinition, s MissionSnapshot) entity.Mission {
	progress := s.Deposits
	if def.Metric == MetricClaimedWeightGram {
		progress = s.WeightGrams
	}
	m := entity.Mission{ID: def.ID, Title: def.Title, Description: def.Description, Metric: def.Metric, Unit: def.Unit,
		Target: def.Target, Progress: progress, RewardXP: def.RewardXP, Status: entity.MissionInProgress}
	if at, ok := s.Claimed[def.ID]; ok {
		at := at
		m.Status, m.ClaimedAt = entity.MissionClaimed, &at
	} else if progress >= def.Target {
		m.Status = entity.MissionClaimable
	}
	return m
}

// MissionListFromSnapshot is the single place a snapshot becomes the wire list.
func MissionListFromSnapshot(snap MissionSnapshot) entity.MissionList {
	out := entity.MissionList{PeriodStart: snap.PeriodStart, PeriodEnd: snap.PeriodEnd, ServerTime: snap.Now,
		WeeklyXP: snap.WeeklyXP, TotalXP: snap.TotalXP, Missions: make([]entity.Mission, 0, len(Missions))}
	for _, def := range Missions {
		out.Missions = append(out.Missions, missionView(def, snap))
	}
	return out
}

func (s *GamificationService) Missions(ctx context.Context, uid string) (entity.MissionList, error) {
	snap, err := s.store.Missions(ctx, uid)
	if err != nil {
		return entity.MissionList{}, err
	}
	return MissionListFromSnapshot(snap), nil
}

// ClaimMission awards XP once per mission and week. periodStart is the week
// token the client saw; a stale token yields ErrMissionExpired. created is false when the
// mission was already claimed this week (replay: same result, no extra XP).
func (s *GamificationService) ClaimMission(ctx context.Context, uid, missionID string, periodStart time.Time) (entity.MissionClaimResult, bool, error) {
	for _, def := range Missions {
		if def.ID != missionID {
			continue
		}
		snap, created, err := s.store.ClaimMission(ctx, uid, def, periodStart)
		if err != nil {
			return entity.MissionClaimResult{}, false, err
		}
		return entity.MissionClaimResult{Mission: missionView(def, snap), XPAwarded: def.RewardXP, WeeklyXP: snap.WeeklyXP,
			TotalXP: snap.TotalXP, PeriodStart: snap.PeriodStart, PeriodEnd: snap.PeriodEnd, ServerTime: snap.Now}, created, nil
	}
	return entity.MissionClaimResult{}, false, ErrMissionNotFound
}

func (s *GamificationService) Badges(ctx context.Context, uid string) (entity.BadgeList, error) {
	snap, err := s.store.Badges(ctx, uid)
	if err != nil {
		return entity.BadgeList{}, err
	}
	badge := func(id, title, desc, metric string, target, progress int64, at *time.Time) entity.Badge {
		return entity.Badge{ID: id, Title: title, Description: desc, Metric: metric, Target: target, Progress: progress, Earned: at != nil, UnlockedAt: at}
	}
	return entity.BadgeList{ServerTime: snap.Now, ClaimedDeposits: snap.Deposits, ClaimedWeightGrams: snap.WeightGrams,
		Badges: []entity.Badge{
			badge("deposit_1", "Langkah Pertama", "Klaim setoran pertamamu.", MetricClaimedDeposits, BadgeDepositsSmall, snap.Deposits, snap.SmallAt),
			badge("deposit_10", "Konsisten", "Klaim 10 setoran.", MetricClaimedDeposits, BadgeDepositsLarge, snap.Deposits, snap.LargeAt),
			badge("weight_10kg", "Pejuang Daur Ulang", "Kumpulkan total 10 kg dari setoran yang diklaim.", MetricClaimedWeightGram, BadgeWeightGrams, snap.WeightGrams, snap.WeightAt),
		}}, nil
}

// Leaderboard ranks the current week by claimed mission XP. limit must be 1..100.
func (s *GamificationService) Leaderboard(ctx context.Context, uid string, limit int) (entity.Leaderboard, error) {
	if limit < 1 || limit > LeaderboardMaxLimit {
		return entity.Leaderboard{}, ErrInvalidGamificationQuery
	}
	snap, err := s.store.Leaderboard(ctx, uid, limit)
	if err != nil {
		return entity.Leaderboard{}, err
	}
	if snap.Entries == nil {
		snap.Entries = []entity.LeaderboardEntry{}
	}
	return entity.Leaderboard{PeriodStart: snap.PeriodStart, PeriodEnd: snap.PeriodEnd, ServerTime: snap.Now,
		Limit: limit, Entries: snap.Entries, Me: snap.Me}, nil
}

// WIB is fixed UTC+7 (Asia/Jakarta has no DST), so no tzdata is required.
var WIB = time.FixedZone("WIB", 7*60*60)

// WeekPeriod returns the Monday 00:00 WIB week containing now: [start, end).
func WeekPeriod(now time.Time) (start, end time.Time) {
	local := now.In(WIB)
	back := (int(local.Weekday()) + 6) % 7 // Monday=0
	start = time.Date(local.Year(), local.Month(), local.Day()-back, 0, 0, 0, 0, WIB)
	return start, start.AddDate(0, 0, 7)
}

// SetLeaderboardVisibility is the only user control over leaderboard privacy.
func (s *GamificationService) SetLeaderboardVisibility(ctx context.Context, uid string, visible bool) (entity.LeaderboardVisibility, error) {
	if err := s.store.SetLeaderboardVisibility(ctx, uid, visible); err != nil {
		return entity.LeaderboardVisibility{}, err
	}
	return entity.LeaderboardVisibility{Visible: visible}, nil
}
