package usecases

import (
	"context"
	"fmt"

	"notiboy/pkg/entities"
	"notiboy/pkg/repo"
)

type UserUseCases struct {
	repo repo.UserRepoImply
}

type UserUseCaseImply interface {
	ProfileUpdate(context.Context, entities.UserInfo) error
	Onboarding(context.Context, entities.OnboardingRequest) error
	Offboarding(context.Context, string, string) error
	GlobalStatistics(context.Context) ([]entities.GlobalStatistics, error)
	UserStatistics(ctx context.Context, chain, statType, startDate, endDate string) ([]entities.UserActivity, error)
	GetUser(context.Context, entities.UserIdentifier) (*entities.Response, error)
	Login(context.Context, entities.UserIdentifier) (string, error)
	Logout(context.Context, entities.UserIdentifier) error
	GeneratePAT(context.Context, string, string, string) (string, error)
	GetPAT(context.Context, string) ([]entities.PATTokens, error)
	RevokePAT(context.Context, string, string) error
	StoreFCMToken(ctx context.Context, fcm entities.FCM) error
}

// NewUserUseCases
func NewUserUseCases(userRepo repo.UserRepoImply) UserUseCaseImply {
	return &UserUseCases{
		repo: userRepo,
	}
}

// ProfileUpdate updates the user's profile with the provided data.
func (user *UserUseCases) ProfileUpdate(ctx context.Context, data entities.UserInfo) error {
	return user.repo.ProfileUpdate(ctx, data)
}

// Onboarding performs the onboarding process for a user using the provided data.
func (user *UserUseCases) Onboarding(ctx context.Context, data entities.OnboardingRequest) error {
	return user.repo.Onboarding(ctx, data)
}

// Offboarding removes the user's account and associated data from the system.
func (user *UserUseCases) Offboarding(ctx context.Context, address string, chain string) error {
	return user.repo.Offboarding(ctx, address, chain)
}

// GlobalStatistics retrieves global statistics from the repository.
func (user *UserUseCases) GlobalStatistics(ctx context.Context) ([]entities.GlobalStatistics, error) {
	data, err := user.repo.GlobalStatistics(ctx)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no records found")
	}
	return data, nil
}

// UserStatistics retrieves user activity statistics based on the specified parameters.
func (user *UserUseCases) UserStatistics(ctx context.Context, chain string, statType string, startDate string, endDate string) ([]entities.UserActivity, error) {
	data, err := user.repo.UserStatistics(ctx, chain, statType, startDate, endDate)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// GetUser retrieves user information based on the specified request.
func (user *UserUseCases) GetUser(ctx context.Context, request entities.UserIdentifier) (*entities.Response, error) {
	return user.repo.GetUser(ctx, request)
}

// Login performs user login based on the provided login request.
func (user *UserUseCases) Login(ctx context.Context, request entities.UserIdentifier) (string, error) {
	return user.repo.Login(ctx, request)
}

// Logout performs user logout based on the provided logout request.
func (user *UserUseCases) Logout(ctx context.Context, request entities.UserIdentifier) error {
	return user.repo.Logout(ctx, request)
}

func (user *UserUseCases) GeneratePAT(ctx context.Context, name, kind, description string) (string, error) {
	return user.repo.GeneratePAT(ctx, name, kind, description)
}

func (user *UserUseCases) GetPAT(ctx context.Context, kind string) ([]entities.PATTokens, error) {
	return user.repo.GetPAT(ctx, kind)
}

func (user *UserUseCases) RevokePAT(ctx context.Context, id, kind string) error {
	return user.repo.RevokePAT(ctx, id, kind)
}

func (user *UserUseCases) StoreFCMToken(ctx context.Context, fcm entities.FCM) error {
	return user.repo.StoreFCMToken(ctx, fcm)
}
