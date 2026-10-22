package hospital

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/farhan-hamzah/Akesa/backend/internal/auth"
	"github.com/farhan-hamzah/Akesa/backend/internal/response"
	"github.com/google/uuid"
)

const maxHospitalDocumentSize = 10 * 1024 * 1024

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	hospitals, err := h.service.List(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "internal_server_error")
		return
	}

	response.JSON(w, http.StatusOK, hospitals)
}

func (h *Handler) ListActive(w http.ResponseWriter, r *http.Request) {
	hospitals, err := h.service.List(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "internal_server_error")
		return
	}

	active := make([]*Hospital, 0, len(hospitals))
	for _, hospitalRecord := range hospitals {
		if hospitalRecord.Status == StatusActive {
			active = append(active, hospitalRecord)
		}
	}

	response.JSON(w, http.StatusOK, active)
}

func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.service.Verify)
}

func (h *Handler) Activate(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.service.Activate)
}

func (h *Handler) Deactivate(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.service.Deactivate)
}

func (h *Handler) transition(
	w http.ResponseWriter,
	r *http.Request,
	fn func(ctx context.Context, id uuid.UUID) (*Hospital, error),
) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_hospital_id")
		return
	}

	hospitalRecord, err := fn(r.Context(), id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, hospitalRecord)
}

// AddStaff - POST /api/v1/admin/hospitals/{id}/staff (role: ADMIN)
func (h *Handler) AddStaff(w http.ResponseWriter, r *http.Request) {
	hospitalID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_hospital_id")
		return
	}

	var req StaffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_json_body")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_user_id")
		return
	}

	staff, err := h.service.AddStaff(r.Context(), hospitalID, userID, req.FullName, req.Position)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, staff)
}

// GenerateInvitation - POST /api/v1/admin/hospitals/invitations (role: ADMIN)
func (h *Handler) GenerateInvitation(w http.ResponseWriter, r *http.Request) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req GenerateInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_json_body")
		return
	}

	invResp, err := h.service.GenerateInvitation(r.Context(), authUser.ID, req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, invResp)
}

// ValidateInvitation - GET /api/v1/hospitals/invitations/validate?key=... (role: any synced user)
func (h *Handler) ValidateInvitation(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		response.Error(w, http.StatusBadRequest, "key_query_param_required")
		return
	}

	valResp, err := h.service.ValidateInvitation(r.Context(), key)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, valResp)
}

// RegisterWithKey - POST /api/v1/hospitals/register (role: any synced user)
func (h *Handler) RegisterWithKey(w http.ResponseWriter, r *http.Request) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req RegisterWithKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_json_body")
		return
	}

	appRecord, err := h.service.RegisterWithKey(r.Context(), authUser.ID, req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, appRecord)
}

// GetMyApplication - GET /api/v1/hospitals/my-application (role: any synced user)
func (h *Handler) GetMyApplication(w http.ResponseWriter, r *http.Request) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	appRecord, err := h.service.GetMyApplication(r.Context(), authUser.ID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, appRecord)
}

// ResubmitApplication - PUT /api/v1/hospitals/my-application (role: any synced user)
func (h *Handler) ResubmitApplication(w http.ResponseWriter, r *http.Request) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req UpdateApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_json_body")
		return
	}

	appRecord, err := h.service.ResubmitApplication(r.Context(), authUser.ID, req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, appRecord)
}

// ListApplications - GET /api/v1/admin/hospital-applications (role: ADMIN)
func (h *Handler) ListApplications(w http.ResponseWriter, r *http.Request) {
	status := ApplicationStatus(r.URL.Query().Get("status"))
	if status != "" && !status.IsValid() {
		response.Error(w, http.StatusBadRequest, "invalid_status_filter")
		return
	}

	apps, err := h.service.ListApplications(r.Context(), status)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, apps)
}

// GetApplication - GET /api/v1/admin/hospital-applications/{id} (role: ADMIN)
func (h *Handler) GetApplication(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_application_id")
		return
	}

	appRecord, err := h.service.GetApplication(r.Context(), id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, appRecord)
}

// RequestRevision - POST /api/v1/admin/hospital-applications/{id}/request-revision (role: ADMIN)
func (h *Handler) RequestRevision(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_application_id")
		return
	}

	var req AdminReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_json_body")
		return
	}

	notes, err := req.Validate("permintaan revisi")
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	appRecord, err := h.service.RequestRevision(r.Context(), id, notes)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, appRecord)
}

// RejectApplication - POST /api/v1/admin/hospital-applications/{id}/reject (role: ADMIN)
func (h *Handler) RejectApplication(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_application_id")
		return
	}

	var req AdminReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_json_body")
		return
	}

	reason, err := req.Validate("penolakan")
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	appRecord, err := h.service.RejectApplication(r.Context(), id, reason)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, appRecord)
}

// ApproveApplication - POST /api/v1/admin/hospital-applications/{id}/approve (role: ADMIN)
func (h *Handler) ApproveApplication(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_application_id")
		return
	}

	hospitalRecord, err := h.service.ApproveApplication(r.Context(), id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, hospitalRecord)
}

