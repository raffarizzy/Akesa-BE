package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/farhan-hamzah/Akesa/backend/internal/blockchain"
	"github.com/google/uuid"
)

type Service struct {
	repository *Repository
	bcClient   blockchain.Client
}

func NewService(repository *Repository, bcClient blockchain.Client) *Service {
	return &Service{
		repository: repository,
		bcClient:   bcClient,
	}
}

func (s *Service) Append(
	ctx context.Context,
	entityType string,
	entityID uuid.UUID,
	action string,
	actorID uuid.UUID,
	payload any,
) (*Log, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal audit payload: %w", err)
	}

	return s.repository.InsertWithChain(ctx, entityType, entityID, action, actorID, data)
}

func (s *Service) RecordProfileHash(ctx context.Context, patientID uuid.UUID, profileHash string) error {
	_, err := s.Append(
		ctx,
		"PATIENT_PROFILE",
		patientID,
		ActionProfileUpdated,
		patientID,
		map[string]string{"profileHash": profileHash},
	)
	if err != nil {
		return err
	}

	if s.bcClient != nil && s.bcClient.IsEnabled() {
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			txHash, bcErr := s.bcClient.RecordProfileHash(bgCtx, patientID, profileHash)
			if bcErr != nil {
				log.Printf("[blockchain] warning: failed to sync profile hash on-chain (fallback active): %v", bcErr)
			} else {
				log.Printf("[blockchain] profile hash anchored on-chain! txHash=%s", txHash)
			}
		}()
	}

	return nil
}

func (s *Service) RecordAccessEvent(
	ctx context.Context,
	entityType string,
	entityID uuid.UUID,
	action string,
	actorID uuid.UUID,
	payload map[string]any,
) error {
	_, err := s.Append(ctx, entityType, entityID, action, actorID, payload)
	if err != nil {
		return err
	}

	if s.bcClient != nil && s.bcClient.IsEnabled() {
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var status uint8
			switch action {
			case ActionAccessApproved:
				status = 2
			case ActionAccessRejected:
				status = 3
			case ActionAccessRevoked:
				status = 4
			}

			if status > 0 {
				metaBytes, _ := json.Marshal(payload)
				txHash, bcErr := s.bcClient.RecordConsent(bgCtx, entityID, actorID, uuid.Nil, status, string(metaBytes))
				if bcErr != nil {
					log.Printf("[blockchain] warning: failed to sync consent on-chain (fallback active): %v", bcErr)
				} else {
					log.Printf("[blockchain] consent anchored on-chain! txHash=%s", txHash)
				}
			} else if action == ActionDataAccessed {
				metaBytes, _ := json.Marshal(payload)
				txHash, bcErr := s.bcClient.RecordDataAccess(bgCtx, entityID, actorID, string(metaBytes))
				if bcErr != nil {
					log.Printf("[blockchain] warning: failed to sync data access on-chain (fallback active): %v", bcErr)
				} else {
					log.Printf("[blockchain] data access anchored on-chain! txHash=%s", txHash)
				}
			}
		}()
	}

	return nil
}

func (s *Service) History(ctx context.Context, entityType string, entityID uuid.UUID) ([]*Log, error) {
	return s.repository.ListByEntity(ctx, entityType, entityID)
}

func (s *Service) SecurityAlerts(ctx context.Context, limit, offset int) ([]*Log, int, error) {
	return s.repository.ListSecurityAlerts(ctx, limit, offset)
}

func (s *Service) PatientTimeline(ctx context.Context, patientID uuid.UUID) ([]*Log, error) {
	return s.repository.ListPatientTimeline(ctx, patientID)
}

func (s *Service) VerifyChainIntegrity(ctx context.Context) (bool, int, error) {
	return s.repository.VerifyChainIntegrity(ctx)
}

func (s *Service) VerifyProfileOnChain(ctx context.Context, patientID uuid.UUID, currentHash string) (bool, string, error) {
	if s.bcClient == nil || !s.bcClient.IsEnabled() {

		latestLog, err := s.repository.GetLatestEntityLog(ctx, "PATIENT_PROFILE", patientID, ActionProfileUpdated)
		if err != nil {
			return false, "", fmt.Errorf("local ledger verification: %w", err)
		}
		var payload map[string]string
		if err := json.Unmarshal(latestLog.Payload, &payload); err != nil {
			return false, "", fmt.Errorf("unmarshal audit payload: %w", err)
		}
		recordedHash := payload["profileHash"]
		return recordedHash == currentHash, recordedHash, nil
	}
	return s.bcClient.VerifyProfileHash(ctx, patientID, currentHash)
}

