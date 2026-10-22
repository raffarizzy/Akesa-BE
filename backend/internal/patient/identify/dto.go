package identity

type VerificationResponse struct {
	ID              *string `json:"id"`
	Status          *string `json:"status"`
	DocumentType    *string `json:"documentType"`
	DocumentStatus  *string `json:"documentStatus,omitempty"`
	LivenessStatus  *string `json:"livenessStatus,omitempty"`
	FaceMatchStatus *string `json:"faceMatchStatus,omitempty"`
	FailureReason   *string `json:"failureReason,omitempty"`
	VerifiedAt      *string `json:"verifiedAt,omitempty"`
	ExpiresAt       *string `json:"expiresAt,omitempty"`

	DocumentUploaded bool `json:"documentUploaded"`
	SelfieUploaded   bool `json:"selfieUploaded"`
}

type AdminVerificationResponse struct {
	ID                   string             `json:"id"`
	PatientID            string             `json:"patientId"`
	Status               VerificationStatus `json:"status"`
	DocumentType         DocumentType       `json:"documentType"`
	DocumentUploaded     bool               `json:"documentUploaded"`
	SelfieUploaded       bool               `json:"selfieUploaded"`
	ExtractedNIK         *string            `json:"extractedNik,omitempty"`
	ExtractedFullName    *string            `json:"extractedFullName,omitempty"`
	ExtractedDateOfBirth *string            `json:"extractedDateOfBirth,omitempty"`
	ExtractedGender      *string            `json:"extractedGender,omitempty"`
	DocumentStatus       *string            `json:"documentStatus,omitempty"`
	LivenessStatus       *string            `json:"livenessStatus,omitempty"`
	FaceMatchStatus      *string            `json:"faceMatchStatus,omitempty"`
	FaceMatchScore       *float64           `json:"faceMatchScore,omitempty"`
	Provider             *string            `json:"provider,omitempty"`
	FailureReason        *string            `json:"failureReason,omitempty"`
	VerifiedAt           *string            `json:"verifiedAt,omitempty"`
	ExpiresAt            *string            `json:"expiresAt,omitempty"`
	CreatedAt            string             `json:"createdAt"`
	UpdatedAt            string             `json:"updatedAt"`
}
