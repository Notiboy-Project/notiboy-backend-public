package db

import (
	"fmt"
	"time"

	"github.com/gocql/gocql"

	"notiboy/config"
)

var session *gocql.Session

func NewCassandraSession(cfg config.DB) (*gocql.Session, error) {
	// Define the cluster configuration
	clusterConfig := gocql.NewCluster(cfg.Host)
	clusterConfig.Authenticator = gocql.PasswordAuthenticator{
		Username: cfg.Username,
		Password: cfg.Password,
	}
	clusterConfig.Consistency = gocql.Quorum

	var err error
	// Establish a session with the cluster
	session, err = clusterConfig.CreateSession()

	if err != nil {
		return nil, err
	}

	// Create a new keyspace
	err = session.Query(`CREATE KEYSPACE IF NOT EXISTS ` + cfg.Keyspace + ` WITH REPLICATION = {'class' : 'SimpleStrategy', 'replication_factor' : 1}`).Exec()
	if err != nil {
		return nil, err
	}
	// Define cluster keyspace
	clusterConfig.Keyspace = cfg.Keyspace
	clusterConfig.ConnectTimeout = time.Second * 10

	// Create a new session with the new keyspace
	session, err = clusterConfig.CreateSession()
	if err != nil {
		return nil, err
	}

	if err = createTables(session, cfg.Keyspace); err != nil {
		return nil, err
	}

	return session, nil
}

func createTables(session *gocql.Session, keyspace string) error {
	for _, table := range dbTableSchemas {
		createTableCmd := fmt.Sprintf(table, keyspace)
		if err := session.Query(createTableCmd).Exec(); err != nil {
			return fmt.Errorf("failed to exec query for db table creation, CMD: %s: %w", createTableCmd, err)
		}
	}

	return nil
}

func GetCassandraSession() *gocql.Session {
	return session
}
