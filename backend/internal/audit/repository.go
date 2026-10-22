package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const genesisHash = "0000000000000000000000000000000000000000000000000000000000000"

const chainName = "global"

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) InsertWithChain(
	ctx context.Context,
	entityType string,
	entityID uuid.UUID,
	action string,
	actorID uuid.UUID,
	payload []byte,
) (*Log, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var prevHash string
	err = tx.QueryRow(
		ctx,
		`SELECT last_hash FROM audit_chain_state WHERE chain_name = $1 FOR UPDATE`,
		chainName,
	).Scan(&prevHash)

	if err != nil {
		prevHash = genesisHash
		if _, insertErr := tx.Exec(
			ctx,
			`INSERT INTO audit_chain_state (chain_name, last_hash) VALUES ($1, $2)
			 ON CONFLICT (chain_name) DO NOTHING`,
			chainName, prevHash,
		); insertErr != nil {
			return nil, fmt.Errorf("seed chain state: %w", insertErr)
		}
	}

	now := time.Now().UTC()
	hash := computeHash(prevHash, entityType, entityID, action, actorID, payload, now)

	entry := &Log{
		ID:         uuid.New(),
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		ActorID:    actorID,
		Payload:    payload,
		PrevHash:   prevHash,
		Hash:       hash,
		CreatedAt:  now,
	}

	_, err = tx.Exec(
		ctx,
		`INSERT INTO audit_logs (
			id, entity_type, entity_id, action, actor_id, payload, prev_hash, hash, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		entry.ID, entry.EntityType, entry.EntityID, entry.Action, entry.ActorID,
		entry.Payload, entry.PrevHash, entry.Hash, entry.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert audit log: %w", err)
	}

	_, err = tx.Exec(
		ctx,
		`UPDATE audit_chain_state SET last_hash = $1 WHERE chain_name = $2`,
		entry.Hash, chainName,
	)
	if err != nil {
		return nil, fmt.Errorf("advance chain state: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return entry, nil
}

func (r *Repository) ListByEntity(ctx context.Context, entityType string, entityID uuid.UUID) ([]*Log, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT id, entity_type, entity_id, action, actor_id, payload, prev_hash, hash, created_at
		 FROM audit_logs
		 WHERE entity_type = $1 AND entity_id = $2
		 ORDER BY created_at ASC`,
		entityType, entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*Log
	for rows.Next() {
		entry := &Log{}
		if err := rows.Scan(
			&entry.ID, &entry.EntityType, &entry.EntityID, &entry.Action, &entry.ActorID,
			&entry.Payload, &entry.PrevHash, &entry.Hash, &entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, entry)
	}

	return logs, rows.Err()
}

func (r *Repository) VerifyChainIntegrity(ctx context.Context) (bool, int, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT id, entity_type, entity_id, action, actor_id, payload, prev_hash, hash, created_at
		 FROM audit_logs
		 ORDER BY created_at ASC`,
	)
	if err != nil {
		return false, 0, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	expectedPrevHash := genesisHash
	count := 0

	for rows.Next() {
		entry := &Log{}
		if err := rows.Scan(
			&entry.ID, &entry.EntityType, &entry.EntityID, &entry.Action, &entry.ActorID,
			&entry.Payload, &entry.PrevHash, &entry.Hash, &entry.CreatedAt,
		); err != nil {
			return false, count, fmt.Errorf("scan audit log at index %d: %w", count, err)
		}

		if entry.PrevHash != expectedPrevHash {
			return false, count, fmt.Errorf("chain broken at entry %s (index %d): expected prev_hash %s, got %s",
				entry.ID, count, expectedPrevHash, entry.PrevHash)
		}

		computed := computeHash(
			entry.PrevHash,
			entry.EntityType,
			entry.EntityID,
			entry.Action,
			entry.ActorID,
			entry.Payload,
			entry.CreatedAt,
		)
		if entry.Hash != computed {
			return false, count, fmt.Errorf("tamper detected at entry %s (index %d): expected hash %s, got %s",
				entry.ID, count, computed, entry.Hash)
		}

		expectedPrevHash = entry.Hash
		count++
	}

	if err := rows.Err(); err != nil {
		return false, count, fmt.Errorf("iterate audit logs: %w", err)
	}

	if count > 0 {
		var lastHash string
		err := r.db.QueryRow(ctx, `SELECT last_hash FROM audit_chain_state WHERE chain_name = $1`, chainName).Scan(&lastHash)
		if err != nil {
			return false, count, fmt.Errorf("query chain tip: %w", err)
		}
		if lastHash != expectedPrevHash {
			return false, count, fmt.Errorf("chain tip mismatch: state has %s, computed chain ended at %s", lastHash, expectedPrevHash)
		}
	}

	return true, count, nil
}
func (r *Repository) GetLatestEntityLog(ctx context.Context, entityType string, entityID uuid.UUID, action string) (*Log, error) {
	entry := &Log{}
	err := r.db.QueryRow(
		ctx,
		`SELECT id, entity_type, entity_id, action, actor_id, payload, prev_hash, hash, created_at
		 FROM audit_logs
		 WHERE entity_type = $1 AND entity_id = $2 AND action = $3
		 ORDER BY created_at DESC
		 LIMIT 1`,
		entityType, entityID, action,
	).Scan(
		&entry.ID, &entry.EntityType, &entry.EntityID, &entry.Action, &entry.ActorID,
		&entry.Payload, &entry.PrevHash, &entry.Hash, &entry.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func (r *Repository) ListSecurityAlerts(ctx context.Context, limit, offset int) ([]*Log, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	err := r.db.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE action = $1`,
		ActionIntegrityViolationBlocked,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count security alerts: %w", err)
	}

	rows, err := r.db.Query(
		ctx,
		`SELECT id, entity_type, entity_id, action, actor_id, payload, prev_hash, hash, created_at
		 FROM audit_logs
		 WHERE action = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		ActionIntegrityViolationBlocked, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query security alerts: %w", err)
	}
	defer rows.Close()

	var logs []*Log
	for rows.Next() {
		entry := &Log{}
		if err := rows.Scan(
			&entry.ID, &entry.EntityType, &entry.EntityID, &entry.Action, &entry.ActorID,
			&entry.Payload, &entry.PrevHash, &entry.Hash, &entry.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan security alert: %w", err)
		}
		logs = append(logs, entry)
	}

	return logs, total, rows.Err()
}

func (r *Repository) ListPatientTimeline(ctx context.Context, patientID uuid.UUID) ([]*Log, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT id, entity_type, entity_id, action, actor_id, payload, prev_hash, hash, created_at
		 FROM audit_logs
		 WHERE (entity_type = 'PATIENT_PROFILE' AND entity_id = $1)
		    OR (payload->>'patientId' = $2)
		 ORDER BY created_at DESC`,
		patientID, patientID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("query patient timeline: %w", err)
	}
	defer rows.Close()

	var logs []*Log
	for rows.Next() {
		entry := &Log{}
		if err := rows.Scan(
			&entry.ID, &entry.EntityType, &entry.EntityID, &entry.Action, &entry.ActorID,
			&entry.Payload, &entry.PrevHash, &entry.Hash, &entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan patient timeline log: %w", err)
		}
		logs = append(logs, entry)
	}

	return logs, rows.Err()
}

