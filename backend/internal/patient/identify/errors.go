package identity

import "errors"

var (
	ErrVerificationNotFound  = errors.New("identity verification not found")
	ErrVerificationExpired   = errors.New("identity verification expired")
	ErrVerificationCompleted = errors.New("identity verification already completed")
	ErrTooManyAttempts       = errors.New("too many verification attempts")

	ErrDocumentNotUploaded = errors.New("identity document not uploaded")
	ErrSelfieNotUploaded   = errors.New("identity selfie not uploaded")
	ErrInvalidDocumentType = errors.New("invalid identity document type")
	ErrDocumentTooLarge    = errors.New("identity document is too large")

	ErrInvalidDocument         = errors.New("invalid identity document")
	ErrVerificationNotInReview = errors.New("identity verification is not in manual review")
	ErrDocumentRejected        = errors.New("identity document rejected")
	ErrDataMismatch            = errors.New("identity data mismatch")
	ErrLivenessFailed          = errors.New("liveness verification failed")
	ErrFaceMismatch            = errors.New("face does not match identity document")
)
