package access

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/farhan-hamzah/Akesa/backend/internal/hospital"
	"github.com/farhan-hamzah/Akesa/backend/internal/notification"
	"github.com/farhan-hamzah/Akesa/backend/internal/patient"
	"github.com/google/uuid"
)

type PatientLookup interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) (*patient.Profile, error)
	GetByPatientCode(ctx context.Context, patientCode string) (*patient.Profile, error)
	GetByID(ctx context.Context, id uuid.UUID) (*patient.Profile, error)
	ComputeHash(profile *patient.Profile) string
}

type StaffLookup interface {
	GetStaffByUserID(ctx context.Context, userID uuid.UUID) (*hospital.Staff, error)
}

type AuditRecorder interface {
	RecordProfileHash(ctx context.Context, patientID uuid.UUID, profileHash string) error
	RecordAccessEvent(ctx context.Context, entityType string, entityID uuid.UUID, action string, actorID uuid.UUID, payload map[string]any) error
	VerifyProfileOnChain(ctx context.Context, patientID uuid.UUID, currentHash string) (bool, string, error)
	GetLatestProfileHash(ctx context.Context, patientID uuid.UUID) (string, error)
}

const entityTypeAccessRequest = "ACCESS_REQUEST"

const (
	actionRequested = "ACCESS_REQUESTED"
	actionApproved  = "ACCESS_APPROVED"
	actionRejected  = "ACCESS_REJECTED"
	actionRevoked   = "ACCESS_REVOKED"
	actionAccessed  = "DATA_ACCESSED"
)

type Service struct {
	repository *Repository
	patients   PatientLookup
	staff      StaffLookup
	audit      AuditRecorder
	notifier   notification.Notifier
}

func NewService(repository *Repository, patients PatientLookup, staff StaffLookup, audit AuditRecorder) *Service {
	return &Service{repository: repository, patients: patients, staff: staff, audit: audit}
}

func (s *Service) SetNotifier(notifier notification.Notifier) {
	s.notifier = notifier
}

func (s *Service) CreateRequest(ctx context.Context, staffUserID uuid.UUID, in CreateRequestInput) (*Request, error) {
	staff, err := s.staff.GetStaffByUserID(ctx, staffUserID)
	if err != nil {
		return nil, err
	}

	patientProfile, err := s.patients.GetByPatientCode(ctx, in.PatientCode)
	if err != nil {
		if errors.Is(err, patient.ErrProfileNotFound) {
			return nil, ErrPatientNotFound
		}
		return nil, err
	}

	req, err := s.repository.Create(ctx, staff.HospitalID, patientProfile.ID, staffUserID, in.Purpose, in.Categories)
	if err != nil {
		return nil, err
	}

	s.recordEvent(ctx, req.ID, actionRequested, staffUserID, map[string]any{
		"hospitalId": staff.HospitalID.String(),
		"categories": in.Categories,
		"patientId":  patientProfile.ID.String(),
	})

	if s.notifier != nil {
		_ = s.notifier.NotifyUser(ctx, patientProfile.UserID, notification.NotificationInput{
			Title:     "Permintaan Izin Akses Rekam Medis",
			Message:   "Sebuah rumah sakit meminta izin untuk mengakses rekam medis Anda.",
			Category:  notification.CategoryAccessRequest,
			Severity:  notification.SeverityInfo,
			ActionURL: "/patient/access-requests",
			Metadata: map[string]any{
				"requestId":  req.ID.String(),
				"hospitalId": staff.HospitalID.String(),
				"purpose":    in.Purpose,
			},
		})
	}

	return req, nil
}

func (s *Service) ListForPatient(ctx context.Context, userID uuid.UUID) ([]*Request, error) {
	patientProfile, err := s.patients.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, patient.ErrProfileNotFound) {
			return []*Request{}, nil
		}
		return nil, err
	}

	return s.repository.ListByPatient(ctx, patientProfile.ID)
}

func (s *Service) ListForHospitalStaff(ctx context.Context, staffUserID uuid.UUID) ([]*Request, error) {
	staff, err := s.staff.GetStaffByUserID(ctx, staffUserID)
	if err != nil {
		return nil, err
	}
	return s.repository.ListByHospital(ctx, staff.HospitalID)
}

