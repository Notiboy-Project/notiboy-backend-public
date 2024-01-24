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

type UserGroupsCacheModel struct {
	sync.RWMutex
	cache map[string]map[string]map[string]bool
}

var UserGroupsCache *UserGroupsCacheModel

func InitUserGroupsCache() *UserGroupsCacheModel {
	UserGroupsCache = new(UserGroupsCacheModel)
	UserGroupsCache.cache = make(map[string]map[string]map[string]bool)

	query := fmt.Sprintf(
		`SELECT chain, user, gids FROM %s.%s`,
		config.GetConfig().DB.Keyspace, consts.ChatUserGroupTable,
	)

	var (
		chain, user string
		groups      []string
	)

	iter := db.GetCassandraSession().Query(query).Iter()
	for iter.Scan(&chain, &user, &groups) {
		for _, group := range groups {
			UserGroupsCache.AddUserGroupsCache(chain, user, group)
		}
	}

	if err := iter.Close(); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to retrieve user groups in init cache")
		}
	}

	log.Info("Successfully loaded users groups cache")
	return UserGroupsCache
}

func (c *UserGroupsCacheModel) AddUserGroupsCache(chain, user, group string) {
	c.Lock()
	defer c.Unlock()

	log.Debugf("Adding group %s to user %s of chain %s", group, user, chain)

	if _, ok := c.cache[chain]; !ok {
		c.cache[chain] = make(map[string]map[string]bool)
	}

	if _, ok := c.cache[chain][user]; !ok {
		c.cache[chain][user] = make(map[string]bool)
	}

	c.cache[chain][user][group] = true
}

func (c *UserGroupsCacheModel) RemoveUserGroupsCache(chain, user, group string) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.cache[chain]; !ok {
		return
	}

	if _, ok := c.cache[chain][user]; !ok {
		return
	}

	delete(c.cache[chain][user], group)
}

func (c *UserGroupsCacheModel) IsUserInGroupCache(chain, user, group string) bool {
	c.RLock()
	defer c.RUnlock()

	if _, ok := c.cache[chain]; !ok {
		return false
	}

	if _, ok := c.cache[chain][user]; !ok {
		return false
	}

	return c.cache[chain][user][group]
}

func (c *UserGroupsCacheModel) GetUserGroupsFromCache(chain, user string) []string {
	c.RLock()
	defer c.RUnlock()

	var groups []string

	if _, ok := c.cache[chain]; !ok {
		return groups
	}

	if _, ok := c.cache[chain][user]; !ok {
		return groups
	}

	for group := range c.cache[chain][user] {
		groups = append(groups, group)
	}

	return groups
}
