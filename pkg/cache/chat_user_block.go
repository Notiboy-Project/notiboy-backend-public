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

type BlockedUserCacheModel struct {
	sync.RWMutex
	cache map[string]map[string]map[string]bool
}

var BlockedUserCache *BlockedUserCacheModel

func InitBlockedUserCache() *BlockedUserCacheModel {
	BlockedUserCache = new(BlockedUserCacheModel)
	BlockedUserCache.cache = make(map[string]map[string]map[string]bool)

	query := fmt.Sprintf(
		`SELECT chain, user, blocked_users FROM %s.%s`,
		config.GetConfig().DB.Keyspace, consts.ChatUserBlockTable,
	)

	var (
		chain, user  string
		blockedUsers []string
	)

	iter := db.GetCassandraSession().Query(query).Iter()
	for iter.Scan(&chain, &user, &blockedUsers) {
		for _, blockedUser := range blockedUsers {
			BlockedUserCache.AddBlockedUserCache(chain, user, blockedUser)
		}
	}

	if err := iter.Close(); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("failed to retrieve blocked users in init cache")
		}
	}

	log.Info("Successfully loaded blocked users cache")
	return BlockedUserCache
}

func (c *BlockedUserCacheModel) AddBlockedUserCache(chain, user, blockedUser string) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.cache[chain]; !ok {
		c.cache[chain] = make(map[string]map[string]bool)
	}

	if _, ok := c.cache[chain][user]; !ok {
		c.cache[chain][user] = make(map[string]bool)
	}

	c.cache[chain][user][blockedUser] = true
}

func (c *BlockedUserCacheModel) RemoveBlockedUserCache(chain, user, blockedUser string) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.cache[chain]; !ok {
		return
	}

	if _, ok := c.cache[chain][user]; !ok {
		return
	}

	delete(c.cache[chain][user], blockedUser)
}

func (c *BlockedUserCacheModel) IsBlockedUserInCache(chain, user, blockedUser string) bool {
	c.RLock()
	defer c.RUnlock()

	if _, ok := c.cache[chain]; !ok {
		return false
	}

	if _, ok := c.cache[chain][user]; !ok {
		return false
	}

	return c.cache[chain][user][blockedUser]
}