func (s *Service) Approve(ctx context.Context, userID, requestID uuid.UUID) (*Request, error) {
	req, err := s.ownedPendingRequest(ctx, userID, requestID)
	if err != nil {
		return nil, err
	}

	updated, err := s.repository.UpdateStatus(
		ctx,
		req.ID,
		StatusPending,
		StatusApproved,
	)
	if err != nil {
		return nil, err
	}

	s.recordEvent(ctx, updated.ID, actionApproved, userID, map[string]any{
		"categories": updated.Categories,
		"patientId":  updated.PatientID.String(),
		"hospitalId": updated.HospitalID.String(),
	})

	if s.notifier != nil {
		_ = s.notifier.NotifyUser(ctx, updated.RequestedBy, notification.NotificationInput{
			Title:     "Permintaan Akses Disetujui",
			Message:   "Pasien telah menyetujui permintaan akses rekam medis.",
			Category:  notification.CategoryAccessRequest,
			Severity:  notification.SeverityInfo,
			ActionURL: fmt.Sprintf("/hospital/access-requests/%s/data", updated.ID),
			Metadata: map[string]any{
				"requestId": updated.ID.String(),
				"patientId": updated.PatientID.String(),
			},
		})
	}

	return updated, nil
}

func (s *Service) Reject(ctx context.Context, userID, requestID uuid.UUID) (*Request, error) {
	req, err := s.ownedPendingRequest(ctx, userID, requestID)
	if err != nil {
		return nil, err
	}

	updated, err := s.repository.UpdateStatus(
		ctx,
		req.ID,
		StatusPending,
		StatusRejected,
	)
	if err != nil {
		return nil, err
	}

	s.recordEvent(ctx, updated.ID, actionRejected, userID, map[string]any{
		"patientId":  updated.PatientID.String(),
		"hospitalId": updated.HospitalID.String(),
	})

	if s.notifier != nil {
		_ = s.notifier.NotifyUser(ctx, updated.RequestedBy, notification.NotificationInput{
			Title:     "Permintaan Akses Ditolak",
			Message:   "Pasien telah menolak permintaan akses rekam medis.",
			Category:  notification.CategoryAccessRequest,
			Severity:  notification.SeverityWarning,
			ActionURL: "/hospital/access-requests",
			Metadata: map[string]any{
				"requestId": updated.ID.String(),
				"patientId": updated.PatientID.String(),
			},
		})
	}

	return updated, nil
}

func (s *Service) Revoke(ctx context.Context, patientUserID, requestID uuid.UUID) (*Request, error) {
	profile, err := s.patients.GetByUserID(ctx, patientUserID)
	if err != nil {
		if errors.Is(err, patient.ErrProfileNotFound) {
			return nil, ErrNotOwner
		}
		return nil, err
	}

	req, err := s.repository.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if req.PatientID != profile.ID {
		return nil, ErrNotOwner
	}

	if req.Status != StatusApproved {
		return nil, ErrNotApproved
	}

	updated, err := s.repository.UpdateStatus(
		ctx,
		req.ID,
		StatusApproved,
		StatusRevoked,
	)
	if err != nil {
		return nil, err
	}

	s.recordEvent(ctx, updated.ID, actionRevoked, patientUserID, map[string]any{
		"patientId":  updated.PatientID.String(),
		"hospitalId": updated.HospitalID.String(),
	})

	if s.notifier != nil {
		_ = s.notifier.NotifyUser(ctx, updated.RequestedBy, notification.NotificationInput{
			Title:     "Izin Akses Dicabut",
			Message:   "Pasien telah mencabut izin akses rekam medis.",
			Category:  notification.CategoryAccessRequest,
			Severity:  notification.SeverityWarning,
			ActionURL: "/hospital/access-requests",
			Metadata: map[string]any{
				"requestId": updated.ID.String(),
				"patientId": updated.PatientID.String(),
				"status":    "REVOKED",
			},
		})
	}
	return updated, nil
}

