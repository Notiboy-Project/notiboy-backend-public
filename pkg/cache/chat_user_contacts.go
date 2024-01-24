package cache

import (
	"errors"
	"fmt"
	"sync"

	"github.com/gocql/gocql"
	log "github.com/sirupsen/logrus"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/repo/driver/db"
)

type UserContactsCacheModel struct {
	sync.RWMutex
	cache map[string]map[string]map[string]bool
}

var UserContactsCache *UserContactsCacheModel

func InitUserContactsCache() *UserContactsCacheModel {
	UserContactsCache = new(UserContactsCacheModel)
	UserContactsCache.cache = make(map[string]map[string]map[string]bool)

	query := fmt.Sprintf(
		`SELECT chain, user, contacts FROM %s.%s`,
		config.GetConfig().DB.Keyspace, consts.ChatUserContactsTable,
	)

	var (
		chain, user string
		contacts    []string
	)

	iter := db.GetCassandraSession().Query(query).Iter()
	for iter.Scan(&chain, &user, &contacts) {
		for _, contact := range contacts {
			UserContactsCache.AddUserContactsCache(chain, user, contact)
		}
	}

	if err := iter.Close(); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to retrieve user contacts in init cache")
		}
	}

	log.Info("Successfully loaded users contacts cache")
	return UserContactsCache
}

func (c *UserContactsCacheModel) AddUserContactsCache(chain, user, contact string) {
	c.Lock()
	defer c.Unlock()

	log.Debugf("Adding user %s as contact to user %s of chain %s", contact, user, chain)

	if _, ok := c.cache[chain]; !ok {
		c.cache[chain] = make(map[string]map[string]bool)
	}

	if _, ok := c.cache[chain][user]; !ok {
		c.cache[chain][user] = make(map[string]bool)
	}

	c.cache[chain][user][contact] = true
}

func (c *UserContactsCacheModel) RemoveUserContactsCache(chain, user, contact string) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.cache[chain]; !ok {
		return
	}

	if _, ok := c.cache[chain][user]; !ok {
		return
	}

	delete(c.cache[chain][user], contact)
}

func (c *UserContactsCacheModel) IsUserInContactsCache(chain, user, contact string) bool {
	c.RLock()
	defer c.RUnlock()

	if _, ok := c.cache[chain]; !ok {
		return false
	}

	if _, ok := c.cache[chain][user]; !ok {
		return false
	}

	return c.cache[chain][user][contact]
}

func (c *UserContactsCacheModel) GetUserContactsFromCache(chain, user string) []string {
	c.RLock()
	defer c.RUnlock()

	var contacts []string

	if _, ok := c.cache[chain]; !ok {
		return contacts
	}

	if _, ok := c.cache[chain][user]; !ok {
		return contacts
	}

	for contact := range c.cache[chain][user] {
		contacts = append(contacts, contact)
	}

	return contacts
}
