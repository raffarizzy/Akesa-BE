package access

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/farhan-hamzah/Akesa/backend/internal/auth"
	"github.com/farhan-hamzah/Akesa/backend/internal/patient"
	"github.com/farhan-hamzah/Akesa/backend/internal/response"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Create - POST /api/v1/hospital/access-requests (role: HOSPITAL_STAFF)
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body CreateRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_json_body")
		return
	}

	input, err := body.Validate()
	if err != nil {
		response.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	req, err := h.service.CreateRequest(r.Context(), authUser.ID, input)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, req)
}

// ListMine - GET /api/v1/patient/access-requests (role: PATIENT)
func (h *Handler) ListMine(w http.ResponseWriter, r *http.Request) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	requests, err := h.service.ListForPatient(r.Context(), authUser.ID)
	if err != nil {
		log.Printf("[access handler] ListMine failed for patient userID=%s (%s): %v", authUser.ID, authUser.Role, err)
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, requests)
}

// ListForMyHospital - GET /api/v1/hospital/access-requests (role: HOSPITAL_STAFF)
func (h *Handler) ListForMyHospital(w http.ResponseWriter, r *http.Request) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	requests, err := h.service.ListForHospitalStaff(r.Context(), authUser.ID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, requests)
}

// Approve - POST /api/v1/patient/access-requests/{id}/approve (role: PATIENT)
func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, h.service.Approve)
}

// Reject - POST /api/v1/patient/access-requests/{id}/reject (role: PATIENT)
func (h *Handler) Reject(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, h.service.Reject)
}

// Revoke - POST /api/v1/patient/access-requests/{id}/revoke (role: PATIENT)
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, h.service.Revoke)
}

// decide is the shared body for approve/reject/revoke: all three take the
// current patient's id plus a request id from the path, and return the
// updated request.
func (h *Handler) decide(
	w http.ResponseWriter,
	r *http.Request,
	fn func(ctx context.Context, patientID, requestID uuid.UUID) (*Request, error),
) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	requestID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_request_id")
		return
	}

	updated, err := fn(r.Context(), authUser.ID, requestID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, updated)
}

// FetchData - GET /api/v1/hospital/access-requests/{id}/data (role: HOSPITAL_STAFF)
// Returns only the categories of patient data the patient approved for
// this specific request.
func (h *Handler) FetchData(w http.ResponseWriter, r *http.Request) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	requestID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_request_id")
		return
	}

	data, err := h.service.FetchApprovedData(r.Context(), authUser.ID, requestID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, data)
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrRequestNotFound):
		response.Error(w, http.StatusNotFound, "access_request_not_found")
	case errors.Is(err, ErrPatientNotFound):
		response.Error(w, http.StatusNotFound, "patient_not_found")
	case errors.Is(err, ErrNotOwner):
		response.Error(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, ErrNotPending):
		response.Error(w, http.StatusConflict, "access_request_not_pending")
	case errors.Is(err, ErrNotApproved):
		response.Error(w, http.StatusConflict, "access_request_not_approved")
	case errors.Is(err, ErrDataTampered):
		response.Error(w, http.StatusConflict, "patient_data_integrity_violation")
	case errors.Is(err, patient.ErrProfileNotFound):
		response.Error(w, http.StatusNotFound, "patient_not_found")
	default:
		log.Printf("[access handler] unexpected error: %v", err)
		response.Error(w, http.StatusInternalServerError, "internal_server_error")
	}
}
