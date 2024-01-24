package usecases

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cast"

	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/pkg/repo"
	"notiboy/pkg/repo/driver/db"
	"notiboy/utilities"
)

type ChannelUseCases struct {
	repo     repo.ChannelRepoImpl
	userRepo repo.UserRepoImply
}

type ChannelUseCaseImply interface {
	ChannelCreate(context.Context, entities.ChannelInfo, string) (*entities.Response, error)
	ChannelUpdate(context.Context, entities.ChannelInfo) error
	ChannelUsers(context.Context, *http.Request, *entities.ListChannelUsersRequest) (*entities.Response, error)
	ListOptedInChannels(ctx context.Context, chain, address string, withLogo bool) (*entities.Response, error)
	ListChannels(ctx context.Context, req *entities.ListChannelRequest) (*entities.Response, error)
	ListUserOwnedChannels(ctx context.Context, r *http.Request, chain string, address string, withLogo bool) (*entities.Response, error)
	DeleteChannel(ctx context.Context, chain string, appId string, address string) error
	ChannelStatistics(ctx context.Context, r *http.Request, chain, typeStr, startDate, endDate string) ([]entities.ChannelActivity, error)
	ChannelReadSentStatistics(ctx context.Context, r *http.Request, chain, channel, fetchKind, startDate, endDate string) ([]entities.ChannelReadSentResponse, error)
	ChannelNotificationStatistics(ctx context.Context, r *http.Request, chain string, channel string, address string, typeStr, startDate, endDate string, limit int, offset int) (*repo.ChannelStats, error)
	VerifyChannel(ctx context.Context, chain, appID string) error
}

func NewChannelUseCases(repo repo.ChannelRepoImpl, userRepo repo.UserRepoImply) ChannelUseCaseImply {
	return &ChannelUseCases{
		repo:     repo,
		userRepo: userRepo,
	}
}

// ChannelNotificationStatistics retrieves the notification statistics for a specific channel.
func (cuc *ChannelUseCases) ChannelNotificationStatistics(ctx context.Context, r *http.Request, chain string, channel string, address string, typeStr, startDate, endDate string, limit int, page int) (*repo.ChannelStats, error) {
	stats, err := cuc.repo.ChannelNotificationStatistics(ctx, r, chain, channel, address, typeStr, startDate, endDate, limit, page)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// ChannelUsers retrieves the list of users associated with a specific channel.
func (cuc *ChannelUseCases) ChannelUsers(ctx context.Context, r *http.Request, req *entities.ListChannelUsersRequest) (*entities.Response, error) {
	response, err := cuc.repo.ChannelUsers(ctx, r, req)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// ChannelCreate creates a new channel with the provided channel information and chain.
func (cuc *ChannelUseCases) ChannelCreate(ctx context.Context, data entities.ChannelInfo, chain string) (*entities.Response, error) {
	userModel, err := db.GetUserModel(ctx, chain, data.Address)
	if err != nil {
		return nil, fmt.Errorf("getting user model failed")
	}
	membership := consts.MembershipStringToEnum(userModel.Membership)
	permittedChannelCount := consts.ChannelCount[membership]
	newChannelCount := len(userModel.Channels) + 1
	if newChannelCount > permittedChannelCount {
		return nil, fmt.Errorf("cannot create channel as your membership allows only %d channels", permittedChannelCount)
	}
	return cuc.repo.ChannelCreate(ctx, data, chain)
}

// ChannelUpdate updates an existing channel with the provided channel information.
func (cuc *ChannelUseCases) ChannelUpdate(ctx context.Context, data entities.ChannelInfo) error {

	return cuc.repo.ChannelUpdate(ctx, data)
}

func (cuc *ChannelUseCases) ListOptedInChannels(ctx context.Context, chain, address string, withLogo bool) (*entities.Response, error) {
	log := utilities.NewLoggerWithFields("ListOptedInChannels", map[string]interface{}{
		"chain":   chain,
		"address": address,
	})

	userModel, err := cuc.userRepo.GetUser(ctx, entities.UserIdentifier{
		Chain:   chain,
		Address: address,
	})
	if err != nil {
		return &entities.Response{}, err
	}

	data := userModel.Data.(*entities.UserModel)
	if len(data.Optins) == 0 {
		log.Warn("no optins found")
		return &entities.Response{}, nil
	}

	response, err := cuc.repo.ListOptedInChannels(ctx, chain, address, data.Optins, withLogo)
	if err != nil {
		return &entities.Response{}, err
	}

	return response, nil
}

// ListChannels retrieves a list of channels based on the provided criteria.
func (cuc *ChannelUseCases) ListChannels(ctx context.Context, req *entities.ListChannelRequest) (*entities.Response, error) {
	response, err := cuc.repo.ListChannels(ctx, req)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// ListUserOwnedChannels retrieves a list of channels associated with a specific user based on the provided criteria.
func (cuc *ChannelUseCases) ListUserOwnedChannels(ctx context.Context, r *http.Request, chain string, address string, withLogo bool) (*entities.Response, error) {
	response, err := cuc.repo.ListUserOwnedChannels(ctx, r, chain, address, withLogo)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// DeleteChannel deletes a channel based on the provided chain, appID, and address.
func (cuc *ChannelUseCases) DeleteChannel(ctx context.Context, chain string, appID string, address string) error {
	// Call the repository function to delete the channel
	err := cuc.repo.DeleteChannel(ctx, chain, appID, address)
	if err != nil {
		return err
	}

	return nil
}

func (cuc *ChannelUseCases) ChannelReadSentStatistics(ctx context.Context, r *http.Request, chain, channel, fetchKind, startDate, endDate string) ([]entities.ChannelReadSentResponse, error) {
	sender := cast.ToString(ctx.Value(consts.UserAddress))

	userModel, err := db.GetUserModel(ctx, chain, sender)
	if err != nil {
		return nil, fmt.Errorf("getting user model failed")
	}

	membership := consts.MembershipStringToEnum(userModel.Membership)
	allowed := consts.Analytics[consts.CHANNEL_READ_SENT_STATS][membership]

	if !allowed {
		return nil, fmt.Errorf("your membership doesn't allow analytics for channel read/sent metrics")
	}

	owned := false
	for _, userChannel := range userModel.Channels {
		if userChannel == channel {
			owned = true
			break
		}
	}

	if !owned {
		return nil, fmt.Errorf("channel %s is not owned by user %s", channel, sender)
	}

	data, err := cuc.repo.ChannelReadSentStatistics(ctx, r, chain, channel, fetchKind, startDate, endDate)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ChannelStatistics retrieves the channel activity statistics based on the provided chain, typeStr, startDate, and endDate.
func (cuc *ChannelUseCases) ChannelStatistics(ctx context.Context, r *http.Request, chain, typeStr, startDate, endDate string) ([]entities.ChannelActivity, error) {
	data, err := cuc.repo.ChannelStatistics(ctx, r, chain, typeStr, startDate, endDate)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (cuc *ChannelUseCases) VerifyChannel(ctx context.Context, chain, appID string) error {
	return cuc.repo.VerifyChannel(ctx, chain, appID)
}