func (s *Service) FetchApprovedData(ctx context.Context, staffUserID, requestID uuid.UUID) (map[string]any, error) {
	staff, err := s.staff.GetStaffByUserID(ctx, staffUserID)
	if err != nil {
		return nil, err
	}

	req, err := s.repository.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if req.HospitalID != staff.HospitalID {
		return nil, ErrNotOwner
	}
	if !req.IsGrantedNow() {
		return nil, ErrNotApproved
	}

	patientProfile, err := s.patients.GetByID(ctx, req.PatientID)
	if err != nil {
		return nil, err
	}

	currentHash := s.patients.ComputeHash(patientProfile)
	if s.audit != nil {
		valid, recordedHash, err := s.audit.VerifyProfileOnChain(ctx, req.PatientID, currentHash)
		if err != nil || recordedHash == "" {
			if err != nil {
				log.Printf("[integrity] blockchain verify error (%v), falling back to local audit ledger", err)
			} else {
				log.Printf("[integrity] patient %s profile hash missing on-chain, falling back to local audit ledger", req.PatientID)
			}
			latestHash, lerr := s.audit.GetLatestProfileHash(ctx, req.PatientID)
			if lerr == nil && latestHash != "" {
				valid = (latestHash == currentHash)
				recordedHash = latestHash
			}
		}

		// Auto-anchor unanchored patient profile (e.g. legacy/dump data without previous audit log)
		if recordedHash == "" {
			log.Printf("[integrity] patient %s has never been anchored in blockchain or audit ledger. Auto-anchoring baseline hash...", req.PatientID)
			_ = s.audit.RecordProfileHash(ctx, req.PatientID, currentHash)
			valid = true
		}

		if !valid && recordedHash != "" {
			log.Printf("[SECURITY ALERT - FR-BC-04] Patient data integrity violation! PatientID=%s currentHash=%s recordedHash=%s",
				req.PatientID, currentHash, recordedHash)
			s.recordEvent(ctx, req.ID, "INTEGRITY_VIOLATION_BLOCKED", staffUserID, map[string]any{
				"error":        "hash_mismatch",
				"currentHash":  currentHash,
				"recordedHash": recordedHash,
				"patientId":    req.PatientID.String(),
				"hospitalId":   req.HospitalID.String(),
				"reason":       "Real-time patient profile hash does not match anchored ledger hash",
			})

			if s.notifier != nil {
				_ = s.notifier.NotifyUser(ctx, patientProfile.UserID, notification.NotificationInput{
					Title:     "Peringatan Integritas Data",
					Message:   "Upaya pembacaan rekam medis Anda diblokir otomatis karena hash data tidak cocok dengan rekaman blockchain.",
					Category:  notification.CategorySecurityAlert,
					Severity:  notification.SeverityCritical,
					ActionURL: "/patient/history",
					Metadata: map[string]any{
						"requestId":    req.ID.String(),
						"error":        "hash_mismatch",
						"currentHash":  currentHash,
						"recordedHash": recordedHash,
					},
				})
				_ = s.notifier.NotifyRole(ctx, "ADMIN", notification.NotificationInput{
					Title:     "Insiden Keamanan: Data Tampered Terdeteksi",
					Message:   fmt.Sprintf("Hash mismatch pada pasien %s saat diakses staf %s.", req.PatientID, staffUserID),
					Category:  notification.CategorySecurityAlert,
					Severity:  notification.SeverityCritical,
					ActionURL: "/admin/security-alerts",
					Metadata: map[string]any{
						"patientId":   req.PatientID.String(),
						"staffUserId": staffUserID.String(),
					},
				})
			}

			return nil, ErrDataTampered
		}
	}

	view := patientProfile.FilteredView(req.Categories)

	s.recordEvent(ctx, req.ID, actionAccessed, staffUserID, map[string]any{
		"categories": req.Categories,
		"dataHash":   currentHash,
		"patientId":  req.PatientID.String(),
		"hospitalId": req.HospitalID.String(),
	})

	if s.notifier != nil {
		_ = s.notifier.NotifyUser(ctx, patientProfile.UserID, notification.NotificationInput{
			Title:     "Data Rekam Medis Diakses",
			Message:   "Rumah sakit sedang mengakses rekam medis Anda.",
			Category:  notification.CategoryAccessRequest,
			Severity:  notification.SeverityInfo,
			ActionURL: "/patient/history",
			Metadata: map[string]any{
				"requestId":  req.ID.String(),
				"patientId":  req.PatientID.String(),
				"hospitalId": req.HospitalID.String(),
				"action":     actionAccessed,
			},
		})
	}

	return view, nil
}

func (s *Service) ownedPendingRequest(ctx context.Context, patientUserID, requestID uuid.UUID) (*Request, error) {
	profile, err := s.patients.GetByUserID(ctx, patientUserID)
	if err != nil {
		if errors.Is(err, patient.ErrProfileNotFound) {
			return nil, ErrNotOwner
		}
		return nil, err
	}

	req, err := s.repository.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if req.PatientID != profile.ID {
		return nil, ErrNotOwner
	}
	if req.Status != StatusPending {
		return nil, ErrNotPending
	}
	return req, nil
}

func (s *Service) recordEvent(ctx context.Context, requestID uuid.UUID, action string, actorID uuid.UUID, payload map[string]any) {
	if s.audit == nil {
		return
	}
	if err := s.audit.RecordAccessEvent(ctx, entityTypeAccessRequest, requestID, action, actorID, payload); err != nil {
		log.Printf("audit: failed to record %s for access_request=%s: %v", action, requestID, err)
	}
}
