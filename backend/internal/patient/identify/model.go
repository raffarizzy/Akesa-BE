package identity

import (
	"time"

	"github.com/google/uuid"
)

type VerificationStatus string

const (
	StatusPending          VerificationStatus = "PENDING"
	StatusProcessing       VerificationStatus = "PROCESSING"
	StatusDocumentRejected VerificationStatus = "DOCUMENT_REJECTED"
	StatusDataMismatch     VerificationStatus = "DATA_MISMATCH"
	StatusLivenessFailed   VerificationStatus = "LIVENESS_FAILED"
	StatusFaceMismatch     VerificationStatus = "FACE_MISMATCH"
	StatusManualReview     VerificationStatus = "MANUAL_REVIEW"
	StatusVerified         VerificationStatus = "VERIFIED"
	StatusExpired          VerificationStatus = "EXPIRED"
)

func (s VerificationStatus) IsValid() bool {
	switch s {
	case StatusPending,
		StatusProcessing,
		StatusDocumentRejected,
		StatusDataMismatch,
		StatusLivenessFailed,
		StatusFaceMismatch,
		StatusManualReview,
		StatusVerified,
		StatusExpired:
		return true
	default:
		return false
	}
}

type DocumentType string

const (
	DocumentKTP DocumentType = "KTP"
)

type Verification struct {
	ID                   uuid.UUID          `json:"id"`
	PatientID            uuid.UUID          `json:"patientId"`
	Status               VerificationStatus `json:"status"`
	DocumentType         DocumentType       `json:"documentType"`
	DocumentStorageKey   *string            `json:"-"`
	SelfieStorageKey     *string            `json:"-"`
	ExtractedNIK         *string            `json:"-"`
	ExtractedFullName    *string            `json:"-"`
	ExtractedDateOfBirth *time.Time         `json:"-"`
	ExtractedGender      *string            `json:"-"`
	DocumentStatus       *string            `json:"documentStatus,omitempty"`
	LivenessStatus       *string            `json:"livenessStatus,omitempty"`
	FaceMatchStatus      *string            `json:"faceMatchStatus,omitempty"`
	FaceMatchScore       *float64           `json:"-"`
	Provider             *string            `json:"provider,omitempty"`
	ProviderReference    *string            `json:"-"`
	FailureReason        *string            `json:"failureReason,omitempty"`
	VerifiedAt           *time.Time         `json:"verifiedAt,omitempty"`
	ExpiresAt            *time.Time         `json:"expiresAt,omitempty"`
	CreatedAt            time.Time          `json:"createdAt"`
	UpdatedAt            time.Time          `json:"updatedAt"`
}
