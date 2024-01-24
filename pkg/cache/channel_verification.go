package cache

import (
	"fmt"
	"sync"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/repo/driver/db"
	"notiboy/utilities"
)

var channelVerifyCacheObject *ChannelVerifyCache

// chain: app_id: struct{}{}
type ChannelVerifyCache struct {
	unverifiedChainAppIDs map[string]map[string]struct{}
	verifiedChainAppIDs   map[string]map[string]struct{}
	sync.RWMutex
}

func GetChannelVerifyCache() *ChannelVerifyCache {
	log := utilities.NewLogger("GetChannelVerifyCache")

	if channelVerifyCacheObject != nil {
		return channelVerifyCacheObject
	}

	channelVerifyCacheObject = new(ChannelVerifyCache)
	channelVerifyCacheObject.unverifiedChainAppIDs = make(map[string]map[string]struct{})
	channelVerifyCacheObject.verifiedChainAppIDs = make(map[string]map[string]struct{})
	channelVerifyCacheObject.RWMutex = sync.RWMutex{}

	if err := channelVerifyCacheObject.init(); err != nil {
		log.WithError(err).Fatal("failed to init cache")
	} else {
		log.Info("Loaded cache")
	}

	return channelVerifyCacheObject
}

func (c *ChannelVerifyCache) IsVerified(chain, appID string) bool {
	c.RLock()
	defer c.RUnlock()

	if _, ok := c.verifiedChainAppIDs[chain]; !ok {
		return false
	}

	_, ok := c.verifiedChainAppIDs[chain][appID]
	if ok {
		return true
	}

	return false
}

func (c *ChannelVerifyCache) AddVerified(chain, appID string) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.verifiedChainAppIDs[chain]; !ok {
		c.verifiedChainAppIDs[chain] = make(map[string]struct{})
	}

	c.verifiedChainAppIDs[chain][appID] = struct{}{}
}

func (c *ChannelVerifyCache) AddUnverified(chain, appID string) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.unverifiedChainAppIDs[chain]; !ok {
		c.unverifiedChainAppIDs[chain] = make(map[string]struct{})
	}

	c.unverifiedChainAppIDs[chain][appID] = struct{}{}
}

func (c *ChannelVerifyCache) PopVerified(chain, appID string) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.verifiedChainAppIDs[chain]; !ok {
		return
	}

	delete(c.verifiedChainAppIDs[chain], appID)
}

func (c *ChannelVerifyCache) PopUnverified(chain, appID string) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.unverifiedChainAppIDs[chain]; !ok {
		return
	}

	delete(c.unverifiedChainAppIDs[chain], appID)
}

func (c *ChannelVerifyCache) init() error {
	cass := db.GetCassandraSession()

	chains := config.GetConfig().Chain.Supported

	var (
		appID string
	)
	for _, chain := range chains {
		if _, ok := c.unverifiedChainAppIDs[chain]; !ok {
			c.unverifiedChainAppIDs[chain] = make(map[string]struct{})
		}
		if _, ok := c.verifiedChainAppIDs[chain]; !ok {
			c.verifiedChainAppIDs[chain] = make(map[string]struct{})
		}

		query := fmt.Sprintf(
			"SELECT app_id FROM %s.%s WHERE chain = ?",
			config.GetConfig().DB.Keyspace, consts.UnverifiedChannelInfo,
		)
		iter := cass.Query(query, chain).Iter()

		for iter.Scan(&appID) {
			c.unverifiedChainAppIDs[chain][appID] = struct{}{}
		}

		if err := iter.Close(); err != nil {
			return err
		}

		query = fmt.Sprintf(
			"SELECT app_id FROM %s.%s WHERE chain = ?",
			config.GetConfig().DB.Keyspace, consts.VerifiedChannelInfo,
		)
		iter = cass.Query(query, chain).Iter()

		for iter.Scan(&appID) {
			c.verifiedChainAppIDs[chain][appID] = struct{}{}
		}

		if err := iter.Close(); err != nil {
			return err
		}
	}

	return nil
}
