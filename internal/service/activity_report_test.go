package service_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type activityStore struct {
	rec      service.ActivityReportRecord
	err      error
	from, to time.Time
	calls    int
}

func (s *activityStore) ActivityReport(_ context.Context, _ string, from, to time.Time) (service.ActivityReportRecord, error) {
	s.calls++
	s.from, s.to = from, to
	return s.rec, s.err
}

func wib(y int, m time.Month, d, h, mi int) time.Time {
	return time.Date(y, m, d, h, mi, 0, 0, service.WIB)
}

func TestActivityReportRangeRules(t *testing.T) {
	now := time.Date(2026, 11, 21, 10, 0, 0, 123456789, time.UTC) // 17:00 WIB
	store := &activityStore{rec: service.ActivityReportRecord{Role: "user", FullName: "A"}}
	svc := service.NewActivityReportService(store).WithClock(func() time.Time { return now })
	ctx := context.Background()

	t.Run("default_is_current_wib_month_up_to_now", func(t *testing.T) {
		r, err := svc.Report(ctx, "u", service.ActivityReportQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if !store.from.Equal(wib(2026, 11, 1, 0, 0)) || !store.to.Equal(now.Truncate(time.Microsecond)) {
			t.Fatalf("range %v %v", store.from, store.to)
		}
		if r.Period.Days != 21 || r.Daily[0].Date != "2026-11-01" || r.Daily[20].Date != "2026-11-21" || r.Period.Timezone != "Asia/Jakarta" {
			t.Fatalf("period %+v first=%s", r.Period, r.Daily[0].Date)
		}
		if _, off := r.Period.From.Zone(); off != 7*3600 {
			t.Fatal("period must be rendered in WIB")
		}
	})
	t.Run("default_at_month_boundary_is_not_empty", func(t *testing.T) {
		at := wib(2026, 12, 1, 0, 0)
		b := service.NewActivityReportService(store).WithClock(func() time.Time { return at })
		r, err := b.Report(ctx, "u", service.ActivityReportQuery{})
		if err != nil || !store.to.After(store.from) || r.Period.Days != 1 {
			t.Fatalf("%v %+v", err, r.Period)
		}
	})
	t.Run("explicit_future_to_is_allowed_with_zero_buckets", func(t *testing.T) {
		r, err := svc.Report(ctx, "u", service.ActivityReportQuery{From: new(wib(2026, 11, 1, 0, 0)), To: new(wib(2026, 12, 1, 0, 0))})
		if err != nil || r.Period.Days != 30 || r.Totals.ClaimedDeposits != 0 {
			t.Fatalf("%v %+v", err, r.Period)
		}
		for _, d := range r.Daily {
			if d.ClaimedDeposits != 0 || d.TotalWeightKG != "0.000" || *d.PointsEarned != 0 || *d.PointsSpent != 0 {
				t.Fatalf("not zero: %+v", d)
			}
		}
	})
	t.Run("offset_instants_use_wib_buckets_and_clip_partial_days", func(t *testing.T) {
		from := time.Date(2026, 11, 1, 20, 0, 0, 0, time.UTC) // 2026-11-02 03:00 WIB
		to := time.Date(2026, 11, 3, 17, 0, 0, 0, time.UTC)   // 2026-11-04 00:00 WIB exclusive
		r, err := svc.Report(ctx, "u", service.ActivityReportQuery{From: &from, To: &to})
		if err != nil {
			t.Fatal(err)
		}
		if r.Period.Days != 2 || r.Daily[0].Date != "2026-11-02" || r.Daily[1].Date != "2026-11-03" {
			t.Fatalf("%+v", r.Daily)
		}
	})
	t.Run("max_span_boundaries", func(t *testing.T) {
		from := wib(2026, 1, 1, 0, 0)
		ok := from.Add(service.MaxActivityReportSpan)
		if _, err := svc.Report(ctx, "u", service.ActivityReportQuery{From: &from, To: &ok}); err != nil {
			t.Fatalf("366 days: %v", err)
		}
		over := ok.Add(time.Microsecond)
		if _, err := svc.Report(ctx, "u", service.ActivityReportQuery{From: &from, To: &over}); !errors.Is(err, service.ErrInvalidActivityReportQuery) {
			t.Fatalf("over span: %v", err)
		}
	})
	t.Run("invalid_ranges_never_reach_the_store", func(t *testing.T) {
		store.calls = 0
		a, b := wib(2026, 11, 2, 0, 0), wib(2026, 11, 1, 0, 0)
		for name, q := range map[string]service.ActivityReportQuery{
			"from_only": {From: &a}, "to_only": {To: &a}, "reversed": {From: &a, To: &b}, "empty": {From: &a, To: &a},
			"sub_microsecond": {From: &a, To: new(a.Add(500 * time.Nanosecond))},
		} {
			if _, err := svc.Report(ctx, "u", q); !errors.Is(err, service.ErrInvalidActivityReportQuery) {
				t.Fatalf("%s: %v", name, err)
			}
		}
		if store.calls != 0 {
			t.Fatal("store called")
		}
	})
	t.Run("store_errors_pass_through", func(t *testing.T) {
		e := &activityStore{err: service.ErrAccountForbidden}
		if _, err := service.NewActivityReportService(e).Report(ctx, "u", service.ActivityReportQuery{}); !errors.Is(err, service.ErrAccountForbidden) {
			t.Fatal(err)
		}
	})
}

func TestActivityReportAggregation(t *testing.T) {
	now := wib(2026, 11, 10, 12, 0)
	from, to := wib(2026, 11, 1, 0, 0), wib(2026, 11, 4, 0, 0)
	q := service.ActivityReportQuery{From: &from, To: &to}
	userRec := service.ActivityReportRecord{Role: "user", FullName: "Siti", Days: []service.ActivityDayRecord{
		{Date: "2026-11-01", Claimed: 2, Grams: 1500, Points: 200},
		{Date: "2026-11-03", Claimed: 1, Grams: 250, Points: 40},
		{Date: "2026-11-03", Spent: 100}, // redemption row for the same date
	}, Categories: []service.ActivityCategoryRecord{{ID: "c1", Name: "Plastik", Deposits: 2, Gram: 1000}, {ID: "c2", Name: "Kertas", Deposits: 1, Gram: 750}}}
	svc := service.NewActivityReportService(&activityStore{rec: userRec}).WithClock(func() time.Time { return now })
	r, err := svc.Report(context.Background(), "u", q)
	if err != nil {
		t.Fatal(err)
	}
	if r.Totals.ClaimedDeposits != 3 || r.Totals.TotalWeightGram != 1750 || r.Totals.TotalWeightKG != "1.750" ||
		*r.Totals.PointsEarned != 240 || *r.Totals.PointsSpent != 100 || r.Totals.PointsCredited != nil {
		t.Fatalf("totals %+v", r.Totals)
	}
	d3 := r.Daily[2]
	if len(r.Daily) != 3 || r.Daily[1].ClaimedDeposits != 0 || d3.ClaimedDeposits != 1 || *d3.PointsEarned != 40 || *d3.PointsSpent != 100 || d3.TotalWeightKG != "0.250" {
		t.Fatalf("daily %+v", r.Daily)
	}
	if len(r.Categories) != 2 || r.Categories[1].TotalWeightKG != "0.750" || r.Categories[0].DepositCount != 2 {
		t.Fatalf("categories %+v", r.Categories)
	}

	mitraRec := service.ActivityReportRecord{Role: "mitra", FullName: "Owner", BusinessName: new("Tunas"), Days: []service.ActivityDayRecord{{Date: "2026-11-02", Claimed: 1, Grams: 1000, Points: 150}}}
	m, err := service.NewActivityReportService(&activityStore{rec: mitraRec}).WithClock(func() time.Time { return now }).Report(context.Background(), "m", q)
	if err != nil || m.Totals.PointsEarned != nil || m.Totals.PointsSpent != nil || *m.Totals.PointsCredited != 150 || *m.Daily[1].PointsCredited != 150 || *m.Account.BusinessName != "Tunas" {
		t.Fatalf("mitra %+v %v", m, err)
	}

	outside := service.ActivityReportRecord{Role: "user", Days: []service.ActivityDayRecord{{Date: "2026-11-09", Claimed: 1}}}
	if _, err := service.NewActivityReportService(&activityStore{rec: outside}).Report(context.Background(), "u", q); err == nil {
		t.Fatal("a bucket outside the range must be an error, not silently dropped")
	}
	if _, err := service.NewActivityReportService(&activityStore{rec: service.ActivityReportRecord{Role: "admin"}}).Report(context.Background(), "u", q); err == nil {
		t.Fatal("unknown role")
	}
}

var (
	pdfObjRE   = regexp.MustCompile(`(?m)^(\d{10}) (\d{5}) n`)
	pdfPageRE  = regexp.MustCompile(`/Type /Page[^s]`)
	pdfCountRE = regexp.MustCompile(`/Count (\d+)`)
)

// checkPDF verifies container structure without external tools: header,
// trailer, startxref pointing at the xref table, and every in-use xref entry
// pointing at "N 0 obj". It returns the page count.
func checkPDF(t *testing.T, b []byte) int {
	t.Helper()
	if !bytes.HasPrefix(b, []byte("%PDF-1.")) || !bytes.HasSuffix(bytes.TrimRight(b, "\r\n "), []byte("%%EOF")) {
		t.Fatal("missing PDF header or EOF marker")
	}
	i := bytes.LastIndex(b, []byte("startxref"))
	if i < 0 {
		t.Fatal("no startxref")
	}
	fields := strings.Fields(string(b[i+len("startxref"):]))
	off, err := strconv.Atoi(fields[0])
	if err != nil || off <= 0 || off >= len(b) || !bytes.HasPrefix(b[off:], []byte("xref")) {
		t.Fatalf("startxref %v does not point at xref", fields[0])
	}
	for n, m := range pdfObjRE.FindAllSubmatch(b[off:], -1) {
		o, _ := strconv.Atoi(string(m[1]))
		if !bytes.HasPrefix(b[o:], []byte(fmt.Sprintf("%d 0 obj", n+1))) {
			t.Fatalf("xref entry %d points at %q", n+1, b[o:o+12])
		}
	}
	pages := len(pdfPageRE.FindAll(b, -1))
	c := pdfCountRE.FindSubmatch(b)
	if pages == 0 || c == nil || string(c[1]) != strconv.Itoa(pages) {
		t.Fatalf("page objects %d, /Count %q", pages, c)
	}
	if !bytes.Contains(b, []byte("/FontFile2")) {
		t.Fatal("font not embedded")
	}
	return pages
}

// writeSample saves a PDF for manual inspection when
// ECOSNAP_ACTIVITY_PDF_SAMPLE_DIR is set; nothing is written otherwise.
func writeSample(t *testing.T, name string, b []byte) {
	t.Helper()
	if dir := os.Getenv("ECOSNAP_ACTIVITY_PDF_SAMPLE_DIR"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestActivityReportPDF(t *testing.T) {
	now := wib(2026, 11, 21, 9, 0)
	from, to := wib(2026, 1, 1, 0, 0), wib(2027, 1, 1, 0, 0)
	long := strings.Repeat("Botol plastik PET bening berukuran besar ", 6) + "Ünïcödé Ωmega Привет"
	rec := service.ActivityReportRecord{Role: "user", FullName: "Dewi Šumarni Ñ", Days: []service.ActivityDayRecord{
		{Date: "2026-01-01", Claimed: 1, Grams: 1234, Points: 99},
		{Date: "2026-12-31", Claimed: 2, Grams: 2000, Points: 10, Spent: 5},
	}}
	for i := range 40 {
		rec.Categories = append(rec.Categories, service.ActivityCategoryRecord{ID: fmt.Sprintf("c%02d", i), Name: fmt.Sprintf("Kategori %02d %s", i, long), Deposits: 1, Gram: 1000})
	}
	svc := service.NewActivityReportService(&activityStore{rec: rec}).WithClock(func() time.Time { return now })
	doc, err := svc.PDF(context.Background(), "u", service.ActivityReportQuery{From: &from, To: &to})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Filename != "ecosnap-activity-report-20260101-20261231.pdf" {
		t.Fatalf("filename %q", doc.Filename)
	}
	pages := checkPDF(t, doc.Content)
	if pages < 8 {
		t.Fatalf("365 daily rows and 40 wrapped categories should paginate over many pages, got %d", pages)
	}
	writeSample(t, "user-year.pdf", doc.Content)

	// Mitra document: no user points-spent column, business name shown,
	// characters outside the font are flagged rather than turned into "?".
	m := service.ActivityReportRecord{Role: "mitra", FullName: "Pemilik 漢字", BusinessName: new("Bank Sampah Tunas")}
	md, err := service.NewActivityReportService(&activityStore{rec: m}).WithClock(func() time.Time { return now }).PDF(context.Background(), "m", service.ActivityReportQuery{From: &from, To: new(wib(2026, 1, 8, 0, 0))})
	if err != nil {
		t.Fatal(err)
	}
	checkPDF(t, md.Content)
	writeSample(t, "mitra-week.pdf", md.Content)
}
func TestRenderActivityReportPDFNeverUsesPlaceholders(t *testing.T) {
	r := entity.ActivityReport{
		Account:     entity.ActivityReportAccount{Role: "user", FullName: "A\nB\x00C"},
		Period:      entity.ActivityReportPeriod{From: wib(2026, 1, 1, 0, 0), To: wib(2026, 1, 2, 0, 0), Timezone: "Asia/Jakarta", Days: 1},
		GeneratedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		Totals:      entity.ActivityReportTotals{TotalWeightKG: "0.000", PointsEarned: new(int64(0)), PointsSpent: new(int64(0))},
		Daily:       []entity.ActivityReportDay{{Date: "2026-01-01", TotalWeightKG: "0.000", PointsEarned: new(int64(0)), PointsSpent: new(int64(0))}},
	}
	b, err := service.RenderActivityReportPDF(r)
	if err != nil {
		t.Fatal(err)
	}
	if checkPDF(t, b) != 1 {
		t.Fatal("empty one-day report should be one page")
	}
}
