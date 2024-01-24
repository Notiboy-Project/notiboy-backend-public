package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gocql/gocql"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/utilities"
)

func IsTokenPresent(ctx context.Context, address, chain, token, kind, uuid string) (bool, error) {
	log := utilities.NewLoggerWithFields("IsTokenPresent", map[string]interface{}{
		"chain":   chain,
		"address": address,
		"uuid":    uuid,
		"kind":    kind,
	})
	var tokenFromDB string

	if uuid != "" {
		patTable := fmt.Sprintf("%s.%s", config.GetConfig().DB.Keyspace, consts.PATInfo)

		queryProfile := fmt.Sprintf("SELECT jwt FROM %s WHERE address = ? AND chain = ? AND kind = ? AND uuid = ?", patTable)
		if err := GetCassandraSession().Query(queryProfile, address, chain, kind, uuid).Exec(); err != nil {
			if errors.Is(err, gocql.ErrNotFound) {
				log.WithError(err).Errorf("token doesn't exist in DB for user %s", address)
				return false, nil
			} else {
				log.WithError(err).Error("failed to revoke pa token")
				return false, fmt.Errorf("failed to revoke pa token: %w", err)
			}
		}

		return true, nil
	}

	loginInfoTable := fmt.Sprintf("%s.%s", config.GetConfig().DB.Keyspace, consts.LoginTable)

	query := fmt.Sprintf("SELECT jwt FROM %s WHERE address = ? AND chain = ? AND jwt = ? LIMIT 1", loginInfoTable)
	if err := GetCassandraSession().Query(query, address, chain, token).Scan(&tokenFromDB); err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Errorf("token doesn't exist in DB for user %s", address)
			return false, nil
		} else {
			log.WithError(err).Errorf("error talking to DB for user %s", address)
			return false, err
		}
	}

	return true, nil
}

func GetUserStatus(ctx context.Context, chain, address string) (string, error) {
	var userStat string
	query := fmt.Sprintf("SELECT status FROM %s.%s WHERE chain = ? AND address = ?",
		config.GetConfig().DB.Keyspace, consts.UserTable)
	if err := GetCassandraSession().Query(query, chain, address).Scan(&userStat); err != nil {
		return userStat, err
	}

	return userStat, nil
}

func GetUserModel(ctx context.Context, chain, address string) (*entities.UserModel, error) {
	var allowedMediums, supportedMediums, channels, optins []string
	var status, membership string
	var mediumMetadataStr, logo string

	keyspace := config.GetConfig().DB.Keyspace
	tblUserInfo := fmt.Sprintf("%s.%s", keyspace, consts.UserTable)

	infoQuery := "SELECT channels, optins, membership, logo, status, allowed_mediums, supported_mediums, medium_metadata FROM " + tblUserInfo + " WHERE chain = ? AND address = ?"
	if err := GetCassandraSession().Query(infoQuery, chain, address).Scan(&channels, &optins, &membership, &logo, &status, &allowedMediums, &supportedMediums, &mediumMetadataStr); err != nil {
		return nil, fmt.Errorf("failed to query db, query: %s (chain: %s, address: %s): %w", infoQuery, chain, address, err)
	}

	mediumMetadata := new(entities.MediumMetadata)
	if strings.TrimSpace(mediumMetadataStr) != "" {
		if err := mediumMetadata.Unmarshal(mediumMetadataStr); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s: %w", mediumMetadataStr, err)
		}
	}

	userInfo := &entities.UserModel{
		UserIdentifier: entities.UserIdentifier{
			Chain:   chain,
			Address: address,
		},
		SupportedMediums: supportedMediums,
		AllowedMediums:   allowedMediums,
		Membership:       membership,
		Logo:             logo,
		MediumMetadata:   *mediumMetadata,
		Status:           status,
		Channels:         channels,
		Optins:           optins,
	}

	return userInfo, nil
}

func IsUserOnboarded(ctx context.Context, chain, address string) (bool, error) {
	var userStat string
	query := fmt.Sprintf("SELECT status FROM %s.%s WHERE chain = ? AND address = ?",
		config.GetConfig().DB.Keyspace, consts.UserTable)
	if err := GetCassandraSession().Query(query, chain, address).Scan(&userStat); err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return false, nil
		} else {
			return false, err
		}
	}

	return true, nil
}
