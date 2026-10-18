package patient

import (
	"time"

	"github.com/farhan-hamzah/Akesa/backend/internal/datacategory"
	"github.com/google/uuid"
)

type Gender string

const (
	GenderMale   Gender = "MALE"
	GenderFemale Gender = "FEMALE"
)

type Profile struct {
	ID                         uuid.UUID `json:"id"`
	UserID                     uuid.UUID `json:"userId"`
	PatientCode                string    `json:"patientCode"`
	FullName                   string    `json:"fullName"`
	NIK                        string    `json:"nik"`
	DateOfBirth                time.Time `json:"dateOfBirth"`
	Gender                     Gender    `json:"gender"`
	PhoneNumber                string    `json:"phoneNumber"`
	Address                    string    `json:"address"`
	BloodType                  *string   `json:"bloodType,omitempty"`
	DrugAllergy                *string   `json:"drugAllergy,omitempty"`
	MedicalHistory             *string   `json:"medicalHistory,omitempty"`
	InsuranceNumber            *string   `json:"insuranceNumber,omitempty"`
	EmergencyContactName       *string   `json:"emergencyContactName,omitempty"`
	EmergencyContactPhone      *string   `json:"emergencyContactPhone,omitempty"`
	CreatedAt                  time.Time `json:"createdAt"`
	UpdatedAt                  time.Time `json:"updatedAt"`
	IdentityVerificationStatus string    `json:"identityVerificationStatus"`
	ProfileCompleted           bool      `json:"profileCompleted"`
}

func (p *Profile) FilteredView(categories []datacategory.Category) map[string]any {
	view := map[string]any{
		"patientCode": p.PatientCode,
	}

	allowed := make(map[datacategory.Category]bool, len(categories))
	for _, c := range categories {
		allowed[c] = true
	}

	if allowed[datacategory.Identity] {
		view["fullName"] = p.FullName
		view["nik"] = p.NIK
		view["dateOfBirth"] = p.DateOfBirth.Format("2006-01-02")
		view["gender"] = p.Gender
	}
	if allowed[datacategory.Contact] {
		view["phoneNumber"] = p.PhoneNumber
		view["address"] = p.Address
	}
	if allowed[datacategory.MedicalBasic] {
		view["bloodType"] = p.BloodType
	}
	if allowed[datacategory.Insurance] {
		view["insuranceNumber"] = p.InsuranceNumber
	}
	if allowed[datacategory.EmergencyContact] {
		view["emergencyContactName"] = p.EmergencyContactName
		view["emergencyContactPhone"] = p.EmergencyContactPhone
	}

	return view
}
