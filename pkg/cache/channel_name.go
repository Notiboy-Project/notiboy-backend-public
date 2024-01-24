package cache

import (
	"fmt"
	"strings"
	"sync"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/repo/driver/db"
	"notiboy/utilities"
)

var channelNameCacheObject *ChannelNameCache

type ChannelNameCache struct {
	chainChannelAppIDs map[string]map[string][]string
	sync.RWMutex
}

func GetChannelNameCache() *ChannelNameCache {
	log := utilities.NewLogger("GetChannelNameCache")

	if channelNameCacheObject != nil {
		return channelNameCacheObject
	}

	channelNameCacheObject = new(ChannelNameCache)
	channelNameCacheObject.chainChannelAppIDs = make(map[string]map[string][]string)
	channelNameCacheObject.RWMutex = sync.RWMutex{}

	if err := channelNameCacheObject.init(); err != nil {
		log.WithError(err).Fatal("failed to init cache")
	} else {
		log.Info("Loaded cache")
	}

	return channelNameCacheObject
}

func (c *ChannelNameCache) GetAppIDs(chain, channelName string) []string {
	c.RLock()
	defer c.RUnlock()

	appIDs := make([]string, 0)

	channelName = strings.ToLower(channelName)

	if _, ok := c.chainChannelAppIDs[chain]; !ok {
		return []string{}
	}

	for nameFromC, appIDsFromC := range c.chainChannelAppIDs[chain] {
		if strings.Contains(nameFromC, channelName) {
			appIDs = append(appIDs, appIDsFromC...)
		}
	}

	return appIDs
}

func (c *ChannelNameCache) Add(chain, channelName, appID string) {
	c.Lock()
	defer c.Unlock()
	channelName = strings.ToLower(channelName)

	if _, ok := c.chainChannelAppIDs[chain]; !ok {
		c.chainChannelAppIDs[chain] = make(map[string][]string)
	}

	if _, ok := c.chainChannelAppIDs[chain][channelName]; !ok {
		c.chainChannelAppIDs[chain][channelName] = make([]string, 0)
	}

	c.chainChannelAppIDs[chain][channelName] = append(c.chainChannelAppIDs[chain][channelName], appID)
}

func (c *ChannelNameCache) Pop(chain, channelName, appID string) {
	c.Lock()
	defer c.Unlock()
	channelName = strings.ToLower(channelName)

	if _, ok := c.chainChannelAppIDs[chain]; !ok {
		return
	}

	if _, ok := c.chainChannelAppIDs[chain][channelName]; !ok {
		return
	}

	c.chainChannelAppIDs[chain][channelName] = append(c.chainChannelAppIDs[chain][channelName], appID)
	appIDs := make([]string, 0)
	for _, appIDFromCache := range c.chainChannelAppIDs[chain][channelName] {
		if appIDFromCache == appID {
			continue
		}
		appIDs = append(appIDs, appIDFromCache)
	}

	c.chainChannelAppIDs[chain][channelName] = appIDs
	// if no app IDs, delete channel name from cache since the last channel with the said name was deleted
	if len(appIDs) == 0 {
		delete(c.chainChannelAppIDs[chain], channelName)
	}
}

func (c *ChannelNameCache) init() error {
	cass := db.GetCassandraSession()

	chains := config.GetConfig().Chain.Supported

	var (
		channelName string
		appIDs      []string
	)
	for _, chain := range chains {
		if _, ok := c.chainChannelAppIDs[chain]; !ok {
			c.chainChannelAppIDs[chain] = make(map[string][]string)
		}

		query := fmt.Sprintf(
			"SELECT name, app_id FROM %s.%s WHERE chain = ?",
			config.GetConfig().DB.Keyspace, consts.ChannelName,
		)
		iter := cass.Query(query, chain).Iter()

		for iter.Scan(&channelName, &appIDs) {
			c.chainChannelAppIDs[chain][channelName] = appIDs
		}

		if err := iter.Close(); err != nil {
			return err
		}
	}

	return nil
}
