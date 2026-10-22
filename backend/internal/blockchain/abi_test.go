package blockchain

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestABIEncodingAndDecoding(t *testing.T) {
	patientID := uuid.New()
	hashStr := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	patB32 := UUIDToBytes32(patientID)
	hashB32, err := HexToBytes32(hashStr)
	if err != nil {
		t.Fatalf("HexToBytes32 error: %v", err)
	}

	encoded := EncodeRecordProfileHash(patB32, hashB32)
	if !strings.HasPrefix(encoded, "0x") {
		t.Errorf("expected 0x prefix, got %s", encoded)
	}

	// 4 bytes selector + 32 bytes patientID + 32 bytes profileHash = 68 bytes = 136 hex chars + 2 prefix = 138 chars
	if len(encoded) != 138 {
		t.Errorf("expected length 138, got %d", len(encoded))
	}

	// Test DecodeVerifyProfileHashResult with valid true
	var mockOutput [64]byte
	mockOutput[31] = 1 // bool true
	copy(mockOutput[32:64], hashB32[:])

	mockHex := "0x" + hex.EncodeToString(mockOutput[:])
	isValid, recorded, err := DecodeVerifyProfileHashResult(mockHex)
	if err != nil {
		t.Fatalf("DecodeVerifyProfileHashResult failed: %v", err)
	}

	if !isValid {
		t.Errorf("expected isValid to be true")
	}

	if recorded != hashStr {
		t.Errorf("expected recorded hash to match %s, got %s", hashStr, recorded)
	}

	// Test DecodeVerifyProfileHashResult with bytes32(0) (unanchored record on-chain)
	var zeroOutput [64]byte
	zeroHex := "0x" + hex.EncodeToString(zeroOutput[:])
	isValidZero, recordedZero, err := DecodeVerifyProfileHashResult(zeroHex)
	if err != nil {
		t.Fatalf("DecodeVerifyProfileHashResult on zero bytes failed: %v", err)
	}
	if isValidZero {
		t.Errorf("expected isValid to be false for zero bytes32")
	}
	if recordedZero != "" {
		t.Errorf("expected recordedHash to be empty string for zero bytes32, got %q", recordedZero)
	}
}
