package blockchain

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/sha3"
)

// selector returns the 4-byte Ethereum function selector for a signature.
func selector(sig string) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte(sig))
	return h.Sum(nil)[:4]
}

// UUIDToBytes32 converts a UUID to a 32-byte array (left-padded or raw bytes).
func UUIDToBytes32(id uuid.UUID) [32]byte {
	var b [32]byte
	copy(b[:16], id[:])
	return b
}

// HexToBytes32 converts a hex string (with or without 0x prefix) to [32]byte.
func HexToBytes32(s string) ([32]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	var b [32]byte
	if len(s) == 0 {
		return b, nil
	}
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return b, fmt.Errorf("invalid hex string: %w", err)
	}
	copy(b[:], decoded)
	return b, nil
}

// EncodeRecordProfileHash encodes call to recordProfileHash(bytes32,bytes32).
func EncodeRecordProfileHash(patientID [32]byte, profileHash [32]byte) string {
	data := append(selector("recordProfileHash(bytes32,bytes32)"), patientID[:]...)
	data = append(data, profileHash[:]...)
	return "0x" + hex.EncodeToString(data)
}

// EncodeRecordConsentDecision encodes call to recordConsentDecision(bytes32,bytes32,bytes32,uint8,bytes32).
func EncodeRecordConsentDecision(
	requestID [32]byte,
	patientID [32]byte,
	hospitalID [32]byte,
	status uint8,
	metadataHash [32]byte,
) string {
	data := append(selector("recordConsentDecision(bytes32,bytes32,bytes32,uint8,bytes32)"), requestID[:]...)
	data = append(data, patientID[:]...)
	data = append(data, hospitalID[:]...)

	// uint8 padded to 32 bytes
	var statusWord [32]byte
	statusWord[31] = status
	data = append(data, statusWord[:]...)

	data = append(data, metadataHash[:]...)
	return "0x" + hex.EncodeToString(data)
}

// EncodeRecordDataAccess encodes call to recordDataAccess(bytes32,bytes32,bytes32).
func EncodeRecordDataAccess(requestID [32]byte, actorID [32]byte, dataHash [32]byte) string {
	data := append(selector("recordDataAccess(bytes32,bytes32,bytes32)"), requestID[:]...)
	data = append(data, actorID[:]...)
	data = append(data, dataHash[:]...)
	return "0x" + hex.EncodeToString(data)
}

// EncodeVerifyProfileHash encodes call to verifyProfileHash(bytes32,bytes32).
func EncodeVerifyProfileHash(patientID [32]byte, currentHash [32]byte) string {
	data := append(selector("verifyProfileHash(bytes32,bytes32)"), patientID[:]...)
	data = append(data, currentHash[:]...)
	return "0x" + hex.EncodeToString(data)
}

// DecodeVerifyProfileHashResult decodes the (bool isValid, bytes32 recordedHash) return values.
func DecodeVerifyProfileHashResult(hexOutput string) (bool, string, error) {
	hexOutput = strings.TrimPrefix(hexOutput, "0x")
	if len(hexOutput) < 128 {
		return false, "", fmt.Errorf("output too short to decode (got %d chars, want >= 128)", len(hexOutput))
	}

	raw, err := hex.DecodeString(hexOutput)
	if err != nil {
		return false, "", fmt.Errorf("decode output hex: %w", err)
	}

	isValid := raw[31] == 1

	// Check if recordedHash is all zeros (Solidity bytes32(0), meaning uninitialized / no record)
	isZero := true
	for _, b := range raw[32:64] {
		if b != 0 {
			isZero = false
			break
		}
	}

	var recordedHash string
	if !isZero {
		recordedHash = hex.EncodeToString(raw[32:64])
	}
	return isValid, recordedHash, nil
}
