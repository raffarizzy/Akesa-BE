package config

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv         string
	Port           string
	DatabaseURL    string
	ClerkSecretKey string

	FieldEncryptionKey []byte

	NIKHashKey []byte

	AuditHashKey []byte

	BlockchainRPCURL          string
	BlockchainContractAddress string
	BlockchainFromAddress     string

	// MagicLinkBaseURL is the base URL/scheme used to construct invitation magic links.
	// e.g. "akesa://register-hospital" or "https://app.akesa.id/register-hospital"
	MagicLinkBaseURL string

	// FirebaseCredentialsPath is the path to the serviceAccountKey.json file.
	FirebaseCredentialsPath string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	config := &Config{
		AppEnv:                    os.Getenv("APP_ENV"),
		Port:                      os.Getenv("PORT"),
		DatabaseURL:               os.Getenv("DATABASE_URL"),
		ClerkSecretKey:            os.Getenv("CLERK_SECRET_KEY"),
		BlockchainRPCURL:          os.Getenv("BLOCKCHAIN_RPC_URL"),
		BlockchainContractAddress: os.Getenv("BLOCKCHAIN_CONTRACT_ADDRESS"),
		BlockchainFromAddress:     os.Getenv("BLOCKCHAIN_FROM_ADDRESS"),
		MagicLinkBaseURL:          os.Getenv("APP_MAGIC_LINK_BASE_URL"),
		FirebaseCredentialsPath:   os.Getenv("FIREBASE_CREDENTIALS_PATH"),
	}

	if config.FirebaseCredentialsPath == "" {
		config.FirebaseCredentialsPath = "serviceAccountKey.json"
	}

	if config.MagicLinkBaseURL == "" {
		config.MagicLinkBaseURL = "akesa://register-hospital"
	}

	if config.AppEnv == "" {
		config.AppEnv = "development"
	}

	if config.Port == "" {
		config.Port = "8080"
	}

	if config.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if config.ClerkSecretKey == "" {
		return nil, fmt.Errorf("CLERK_SECRET_KEY is required")
	}

	fieldKey, err := decodeBase64Key("FIELD_ENCRYPTION_KEY", 32)
	if err != nil {
		return nil, err
	}
	config.FieldEncryptionKey = fieldKey

	nikHashKey, err := requireSecret("NIK_HASH_KEY")
	if err != nil {
		return nil, err
	}
	config.NIKHashKey = nikHashKey

	auditHashKey, err := requireSecret("AUDIT_HASH_KEY")
	if err != nil {
		return nil, err
	}
	config.AuditHashKey = auditHashKey

	return config, nil
}

func decodeBase64Key(envVar string, wantBytes int) ([]byte, error) {
	raw := os.Getenv(envVar)
	if raw == "" {
		return nil, fmt.Errorf("%s is required (generate one with: openssl rand -base64 32)", envVar)
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be valid base64: %w", envVar, err)
	}

	if len(decoded) != wantBytes {
		return nil, fmt.Errorf("%s must decode to %d bytes, got %d", envVar, wantBytes, len(decoded))
	}

	return decoded, nil
}

func requireSecret(envVar string) ([]byte, error) {
	raw := os.Getenv(envVar)
	if len(raw) < 16 {
		return nil, fmt.Errorf("%s is required and should be at least 16 random characters", envVar)
	}
	return []byte(raw), nil
}
