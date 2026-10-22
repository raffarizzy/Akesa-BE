package identity

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/farhan-hamzah/Akesa/backend/internal/notification"
	"github.com/farhan-hamzah/Akesa/backend/internal/patient"
	"github.com/farhan-hamzah/Akesa/backend/internal/patient/storage"
	"github.com/google/uuid"
)

const verificationSessionDuration = 15 * time.Minute

type PatientProfileFinder interface {
	GetByUserID(
		ctx context.Context,
		userID uuid.UUID,
	) (*patient.Profile, error)
	GetByID(
		ctx context.Context,
		id uuid.UUID,
	) (*patient.Profile, error)
}

type Service struct {
	repository    *Repository
	patientFinder PatientProfileFinder
	fileStorage   storage.FileStorage
	notifier      notification.Notifier
}

func NewService(
	repository *Repository,
	patientFinder PatientProfileFinder,
	fileStorage storage.FileStorage,
) *Service {
	return &Service{
		repository:    repository,
		patientFinder: patientFinder,
		fileStorage:   fileStorage,
	}
}

func (s *Service) SetNotifier(notifier notification.Notifier) {
	s.notifier = notifier
}

func (s *Service) CreateVerification(
	ctx context.Context,
	userID uuid.UUID,
) (*Verification, error) {
	profile, err := s.patientFinder.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, patient.ErrProfileNotFound) {
			return nil, patient.ErrProfileNotFound
		}

		return nil, fmt.Errorf("get patient profile: %w", err)
	}

	latest, err := s.repository.FindLatestByPatientID(
		ctx,
		profile.ID,
	)

	if err == nil {
		if latest.Status == StatusPending ||
			latest.Status == StatusProcessing {

			if latest.ExpiresAt != nil &&
				time.Now().Before(*latest.ExpiresAt) {
				return latest, nil
			}

			_, _ = s.repository.UpdateStatus(
				ctx,
				latest.ID,
				StatusExpired,
				nil,
				nil,
			)
		}

	} else if !errors.Is(err, ErrVerificationNotFound) {
		return nil, fmt.Errorf(
			"get latest verification: %w",
			err,
		)
	}

	now := time.Now()
	expiresAt := now.Add(verificationSessionDuration)

	verification := &Verification{
		ID:           uuid.New(),
		PatientID:    profile.ID,
		Status:       StatusPending,
		DocumentType: DocumentKTP,
		ExpiresAt:    &expiresAt,
	}

	created, err := s.repository.Create(
		ctx,
		verification,
	)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (s *Service) GetVerification(
	ctx context.Context,
	userID uuid.UUID,
	verificationID uuid.UUID,
) (*Verification, error) {

	profile, err := s.patientFinder.GetByUserID(
		ctx,
		userID,
	)
	if err != nil {
		return nil, err
	}

	verification, err := s.repository.FindByID(
		ctx,
		verificationID,
	)
	if err != nil {
		return nil, err
	}

	if verification.PatientID != profile.ID {
		return nil, ErrVerificationNotFound
	}

	if verification.ExpiresAt != nil &&
		time.Now().After(*verification.ExpiresAt) {

		if verification.Status == StatusPending ||
			verification.Status == StatusProcessing {

			verification, err =
				s.repository.UpdateStatus(
					ctx,
					verification.ID,
					StatusExpired,
					nil,
					nil,
				)

			if err != nil {
				return nil, err
			}
		}
	}

	return verification, nil
}

func (s *Service) UploadDocument(
	ctx context.Context,
	userID uuid.UUID,
	verificationID uuid.UUID,
	reader io.Reader,
	extension string,
) (*Verification, error) {

	verification, err := s.GetVerification(
		ctx,
		userID,
		verificationID,
	)
	if err != nil {
		return nil, err
	}

	if verification.Status == StatusVerified {
		return nil, ErrVerificationCompleted
	}

	if verification.Status == StatusExpired {
		return nil, ErrVerificationExpired
	}

	if verification.Status != StatusPending &&
		verification.Status != StatusProcessing {
		return nil, ErrVerificationCompleted
	}

	storageKey := fmt.Sprintf(
		"identity/%s/%s/%s%s",
		verification.PatientID,
		verification.ID,
		uuid.New().String(),
		extension,
	)

	if err := s.fileStorage.Upload(
		ctx,
		storageKey,
		reader,
	); err != nil {
		return nil, fmt.Errorf(
			"upload identity document: %w",
			err,
		)
	}

	updated, err := s.repository.UpdateDocument(
		ctx,
		verification.ID,
		storageKey,
		StatusPending,
	)
	if err != nil {
		_ = s.fileStorage.Delete(
			ctx,
			storageKey,
		)

		return nil, err
	}

	return updated, nil
}

