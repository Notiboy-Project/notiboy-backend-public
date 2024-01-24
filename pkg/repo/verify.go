package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/pkg/repo/driver/medium"
	"notiboy/utilities"

	"github.com/gocql/gocql"
)

type VerifyRepo struct {
	db   *gocql.Session
	conf *config.NotiboyConfModel
}

// VerifyRepoImply represents the interface for the repository that handles verification-related operations.
type VerifyRepoImply interface {
	Callback(context.Context, entities.UserIdentifier, string, string) (string, error)
	Verify(context.Context, entities.UserIdentifier, entities.VerifyMedium, string, string) error
	VerifySent(context.Context, entities.UserIdentifier, string, string, string) error
	CallbackDiscord(context.Context, string, string, string) error
}

// NewVerifyRepo
func NewVerifyRepo(db *gocql.Session, conf *config.NotiboyConfModel) VerifyRepoImply {
	return &VerifyRepo{db: db, conf: conf}
}

// Verify verifies the specified medium address for a user.
func (verify *VerifyRepo) Verify(
	_ context.Context, user entities.UserIdentifier,
	mediumAddress entities.VerifyMedium, medium, token string,
) error {
	log := utilities.NewLogger("Verify")

	query := fmt.Sprintf(
		`SELECT medium_metadata
		FROM %s.%s
		WHERE address = ?
			AND chain = ?`,
		verify.conf.DB.Keyspace, consts.UserInfo,
	)

	var mediumMetadataStr string
	if err := verify.db.Query(query, user.Address, user.Chain).Scan(&mediumMetadataStr); err != nil {
		if err.Error() == "not found" {
		} else {
			log.WithError(err).Error("Failed to check medium address")
			return fmt.Errorf("failed to check medium address: %w", err)
		}
	}

	mediumMetadata := new(entities.MediumMetadata)
	if mediumMetadataStr != "" {
		err := mediumMetadata.Unmarshal(mediumMetadataStr)
		if err != nil {
			return err
		}
	} else {
		mediumMetadata = &entities.MediumMetadata{
			Email: &entities.EmailMedium{},
		}
	}

	if mediumMetadata.Email != nil && mediumMetadata.Email.ID == mediumAddress.MediumAddress && mediumMetadata.Email.Verified {
		return errors.New("medium already verified")
	}

	query = fmt.Sprintf(
		"SELECT address FROM %s.%s WHERE address = ?",
		verify.conf.DB.Keyspace, consts.UserInfo,
	)

	var userAddress string
	if err := verify.db.Query(query, user.Address).Scan(&userAddress); err != nil {
		log.WithError(err).Error("Failed to check user address")
		return fmt.Errorf("failed to check user address: %w", err)
	}

	if userAddress == "" {
		log.Warn("User address not found")
		return errors.New("address not found")
	}

	// Insert verify info
	query = fmt.Sprintf(
		`INSERT INTO %s.%s ("token",address,chain,metadata,medium,verified,deleted,sent)
		VALUES (?,?,?,?,?,?,?,?) USING TTL %d`,
		verify.conf.DB.Keyspace, consts.VerifyInfo, verify.conf.TTL.VerifyToken,
	)
	if err := verify.db.Query(
		query,
		token, user.Address, user.Chain,
		mediumAddress.MediumAddress, medium,
		false, false, false,
	).Exec(); err != nil {
		log.WithError(err).Error("Failed to insert verification information")
		return err
	}

	if mediumMetadata.Email == nil {
		mediumMetadata.Email = new(entities.EmailMedium)
	}

	mediumMetadata.Email.ID = mediumAddress.MediumAddress

	mediumMetadataStr, err := mediumMetadata.Marshal()
	if err != nil {
		return err
	}

	query = fmt.Sprintf(
		`UPDATE %s.%s SET medium_metadata = ?
	WHERE address = ? AND chain = ? IF EXISTS`,
		verify.conf.DB.Keyspace, consts.UserInfo,
	)

	if err := verify.db.Query(query, mediumMetadataStr, user.Address, user.Chain).Exec(); err != nil {
		return fmt.Errorf("failed to update medium_metadata for user: %w", err)
	}

	return nil
}

// VerifySent updates the verification information for a sent verification token.
func (verify *VerifyRepo) VerifySent(_ context.Context, user entities.UserIdentifier, token, _, _ string) error {

	log := utilities.NewLogger("VerifySent")

	ttl := verify.conf.TTL.VerifyToken
	expiryTime := time.Now().Add(time.Duration(ttl) * time.Second).UTC()

	query := fmt.Sprintf(
		`UPDATE %s.%s USING TTL %d SET sent = ?, expiry = ?
	WHERE address = ? AND "token" = ? AND chain = ?`,
		verify.conf.DB.Keyspace, consts.VerifyInfo, ttl,
	)

	if err := verify.db.Query(
		query, true, expiryTime.String(),
		user.Address, token, user.Chain,
	).Exec(); err != nil {
		log.WithError(err).Error("Failed to update verification status as sent")
		return err
	}

	return nil

}

