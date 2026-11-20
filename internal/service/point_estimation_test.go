package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type pointRatesFixture struct{ cents int64 }

func (f pointRatesFixture) SelectRates(_ context.Context, _ *string, ids []string) (service.PointRateSelection, error) {
	rates := map[string]service.PointTariff{}
	for _, id := range ids {
		rates[id] = service.PointTariff{CategoryID: id, Name: "Demo", RateID: "50000000-0000-4000-8000-000000000001", RateHundredths: f.cents, Accepted: true, IsDemo: true}
	}
	return service.PointRateSelection{AsOf: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Rates: rates}, nil
}

func TestPointEstimatePrecisionAndLimits(t *testing.T) {
	const id = "10000000-0000-4000-8000-000000000001"
	for _, tc := range []struct {
		cents         int64
		weight, exact string
		points        int64
	}{
		{123, "0.125", "0.15375", 0},
		{1, "0.001", "0.00001", 0},
		{100000000, "1000.000", "1000000000.00000", 1000000000},
	} {
		s := service.NewPointEstimationService(pointRatesFixture{tc.cents})
		out, err := s.Estimate(context.Background(), entity.PointEstimateRequest{Items: []entity.PointEstimateInput{{CategoryID: id, WeightKG: tc.weight}}})
		if err != nil || out.EstimatedPoints != tc.points || out.Items[0].ExactPoints != tc.exact {
			t.Fatalf("precision: %+v %v", out, err)
		}
	}
	s := service.NewPointEstimationService(pointRatesFixture{10000})
	empty := ""
	for _, request := range []entity.PointEstimateRequest{
		{},
		{PartnerID: &empty, Items: []entity.PointEstimateInput{{CategoryID: id, WeightKG: "1"}}},
		{Items: []entity.PointEstimateInput{{CategoryID: "invalid", WeightKG: "1"}}},
		{Items: []entity.PointEstimateInput{{CategoryID: id, WeightKG: "1000"}, {CategoryID: "10000000-0000-4000-8000-000000000002", WeightKG: "0.001"}}},
		{Items: make([]entity.PointEstimateInput, 51)},
	} {
		if _, err := s.Estimate(context.Background(), request); !errors.Is(err, service.ErrInvalidEstimate) {
			t.Fatalf("invalid request accepted: %v", err)
		}
	}
}
func TestPointEstimateFloorsAfterTotal(t *testing.T) {
	s := service.NewPointEstimationService(pointRatesFixture{10000})
	out, err := s.Estimate(context.Background(), entity.PointEstimateRequest{Items: []entity.PointEstimateInput{
		{CategoryID: "10000000-0000-4000-8000-000000000001", WeightKG: "0.006"},
		{CategoryID: "10000000-0000-4000-8000-000000000002", WeightKG: "0.006"},
	}})
	if err != nil || out.EstimatedPoints != 1 || out.TotalWeightKG != "0.012" || len(out.Items) != 2 || out.Items[0].ExactPoints != "0.60000" {
		t.Fatalf("floor total: %+v %v", out, err)
	}
}
func TestPointEstimateRejectsInvalidWeightsAndDuplicates(t *testing.T) {
	s := service.NewPointEstimationService(pointRatesFixture{10000})
	id := "10000000-0000-4000-8000-000000000001"
	for _, weight := range []string{"0", "-1", "1.0001", "2,5", "1e3", "NaN", "1000.001", "99999999999999999"} {
		_, err := s.Estimate(context.Background(), entity.PointEstimateRequest{Items: []entity.PointEstimateInput{{CategoryID: id, WeightKG: weight}}})
		if !errors.Is(err, service.ErrInvalidEstimate) {
			t.Fatalf("weight %q accepted: %v", weight, err)
		}
	}
	_, err := s.Estimate(context.Background(), entity.PointEstimateRequest{Items: []entity.PointEstimateInput{{CategoryID: id, WeightKG: "1"}, {CategoryID: id, WeightKG: "2"}}})
	if !errors.Is(err, service.ErrInvalidEstimate) {
		t.Fatalf("duplicate accepted: %v", err)
	}
}
