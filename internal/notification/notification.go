// Package notification implements the abstraction from docs/architecture.md
// section E. Every notification is always written in-app; email is an
// additional channel dispatched asynchronously via the job queue so a slow
// SMTP server never blocks the request that triggered it.
package notification

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
	"github.com/lakhans7/eventbasedhttp/internal/jobs"
	"github.com/lakhans7/eventbasedhttp/internal/store"
)

type Service struct {
	store       *store.Store
	asynqClient *asynq.Client
	userEmailFn func(ctx context.Context, userID string) (string, error)
}

func NewService(st *store.Store, asynqClient *asynq.Client, userEmailFn func(ctx context.Context, userID string) (string, error)) *Service {
	return &Service{store: st, asynqClient: asynqClient, userEmailFn: userEmailFn}
}

type Notify struct {
	UserID       string
	Type         string
	Title        string
	Body         string
	ResourceType string
	ResourceID   string
	Email        bool // also send via email, not just in-app
}

func (s *Service) Send(ctx context.Context, n Notify) (*domain.Notification, error) {
	channels := []string{"in_app"}
	if n.Email {
		channels = append(channels, "email")
	}

	var resourceType, resourceID *string
	if n.ResourceType != "" {
		resourceType = &n.ResourceType
	}
	if n.ResourceID != "" {
		resourceID = &n.ResourceID
	}

	notif, err := s.store.CreateNotification(ctx, store.NotificationInput{
		UserID: n.UserID, Type: n.Type, Title: n.Title, Body: n.Body,
		ResourceType: resourceType, ResourceID: resourceID, Channels: channels,
	})
	if err != nil {
		return nil, err
	}

	if n.Email && s.asynqClient != nil && s.userEmailFn != nil {
		if email, err := s.userEmailFn(ctx, n.UserID); err == nil && email != "" {
			task, err := jobs.NewSendEmailTask(jobs.SendEmailPayload{To: email, Subject: n.Title, Body: n.Body})
			if err == nil {
				_, _ = s.asynqClient.Enqueue(task)
			}
		}
	}

	return notif, nil
}

// Helpers for the specific events in product section 15.

func (s *Service) NewMessage(ctx context.Context, userID, conversationID, buyerName string) (*domain.Notification, error) {
	return s.Send(ctx, Notify{UserID: userID, Type: domain.NotificationTypeNewMessage,
		Title: "New buyer message", Body: fmt.Sprintf("%s sent you a new message.", buyerName),
		ResourceType: "conversation", ResourceID: conversationID})
}

func (s *Service) NewOrder(ctx context.Context, userID, orderID string) (*domain.Notification, error) {
	return s.Send(ctx, Notify{UserID: userID, Type: domain.NotificationTypeNewOrder,
		Title: "New order", Body: "A new order was added to your dashboard.",
		ResourceType: "order", ResourceID: orderID, Email: true})
}

func (s *Service) DeadlineApproaching(ctx context.Context, userID, orderID string) (*domain.Notification, error) {
	return s.Send(ctx, Notify{UserID: userID, Type: domain.NotificationTypeDeadline,
		Title: "Order deadline approaching", Body: "One of your orders is due soon.",
		ResourceType: "order", ResourceID: orderID, Email: true})
}

func (s *Service) SyncFailure(ctx context.Context, userID, accountID, reason string) (*domain.Notification, error) {
	return s.Send(ctx, Notify{UserID: userID, Type: domain.NotificationTypeSyncFailure,
		Title: "Import failed", Body: reason,
		ResourceType: "fiverr_account", ResourceID: accountID, Email: true})
}
