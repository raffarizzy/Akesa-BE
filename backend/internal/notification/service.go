package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Store interface {
	Create(ctx context.Context, n *Notification) error
	ListByUserID(ctx context.Context, userID uuid.UUID, unreadOnly bool, limit, offset int) ([]*Notification, int, error)
	CountUnread(ctx context.Context, userID uuid.UUID) (int, error)
	MarkAsRead(ctx context.Context, id, userID uuid.UUID) (*Notification, error)
	MarkAllAsRead(ctx context.Context, userID uuid.UUID) error
	UpsertDeviceToken(ctx context.Context, dt *DeviceToken) error
	DeleteDeviceToken(ctx context.Context, userID uuid.UUID, token string) error
	GetDeviceTokensByUserID(ctx context.Context, userID uuid.UUID) ([]string, error)
	GetUsersByRole(ctx context.Context, role string) ([]uuid.UUID, error)
}

type Notifier interface {
	NotifyUser(ctx context.Context, userID uuid.UUID, in NotificationInput) error
	NotifyRole(ctx context.Context, role string, in NotificationInput) error
}

type RealtimeBroadcaster interface {
	BroadcastToUser(userID uuid.UUID, event string, data map[string]any)
}

type Service struct {
	repo        Store
	pushClient  PushClient
	broadcaster RealtimeBroadcaster
}

func NewService(repo Store, pushClient PushClient) *Service {
	if pushClient == nil {
		pushClient = NewLoggerPushClient()
	}
	return &Service{
		repo:       repo,
		pushClient: pushClient,
	}
}

func (s *Service) SetBroadcaster(b RealtimeBroadcaster) {
	s.broadcaster = b
}

func (s *Service) NotifyUser(ctx context.Context, userID uuid.UUID, in NotificationInput) error {
	cat := in.Category
	if cat == "" {
		cat = CategorySystem
	}
	sev := in.Severity
	if sev == "" {
		sev = SeverityInfo
	}

	metaBytes := []byte("{}")
	if in.Metadata != nil {
		if b, err := json.Marshal(in.Metadata); err == nil {
			metaBytes = b
		}
	}

	var actionURL *string
	if strings.TrimSpace(in.ActionURL) != "" {
		u := strings.TrimSpace(in.ActionURL)
		actionURL = &u
	}

	now := time.Now().UTC()
	n := &Notification{
		ID:        uuid.New(),
		UserID:    userID,
		Title:     in.Title,
		Message:   in.Message,
		Category:  cat,
		Severity:  sev,
		IsRead:    false,
		ActionURL: actionURL,
		Metadata:  metaBytes,
		CreatedAt: now,
	}

	if err := s.repo.Create(ctx, n); err != nil {
		return fmt.Errorf("save notification: %w", err)
	}

	// 1. Broadcast real-time event via WebSocket jika user sedang online
	if s.broadcaster != nil {
		wsData := map[string]any{
			"id":        n.ID.String(),
			"title":     in.Title,
			"message":   in.Message,
			"category":  string(n.Category),
			"severity":  string(n.Severity),
			"createdAt": n.CreatedAt.Format(time.RFC3339),
		}
		if n.ActionURL != nil {
			wsData["actionUrl"] = *n.ActionURL
		}
		if in.Metadata != nil {
			wsData["metadata"] = in.Metadata
		}
		s.broadcaster.BroadcastToUser(userID, "notification", wsData)
	}

	// 2. Fetch device tokens for FCM push notification
	tokens, err := s.repo.GetDeviceTokensByUserID(ctx, userID)
	if err != nil {
		log.Printf("[notification] warning: failed to fetch device tokens for user %s: %v", userID, err)
		return nil
	}

	if len(tokens) > 0 && s.pushClient != nil {
		data := make(map[string]string)
		data["notificationId"] = n.ID.String()
		data["category"] = string(n.Category)
		data["severity"] = string(n.Severity)
		if n.ActionURL != nil {
			data["actionUrl"] = *n.ActionURL
		}
		if in.Metadata != nil {
			for k, v := range in.Metadata {
				data[k] = fmt.Sprintf("%v", v)
			}
		}

		if pushErr := s.pushClient.SendPush(ctx, tokens, in.Title, in.Message, data); pushErr != nil {
			log.Printf("[notification] push delivery failed for user %s: %v", userID, pushErr)
		}
	}

	return nil
}

func (s *Service) NotifyRole(ctx context.Context, role string, in NotificationInput) error {
	userIDs, err := s.repo.GetUsersByRole(ctx, role)
	if err != nil {
		return fmt.Errorf("fetch users by role: %w", err)
	}

	for _, uid := range userIDs {
		if err := s.NotifyUser(ctx, uid, in); err != nil {
			log.Printf("[notification] failed to notify user %s (role %s): %v", uid, role, err)
		}
	}
	return nil
}

func (s *Service) RegisterDeviceToken(ctx context.Context, userID uuid.UUID, token, platform string) error {
	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		return fmt.Errorf("device token cannot be empty")
	}

	plat := strings.ToLower(strings.TrimSpace(platform))
	if plat == "" {
		plat = "android"
	}

	now := time.Now().UTC()
	dt := &DeviceToken{
		ID:        uuid.New(),
		UserID:    userID,
		Token:     trimmedToken,
		Platform:  plat,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return s.repo.UpsertDeviceToken(ctx, dt)
}

func (s *Service) UnregisterDeviceToken(ctx context.Context, userID uuid.UUID, token string) error {
	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		return fmt.Errorf("device token cannot be empty")
	}
	return s.repo.DeleteDeviceToken(ctx, userID, trimmedToken)
}

func (s *Service) ListNotifications(ctx context.Context, userID uuid.UUID, unreadOnly bool, limit, offset int) (*NotificationListResponse, error) {
	items, total, err := s.repo.ListByUserID(ctx, userID, unreadOnly, limit, offset)
	if err != nil {
		return nil, err
	}

	unreadCount, err := s.repo.CountUnread(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &NotificationListResponse{
		Notifications: items,
		Total:         total,
		UnreadCount:   unreadCount,
	}, nil
}

func (s *Service) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.repo.CountUnread(ctx, userID)
}

func (s *Service) MarkAsRead(ctx context.Context, id, userID uuid.UUID) (*Notification, error) {
	return s.repo.MarkAsRead(ctx, id, userID)
}

func (s *Service) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllAsRead(ctx, userID)
}
