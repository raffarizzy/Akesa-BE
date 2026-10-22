package hospital

import "errors"

var (
	ErrNotFound           = errors.New("hospital not found")
	ErrEmailTaken         = errors.New("email already registered to another hospital")
	ErrRegistrationTaken  = errors.New("registration number already registered")
	ErrStaffNotFound      = errors.New("hospital staff record not found")
	ErrStaffAlreadyLinked    = errors.New("user is already linked to a hospital as staff")
	ErrHospitalNotActive     = errors.New("hospital is not active")
	ErrInvitationNotFound              = errors.New("invitation key not found")
	ErrInvitationExpired               = errors.New("invitation key has expired")
	ErrInvitationAlreadyUsed           = errors.New("invitation key has already been used")
	ErrApplicationNotFound             = errors.New("hospital application not found")
	ErrApplicationNotRevisionRequired  = errors.New("hospital application does not require revision")
	ErrApplicationAlreadyFinalized     = errors.New("hospital application has already been finalized (approved or rejected)")
	ErrApplicationAlreadyPendingReview = errors.New("hospital application is already pending review")
	ErrDocumentNotUploaded             = errors.New("hospital license document has not been uploaded")
	ErrInvalidDocumentType             = errors.New("unsupported hospital document type")
	ErrDocumentTooLarge                = errors.New("hospital document file is too large")
)
