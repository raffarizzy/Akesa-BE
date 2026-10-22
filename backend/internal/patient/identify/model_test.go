package identity

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestVerificationStatus_IsValid(t *testing.T) {
	tests := []struct {
		status VerificationStatus
		valid  bool
	}{
		{StatusPending, true},
		{StatusProcessing, true},
		{StatusDocumentRejected, true},
		{StatusDataMismatch, true},
		{StatusLivenessFailed, true},
		{StatusFaceMismatch, true},
		{StatusManualReview, true},
		{StatusVerified, true},
		{StatusExpired, true},
		{VerificationStatus("INVALID_STATUS"), false},
		{VerificationStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.valid {
				t.Errorf("VerificationStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

func TestToAdminVerificationResponse(t *testing.T) {
	docKey := "identity/patient-1/doc.jpg"
	selfieKey := "identity/patient-1/selfie.jpg"
	nik := "3201012345670001"
	name := "John Doe"
	gender := "MALE"
	dob := time.Date(1990, 5, 15, 0, 0, 0, 0, time.UTC)
	verifiedAt := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, 10, 29, 10, 15, 0, 0, time.UTC)
	score := 92.5
	provider := "custom-ocr"

	v := &Verification{
		ID:                   uuid.New(),
		PatientID:            uuid.New(),
		Status:               StatusManualReview,
		DocumentType:         DocumentKTP,
		DocumentStorageKey:   &docKey,
		SelfieStorageKey:     &selfieKey,
		ExtractedNIK:         &nik,
		ExtractedFullName:    &name,
		ExtractedDateOfBirth: &dob,
		ExtractedGender:      &gender,
		FaceMatchScore:       &score,
		Provider:             &provider,
		VerifiedAt:           &verifiedAt,
		ExpiresAt:            &expiresAt,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	resp := toAdminVerificationResponse(v)

	if resp.ID != v.ID.String() {
		t.Errorf("expected ID %s, got %s", v.ID.String(), resp.ID)
	}
	if resp.PatientID != v.PatientID.String() {
		t.Errorf("expected PatientID %s, got %s", v.PatientID.String(), resp.PatientID)
	}
	if !resp.DocumentUploaded {
		t.Errorf("expected DocumentUploaded to be true")
	}
	if !resp.SelfieUploaded {
		t.Errorf("expected SelfieUploaded to be true")
	}
	if resp.ExtractedNIK == nil || *resp.ExtractedNIK != nik {
		t.Errorf("expected ExtractedNIK %s, got %v", nik, resp.ExtractedNIK)
	}
	if resp.ExtractedFullName == nil || *resp.ExtractedFullName != name {
		t.Errorf("expected ExtractedFullName %s, got %v", name, resp.ExtractedFullName)
	}
	if resp.ExtractedDateOfBirth == nil || *resp.ExtractedDateOfBirth != "1990-05-15" {
		t.Errorf("expected ExtractedDateOfBirth 1990-05-15, got %v", resp.ExtractedDateOfBirth)
	}
	if resp.FaceMatchScore == nil || *resp.FaceMatchScore != score {
		t.Errorf("expected FaceMatchScore %f, got %v", score, resp.FaceMatchScore)
	}
}

