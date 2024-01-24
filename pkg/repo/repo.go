package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/gocql/gocql"

	"notiboy/config"
	"notiboy/pkg/consts"
)

type Repo struct {
	db   *gocql.Session
	conf *config.NotiboyConfModel
}
type Imply interface {
	DBHealthCheck(context.Context) error
	VerifyUserOnboarded(context.Context, any, string) error
}

// NewRepo
func NewRepo(db *gocql.Session, conf *config.NotiboyConfModel) Imply {
	return &Repo{db: db, conf: conf}
}

// HealthHandler
func (repo *Repo) DBHealthCheck(_ context.Context) error {
	if err := repo.db.Query("SELECT now() FROM system.local").Exec(); err != nil {
		return err
	}
	return nil
}

func (repo *Repo) VerifyUserOnboarded(_ context.Context, address any, chain string) error {
	// Get the user of the channel
	var userStat string
	query := fmt.Sprintf(
		"SELECT status FROM %s.%s WHERE address = ? AND chain = ?",
		config.GetConfig().DB.Keyspace, consts.UserInfo,
	)
	if err := repo.db.Query(query, address, chain).Scan(&userStat); err != nil {
		return err
	}

	// Check if the user status is active
	if userStat != "ACTIVE" {
		return errors.New("user status not active")
	}

	return nil
}
