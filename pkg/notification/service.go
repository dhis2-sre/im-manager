package notification

import (
	"context"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

type Service struct {
	repository *repository
}

func NewService(repository *repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) FindByUserID(ctx context.Context, userID uint) ([]model.Notification, error) {
	return s.repository.findByUserID(ctx, userID)
}

func (s *Service) MarkRead(ctx context.Context, id, userID uint) error {
	return s.repository.markRead(ctx, id, userID)
}

func (s *Service) MarkAllRead(ctx context.Context, userID uint) error {
	return s.repository.markAllRead(ctx, userID)
}