// Callback updates the verification status and medium address for a callback request.
func (verify *VerifyRepo) Callback(_ context.Context, user entities.UserIdentifier, token, medium string) (
	string, error,
) {

	var mediumAddress string
	log := utilities.NewLogger("Callback")

	query := fmt.Sprintf(
		`SELECT expiry,deleted,sent FROM %s.%s where "token"=? and address=? AND chain=?`,
		verify.conf.DB.Keyspace, consts.VerifyInfo,
	)

	var expiry string
	var deleted, sent bool
	if err := verify.db.Query(query, token, user.Address, user.Chain).Scan(&expiry, &deleted, &sent); err != nil {
		log.WithError(err).Error("Failed to check token in the address")
		return mediumAddress, fmt.Errorf("failed to check token in the address: %w", err)
	}

	timeNow := utilities.TimeNow()
	if utilities.TimeStringToTime(expiry).Before(timeNow) || deleted || !sent {
		log.Errorf(
			"failed to verify token - expiry: %s, now: %s, deleted: %v, sent: %v", expiry, timeNow, deleted, sent,
		)
		return mediumAddress, errors.New("failed to verify token")
	}

	query = fmt.Sprintf(
		`UPDATE %s.%s USING TTL %d SET verified = ?, deleted = ?
		 WHERE address = ? AND "token" = ? AND chain = ? IF EXISTS`,
		verify.conf.DB.Keyspace, consts.VerifyInfo, verify.conf.TTL.VerifyToken,
	)

	if err := verify.db.Query(query, true, true, user.Address, token, user.Chain).Exec(); err != nil {
		log.WithError(err).Error("Failed to update verification status and deletion")
		return mediumAddress, fmt.Errorf("failed to update callback: %w", err)
	}
	var verified bool

	query = fmt.Sprintf(
		`SELECT metadata,verified
		FROM %s.%s
		WHERE address = ? AND "token" = ? AND chain = ?`,
		verify.conf.DB.Keyspace, consts.VerifyInfo,
	)
	if err := verify.db.Query(query, user.Address, token, user.Chain).Scan(&mediumAddress, &verified); err != nil {

		if err.Error() == "not found" {
			mediumAddress = ""
		} else {
			log.WithError(err).Error("Failed to check medium address")
			return mediumAddress, fmt.Errorf("failed to check medium address: %w", err)
		}
	}
	if !verified {
		log.Warn("Medium address verification failed: Medium address not verified")
		return mediumAddress, errors.New("failed to verify medium address")
	}

	query = fmt.Sprintf(
		`SELECT medium_metadata
		FROM %s.%s
		WHERE address = ?
			AND chain = ?`,
		verify.conf.DB.Keyspace, consts.UserInfo,
	)

	var mediumMetadataStr string
	if err := verify.db.Query(query, user.Address, user.Chain).Scan(&mediumMetadataStr); err != nil {
		if err.Error() == "not found" {
		} else {
			return "", fmt.Errorf("failed to check medium metadata: %w", err)
		}
	}

	mediumMetadata := new(entities.MediumMetadata)
	err := mediumMetadata.Unmarshal(mediumMetadataStr)
	if err != nil {
		return "", err
	}

	mediumMetadata.Email.Verified = true

	mediumMetadataStr, err = mediumMetadata.Marshal()
	if err != nil {
		return "", err
	}
	query = fmt.Sprintf(
		`UPDATE %s.%s SET medium_metadata = ?, supported_mediums = supported_mediums + {'%s'}, allowed_mediums = allowed_mediums + {'%s'}
	WHERE address = ? AND chain = ? IF EXISTS`,
		verify.conf.DB.Keyspace, consts.UserInfo, medium, medium,
	)

	if err = verify.db.Query(query, mediumMetadataStr, user.Address, user.Chain).Exec(); err != nil {
		return mediumAddress, fmt.Errorf("failed to update medium_metadata for user in callback: %w", err)
	}

	return mediumAddress, nil

}

func (verify *VerifyRepo) CallbackDiscord(ctx context.Context, address, discordId, chain string) error {

	log := utilities.NewLogger("CallbackDiscord")
	query := fmt.Sprintf(
		`SELECT medium_metadata
		FROM %s.%s
		WHERE address = ?
			AND chain = ?`,
		verify.conf.DB.Keyspace, consts.UserInfo,
	)

	var mediumMetadataStr string
	if err := verify.db.Query(query, address, chain).Scan(&mediumMetadataStr); err != nil {
		if err.Error() == "not found" {
		} else {
			return fmt.Errorf("failed to get medium metadata: %w", err)
		}
	}

	mediumMetadata := new(entities.MediumMetadata)
	if mediumMetadataStr != "" {
		if err := mediumMetadata.Unmarshal(mediumMetadataStr); err != nil {
			log.WithError(err).Errorf("failed to unamrshal mediumMetadataStr %s", mediumMetadataStr)
			return err
		}
	}

	dmChannelID, err := medium.GetChannelID(discordId)
	if err != nil {
		return fmt.Errorf("failed to get DM channel ID: %w", err)
	}

	mediumMetadata.Discord = &entities.DiscordMedium{
		ID:          discordId,
		Verified:    true,
		DMChannelID: dmChannelID,
	}

	mediumMetadataStr, err = mediumMetadata.Marshal()
	if err != nil {
		return err
	}
	query = fmt.Sprintf(
		`UPDATE %s.%s SET medium_metadata = ?, supported_mediums = supported_mediums + {'%s'}, allowed_mediums = allowed_mediums + {'%s'} WHERE address = ? AND chain = ? IF EXISTS`,
		verify.conf.DB.Keyspace, consts.UserInfo, consts.Discord, consts.Discord,
	)

	q := verify.db.Query(query, mediumMetadataStr, address, chain).WithContext(ctx)

	if err := q.Exec(); err != nil {
		log.WithError(err).Error("Failed to update user medium metadata")
		return fmt.Errorf("failed to update user medium metadata: %w", err)
	}

	return nil
}
