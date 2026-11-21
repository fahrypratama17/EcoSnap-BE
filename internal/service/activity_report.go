package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
)

var ErrInvalidActivityReportQuery = errors.New("parameter laporan aktivitas tidak valid")

// MaxActivityReportSpan is the longest allowed range (366 days).
const MaxActivityReportSpan = 366 * 24 * time.Hour

// ActivityReportQuery holds optional from (inclusive) and to (exclusive).
// Both or neither must be set.
type ActivityReportQuery struct{ From, To *time.Time }

// ActivityDayRecord is one WIB date's sums as read from storage. Storage may
// return the same date more than once (claims and redemptions come from
// different tables); the service adds them.
type ActivityDayRecord struct {
	Date    string // YYYY-MM-DD, WIB
	Claimed int64
	Grams   int64
	Points  int64
	Spent   int64 // positive number of points spent
}

type ActivityCategoryRecord struct {
	ID, Name       string
	Deposits, Gram int64
}

// ActivityReportRecord is a consistent snapshot of the caller's account and
// activity inside [from, to).
type ActivityReportRecord struct {
	Role         string
	FullName     string
	BusinessName *string
	Days         []ActivityDayRecord
	Categories   []ActivityCategoryRecord
}

type ActivityReportStore interface {
	// ActivityReport reads the active caller's claimed deposits in [from, to)
	// in one snapshot. Users: deposits they claimed. Mitra: deposits of their
	// own location that were claimed (read-only, also for unverified
	// locations). Errors: ErrAccountNotFound, ErrAccountForbidden.
	ActivityReport(ctx context.Context, uid string, from, to time.Time) (ActivityReportRecord, error)
}

type ActivityReportService struct {
	store ActivityReportStore
	now   func() time.Time
}

func NewActivityReportService(store ActivityReportStore) *ActivityReportService {
	return &ActivityReportService{store: store, now: time.Now}
}

// WithClock replaces the clock (tests).
func (s *ActivityReportService) WithClock(now func() time.Time) *ActivityReportService {
	s.now = now
	return s
}

// ActivityReportPDF is a rendered download.
type ActivityReportPDF struct {
	Filename string
	Content  []byte
}

func wibDate(t time.Time) time.Time {
	y, m, d := t.In(WIB).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, WIB)
}

// resolve applies the default (current WIB calendar month up to now) and the
// validation rules. Times are truncated to microseconds, the database
// precision, so the daily buckets and the SQL filter see the same range.
func (s *ActivityReportService) resolve(q ActivityReportQuery) (from, to, now time.Time, err error) {
	now = s.now().Truncate(time.Microsecond)
	switch {
	case q.From == nil && q.To == nil:
		y, m, _ := now.In(WIB).Date()
		from = time.Date(y, m, 1, 0, 0, 0, 0, WIB)
		to = now
		if !to.After(from) {
			to = from.Add(time.Second)
		}
	case q.From != nil && q.To != nil:
		from = q.From.Truncate(time.Microsecond)
		to = q.To.Truncate(time.Microsecond)
	default:
		return from, to, now, ErrInvalidActivityReportQuery
	}
	if !to.After(from) || to.Sub(from) > MaxActivityReportSpan {
		return from, to, now, ErrInvalidActivityReportQuery
	}
	return from.In(WIB), to.In(WIB), now, nil
}

func (s *ActivityReportService) Report(ctx context.Context, uid string, q ActivityReportQuery) (entity.ActivityReport, error) {
	from, to, now, err := s.resolve(q)
	if err != nil {
		return entity.ActivityReport{}, err
	}
	rec, err := s.store.ActivityReport(ctx, uid, from, to)
	if err != nil {
		return entity.ActivityReport{}, err
	}
	return buildActivityReport(rec, from, to, now)
}

// PDF renders the same report (same range, totals, daily rows, categories) as
// a downloadable document. Nothing is stored.
func (s *ActivityReportService) PDF(ctx context.Context, uid string, q ActivityReportQuery) (ActivityReportPDF, error) {
	report, err := s.Report(ctx, uid, q)
	if err != nil {
		return ActivityReportPDF{}, err
	}
	content, err := RenderActivityReportPDF(report)
	if err != nil {
		return ActivityReportPDF{}, err
	}
	last := report.Period.To.Add(-time.Microsecond)
	name := fmt.Sprintf("ecosnap-activity-report-%s-%s.pdf", report.Period.From.Format("20060102"), last.In(WIB).Format("20060102"))
	return ActivityReportPDF{Filename: name, Content: content}, nil
}

func buildActivityReport(rec ActivityReportRecord, from, to, now time.Time) (entity.ActivityReport, error) {
	isUser := rec.Role == "user"
	if !isUser && rec.Role != "mitra" {
		return entity.ActivityReport{}, fmt.Errorf("peran laporan tidak dikenal: %q", rec.Role)
	}
	// Zero-filled WIB dates covering the clipped range.
	first, last := wibDate(from), wibDate(to.Add(-time.Microsecond))
	index := map[string]int{}
	var days []ActivityDayRecord
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		index[key] = len(days)
		days = append(days, ActivityDayRecord{Date: key})
	}
	for _, r := range rec.Days {
		i, ok := index[r.Date]
		if !ok {
			return entity.ActivityReport{}, fmt.Errorf("tanggal laporan di luar rentang: %s", r.Date)
		}
		days[i].Claimed += r.Claimed
		days[i].Grams += r.Grams
		days[i].Points += r.Points
		days[i].Spent += r.Spent
	}
	out := entity.ActivityReport{
		Account:     entity.ActivityReportAccount{Role: rec.Role, FullName: rec.FullName, BusinessName: rec.BusinessName},
		Period:      entity.ActivityReportPeriod{From: from, To: to, Timezone: "Asia/Jakarta", Days: len(days)},
		GeneratedAt: now.UTC(),
		Daily:       make([]entity.ActivityReportDay, 0, len(days)),
		Categories:  make([]entity.ActivityReportCategory, 0, len(rec.Categories)),
	}
	var claimed, grams, points, spent int64
	for _, d := range days {
		claimed, grams, points, spent = claimed+d.Claimed, grams+d.Grams, points+d.Points, spent+d.Spent
		day := entity.ActivityReportDay{Date: d.Date, ClaimedDeposits: d.Claimed, TotalWeightGram: d.Grams, TotalWeightKG: FormatWeightKG(d.Grams)}
		setPoints(isUser, d.Points, d.Spent, &day.PointsEarned, &day.PointsCredited, &day.PointsSpent)
		out.Daily = append(out.Daily, day)
	}
	out.Totals = entity.ActivityReportTotals{ClaimedDeposits: claimed, TotalWeightGram: grams, TotalWeightKG: FormatWeightKG(grams)}
	setPoints(isUser, points, spent, &out.Totals.PointsEarned, &out.Totals.PointsCredited, &out.Totals.PointsSpent)
	for _, c := range rec.Categories {
		out.Categories = append(out.Categories, entity.ActivityReportCategory{CategoryID: c.ID, CategoryName: c.Name, DepositCount: c.Deposits, TotalWeightG: c.Gram, TotalWeightKG: FormatWeightKG(c.Gram)})
	}
	return out, nil
}

func setPoints(isUser bool, points, spent int64, earned, credited, spentOut **int64) {
	if isUser {
		*earned, *spentOut = &points, &spent
		return
	}
	*credited = &points
}