func computeHash(
	prevHash string,
	entityType string,
	entityID uuid.UUID,
	action string,
	actorID uuid.UUID,
	payload []byte,
	ts time.Time,
) string {
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write([]byte(entityType))
	h.Write([]byte(entityID.String()))
	h.Write([]byte(action))
	h.Write([]byte(actorID.String()))
	h.Write(payload)
	h.Write([]byte(ts.Format(time.RFC3339Nano)))
	return hex.EncodeToString(h.Sum(nil))
}

type PatientProfileHashRecord struct {
	PatientID   uuid.UUID
	ProfileHash string
}

type DecidedRequestRecord struct {
	RequestID  uuid.UUID
	PatientID  uuid.UUID
	HospitalID uuid.UUID
	Status     string
}

func (r *Repository) ListAllLatestProfileHashes(ctx context.Context) ([]PatientProfileHashRecord, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT DISTINCT ON (entity_id) entity_id, payload->>'profileHash'
		 FROM audit_logs
		 WHERE entity_type = 'PATIENT_PROFILE' 
		   AND action = $1
		   AND payload->>'profileHash' IS NOT NULL 
		   AND payload->>'profileHash' != ''
		 ORDER BY entity_id, created_at DESC`,
		ActionProfileUpdated,
	)
	if err != nil {
		return nil, fmt.Errorf("query all latest profile hashes: %w", err)
	}
	defer rows.Close()

	var records []PatientProfileHashRecord
	for rows.Next() {
		var rec PatientProfileHashRecord
		if err := rows.Scan(&rec.PatientID, &rec.ProfileHash); err != nil {
			return nil, fmt.Errorf("scan profile hash record: %w", err)
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (r *Repository) ListDecidedAccessRequests(ctx context.Context) ([]DecidedRequestRecord, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT id, patient_id, hospital_id, status
		 FROM access_requests
		 WHERE status IN ('APPROVED', 'REJECTED', 'REVOKED')
		 ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query decided access requests: %w", err)
	}
	defer rows.Close()

	var records []DecidedRequestRecord
	for rows.Next() {
		var rec DecidedRequestRecord
		if err := rows.Scan(&rec.RequestID, &rec.PatientID, &rec.HospitalID, &rec.Status); err != nil {
			return nil, fmt.Errorf("scan decided request record: %w", err)
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}
