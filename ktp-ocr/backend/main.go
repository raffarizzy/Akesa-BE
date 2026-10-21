package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"ktp-ocr/backend/internal/client"
	"ktp-ocr/backend/internal/handler"
	"ktp-ocr/backend/internal/service"
)

func main() {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8000"
	}

	timeoutSec := 60
	if envTimeout := os.Getenv("TIMEOUT_SECONDS"); envTimeout != "" {
		if parsed, err := strconv.Atoi(envTimeout); err == nil && parsed > 0 {
			timeoutSec = parsed
		}
	}
	timeout := time.Duration(timeoutSec) * time.Second

	log.Printf("==================================================")
	log.Printf("KTP OCR Go Backend Service Starting...")
	log.Printf("Port            : %s", port)
	log.Printf("AI Service URL  : %s", aiServiceURL)
	log.Printf("Request Timeout : %v", timeout)
	log.Printf("==================================================")

	aiClient := client.NewAIClient(aiServiceURL, timeout)
	ocrService := service.NewOCRService(aiClient)
	ocrHandler := handler.NewOCRHandler(ocrService)

	router := gin.Default()

	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	router.GET("/health", ocrHandler.HandleHealth)

	v1 := router.Group("/api/v1")
	{
		ktp := v1.Group("/ktp")
		{
			ktp.POST("/ocr", ocrHandler.HandleKTPOCR)
		}
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Go HTTP server listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Server gagal dijalankan: %v", err)
	}
}
