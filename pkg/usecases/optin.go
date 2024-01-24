package usecases

import (
	"context"
	"fmt"

	"github.com/spf13/cast"

	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/pkg/repo"
	"notiboy/pkg/repo/driver/db"
)

type OptinUseCases struct {
	repo repo.OptinRepoImply
}

type OptinUseCaseImply interface {
	Optin(context.Context, string, string, string) error
	Optout(context.Context, string, string, string) error
	OptinoutStatistics(ctx context.Context, chain, appId, statType, startDate, endDate string) (*entities.ChannelOptInOutStats, error)
}

// NewOptinUseCases
func NewOptinUseCases(OptinRepo repo.OptinRepoImply) OptinUseCaseImply {
	return &OptinUseCases{
		repo: OptinRepo,
	}
}

// Optin enables opt-in for a user with the provided user address, chain, and app ID.
func (Optin *OptinUseCases) Optin(ctx context.Context, chain, appId, userAddr string) error {
	return Optin.repo.Optin(ctx, chain, appId, userAddr)
}

// Optout disables opt-in for a user with the provided user address, chain, and app ID.
func (Optin *OptinUseCases) Optout(ctx context.Context, chain, appId, userAddr string) error {
	return Optin.repo.Optout(ctx, chain, appId, userAddr)
}

// OptinoutStatistics retrieves the opt-in/out statistics for a specific chain and app ID within the given date range.
func (Optin *OptinUseCases) OptinoutStatistics(ctx context.Context, chain, appId, statType, startDate, endDate string) (*entities.ChannelOptInOutStats, error) {
	sender := cast.ToString(ctx.Value(consts.UserAddress))

	userModel, err := db.GetUserModel(ctx, chain, sender)
	if err != nil {
		return nil, fmt.Errorf("getting user model failed")
	}

	membership := consts.MembershipStringToEnum(userModel.Membership)
	allowed := consts.Analytics[consts.OPTIN_OPTOUT_STATS][membership]

	if !allowed {
		return nil, fmt.Errorf("your membership doesn't allow analytics for channel opt in/out metrics")
	}

	data, err := Optin.repo.OptinoutStatistics(ctx, chain, appId, statType, startDate, endDate)
	if err != nil {
		return nil, err
	}

	return data, nil
}
