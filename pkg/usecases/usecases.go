package usecases

import (
	"context"

	"notiboy/pkg/repo"
)

type UseCases struct {
	repo repo.Imply
}
type UseCaseImply interface {
	DBHealthHandler(context.Context) error
	VerifyUserOnboarded(context.Context, any, string) error
}

func NewUseCases(repo repo.Imply) UseCaseImply {
	return &UseCases{
		repo: repo,
	}
}

// HealthHandler
func (usecase *UseCases) DBHealthHandler(ctx context.Context) error {
	return usecase.repo.DBHealthCheck(ctx)
}

// IsUserOnboarded
func (usecase *UseCases) VerifyUserOnboarded(ctx context.Context, address any, chain string) error {
	return usecase.repo.VerifyUserOnboarded(ctx, address, chain)
}
