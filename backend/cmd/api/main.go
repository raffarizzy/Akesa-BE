package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"

	"github.com/farhan-hamzah/Akesa/backend/internal/access"
	"github.com/farhan-hamzah/Akesa/backend/internal/audit"
	"github.com/farhan-hamzah/Akesa/backend/internal/blockchain"
	"github.com/farhan-hamzah/Akesa/backend/internal/config"
	"github.com/farhan-hamzah/Akesa/backend/internal/crypto"
	"github.com/farhan-hamzah/Akesa/backend/internal/database"
	"github.com/farhan-hamzah/Akesa/backend/internal/hospital"
	"github.com/farhan-hamzah/Akesa/backend/internal/notification"
	"github.com/farhan-hamzah/Akesa/backend/internal/patient"
	identity "github.com/farhan-hamzah/Akesa/backend/internal/patient/identify"
	"github.com/farhan-hamzah/Akesa/backend/internal/patient/storage"
	"github.com/farhan-hamzah/Akesa/backend/internal/patientqr"
	"github.com/farhan-hamzah/Akesa/backend/internal/server"
	"github.com/farhan-hamzah/Akesa/backend/internal/user"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	clerk.SetKey(cfg.ClerkSecretKey)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	fmt.Println("PostgreSQL connection successful")

	bcClient := blockchain.NewClient(blockchain.Config{
		RPCURL:          cfg.BlockchainRPCURL,
		ContractAddress: cfg.BlockchainContractAddress,
		FromAddress:     cfg.BlockchainFromAddress,
	})
	if bcClient.IsEnabled() {
		fmt.Printf("Blockchain layer connected: contract=%s rpc=%s\n", cfg.BlockchainContractAddress, cfg.BlockchainRPCURL)
	} else {
		fmt.Println("Blockchain layer: local cryptographic hash-chain active (on-chain anchoring disabled)")
	}

	fieldCipher, err := crypto.NewFieldCipher(cfg.FieldEncryptionKey)
	if err != nil {
		log.Fatalf("failed to initialize field encryption: %v", err)
	}
	nikHasher := crypto.NewKeyedHasher(cfg.NIKHashKey)
	auditHasher := crypto.NewKeyedHasher(cfg.AuditHashKey)

	auditRepository := audit.NewRepository(db)
	auditService := audit.NewService(auditRepository, bcClient)

	userRepository := user.NewRepository(db)
	userService := user.NewService(userRepository)
	userHandler := user.NewHandler(userService)

	patientRepository := patient.NewRepository(db, fieldCipher, nikHasher)
	patientService := patient.NewService(patientRepository, auditService, auditHasher)
	patientHandler := patient.NewHandler(patientService)

	auditHandler := audit.NewHandler(auditService, patientService)

	identityStorage, err := storage.NewLocalStorage("storage/identity")
	if err != nil {
		log.Fatalf("failed to initialize identity storage: %v", err)
	}
	identityRepository := identity.NewRepository(db)
	identityService := identity.NewService(identityRepository, patientService, identityStorage)
	identityHandler := identity.NewHandler(identityService)

	patientQRRepository := patientqr.NewRepository(db)
	patientQRService := patientqr.NewService(patientQRRepository)
	patientQRHandler := patientqr.NewHandler(patientQRService)

	hospitalRepository := hospital.NewRepository(db)
	hospitalService := hospital.NewService(hospitalRepository, userService, cfg.MagicLinkBaseURL)
	hospitalHandler := hospital.NewHandler(hospitalService)

	accessRepository := access.NewRepository(db)
	accessService := access.NewService(accessRepository, patientService, hospitalService, auditService)
	accessHandler := access.NewHandler(accessService)

	notificationRepo := notification.NewRepository(db)
	pushClient, err := notification.NewFirebasePushClient(context.Background(), cfg.FirebaseCredentialsPath)
	if err != nil {
		log.Printf("warning: failed to initialize firebase push client: %v; falling back to logger", err)
		pushClient = notification.NewLoggerPushClient()
	}
	notificationService := notification.NewService(notificationRepo, pushClient)
	notificationHandler := notification.NewHandler(notificationService)
	accessService.SetNotifier(notificationService)

	appServer := server.New(server.Dependencies{
		UserLoader:          userService,
		UserHandler:         userHandler,
		PatientHandler:      patientHandler,
		HospitalHandler:     hospitalHandler,
		AccessHandler:       accessHandler,
		AuditHandler:        auditHandler,
		PatientQRHandler:    patientQRHandler,
		IdentityHandler:     identityHandler,
		NotificationHandler: notificationHandler,
	})

	addr := ":" + cfg.Port

	fmt.Printf("Akesa API running on http://localhost%s\n", addr)

	if err := http.ListenAndServe(addr, appServer.Handler()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
