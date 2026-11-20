package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
)

var ErrInvalidEstimate = errors.New("input estimasi tidak valid")
var ErrPointRateUnavailable = errors.New("tarif aktif belum tersedia")
var ErrWasteCategoryNotFound = errors.New("kategori sampah tidak ditemukan")
var ErrWasteNotAccepted = errors.New("material tidak diterima mitra")
var weightDecimal = regexp.MustCompile(`^[0-9]{1,4}(?:\.[0-9]{1,3})?$`)

type PointTariff struct {
	CategoryID, Name, RateID string
	RateHundredths           int64
	Accepted, IsDemo         bool
}
type PointRateSelection struct {
	AsOf  time.Time
	Rates map[string]PointTariff
}
type PointRateStore interface {
	SelectRates(context.Context, *string, []string) (PointRateSelection, error)
}
type PointEstimationService struct{ store PointRateStore }

func NewPointEstimationService(store PointRateStore) *PointEstimationService {
	return &PointEstimationService{store: store}
}
func formatPointDecimal(value int64, scale int) string {
	factor := int64(1)
	for i := 0; i < scale; i++ {
		factor *= 10
	}
	return fmt.Sprintf("%d.%0*d", value/factor, scale, value%factor)
}
func weightGrams(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if !weightDecimal.MatchString(raw) {
		return 0, ErrInvalidEstimate
	}
	parts := strings.SplitN(raw, ".", 2)
	whole, _ := strconv.ParseInt(parts[0], 10, 64)
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	fraction += strings.Repeat("0", 3-len(fraction))
	frac, _ := strconv.ParseInt(fraction, 10, 64)
	grams := whole*1000 + frac
	if grams < 1 || grams > 1000000 {
		return 0, ErrInvalidEstimate
	}
	return grams, nil
}
// estimateLines holds validated items with weights in grams. Estimates and
// deposits share it so both accept and reject exactly the same input.
type estimateLines struct {
	ids        []string
	grams      []int64
	totalGrams int64
}

func parseEstimateItems(items []entity.PointEstimateInput) (estimateLines, error) {
	var out estimateLines
	if len(items) < 1 || len(items) > 50 {
		return out, ErrInvalidEstimate
	}
	out.ids = make([]string, 0, len(items))
	out.grams = make([]int64, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if !partnerID.MatchString(item.CategoryID) {
			return out, ErrInvalidEstimate
		}
		id := strings.ToLower(item.CategoryID)
		if seen[id] {
			return out, ErrInvalidEstimate
		}
		seen[id] = true
		grams, err := weightGrams(item.WeightKG)
		if err != nil {
			return out, err
		}
		out.totalGrams += grams
		if out.totalGrams > 1000000 {
			return out, ErrInvalidEstimate
		}
		out.ids = append(out.ids, id)
		out.grams = append(out.grams, grams)
	}
	return out, nil
}

// priceEstimate applies D-17: sum exact item products, then floor once.
func priceEstimate(partner *string, lines estimateLines, selection PointRateSelection) (entity.PointEstimate, error) {
	var empty entity.PointEstimate
	out := entity.PointEstimate{PartnerID: partner, AsOf: selection.AsOf, TotalWeightKG: formatPointDecimal(lines.totalGrams, 3), Items: make([]entity.PointEstimateItem, 0, len(lines.ids))}
	var numerator int64
	for i, id := range lines.ids {
		rate, ok := selection.Rates[id]
		if !ok {
			return empty, ErrWasteCategoryNotFound
		}
		if !rate.Accepted {
			return empty, ErrWasteNotAccepted
		}
		if rate.RateID == "" || rate.RateHundredths < 1 || rate.RateHundredths > 100000000 {
			return empty, ErrPointRateUnavailable
		}
		// Grams * hundredths of a point/kg gives hundred-thousandths of a
		// point. Keep the fraction until every item has been summed.
		// The weight/rate caps bound the entire sum to 1e14 (safe in int64).
		product := lines.grams[i] * rate.RateHundredths
		numerator += product
		out.Items = append(out.Items, entity.PointEstimateItem{CategoryID: id, CategoryName: rate.Name, WeightKG: formatPointDecimal(lines.grams[i], 3), RateID: rate.RateID, PointsPerKG: formatPointDecimal(rate.RateHundredths, 2), ExactPoints: formatPointDecimal(product, 5), IsDemo: rate.IsDemo})
		out.UsesDemoRates = out.UsesDemoRates || rate.IsDemo
	}
	out.EstimatedPoints = numerator / 100000
	return out, nil
}

func (s *PointEstimationService) Estimate(ctx context.Context, in entity.PointEstimateRequest) (entity.PointEstimate, error) {
	var empty entity.PointEstimate
	var selectedPartner *string
	if in.PartnerID != nil {
		if !partnerID.MatchString(*in.PartnerID) {
			return empty, ErrInvalidEstimate
		}
		id := strings.ToLower(*in.PartnerID)
		selectedPartner = &id
	}
	lines, err := parseEstimateItems(in.Items)
	if err != nil {
		return empty, err
	}
	selection, err := s.store.SelectRates(ctx, selectedPartner, lines.ids)
	if err != nil {
		return empty, err
	}
	return priceEstimate(selectedPartner, lines, selection)
}