// UploadDocument - POST /api/v1/hospitals/documents (role: any synced user)
func (h *Handler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxHospitalDocumentSize)

	file, header, err := r.FormFile("document")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "document_file_required")
		return
	}
	defer file.Close()

	if header.Size > maxHospitalDocumentSize {
		response.Error(w, http.StatusRequestEntityTooLarge, "document_too_large")
		return
	}

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		response.Error(w, http.StatusBadRequest, "failed_to_read_document")
		return
	}
	if n == 0 {
		response.Error(w, http.StatusBadRequest, "empty_document")
		return
	}

	contentType := http.DetectContentType(buffer[:n])
	var extension string
	if contentType == "application/pdf" ||
		bytes.Contains(buffer[:n], []byte("%PDF")) ||
		strings.HasSuffix(strings.ToLower(header.Filename), ".pdf") {
		extension = ".pdf"
	} else if contentType == "image/jpeg" {
		extension = ".jpg"
	} else if contentType == "image/png" {
		extension = ".png"
	} else {
		response.Error(w, http.StatusUnsupportedMediaType, "unsupported_document_type")
		return
	}

	seeker, ok := file.(io.Seeker)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "document_not_seekable")
		return
	}
	if _, err := seeker.Seek(0, io.SeekStart); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed_to_reset_document")
		return
	}

	storageKey, err := h.service.UploadLicenseDocument(r.Context(), authUser.ID, file, extension)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"licenseDocumentUrl": storageKey,
	})
}

// GetMyApplicationDocument - GET /api/v1/hospitals/my-application/document (role: any synced user)
func (h *Handler) GetMyApplicationDocument(w http.ResponseWriter, r *http.Request) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	reader, contentType, err := h.service.GetMyApplicationDocumentFile(r.Context(), authUser.ID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	if contentType == "application/pdf" {
		w.Header().Set("Content-Disposition", "inline; filename=\"dokumen_izin_operasional.pdf\"")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}

// GetApplicationDocumentAdmin - GET /api/v1/admin/hospital-applications/{id}/document (role: ADMIN)
func (h *Handler) GetApplicationDocumentAdmin(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_application_id")
		return
	}

	reader, contentType, err := h.service.GetApplicationDocumentFile(r.Context(), id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	if contentType == "application/pdf" {
		w.Header().Set("Content-Disposition", "inline; filename=\"dokumen_izin_operasional.pdf\"")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}

// GetHospitalDocumentAdmin - GET /api/v1/admin/hospitals/{id}/document (role: ADMIN)
func (h *Handler) GetHospitalDocumentAdmin(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_hospital_id")
		return
	}

	reader, contentType, err := h.service.GetHospitalDocumentFile(r.Context(), id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	if contentType == "application/pdf" {
		w.Header().Set("Content-Disposition", "inline; filename=\"dokumen_izin_operasional.pdf\"")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	var valErr *ValidationError
	if errors.As(err, &valErr) {
		response.Error(w, http.StatusUnprocessableEntity, valErr.Error())
		return
	}

	switch {
	case errors.Is(err, ErrNotFound):
		response.Error(w, http.StatusNotFound, "hospital_not_found")
	case errors.Is(err, ErrEmailTaken):
		response.Error(w, http.StatusConflict, "email_already_registered")
	case errors.Is(err, ErrRegistrationTaken):
		response.Error(w, http.StatusConflict, "registration_number_already_registered")
	case errors.Is(err, ErrStaffAlreadyLinked):
		response.Error(w, http.StatusConflict, "user_already_staff")
	case errors.Is(err, ErrHospitalNotActive):
		response.Error(w, http.StatusUnprocessableEntity, "hospital_not_active")
	case errors.Is(err, ErrInvitationNotFound):
		response.Error(w, http.StatusNotFound, "invitation_key_not_found")
	case errors.Is(err, ErrInvitationExpired):
		response.Error(w, http.StatusGone, "invitation_key_expired")
	case errors.Is(err, ErrInvitationAlreadyUsed):
		response.Error(w, http.StatusConflict, "invitation_key_already_used")
	case errors.Is(err, ErrApplicationNotFound):
		response.Error(w, http.StatusNotFound, "hospital_application_not_found")
	case errors.Is(err, ErrApplicationNotRevisionRequired):
		response.Error(w, http.StatusBadRequest, "application_does_not_require_revision")
	case errors.Is(err, ErrApplicationAlreadyFinalized):
		response.Error(w, http.StatusConflict, "application_already_finalized")
	case errors.Is(err, ErrApplicationAlreadyPendingReview):
		response.Error(w, http.StatusBadRequest, "application_already_pending_review")
	case errors.Is(err, ErrDocumentNotUploaded):
		response.Error(w, http.StatusNotFound, "document_not_uploaded")
	case errors.Is(err, ErrInvalidDocumentType):
		response.Error(w, http.StatusUnsupportedMediaType, "unsupported_document_type")
	case errors.Is(err, ErrDocumentTooLarge):
		response.Error(w, http.StatusRequestEntityTooLarge, "document_too_large")
	default:
		log.Printf("[hospital handler] unexpected error: %v", err)
		response.Error(w, http.StatusInternalServerError, "internal_server_error")
	}
}
