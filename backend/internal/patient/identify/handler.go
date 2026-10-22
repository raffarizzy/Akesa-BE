package identity

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/farhan-hamzah/Akesa/backend/internal/auth"
	"github.com/farhan-hamzah/Akesa/backend/internal/patient"
	"github.com/farhan-hamzah/Akesa/backend/internal/response"
	"github.com/google/uuid"
)

const maxDocumentSize = 5 * 1024 * 1024

type Handler struct {
	service *Service
}

type RejectVerificationRequest struct {
	Reason string `json:"reason"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) CreateVerification(
	w http.ResponseWriter,
	r *http.Request,
) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"unauthorized",
		)
		return
	}

	verification, err := h.service.CreateVerification(
		r.Context(),
		authUser.ID,
	)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(
		w,
		http.StatusCreated,
		toVerificationResponse(verification),
	)
}

func (h *Handler) GetVerification(
	w http.ResponseWriter,
	r *http.Request,
) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"unauthorized",
		)
		return
	}

	id := r.PathValue("id")

	verificationID, err := uuid.Parse(id)
	if err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_verification_id",
		)
		return
	}

	verification, err := h.service.GetVerification(
		r.Context(),
		authUser.ID,
		verificationID,
	)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		toVerificationResponse(verification),
	)
}

func (h *Handler) writeServiceError(
	w http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(err, patient.ErrProfileNotFound):
		response.Error(
			w,
			http.StatusNotFound,
			"profile_not_found",
		)

	case errors.Is(err, ErrVerificationNotFound):
		response.Error(
			w,
			http.StatusNotFound,
			"verification_not_found",
		)

	case errors.Is(err, ErrVerificationExpired):
		response.Error(
			w,
			http.StatusGone,
			"verification_expired",
		)

	case errors.Is(err, ErrTooManyAttempts):
		response.Error(
			w,
			http.StatusTooManyRequests,
			"too_many_verification_attempts",
		)

	case errors.Is(err, ErrVerificationCompleted):
		response.Error(
			w,
			http.StatusConflict,
			"verification_already_completed",
		)

	case errors.Is(err, ErrVerificationNotInReview):
		response.Error(
			w,
			http.StatusConflict,
			"verification_not_in_review",
		)

	case errors.Is(err, ErrDocumentNotUploaded):
		response.Error(
			w,
			http.StatusBadRequest,
			"document_not_uploaded",
		)

	case errors.Is(err, ErrSelfieNotUploaded):
		response.Error(
			w,
			http.StatusBadRequest,
			"selfie_not_uploaded",
		)

	case errors.Is(err, ErrInvalidDocumentType):
		response.Error(
			w,
			http.StatusUnsupportedMediaType,
			"unsupported_document_type",
		)

	case errors.Is(err, ErrDocumentTooLarge):
		response.Error(
			w,
			http.StatusRequestEntityTooLarge,
			"document_too_large",
		)

	default:
		log.Printf("[IdentityHandler] internal error: %v", err)
		response.Error(
			w,
			http.StatusInternalServerError,
			"internal_server_error",
		)
	}
}

func toVerificationResponse(
	verification *Verification,
) VerificationResponse {
	documentUploaded := verification.DocumentStorageKey != nil &&
		strings.TrimSpace(*verification.DocumentStorageKey) != ""

	selfieUploaded := verification.SelfieStorageKey != nil &&
		strings.TrimSpace(*verification.SelfieStorageKey) != ""

	result := VerificationResponse{
		ID:               stringPtr(verification.ID.String()),
		Status:           stringPtr(string(verification.Status)),
		DocumentType:     stringPtr(string(verification.DocumentType)),
		DocumentUploaded: documentUploaded,
		SelfieUploaded:   selfieUploaded,
		DocumentStatus:   verification.DocumentStatus,
		LivenessStatus:   verification.LivenessStatus,
		FaceMatchStatus:  verification.FaceMatchStatus,
		FailureReason:    verification.FailureReason,
	}

	if verification.VerifiedAt != nil {
		value := verification.VerifiedAt.Format(
			"2006-01-02T15:04:05Z07:00",
		)
		result.VerifiedAt = &value
	}

	if verification.ExpiresAt != nil {
		value := verification.ExpiresAt.Format(
			"2006-01-02T15:04:05Z07:00",
		)
		result.ExpiresAt = &value
	}

	return result
}

func toAdminVerificationResponse(
	verification *Verification,
) AdminVerificationResponse {
	documentUploaded := verification.DocumentStorageKey != nil &&
		strings.TrimSpace(*verification.DocumentStorageKey) != ""

	selfieUploaded := verification.SelfieStorageKey != nil &&
		strings.TrimSpace(*verification.SelfieStorageKey) != ""

	resp := AdminVerificationResponse{
		ID:                verification.ID.String(),
		PatientID:         verification.PatientID.String(),
		Status:            verification.Status,
		DocumentType:      verification.DocumentType,
		DocumentUploaded:  documentUploaded,
		SelfieUploaded:    selfieUploaded,
		ExtractedNIK:      verification.ExtractedNIK,
		ExtractedFullName: verification.ExtractedFullName,
		ExtractedGender:   verification.ExtractedGender,
		DocumentStatus:    verification.DocumentStatus,
		LivenessStatus:    verification.LivenessStatus,
		FaceMatchStatus:   verification.FaceMatchStatus,
		FaceMatchScore:    verification.FaceMatchScore,
		Provider:          verification.Provider,
		FailureReason:     verification.FailureReason,
		CreatedAt:         verification.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         verification.UpdatedAt.Format(time.RFC3339),
	}

	if verification.ExtractedDateOfBirth != nil {
		dob := verification.ExtractedDateOfBirth.Format("2006-01-02")
		resp.ExtractedDateOfBirth = &dob
	}

	if verification.VerifiedAt != nil {
		vAt := verification.VerifiedAt.Format(time.RFC3339)
		resp.VerifiedAt = &vAt
	}

	if verification.ExpiresAt != nil {
		exp := verification.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &exp
	}

	return resp
}

func stringPtr(value string) *string {
	return &value
}

func (h *Handler) UploadDocument(
	w http.ResponseWriter,
	r *http.Request,
) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"unauthorized",
		)
		return
	}

	id := r.PathValue("id")

	verificationID, err := uuid.Parse(id)
	if err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_verification_id",
		)
		return
	}

	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		maxDocumentSize,
	)

	file, header, err := r.FormFile("document")
	if err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"document_file_required",
		)
		return
	}
	defer file.Close()

	if header.Size > maxDocumentSize {
		response.Error(
			w,
			http.StatusRequestEntityTooLarge,
			"document_too_large",
		)
		return
	}

	buffer := make([]byte, 512)

	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		response.Error(
			w,
			http.StatusBadRequest,
			"failed_to_read_document",
		)
		return
	}

	if n == 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"empty_document",
		)
		return
	}

	contentType := http.DetectContentType(buffer[:n])

	var extension string

	switch contentType {
	case "image/jpeg":
		extension = ".jpg"

	case "image/png":
		extension = ".png"

	default:
		response.Error(
			w,
			http.StatusUnsupportedMediaType,
			"unsupported_document_type",
		)
		return
	}

	seeker, ok := file.(io.Seeker)
	if !ok {
		response.Error(
			w,
			http.StatusInternalServerError,
			"document_not_seekable",
		)
		return
	}

	if _, err := seeker.Seek(0, io.SeekStart); err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"failed_to_reset_document",
		)
		return
	}

	verification, err := h.service.UploadDocument(
		r.Context(),
		authUser.ID,
		verificationID,
		file,
		extension,
	)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		toVerificationResponse(verification),
	)
}

func (h *Handler) UploadSelfie(
	w http.ResponseWriter,
	r *http.Request,
) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"unauthorized",
		)
		return
	}

	verificationID, err := uuid.Parse(
		r.PathValue("id"),
	)
	if err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_verification_id",
		)
		return
	}

	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		maxDocumentSize,
	)

	file, header, err := r.FormFile("selfie")
	if err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"selfie_file_required",
		)
		return
	}
	defer file.Close()

	if header.Size > maxDocumentSize {
		response.Error(
			w,
			http.StatusRequestEntityTooLarge,
			"selfie_too_large",
		)
		return
	}

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		response.Error(
			w,
			http.StatusBadRequest,
			"failed_to_read_selfie",
		)
		return
	}

	if n == 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"empty_selfie",
		)
		return
	}

	contentType := http.DetectContentType(buffer[:n])

	var extension string

	switch contentType {
	case "image/jpeg":
		extension = ".jpg"
	case "image/png":
		extension = ".png"
	default:
		response.Error(
			w,
			http.StatusUnsupportedMediaType,
			"unsupported_selfie_type",
		)
		return
	}

	seeker, ok := file.(io.Seeker)
	if !ok {
		response.Error(
			w,
			http.StatusInternalServerError,
			"selfie_not_seekable",
		)
		return
	}

	if _, err := seeker.Seek(0, io.SeekStart); err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"failed_to_reset_selfie",
		)
		return
	}

	verification, err := h.service.UploadSelfie(
		r.Context(),
		authUser.ID,
		verificationID,
		file,
		extension,
	)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	// Kembalikan status verifikasi terbaru.
	response.JSON(
		w,
		http.StatusOK,
		toVerificationResponse(verification),
	)
}

func (h *Handler) ApproveVerification(
	w http.ResponseWriter,
	r *http.Request,
) {
	id := r.PathValue("id")

	verificationID, err := uuid.Parse(id)
	if err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_verification_id",
		)
		return
	}

	verification, err := h.service.ApproveVerification(
		r.Context(),
		verificationID,
	)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		toAdminVerificationResponse(verification),
	)
}

func (h *Handler) RejectVerification(
	w http.ResponseWriter,
	r *http.Request,
) {
	id := r.PathValue("id")

	verificationID, err := uuid.Parse(id)
	if err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_verification_id",
		)
		return
	}

	var req RejectVerificationRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_request_body",
		)
		return
	}

	if strings.TrimSpace(req.Reason) == "" {
		response.Error(
			w,
			http.StatusBadRequest,
			"rejection_reason_required",
		)
		return
	}

	verification, err := h.service.RejectVerification(
		r.Context(),
		verificationID,
		strings.TrimSpace(req.Reason),
	)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		toAdminVerificationResponse(verification),
	)
}

func (h *Handler) GetLatestVerification(
	w http.ResponseWriter,
	r *http.Request,
) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"unauthorized",
		)
		return
	}

	verification, err := h.service.GetLatestVerification(
		r.Context(),
		authUser.ID,
	)

	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		toVerificationResponse(verification),
	)
}

// ListVerifications - GET /api/v1/admin/identity-verifications (role: ADMIN)
func (h *Handler) ListVerifications(
	w http.ResponseWriter,
	r *http.Request,
) {
	statusParam := r.URL.Query().Get("status")
	var statusFilter *VerificationStatus

	if statusParam != "" {
		s := VerificationStatus(statusParam)
		if !s.IsValid() {
			response.Error(w, http.StatusBadRequest, "invalid_status_filter")
			return
		}
		statusFilter = &s
	}

	verifications, err := h.service.ListVerifications(r.Context(), statusFilter)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	resp := make([]AdminVerificationResponse, 0, len(verifications))
	for _, v := range verifications {
		resp = append(resp, toAdminVerificationResponse(v))
	}

	response.JSON(w, http.StatusOK, resp)
}

// GetVerificationAdmin - GET /api/v1/admin/identity-verifications/{id} (role: ADMIN)
func (h *Handler) GetVerificationAdmin(
	w http.ResponseWriter,
	r *http.Request,
) {
	id := r.PathValue("id")
	verificationID, err := uuid.Parse(id)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_verification_id")
		return
	}

	verification, err := h.service.GetVerificationByID(r.Context(), verificationID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, toAdminVerificationResponse(verification))
}

// GetDocumentFileAdmin - GET /api/v1/admin/identity-verifications/{id}/document (role: ADMIN)
func (h *Handler) GetDocumentFileAdmin(
	w http.ResponseWriter,
	r *http.Request,
) {
	id := r.PathValue("id")
	verificationID, err := uuid.Parse(id)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_verification_id")
		return
	}

	reader, contentType, err := h.service.GetDocumentFile(r.Context(), verificationID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}

// GetSelfieFileAdmin - GET /api/v1/admin/identity-verifications/{id}/selfie (role: ADMIN)
func (h *Handler) GetSelfieFileAdmin(
	w http.ResponseWriter,
	r *http.Request,
) {
	id := r.PathValue("id")
	verificationID, err := uuid.Parse(id)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_verification_id")
		return
	}

	reader, contentType, err := h.service.GetSelfieFile(r.Context(), verificationID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}

// GetDocumentFilePatient - GET /api/v1/patient/identity/verifications/{id}/document (role: PATIENT)
func (h *Handler) GetDocumentFilePatient(
	w http.ResponseWriter,
	r *http.Request,
) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := r.PathValue("id")
	verificationID, err := uuid.Parse(id)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_verification_id")
		return
	}

	reader, contentType, err := h.service.GetDocumentFileForPatient(r.Context(), authUser.ID, verificationID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}

// GetSelfieFilePatient - GET /api/v1/patient/identity/verifications/{id}/selfie (role: PATIENT)
func (h *Handler) GetSelfieFilePatient(
	w http.ResponseWriter,
	r *http.Request,
) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := r.PathValue("id")
	verificationID, err := uuid.Parse(id)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_verification_id")
		return
	}

	reader, contentType, err := h.service.GetSelfieFileForPatient(r.Context(), authUser.ID, verificationID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}
