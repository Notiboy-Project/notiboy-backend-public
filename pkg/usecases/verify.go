package usecases

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"notiboy/config"
	"notiboy/pkg/entities"
	"notiboy/pkg/repo"
	mediumLib "notiboy/pkg/repo/driver/medium"
	"notiboy/ui/templates"
	"notiboy/utilities"
)

type VerifyUseCases struct {
	repo repo.VerifyRepoImply
}

type VerifyUseCaseImply interface {
	Verify(context.Context, string, entities.UserIdentifier, entities.VerifyMedium) error
	Callback(context.Context, entities.UserIdentifier, string, string) error
}

// NewVerifyUseCases
func NewVerifyUseCases(VerifyRepo repo.VerifyRepoImply) VerifyUseCaseImply {
	return &VerifyUseCases{
		repo: VerifyRepo,
	}
}

// Verify performs user verification for the specified medium and user.
func (verify *VerifyUseCases) Verify(ctx context.Context, medium string, user entities.UserIdentifier, mediumAddress entities.VerifyMedium) error {

	log := utilities.NewLogger("Verify")

	if medium != "email" {
		return errors.New("please enter a valid medium")
	}

	var renderData entities.TplRenderData
	token, err := utilities.GenerateRandomToken()
	if err != nil {
		log.Errorf("Error while generating random token %s", err)
		return err
	}

	if token == "" {
		log.Error("Failed to generate token")
		return errors.New("failed to generate token")
	}

	token = utilities.Encrypt(token)

	if token == "" {
		log.Error("failed to encrypt token")
		return errors.New("failed to encrypt token")
	}

	if err = verify.repo.Verify(ctx, user, mediumAddress, medium, token); err != nil {
		log.WithError(err).Error("verification failed")
		return err
	}

	link, err := url.JoinPath(config.ServerBaseURL, fmt.Sprintf("chains/%s/user/%s/verification/%s/mediums/%s", user.Chain, user.Address, token, medium))
	if err != nil {
		log.WithError(err).Error("url joining failed")
		return err
	}

	renderData.Message = "Click the link to verify your email with us"
	renderData.ButtonDescription = "VERIFY"
	renderData.CallbackUrl = link
	body, err := utilities.TemplateRendering(templates.VerificationTemplate, renderData)
	if err != nil {
		log.WithError(err).Error("failed to render email template")
		return err
	}

	emailVerificationSubject := "Email Verification"

	err = mediumLib.GetEmailClient().SendMail(
		ctx,
		config.GetConfig().Email.Verify.From,
		mediumAddress.MediumAddress,
		emailVerificationSubject,
		body.String())
	if err != nil {
		log.Error("Failed to send email:", err)
		return err
	}

	return verify.repo.VerifySent(ctx, user, token, medium, mediumAddress.MediumAddress)
}

// Callback performs the callback action after the user completes verification.
func (verify *VerifyUseCases) Callback(ctx context.Context, user entities.UserIdentifier, token, medium string) error {
	var discordToken, userId string
	log := utilities.NewLogger("Callback")

	var err error
	if medium == "discord" {
		chainAddress := strings.SplitN(user.Address, ",", 2)
		if len(chainAddress) != 2 {
			log.Errorf("malformed state received from dicord %s", chainAddress)
			return fmt.Errorf("malformed state received from dicord %s", chainAddress)
		}

		if discordToken, err = mediumLib.GetToken(token); err != nil {
			log.Error("Failed to get Discord token:", err)
			return fmt.Errorf("failed to get Discord token: %w", err)
		}

		if userId, err = mediumLib.GetCurrentUserID(discordToken); err != nil {
			log.Error("Failed to get Discord user id", err)
			return fmt.Errorf("failed to get Discord user ID: %w", err)
		}

		if err := mediumLib.AddServerMember(discordToken, config.GetConfig().Discord.BotServerID, userId); err != nil {
			log.Error("Failed to add Discord guild member:", err)
			return fmt.Errorf("failed to add Discord guild member: %w", err)
		}
		return verify.repo.CallbackDiscord(ctx, chainAddress[1], userId, chainAddress[0])
	}

	if medium == "email" {
		mediumAddress, err := verify.repo.Callback(ctx, user, token, medium)
		if err != nil {
			log.Error("failed to perform callback:", err)
			return err
		}

		var renderData entities.TplRenderData

		emailConfirmationSubject := "Email Confirmation"

		renderData.ButtonDescription = "VIEW MORE"
		renderData.CallbackUrl, err = url.JoinPath(config.GetConfig().Server.RedirectPrefix, "/settings")
		if err != nil {
			log.WithError(err).Error("failed to join path")
			return err
		}
		renderData.Message = "<b> Welcome to Notiboy <b><br> Your Email Address is verified with us"
		body, err := utilities.TemplateRendering(templates.VerificationTemplate, renderData)
		if err != nil {
			log.WithError(err).Error("failed to render email template")
			return err
		}

		return mediumLib.GetEmailClient().SendMail(
			ctx,
			config.GetConfig().Email.Verify.From,
			mediumAddress,
			emailConfirmationSubject,
			body.String())
	}

	return errors.New("please enter a valid medium")
}