func (s *Service) UploadSelfie(
	ctx context.Context,
	userID uuid.UUID,
	verificationID uuid.UUID,
	reader io.Reader,
	extension string,
) (*Verification, error) {

	verification, err := s.GetVerification(
		ctx,
		userID,
		verificationID,
	)
	if err != nil {
		return nil, err
	}

	if verification.Status == StatusVerified {
		return nil, ErrVerificationCompleted
	}

	if verification.Status == StatusExpired {
		return nil, ErrVerificationExpired
	}

	if verification.Status != StatusPending &&
		verification.Status != StatusProcessing {
		return nil, ErrVerificationCompleted
	}

	// Selfie hanya bisa dikirim jika KTP sudah ada.
	if verification.DocumentStorageKey == nil ||
		*verification.DocumentStorageKey == "" {
		return nil, ErrDocumentNotUploaded
	}

	storageKey := fmt.Sprintf(
		"identity/%s/%s/selfie/%s%s",
		verification.PatientID,
		verification.ID,
		uuid.New().String(),
		extension,
	)

	if err := s.fileStorage.Upload(
		ctx,
		storageKey,
		reader,
	); err != nil {
		return nil, fmt.Errorf(
			"upload identity selfie: %w",
			err,
		)
	}

	updated, err := s.repository.UpdateSelfie(
		ctx,
		verification.ID,
		storageKey,
		string(StatusManualReview),
	)
	if err != nil {
		_ = s.fileStorage.Delete(
			ctx,
			storageKey,
		)

		return nil, fmt.Errorf(
			"save identity selfie: %w",
			err,
		)
	}

	if s.notifier != nil {
		_ = s.notifier.NotifyRole(ctx, "ADMIN", notification.NotificationInput{
			Title:     "Pengajuan Verifikasi Identitas Baru",
			Message:   "Seorang pasien telah mengunggah KTP dan swafoto untuk ditinjau.",
			Category:  notification.CategorySystem,
			Severity:  notification.SeverityInfo,
			ActionURL: "/admin/identity-verifications",
			Metadata: map[string]any{
				"type":           "IDENTITY_VERIFICATION",
				"verificationId": updated.ID.String(),
				"status":         string(updated.Status),
			},
		})
	}

	return updated, nil
}

func (s *Service) ApproveVerification(
	ctx context.Context,
	verificationID uuid.UUID,
) (*Verification, error) {

	verification, err := s.repository.FindByID(ctx, verificationID)
	if err != nil {
		return nil, err
	}

	hasDocuments := verification.DocumentStorageKey != nil && strings.TrimSpace(*verification.DocumentStorageKey) != "" &&
		verification.SelfieStorageKey != nil && strings.TrimSpace(*verification.SelfieStorageKey) != ""

	if verification.Status != StatusManualReview &&
		!(hasDocuments && (verification.Status == StatusPending || verification.Status == StatusProcessing)) {
		return nil, ErrVerificationNotInReview
	}

	now := time.Now()

	verification.Status = StatusVerified
	verification.VerifiedAt = &now
	verification.UpdatedAt = now

	if err := s.repository.MarkVerified(
		ctx,
		verificationID,
		now,
	); err != nil {
		return nil, err
	}

	if s.notifier != nil && s.patientFinder != nil {
		if profile, pErr := s.patientFinder.GetByID(ctx, verification.PatientID); pErr == nil && profile != nil {
			_ = s.notifier.NotifyUser(ctx, profile.UserID, notification.NotificationInput{
				Title:     "Verifikasi KTP Disetujui",
				Message:   "Identitas Anda telah berhasil diverifikasi. Kode unik pasien dan QR kini aktif.",
				Category:  notification.CategorySystem,
				Severity:  notification.SeverityInfo,
				ActionURL: "/patient/profile",
				Metadata: map[string]any{
					"type":           "IDENTITY_VERIFICATION",
					"verificationId": verification.ID.String(),
					"status":         string(StatusVerified),
				},
			})
		}
	}

	return verification, nil
}