func (s *Service) GetLatestProfileHash(ctx context.Context, patientID uuid.UUID) (string, error) {
	latestLog, err := s.repository.GetLatestEntityLog(ctx, "PATIENT_PROFILE", patientID, ActionProfileUpdated)
	if err != nil {
		return "", err
	}
	var payload map[string]string
	if err := json.Unmarshal(latestLog.Payload, &payload); err != nil {
		return "", fmt.Errorf("unmarshal audit payload: %w", err)
	}
	return payload["profileHash"], nil
}

type SyncResult struct {
	ProfilesChecked int `json:"profilesChecked"`
	ProfilesSynced  int `json:"profilesSynced"`
	ConsentsChecked int `json:"consentsChecked"`
	ConsentsSynced  int `json:"consentsSynced"`
}

// SyncBlockchain checks and recovers any missing profile hashes or consents on the blockchain.
// This allows the blockchain to self-heal and re-synchronize after downtime or restarts.
func (s *Service) SyncBlockchain(ctx context.Context) (*SyncResult, error) {
	if s.bcClient == nil || !s.bcClient.IsEnabled() {
		return nil, blockchain.ErrClientDisabled
	}

	result := &SyncResult{}

	// 1. Sync Patient Profile Hashes (Krusial untuk verifikasi integritas FR-BC-04)
	profiles, err := s.repository.ListAllLatestProfileHashes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list profile hashes from db: %w", err)
	}

	result.ProfilesChecked = len(profiles)
	for _, p := range profiles {
		// Cek apakah hash profil sudah tercatat dan cocok di smart contract
		isValid, recordedHash, err := s.bcClient.VerifyProfileHash(ctx, p.PatientID, p.ProfileHash)
		if err != nil {
			return nil, fmt.Errorf("blockchain node unreachable: %w", err)
		}

		if !isValid || recordedHash == "" {
			log.Printf("[blockchain-sync] Patient %s hash missing on-chain (on-chain=%q vs db=%q). Re-anchoring...",
				p.PatientID, recordedHash, p.ProfileHash)

			txHash, err := s.bcClient.RecordProfileHash(ctx, p.PatientID, p.ProfileHash)
			if err != nil {
				return nil, fmt.Errorf("record profile hash for %s: %w", p.PatientID, err)
			}
			log.Printf("[blockchain-sync] Patient %s successfully re-anchored on-chain! txHash=%s", p.PatientID, txHash)
			result.ProfilesSynced++
		}
	}

	// 2. Sync Decided Consents (Approved / Rejected / Revoked)
	decidedRequests, err := s.repository.ListDecidedAccessRequests(ctx)
	if err == nil {
		result.ConsentsChecked = len(decidedRequests)
		for _, req := range decidedRequests {
			var status uint8
			switch req.Status {
			case "APPROVED":
				status = 2
			case "REJECTED":
				status = 3
			case "REVOKED":
				status = 4
			}

			if status > 0 {
				_, err := s.bcClient.RecordConsent(ctx, req.RequestID, req.PatientID, req.HospitalID, status, "")
				if err == nil {
					result.ConsentsSynced++
				}
			}
		}
	}

	return result, nil
}

// StartSyncWorker runs a background loop that automatically re-anchors missing database
// audit logs to the blockchain whenever the blockchain node comes back online.
func (s *Service) StartSyncWorker(ctx context.Context, interval time.Duration) {
	if s.bcClient == nil || !s.bcClient.IsEnabled() {
		return
	}

	go func() {
		log.Printf("[blockchain-sync] Background auto-sync worker started (interval: %v)", interval)

		// Delay 5 detik di awal memberi waktu node EVM menyelesaikan booting/deploy
		time.Sleep(5 * time.Second)
		s.runSyncCycle(ctx)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runSyncCycle(ctx)
			}
		}
	}()
}

func (s *Service) runSyncCycle(ctx context.Context) {
	syncCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	result, err := s.SyncBlockchain(syncCtx)
	if err != nil {
		// Log info tenang tanpa panik saat node offline
		log.Printf("[blockchain-sync] Node blockchain sedang offline atau belum siap (%v). Menunggu siklus berikutnya...", err)
		return
	}

	if result.ProfilesSynced > 0 || result.ConsentsSynced > 0 {
		log.Printf("[blockchain-sync] ✅ Auto-recovered & synchronized with blockchain: %d profile(s), %d consent(s)",
			result.ProfilesSynced, result.ConsentsSynced)
	}
}
