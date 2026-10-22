package notification

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type PushClient interface {
	SendPush(ctx context.Context, tokens []string, title, message string, data map[string]string) error
}

// LoggerPushClient is a mock push client that only prints notification payloads to the logs.
type LoggerPushClient struct{}

func NewLoggerPushClient() *LoggerPushClient {
	return &LoggerPushClient{}
}

func (c *LoggerPushClient) SendPush(ctx context.Context, tokens []string, title, message string, data map[string]string) error {
	if len(tokens) == 0 {
		return nil
	}

	log.Printf("[PushGateway-Mock] Delivering notification to %d device(s): title=%q message=%q data=%v",
		len(tokens), title, message, data)
	for i, t := range tokens {
		masked := t
		if len(masked) > 12 {
			masked = masked[:8] + "..." + masked[len(masked)-4:]
		}
		log.Printf("[PushGateway-Mock]   -> target[%d]: token=%s", i, masked)
	}
	return nil
}

// FirebasePushClient delivers push notifications via Firebase Cloud Messaging (FCM).
type FirebasePushClient struct {
	client *messaging.Client
}

// NewFirebasePushClient initializes a Firebase FCM client with the provided credentials path.
// If the file does not exist, it searches for common filenames (*adminsdk*.json, serviceAccountKey.json),
// and if still not found, automatically falls back to LoggerPushClient.
func NewFirebasePushClient(ctx context.Context, credFilePath string) (PushClient, error) {
	resolvedPath := credFilePath
	if resolvedPath != "" {
		if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
			resolvedPath = ""
		}
	}

	if resolvedPath == "" {
		candidates := []string{"serviceAccountKey.json"}
		if matches, _ := filepath.Glob("*firebase-adminsdk*.json"); len(matches) > 0 {
			candidates = append(candidates, matches...)
		}
		if matches, _ := filepath.Glob("*adminsdk*.json"); len(matches) > 0 {
			candidates = append(candidates, matches...)
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				resolvedPath = c
				break
			}
		}
	}

	if resolvedPath == "" {
		log.Printf("[PushGateway] Firebase credentials file not found (tried %q). Falling back to mock LoggerPushClient.", credFilePath)
		return NewLoggerPushClient(), nil
	}

	opt := option.WithCredentialsFile(resolvedPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("init firebase app: %w", err)
	}

	msgClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("init firebase messaging: %w", err)
	}

	log.Printf("[PushGateway] Firebase Cloud Messaging (FCM) initialized successfully using %q", credFilePath)
	return &FirebasePushClient{client: msgClient}, nil
}

func (c *FirebasePushClient) SendPush(ctx context.Context, tokens []string, title, message string, data map[string]string) error {
	if len(tokens) == 0 {
		return nil
	}

	msg := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  message,
		},
		Data: data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Sound:     "default",
				ChannelID: "akesa_notifications",
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound: "default",
				},
			},
		},
	}

	br, err := c.client.SendEachForMulticast(ctx, msg)
	if err != nil {
		return fmt.Errorf("firebase send multicast: %w", err)
	}

	log.Printf("[PushGateway] Sent FCM to %d device(s): %d success, %d failure",
		len(tokens), br.SuccessCount, br.FailureCount)

	if br.FailureCount > 0 {
		for idx, resp := range br.Responses {
			if !resp.Success {
				masked := tokens[idx]
				if len(masked) > 12 {
					masked = masked[:8] + "..." + masked[len(masked)-4:]
				}
				log.Printf("[PushGateway]   -> Failed to send to token[%d] (%s): %v", idx, masked, resp.Error)
			}
		}
	}

	return nil
}
