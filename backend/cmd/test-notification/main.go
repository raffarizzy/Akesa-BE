package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/farhan-hamzah/Akesa/backend/internal/database"
	"github.com/farhan-hamzah/Akesa/backend/internal/notification"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	// 1. Parse command-line flags
	tokenFlag := flag.String("token", "", "FCM device token to send notification directly (optional)")
	titleFlag := flag.String("title", "Permintaan Izin Akses Rekam Medis (TEST)", "Notification title")
	messageFlag := flag.String("message", "Sebuah rumah sakit meminta izin untuk mengakses rekam medis Anda. (Tes Notifikasi Firebase)", "Notification body message")
	credFlag := flag.String("cred", "", "Path to Firebase service account JSON file (optional)")
	flag.Parse()

	// 2. Load .env
	_ = godotenv.Load()

	credPath := *credFlag
	if credPath == "" {
		credPath = os.Getenv("FIREBASE_CREDENTIALS_PATH")
	}

	fmt.Println("==================================================")
	fmt.Println("🚀 AKESA - FIREBASE PUSH NOTIFICATION TEST TOOL")
	fmt.Println("==================================================")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 3. Initialize Firebase Push Client (Same logic as main.go)
	fmt.Printf("[1/3] Menginisialisasi Firebase Push Client (file kredensial: %q)...\n", credPath)
	pushClient, err := notification.NewFirebasePushClient(ctx, credPath)
	if err != nil {
		log.Fatalf("❌ Gagal menginisialisasi FirebasePushClient: %v", err)
	}

	// Cek apakah fallback ke mock
	if _, isLogger := pushClient.(*notification.LoggerPushClient); isLogger {
		log.Printf("⚠️  Peringatan: Push client berjalan dalam mode MOCK (LoggerPushClient) karena kredensial tidak ditemukan.")
		log.Printf("    Pastikan file service account .json sudah diletakkan di folder backend/.")
	}

	// 4. Kasus A: Jika token FCM diberikan lewat flag -token
	if *tokenFlag != "" {
		fmt.Printf("\n[2/3] Mengirimkan push notifikasi langsung ke token spesifik:\n  -> Token: %s\n", maskToken(*tokenFlag))
		data := map[string]string{
			"notificationId": uuid.New().String(),
			"category":       string(notification.CategoryAccessRequest),
			"severity":       string(notification.SeverityInfo),
			"actionUrl":      "/patient/access-requests",
			"requestId":      uuid.New().String(),
			"hospitalId":     uuid.New().String(),
			"purpose":        "Pemeriksaan Umum (Test Langsung FCM)",
		}

		err := pushClient.SendPush(ctx, []string{*tokenFlag}, *titleFlag, *messageFlag, data)
		if err != nil {
			log.Fatalf("❌ Pengiriman FCM gagal: %v", err)
		}

		fmt.Println("\n==================================================")
		fmt.Println("✅ SUKSES! Push notification berhasil dikirim ke Firebase!")
		fmt.Println("   Silakan periksa layar HP Anda.")
		fmt.Println("==================================================")
		return
	}

	// 5. Kasus B: Tidak ada flag -token, gunakan database PostgreSQL seperti alur backend asli
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatalf("❌ DATABASE_URL tidak ditemukan di .env dan flag -token tidak diisi.")
	}

	fmt.Println("\n[2/3] Menghubungkan ke PostgreSQL untuk mengambil device token terdaftar...")
	db, err := database.NewPostgresPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("❌ Gagal terhubung ke database: %v", err)
	}
	defer db.Close()

	repo := notification.NewRepository(db)
	service := notification.NewService(repo, pushClient)

	// Cari device token terakhir di user_device_tokens
	type deviceRecord struct {
		id        uuid.UUID
		userID    uuid.UUID
		token     string
		platform  string
		updatedAt time.Time
	}

	rows, err := db.Query(ctx, `
		SELECT id, user_id, token, platform, updated_at
		FROM user_device_tokens
		ORDER BY updated_at DESC
		LIMIT 10
	`)
	if err != nil {
		log.Fatalf("❌ Gagal query user_device_tokens: %v", err)
	}
	defer rows.Close()

	var devices []deviceRecord
	for rows.Next() {
		var d deviceRecord
		if err := rows.Scan(&d.id, &d.userID, &d.token, &d.platform, &d.updatedAt); err == nil {
			devices = append(devices, d)
		}
	}

	if len(devices) == 0 {
		fmt.Println("\n⚠️  Belum ada device token yang tersimpan di tabel 'user_device_tokens'.")
		fmt.Println("   💡 Solusi:")
		fmt.Println("   1. Buka aplikasi Akesa di HP dan lakukan Login.")
		fmt.Println("      Saat login, aplikasi akan otomatis mendaftarkan FCM token ke backend.")
		fmt.Println("   2. Atau jalankan tool ini dengan menyertakan token FCM secara langsung:")
		fmt.Println("      go run .\\cmd\\test-notification -token=\"<FCM_TOKEN_HP_ANDA>\"")
		return
	}

	fmt.Printf("📱 Ditemukan %d perangkat terdaftar di database:\n", len(devices))
	for i, d := range devices {
		fmt.Printf("   [%d] UserID: %s | Platform: %s | Updated: %s | Token: %s\n",
			i+1, d.userID, d.platform, d.updatedAt.Format("2006-01-02 15:04:05"), maskToken(d.token))
	}

	// Gunakan user pertama (paling baru aktif)
	target := devices[0]
	fmt.Printf("\n[3/3] Menjalankan alur backend ASLI (notificationService.NotifyUser)...\n")
	fmt.Printf("   Target UserID : %s\n", target.userID)
	fmt.Printf("   Platform      : %s\n", target.platform)
	fmt.Printf("   Title         : %s\n", *titleFlag)
	fmt.Printf("   Message       : %s\n", *messageFlag)

	// Panggil NotificationService dengan input yang sama persis seperti request izin rumah sakit
	testInput := notification.NotificationInput{
		Title:     *titleFlag,
		Message:   *messageFlag,
		Category:  notification.CategoryAccessRequest,
		Severity:  notification.SeverityInfo,
		ActionURL: "/patient/access-requests",
		Metadata: map[string]any{
			"requestId":  uuid.New().String(),
			"hospitalId": uuid.New().String(),
			"purpose":    "Pemeriksaan Poli Umum (Tes Notifikasi Real-time)",
		},
	}

	err = service.NotifyUser(ctx, target.userID, testInput)
	if err != nil {
		log.Fatalf("❌ notificationService.NotifyUser gagal: %v", err)
	}

	fmt.Println("\n==================================================")
	fmt.Println("✅ SUKSES! Notifikasi berhasil disimpan ke Database DAN")
	fmt.Println("   ditembakkan via Firebase Cloud Messaging ke HP!")
	fmt.Println("   Periksa banner notifikasi di HP & lonceng di aplikasi.")
	fmt.Println("==================================================")
}

func maskToken(t string) string {
	if len(t) <= 16 {
		return t
	}
	return t[:8] + "..." + t[len(t)-6:]
}
