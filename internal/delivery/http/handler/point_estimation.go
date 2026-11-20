package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type PointEstimationHandler struct {
	estimates *service.PointEstimationService
}

func NewPointEstimationHandler(estimates *service.PointEstimationService) *PointEstimationHandler {
	return &PointEstimationHandler{estimates: estimates}
}
// decodeStrictJSON accepts exactly one JSON object without unknown fields and
// writes the error response itself when it returns false.
func decodeStrictJSON(w http.ResponseWriter, r *http.Request, dst any, invalidMessage string) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		response.Error(w, 415, "unsupported_media_type", "Gunakan application/json.")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(dst); err == nil {
		var extra any
		if next := decoder.Decode(&extra); next == nil {
			err = errors.New("trailing JSON")
		} else if next != io.EOF {
			err = next
		}
	}
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			response.Error(w, 413, "body_too_large", "Body terlalu besar.")
			return false
		}
		response.Error(w, 400, "invalid_request", invalidMessage)
		return false
	}
	return true
}
func (h *PointEstimationHandler) Estimate(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		estimateError(w, service.ErrInvalidEstimate)
		return
	}
	var in entity.PointEstimateRequest
	if !decodeStrictJSON(w, r, &in, "Kirim satu objek JSON estimasi yang valid.") {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.estimates.Estimate(ctx, in)
	if err != nil {
		estimateError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": result})
}
func estimateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidEstimate):
		response.Error(w, 400, "invalid_estimate", err.Error())
	case errors.Is(err, service.ErrWasteNotAccepted):
		response.Error(w, 400, "waste_not_accepted", err.Error())
	case errors.Is(err, service.ErrPointRateUnavailable):
		response.Error(w, 409, "point_rate_unavailable", err.Error())
	case errors.Is(err, service.ErrWasteCategoryNotFound):
		response.Error(w, 404, "waste_category_not_found", err.Error())
	case errors.Is(err, service.ErrPartnerNotFound):
		response.Error(w, 404, "partner_not_found", err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		response.Error(w, 504, "request_timeout", "Batas waktu estimasi terlampaui.")
	default:
		log.Printf("estimasi poin gagal: %v", err)
		response.Error(w, 500, "internal_error", "Estimasi belum dapat dihitung.")
	}
}
