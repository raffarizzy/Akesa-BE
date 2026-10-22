package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

const selectColumns = `
	id,
	patient_id,
	status,
	document_type,
	document_storage_key,
	selfie_storage_key,
	extracted_nik,
	extracted_full_name,
	extracted_date_of_birth,
	extracted_gender,
	document_status,
	liveness_status,
	face_match_status,
	face_match_score,
	provider,
	provider_reference,
	failure_reason,
	verified_at,
	expires_at,
	created_at,
	updated_at
`

func (r *Repository) Create(
	ctx context.Context,
	verification *Verification,
) (*Verification, error) {
	query := `
		INSERT INTO identity_verifications (
			id,
			patient_id,
			status,
			document_type,
			expires_at
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + selectColumns

	created := &Verification{}

	err := r.scanRow(
		r.db.QueryRow(
			ctx,
			query,
			verification.ID,
			verification.PatientID,
			verification.Status,
			verification.DocumentType,
			verification.ExpiresAt,
		),
		created,
	)
	if err != nil {
		return nil, fmt.Errorf("create identity verification: %w", err)
	}

	return created, nil
}

func (r *Repository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*Verification, error) {
	query := `
		SELECT ` + selectColumns + `
		FROM identity_verifications
		WHERE id = $1
	`

	verification := &Verification{}

	err := r.scanRow(
		r.db.QueryRow(ctx, query, id),
		verification,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVerificationNotFound
		}

		return nil, fmt.Errorf("find identity verification: %w", err)
	}

	return verification, nil
}

func (r *Repository) List(
	ctx context.Context,
	status *VerificationStatus,
) ([]*Verification, error) {
	query := `
		SELECT ` + selectColumns + `
		FROM identity_verifications
	`
	var args []any
	if status != nil && *status != "" {
		query += ` WHERE status = $1`
		args = append(args, *status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list identity verifications: %w", err)
	}
	defer rows.Close()

	var results []*Verification
	for rows.Next() {
		v := &Verification{}
		if err := r.scanRow(rows, v); err != nil {
			return nil, fmt.Errorf("scan identity verification row: %w", err)
		}
		results = append(results, v)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate identity verifications: %w", err)
	}

	return results, nil
}

func (r *Repository) FindLatestByPatientID(
	ctx context.Context,
	patientID uuid.UUID,
) (*Verification, error) {
	const query = `
        SELECT
            id,
            patient_id,
            status,
            document_type,
            document_storage_key,
			selfie_storage_key,
            extracted_nik,
            extracted_full_name,
            extracted_date_of_birth,
            extracted_gender,
            document_status,
            liveness_status,
            face_match_status,
            face_match_score,
            provider,
            provider_reference,
            failure_reason,
            verified_at,
            expires_at,
            created_at,
            updated_at
        FROM identity_verifications
        WHERE patient_id = $1
        ORDER BY created_at DESC
        LIMIT 1
    `

	var verification Verification

	err := r.db.QueryRow(ctx, query, patientID).Scan(
		&verification.ID,
		&verification.PatientID,
		&verification.Status,
		&verification.DocumentType,
		&verification.DocumentStorageKey,
		&verification.SelfieStorageKey,
		&verification.ExtractedNIK,
		&verification.ExtractedFullName,
		&verification.ExtractedDateOfBirth,
		&verification.ExtractedGender,
		&verification.DocumentStatus,
		&verification.LivenessStatus,
		&verification.FaceMatchStatus,
		&verification.FaceMatchScore,
		&verification.Provider,
		&verification.ProviderReference,
		&verification.FailureReason,
		&verification.VerifiedAt,
		&verification.ExpiresAt,
		&verification.CreatedAt,
		&verification.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVerificationNotFound
		}

		return nil, fmt.Errorf("find latest identity verification: %w", err)
	}

	return &verification, nil
}

func (r *Repository) UpdateStatus(
	ctx context.Context,
	id uuid.UUID,
	status VerificationStatus,
	failureReason *string,
	verifiedAt *time.Time,
) (*Verification, error) {
	query := `
		UPDATE identity_verifications
		SET
			status = $2,
			failure_reason = $3,
			verified_at = $4,
			updated_at = NOW()
		WHERE id = $1
		RETURNING ` + selectColumns

	verification := &Verification{}

	err := r.scanRow(
		r.db.QueryRow(
			ctx,
			query,
			id,
			status,
			failureReason,
			verifiedAt,
		),
		verification,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVerificationNotFound
		}

		return nil, fmt.Errorf("update identity verification status: %w", err)
	}

	return verification, nil
}

func (r *Repository) scanRow(
	row pgx.Row,
	verification *Verification,
) error {
	err := row.Scan(
		&verification.ID,
		&verification.PatientID,
		&verification.Status,
		&verification.DocumentType,
		&verification.DocumentStorageKey,
		&verification.SelfieStorageKey,
		&verification.ExtractedNIK,
		&verification.ExtractedFullName,
		&verification.ExtractedDateOfBirth,
		&verification.ExtractedGender,
		&verification.DocumentStatus,
		&verification.LivenessStatus,
		&verification.FaceMatchStatus,
		&verification.FaceMatchScore,
		&verification.Provider,
		&verification.ProviderReference,
		&verification.FailureReason,
		&verification.VerifiedAt,
		&verification.ExpiresAt,
		&verification.CreatedAt,
		&verification.UpdatedAt,
	)

	return err
}

func (r *Repository) UpdateDocument(
	ctx context.Context,
	id uuid.UUID,
	storageKey string,
	status VerificationStatus,
) (*Verification, error) {
	const query = `
		UPDATE identity_verifications
		SET
			document_storage_key = $2,
			status = $3,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			patient_id,
			status,
			document_type,
			document_storage_key,
			extracted_nik,
			extracted_full_name,
			extracted_date_of_birth,
			extracted_gender,
			document_status,
			liveness_status,
			face_match_status,
			face_match_score,
			provider,
			provider_reference,
			failure_reason,
			verified_at,
			expires_at,
			created_at,
			updated_at
	`

	var verification Verification

	err := r.db.QueryRow(ctx, query,
		id,
		storageKey,
		status,
	).Scan(
		&verification.ID,
		&verification.PatientID,
		&verification.Status,
		&verification.DocumentType,
		&verification.DocumentStorageKey,
		&verification.ExtractedNIK,
		&verification.ExtractedFullName,
		&verification.ExtractedDateOfBirth,
		&verification.ExtractedGender,
		&verification.DocumentStatus,
		&verification.LivenessStatus,
		&verification.FaceMatchStatus,
		&verification.FaceMatchScore,
		&verification.Provider,
		&verification.ProviderReference,
		&verification.FailureReason,
		&verification.VerifiedAt,
		&verification.ExpiresAt,
		&verification.CreatedAt,
		&verification.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &verification, nil
}

func (r *Repository) UpdateSelfie(
	ctx context.Context,
	id uuid.UUID,
	storageKey string,
	livenessStatus string,
) (*Verification, error) {
	query := `
		UPDATE identity_verifications
		SET
			selfie_storage_key = $2,
			status = 'MANUAL_REVIEW',
			liveness_status = $3,
			updated_at = NOW()
		WHERE id = $1
		RETURNING ` + selectColumns

	verification := &Verification{}

	err := r.scanRow(
		r.db.QueryRow(
			ctx,
			query,
			id,
			storageKey,
			livenessStatus,
		),
		verification,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVerificationNotFound
		}

		return nil, fmt.Errorf(
			"update identity verification selfie: %w",
			err,
		)
	}

	return verification, nil
}

func (r *Repository) MarkVerified(
	ctx context.Context,
	verificationID uuid.UUID,
	verifiedAt time.Time,
) error {
	const query = `
        UPDATE identity_verifications
        SET
            status = 'VERIFIED',
            verified_at = $2,
            updated_at = NOW()
        WHERE id = $1
    `

	_, err := r.db.Exec(
		ctx,
		query,
		verificationID,
		verifiedAt,
	)

	return err
}

func (r *Repository) MarkRejected(
	ctx context.Context,
	verificationID uuid.UUID,
	reason string,
) error {
	const query = `
        UPDATE identity_verifications
        SET
            status = 'DOCUMENT_REJECTED',
            failure_reason = $2,
            updated_at = NOW()
        WHERE id = $1
    `

	_, err := r.db.Exec(
		ctx,
		query,
		verificationID,
		reason,
	)

	return err
}
