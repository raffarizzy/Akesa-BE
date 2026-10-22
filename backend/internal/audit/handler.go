package audit

import (
	"context"
	"net/http"
	"strconv"

	"github.com/farhan-hamzah/Akesa/backend/internal/auth"
	"github.com/farhan-hamzah/Akesa/backend/internal/patient"
	"github.com/farhan-hamzah/Akesa/backend/internal/response"
	"github.com/google/uuid"
)

type PatientResolver interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) (*patient.Profile, error)
}

type Handler struct {
	service         *Service
	patientResolver PatientResolver
}

func NewHandler(service *Service, patientResolver PatientResolver) *Handler {
	return &Handler{
		service:         service,
		patientResolver: patientResolver,
	}
}

// MyProfileHistory - GET /api/v1/patient/history (role: PATIENT)
// Returns full timeline of events relevant to the authenticated patient:
// profile updates, consent decisions, data accesses, and security/tampering alerts.
func (h *Handler) MyProfileHistory(w http.ResponseWriter, r *http.Request) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var patientID uuid.UUID
	if h.patientResolver != nil {
		profile, err := h.patientResolver.GetByUserID(r.Context(), authUser.ID)
		if err != nil {
			response.Error(w, http.StatusNotFound, "patient_profile_not_found")
			return
		}
		patientID = profile.ID
	} else {
		patientID = authUser.ID
	}

	logs, err := h.service.PatientTimeline(r.Context(), patientID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "internal_server_error")
		return
	}
	if logs == nil {
		logs = []*Log{}
	}

	response.JSON(w, http.StatusOK, logs)
}

// SecurityAlerts - GET /api/v1/admin/security-alerts (role: ADMIN)
// Returns list of data integrity violations and tampering events.
func (h *Handler) SecurityAlerts(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	logs, total, err := h.service.SecurityAlerts(r.Context(), limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "internal_server_error")
		return
	}
	if logs == nil {
		logs = []*Log{}
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"total":  total,
		"alerts": logs,
	})
}

// VerifyChain - GET /api/v1/audit/verify-chain
// Cryptographically checks all blocks in the audit hash chain.
func (h *Handler) VerifyChain(w http.ResponseWriter, r *http.Request) {
	valid, totalBlocks, err := h.service.VerifyChainIntegrity(r.Context())
	if err != nil {
		response.JSON(w, http.StatusOK, map[string]any{
			"valid":       false,
			"totalBlocks": totalBlocks,
			"status":      "TAMPERED_OR_BROKEN",
			"error":       err.Error(),
		})
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"valid":       valid,
		"totalBlocks": totalBlocks,
		"status":      "VERIFIED_INTACT",
		"message":     "All SHA-256 block hashes verified and intact",
	})
}

// SyncChain - POST /api/v1/audit/sync-chain
// Manually triggers immediate synchronization of database audit records with the blockchain.
func (h *Handler) SyncChain(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.SyncBlockchain(r.Context())
	if err != nil {
		response.Error(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"status":  "SYNCED",
		"message": "Blockchain state successfully synchronized with database audit records",
		"data":    result,
	})
}

