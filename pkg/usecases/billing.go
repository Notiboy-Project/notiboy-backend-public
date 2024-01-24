package usecases

import (
	"context"

	"notiboy/pkg/entities"
	"notiboy/pkg/repo"
)

type BillingUsecases struct {
	repo     repo.BillingRepoImply
	userRepo repo.UserRepoImply
}

type BillingUsecasesImply interface {
	AddFund(context.Context, entities.BillingRequest) error
	AdminChangeMembership(context.Context, entities.BillingRequest) error
	ChangeMembership(context.Context, entities.BillingRequest) error
	GetBillingDetails(context.Context, entities.BillingRequest) (*entities.BillingInfo, error)
	GetMemershipTiers(context.Context) (map[string]map[string]interface{}, error)
}

// NewBillingUsecases creates a new instance of the BillingUsecases struct
func NewBillingUsecases(billingRepo repo.BillingRepoImply, userRepo repo.UserRepoImply) BillingUsecasesImply {
	return &BillingUsecases{
		repo:     billingRepo,
		userRepo: userRepo,
	}
}

func (b *BillingUsecases) AddFund(ctx context.Context, req entities.BillingRequest) error {
	resp, err := b.userRepo.GetUser(ctx, entities.UserIdentifier{
		Chain:   req.Chain,
		Address: req.Address,
	})
	if err != nil {
		return err
	}

	membership := resp.Data.(*entities.UserModel).Membership

	return b.repo.AddFund(ctx, membership, req)
}

func (b *BillingUsecases) AdminChangeMembership(ctx context.Context, req entities.BillingRequest) error {
	resp, err := b.userRepo.GetUser(ctx, entities.UserIdentifier{
		Chain:   req.Chain,
		Address: req.Address,
	})
	if err != nil {
		return err
	}

	membership := resp.Data.(*entities.UserModel).Membership

	return b.repo.ChangeMembership(ctx, membership, req, true)
}

func (b *BillingUsecases) ChangeMembership(ctx context.Context, req entities.BillingRequest) error {
	resp, err := b.userRepo.GetUser(ctx, entities.UserIdentifier{
		Chain:   req.Chain,
		Address: req.Address,
	})
	if err != nil {
		return err
	}

	membership := resp.Data.(*entities.UserModel).Membership

	return b.repo.ChangeMembership(ctx, membership, req, false)
}

func (b *BillingUsecases) GetBillingDetails(ctx context.Context, req entities.BillingRequest) (*entities.BillingInfo, error) {
	resp, err := b.userRepo.GetUser(ctx, entities.UserIdentifier{
		Chain:   req.Chain,
		Address: req.Address,
	})
	if err != nil {
		return nil, err
	}

	membership := resp.Data.(*entities.UserModel).Membership
	channels := resp.Data.(*entities.UserModel).Channels
	req.OwnedChannels = channels

	totalSent, err := b.userRepo.GetUserSendMetricsForMonth(ctx, req.Chain, req.Address)
	if err != nil {
		return nil, err
	}
	req.TotalSent = totalSent

	return b.repo.GetBillingDetails(ctx, membership, req)
}

func (b *BillingUsecases) GetMemershipTiers(ctx context.Context) (map[string]map[string]interface{}, error) {
	return b.repo.GetMembershipTiers(ctx)
}