func (s *Service) RejectVerification(
	ctx context.Context,
	verificationID uuid.UUID,
	reason string,
) (*Verification, error) {

	verification, err := s.repository.FindByID(
		ctx,
		verificationID,
	)
	if err != nil {
		return nil, err
	}

	hasDocuments := verification.DocumentStorageKey != nil && strings.TrimSpace(*verification.DocumentStorageKey) != "" &&
		verification.SelfieStorageKey != nil && strings.TrimSpace(*verification.SelfieStorageKey) != ""

	if verification.Status != StatusManualReview &&
		!(hasDocuments && (verification.Status == StatusPending || verification.Status == StatusProcessing)) {
		return nil, ErrVerificationNotInReview
	}

	verification.Status = StatusDocumentRejected
	verification.FailureReason = &reason
	verification.UpdatedAt = time.Now()

	if err := s.repository.MarkRejected(
		ctx,
		verificationID,
		reason,
	); err != nil {
		return nil, err
	}

	if s.notifier != nil && s.patientFinder != nil {
		if profile, pErr := s.patientFinder.GetByID(ctx, verification.PatientID); pErr == nil && profile != nil {
			_ = s.notifier.NotifyUser(ctx, profile.UserID, notification.NotificationInput{
				Title:     "Verifikasi KTP Ditolak",
				Message:   fmt.Sprintf("Pengajuan verifikasi KTP Anda ditolak. Alasan: %s", reason),
				Category:  notification.CategorySystem,
				Severity:  notification.SeverityWarning,
				ActionURL: "/patient/profile",
				Metadata: map[string]any{
					"type":           "IDENTITY_VERIFICATION",
					"verificationId": verification.ID.String(),
					"status":         string(StatusDocumentRejected),
					"reason":         reason,
				},
			})
		}
	}

	return verification, nil
}

func (s *Service) GetLatestVerification(
	ctx context.Context,
	userID uuid.UUID,
) (*Verification, error) {
	profile, err := s.patientFinder.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	verification, err := s.repository.FindLatestByPatientID(
		ctx,
		profile.ID,
	)
	if err != nil {
		return nil, err
	}

	if verification.ExpiresAt != nil &&
		time.Now().After(*verification.ExpiresAt) &&
		(verification.Status == StatusPending ||
			verification.Status == StatusProcessing) {

		verification, err = s.repository.UpdateStatus(
			ctx,
			verification.ID,
			StatusExpired,
			nil,
			nil,
		)

		if err != nil {
			return nil, err
		}
	}

	return verification, nil
}

func (s *Service) ListVerifications(
	ctx context.Context,
	status *VerificationStatus,
) ([]*Verification, error) {
	return s.repository.List(ctx, status)
}

func (s *Service) GetVerificationByID(
	ctx context.Context,
	verificationID uuid.UUID,
) (*Verification, error) {
	return s.repository.FindByID(ctx, verificationID)
}

func getContentType(key string) string {
	lower := strings.ToLower(key)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".pdf"):
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

func (s *Service) GetDocumentFile(
	ctx context.Context,
	verificationID uuid.UUID,
) (io.ReadCloser, string, error) {
	verification, err := s.repository.FindByID(ctx, verificationID)
	if err != nil {
		return nil, "", err
	}
	if verification.DocumentStorageKey == nil || strings.TrimSpace(*verification.DocumentStorageKey) == "" {
		return nil, "", ErrDocumentNotUploaded
	}

	rc, err := s.fileStorage.Open(ctx, *verification.DocumentStorageKey)
	if err != nil {
		return nil, "", err
	}

	return rc, getContentType(*verification.DocumentStorageKey), nil
}

func (s *Service) GetSelfieFile(
	ctx context.Context,
	verificationID uuid.UUID,
) (io.ReadCloser, string, error) {
	verification, err := s.repository.FindByID(ctx, verificationID)
	if err != nil {
		return nil, "", err
	}
	if verification.SelfieStorageKey == nil || strings.TrimSpace(*verification.SelfieStorageKey) == "" {
		return nil, "", ErrSelfieNotUploaded
	}

	rc, err := s.fileStorage.Open(ctx, *verification.SelfieStorageKey)
	if err != nil {
		return nil, "", err
	}

	return rc, getContentType(*verification.SelfieStorageKey), nil
}

func (s *Service) GetDocumentFileForPatient(
	ctx context.Context,
	userID uuid.UUID,
	verificationID uuid.UUID,
) (io.ReadCloser, string, error) {
	verification, err := s.GetVerification(ctx, userID, verificationID)
	if err != nil {
		return nil, "", err
	}
	if verification.DocumentStorageKey == nil || strings.TrimSpace(*verification.DocumentStorageKey) == "" {
		return nil, "", ErrDocumentNotUploaded
	}

	rc, err := s.fileStorage.Open(ctx, *verification.DocumentStorageKey)
	if err != nil {
		return nil, "", err
	}

	return rc, getContentType(*verification.DocumentStorageKey), nil
}

func (s *Service) GetSelfieFileForPatient(
	ctx context.Context,
	userID uuid.UUID,
	verificationID uuid.UUID,
) (io.ReadCloser, string, error) {
	verification, err := s.GetVerification(ctx, userID, verificationID)
	if err != nil {
		return nil, "", err
	}
	if verification.SelfieStorageKey == nil || strings.TrimSpace(*verification.SelfieStorageKey) == "" {
		return nil, "", ErrSelfieNotUploaded
	}

	rc, err := s.fileStorage.Open(ctx, *verification.SelfieStorageKey)
	if err != nil {
		return nil, "", err
	}

	return rc, getContentType(*verification.SelfieStorageKey), nil
}
