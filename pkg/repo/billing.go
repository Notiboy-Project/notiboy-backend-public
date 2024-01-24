package repo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"github.com/spf13/cast"

	"notiboy/config"
	"notiboy/pkg/cache"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	chainLib "notiboy/pkg/repo/driver/chain"
	"notiboy/pkg/repo/driver/db"
	"notiboy/utilities"
)

type BillingRepo struct {
	db   *gocql.Session
	conf *config.NotiboyConfModel
}

// BillingRepoImply is an interface that defines the contract for billing implementation.
type BillingRepoImply interface {
	AddFund(context.Context, string, entities.BillingRequest) error
	ChangeMembership(context.Context, string, entities.BillingRequest, bool) error
	GetBillingDetails(context.Context, string, entities.BillingRequest) (*entities.BillingInfo, error)
	GetMembershipTiers(context.Context) (map[string]map[string]interface{}, error)
}

func NewBillingRepo(db *gocql.Session, conf *config.NotiboyConfModel) BillingRepoImply {
	return &BillingRepo{db: db, conf: conf}
}

func membershipCheckerStub() {
	log := utilities.NewLogger("MembershipChecker")

	conf := config.GetConfig()
	dbClient := db.GetCassandraSession()

	var (
		expiry     time.Time
		chain      string
		address    string
		membership string
	)

	updateQuery := fmt.Sprintf(
		"UPDATE %s.%s SET membership = ? WHERE address = ? AND chain = ?",
		conf.DB.Keyspace, consts.UserInfo,
	)
	query := fmt.Sprintf(
		`SELECT expiry, membership, chain, address FROM %s.%s`,
		conf.DB.Keyspace, consts.BillingTable,
	)
	iter := dbClient.Query(query).Iter()

	for iter.Scan(&expiry, &membership, &chain, &address) {
		if consts.MembershipStringToEnum(membership) == consts.FreeTier {
			continue
		}

		if time.Now().Before(expiry) {
			continue
		}

		if err := dbClient.Query(updateQuery, consts.FreeTier.String(), address, chain).Exec(); err != nil {
			log.WithError(err).Errorf("failed to update membership in user info for %s:%s", address, chain)
		} else {
			log.Infof("Updated membership in user info for %s:%s", address, chain)
		}
	}

	if err := iter.Close(); err != nil {
		log.WithError(err).Error("failed to close iter")
	}
}

func MembershipChecker(ctx context.Context) {
	log := utilities.NewLogger("MembershipChecker")

	interval := cast.ToDuration(config.GetConfig().MembershipCheckerInterval)
	ticker := time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Info("Terminating...")
				ticker.Stop()
				return
			case <-ticker.C:
				membershipCheckerStub()
			}
		}
	}()
}

func getBalanceDaysAndMoney(
	curMembershipTier consts.MembershipTier, curMembershipCost, newMembershipCost int, curBalance float64,
	prevUpdatedTime time.Time,
) (float64, float64) {
	timeNow := utilities.TimeNow()

	var (
		costSince               float64
		curMembershipCostPerDay float64
		numDays                 float64
	)
	if curMembershipTier != consts.FreeTier {
		hrsSinceLastPaid := timeNow.Sub(prevUpdatedTime).Hours()
		daysSinceLastPaid := hrsSinceLastPaid / float64(24)
		curMembershipCostPerDay = float64(curMembershipCost) / float64(30)
		costSince = curMembershipCostPerDay * daysSinceLastPaid
	}

	availableBalance := curBalance - costSince

	newMembershipCostPerDay := float64(newMembershipCost) / float64(30)
	if newMembershipCostPerDay > 0 {
		numDays = math.Floor(availableBalance / newMembershipCostPerDay)
	}

	return numDays, availableBalance
}

func (b *BillingRepo) ChangeMembership(
	_ context.Context, curMembership string, req entities.BillingRequest, force bool,
) error {
	chain := req.Chain
	address := req.Address
	leaseInDays := req.Days

	log := utilities.NewLoggerWithFields(
		"repo.ChangeMembership", map[string]interface{}{
			"chain":   chain,
			"address": address,
		},
	)

	newMembership := req.Membership

	newMembershipTier := consts.MembershipStringToEnum(newMembership)
	if newMembershipTier == consts.FreeTier {
		return fmt.Errorf("invalid upgrade, cannot downgrade to free tier")
	}

	var (
		prevUpdatedTime   time.Time
		curMembershipCost int
		curBalance        float64
	)

	query := fmt.Sprintf(
		`SELECT updated, balance, charge FROM %s.%s WHERE chain = ? AND address = ?`,
		b.conf.DB.Keyspace, consts.BillingTable,
	)

	if err := b.db.Query(query, chain, address).Scan(&prevUpdatedTime, &curBalance, &curMembershipCost); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("Getting fund details failed")
			return fmt.Errorf("failed to get fund details: %w", err)
		}
	}

	curMembershipTier := consts.MembershipStringToEnum(curMembership)
	if curMembershipTier == newMembershipTier && !force {
		return fmt.Errorf("invalid upgrade, attempt to upgrade to existing tier %s", newMembershipTier)
	}

	newMembershipCost := consts.MembershipCharge[newMembershipTier]
	numDays, availableBalance := getBalanceDaysAndMoney(
		curMembershipTier, curMembershipCost, newMembershipCost, curBalance, prevUpdatedTime,
	)

	log.Infof(
		"Balance: %f, membership: %s, cost: %d, days: %f",
		availableBalance, newMembershipTier, newMembershipCost, numDays,
	)

	if force {
		numDays += float64(leaseInDays)
	}

	if numDays < 1 {
		return fmt.Errorf(
			"insufficient balance %f - cannot upgrade membership to %s", availableBalance, newMembershipTier,
		)
	}

	expiry := utilities.TimeNow().AddDate(0, 0, int(numDays))

	if err := b.enforceMembershipChangeRestrictions(curMembershipTier, newMembershipTier, chain, address); err != nil {
		return fmt.Errorf("failed to enfore membership change restrictions")
	}

	query = fmt.Sprintf(
		`INSERT INTO %s.%s (chain, address, expiry, membership, updated, balance, charge) VALUES %s`,
		b.conf.DB.Keyspace, consts.BillingTable, utilities.DBMultiValuePlaceholders(7),
	)

	if err := b.db.Query(
		query, chain, address, expiry, newMembershipTier.String(), utilities.TimeNow(), availableBalance,
		newMembershipCost,
	).Exec(); err != nil {
		log.WithError(err).Error("billing table update failed")
		return fmt.Errorf("failed to update billing table: %w", err)
	}

	query = fmt.Sprintf(
		"UPDATE %s.%s SET membership = ? WHERE address = ? AND chain = ?",
		b.conf.DB.Keyspace, consts.UserInfo,
	)
	if err := b.db.Query(query, newMembershipTier.String(), address, chain).Exec(); err != nil {
		log.WithError(err).Error("failed to update membership in user info")
		return err
	}

	return nil
}

func (b *BillingRepo) enforceMembershipChangeRestrictions(old, new consts.MembershipTier, chain, address string) error {
	log := utilities.NewLoggerWithFields(
		"enforceMembershipChangeRestrictions", map[string]interface{}{
			"chain":   chain,
			"address": address,
		},
	)

	upgrade := new > old

	oldChannelCount := consts.ChannelCount[old]
	newChannelCount := consts.ChannelCount[new]

	var channels []string

	query := fmt.Sprintf(
		"SELECT channels from %s.%s where address = ? AND chain = ?",
		b.conf.DB.Keyspace, consts.UserInfo,
	)
	if err := b.db.Query(query, address, chain).Scan(&channels); err != nil {
		log.WithError(err).Error("failed to get channels from user info")
		return err
	}

	getAppIDsToProcess := func(chain string, appIDs []string, min, max int) ([]string, error) {
		log.Infof("AppIDsToProcess: %s, min: %d, max: %d", strings.Join(appIDs, ","), min, max)
		sl := make([]entities.ChannelModel, 0)

		for _, tbl := range []string{consts.VerifiedChannelInfo, consts.UnverifiedChannelInfo} {
			query = fmt.Sprintf(
				"SELECT app_id, created FROM %s.%s WHERE chain = ? AND app_id in ?",
				b.conf.DB.Keyspace, tbl,
			)
			iter := b.db.Query(query, chain, appIDs).Iter()

			var (
				appID   string
				created time.Time
			)

			for iter.Scan(&appID, &created) {
				sl = append(
					sl, entities.ChannelModel{
						AppID:            appID,
						CreatedTimestamp: created,
					},
				)
			}

			if err := iter.Close(); err != nil {
				return nil, err
			}
		}

		//sl now has both verified and unverified channels
		sort.Slice(
			sl, func(i, j int) bool {
				return sl[i].CreatedTimestamp.Before(sl[j].CreatedTimestamp)
			},
		)

		appIDsToProcess := make([]string, 0)
		for _, channelModel := range sl[min:max] {
			appIDsToProcess = append(appIDsToProcess, channelModel.AppID)
		}

		return appIDsToProcess, nil
	}

	var (
		channelsToActivate   []string
		channelsToDeactivate []string
		err                  error
	)

	if upgrade {
		// set active the channels because last few channels exceeded the limit
		if len(channels) > oldChannelCount {
			if len(channels) > newChannelCount {
				channelsToActivate, err = getAppIDsToProcess(chain, channels, oldChannelCount, newChannelCount)
			} else {
				channelsToActivate, err = getAppIDsToProcess(chain, channels, oldChannelCount, len(channels))
			}
		}
	} else {
		// deactivate the channels because last few channels exceeded the limit
		if len(channels) > newChannelCount {
			channelsToDeactivate, err = getAppIDsToProcess(chain, channels, newChannelCount, len(channels))
		}
	}
	if err != nil {
		log.WithError(err).Error("failed to get app IDs for processing")
		return err
	}

	log.Infof(
		"Channels to activate: %s, deactivate: %s", strings.Join(channelsToActivate, ","),
		strings.Join(channelsToDeactivate, ","),
	)

	fn := func(status, chain, tbl string, appIDs []string) error {
		log.Infof("Table: %s, status: %s, channels: %s", tbl, status, strings.Join(appIDs, ","))

		updateQuery := fmt.Sprintf(
			`UPDATE %s.%s SET status = ? WHERE app_id in ? and chain = ?`,
			config.GetConfig().DB.Keyspace, tbl,
		)

		if err := b.db.Query(updateQuery, status, appIDs, chain).Exec(); err != nil {
			return fmt.Errorf("failed to update channel status: %w", err)
		}

		return nil
	}

	if len(channelsToActivate) > 0 {
		verifiedChannels := make([]string, 0)
		unverifiedChannels := make([]string, 0)

		for _, appID := range channelsToActivate {
			if cache.GetChannelVerifyCache().IsVerified(chain, appID) {
				verifiedChannels = append(verifiedChannels, appID)
			} else {
				unverifiedChannels = append(unverifiedChannels, appID)
			}
		}

		if len(verifiedChannels) > 0 {
			if err = fn(consts.STATUS_ACTIVE, chain, consts.VerifiedChannelInfo, verifiedChannels); err != nil {
				return err
			}
		}

		if len(unverifiedChannels) > 0 {
			if err = fn(consts.STATUS_ACTIVE, chain, consts.UnverifiedChannelInfo, unverifiedChannels); err != nil {
				return err
			}
		}
	}

	if len(channelsToDeactivate) > 0 {
		verifiedChannels := make([]string, 0)
		unverifiedChannels := make([]string, 0)

		for _, appID := range channelsToDeactivate {
			if cache.GetChannelVerifyCache().IsVerified(chain, appID) {
				verifiedChannels = append(verifiedChannels, appID)
			} else {
				unverifiedChannels = append(unverifiedChannels, appID)
			}
		}

		if len(verifiedChannels) > 0 {
			if err = fn(
				consts.STATUS_CHANNEL_LIMIT_EXCEEDED, chain, consts.VerifiedChannelInfo, verifiedChannels,
			); err != nil {
				return err
			}
		}
		if len(unverifiedChannels) > 0 {
			if err = fn(
				consts.STATUS_CHANNEL_LIMIT_EXCEEDED, chain, consts.UnverifiedChannelInfo, unverifiedChannels,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func (b *BillingRepo) AddFund(ctx context.Context, curMembership string, req entities.BillingRequest) error {
	chain := req.Chain
	address := req.Address

	log := utilities.NewLoggerWithFields(
		"repo.AddFund", map[string]interface{}{
			"chain":   chain,
			"address": address,
		},
	)

	txn := req.SignedTxn
	txnID := req.TxnID
	timeNow := utilities.TimeNow()

	usd, txnID, err := chainLib.GetBlockchainClient(chain).VerifyPayment(ctx, address, txn, txnID)
	if err != nil {
		return fmt.Errorf("payment failed: %w", err)
	}

	if usd < 1 {
		return fmt.Errorf("new fund should be at least 1 USD")
	}

	var (
		prevUpdatedTime   time.Time
		curMembershipCost int
		curBalance        float64
	)

	query := fmt.Sprintf(
		`SELECT updated, balance, charge FROM %s.%s WHERE chain = ? AND address = ?`,
		b.conf.DB.Keyspace, consts.BillingTable,
	)

	if err = b.db.Query(query, chain, address).Scan(&prevUpdatedTime, &curBalance, &curMembershipCost); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("Getting fund details failed")
			return fmt.Errorf("failed to get fund details: %w", err)
		}
	}

	// if somehow cur membership details go missing from DB
	curMembershipTier := consts.MembershipStringToEnum(curMembership)
	if curMembershipCost <= 0 && curMembershipTier != consts.FreeTier {
		curMembershipCost = consts.MembershipCharge[curMembershipTier]
	}

	balance := curBalance + usd
	log.Infof(
		"Current Balance: %f, new fund: %f, total balance: %f, membership cost: %d,",
		curBalance, usd, balance, curMembershipCost,
	)

	numDays, availableBalance := getBalanceDaysAndMoney(
		curMembershipTier, curMembershipCost, curMembershipCost, balance, prevUpdatedTime,
	)
	expiry := utilities.TimeNow().AddDate(0, 0, int(numDays))

	columnClause := []string{"chain", "address", "updated", "balance", "charge"}
	valuePlaceholderClause := utilities.DBMultiValuePlaceholders(5)
	valueClause := []interface{}{chain, address, timeNow, availableBalance, curMembershipCost}
	if numDays != math.MaxFloat64 {
		columnClause = append(columnClause, "expiry")
		valuePlaceholderClause = utilities.DBMultiValuePlaceholders(6)
		valueClause = append(valueClause, expiry)
	}

	query = fmt.Sprintf(
		`INSERT INTO %s.%s (chain, address, txn_id, paid_time, paid_amt) VALUES %s`,
		b.conf.DB.Keyspace, consts.BillingHistoryTable, utilities.DBMultiValuePlaceholders(5),
	)

	if err = b.db.Query(query, chain, address, txnID, timeNow, usd).Exec(); err != nil {
		log.WithError(err).Error("billing history update failed")
		return fmt.Errorf("failed to update billing history: %w", err)
	}

	query = fmt.Sprintf(
		`INSERT INTO %s.%s (%s) VALUES %s`,
		b.conf.DB.Keyspace, consts.BillingTable, strings.Join(columnClause, ","), valuePlaceholderClause,
	)

	if err = b.db.Query(query, valueClause...).Exec(); err != nil {
		log.WithError(err).Error("billing update failed")
		return fmt.Errorf("failed to update billing: %w", err)
	}

	return nil
}

func (b *BillingRepo) GetBillingDetails(
	_ context.Context, curMembership string, req entities.BillingRequest,
) (*entities.BillingInfo, error) {
	chain := req.Chain
	address := req.Address

	log := utilities.NewLoggerWithFields(
		"repo.GetBillingDetails", map[string]interface{}{
			"chain":   chain,
			"address": address,
		},
	)

	var (
		prevUpdatedTime   time.Time
		expiry            time.Time
		curMembershipCost int
		curBalance        float64
	)

	query := fmt.Sprintf(
		`SELECT expiry, updated, balance, charge FROM %s.%s WHERE chain = ? AND address = ?`,
		b.conf.DB.Keyspace, consts.BillingTable,
	)

	if err := b.db.Query(query, chain, address).Scan(
		&expiry, &prevUpdatedTime, &curBalance, &curMembershipCost,
	); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("Getting fund details failed")
			return nil, fmt.Errorf("failed to get fund details: %w", err)
		}
	}

	curMembershipTier := consts.MembershipStringToEnum(curMembership)

	remainingNotifications := consts.NotificationCount[curMembershipTier] - req.TotalSent
	if remainingNotifications < 0 {
		remainingNotifications = 0
	}

	blockedChannels := make([]string, 0)
	blockedChannelsCount := len(req.OwnedChannels) - consts.ChannelCount[curMembershipTier]
	log.Infof(
		"Permitted channel count: %d, current: %d", consts.ChannelCount[curMembershipTier], len(req.OwnedChannels),
	)
	if blockedChannelsCount > 0 {
		for _, tbl := range []string{consts.VerifiedChannelInfo, consts.UnverifiedChannelInfo} {
			query = fmt.Sprintf(
				"SELECT app_id, status FROM %s.%s WHERE chain = ? AND app_id in ?",
				b.conf.DB.Keyspace, tbl,
			)
			iter := b.db.Query(query, chain, req.OwnedChannels).Iter()

			var (
				appID  string
				status string
			)

			for iter.Scan(&appID, &status) {
				if status != consts.STATUS_ACTIVE {
					blockedChannels = append(blockedChannels, appID)
				}
			}

			if err := iter.Close(); err != nil {
				return nil, err
			}
		}
	}

	info := &entities.BillingInfo{
		Address:                address,
		Chain:                  chain,
		Expiry:                 expiry,
		LastUpdated:            prevUpdatedTime,
		Balance:                fmt.Sprintf("%.2f", curBalance),
		Membership:             curMembershipTier.String(),
		RemainingNotifications: remainingNotifications,
		BlockedChannels:        blockedChannels,
	}

	billingRecords := make([]entities.BillingRecords, 0)

	var (
		paidTime time.Time
		paidAmt  float64
		txnID    string
	)

	query = fmt.Sprintf(
		`SELECT paid_time, txn_id, paid_amt FROM %s.%s WHERE chain = ? AND address = ?`,
		b.conf.DB.Keyspace, consts.BillingHistoryTable,
	)

	iter := b.db.Query(query, chain, address).Iter()
	for iter.Scan(&paidTime, &txnID, &paidAmt) {
		billingRecords = append(
			billingRecords, entities.BillingRecords{
				PaidAmount: paidAmt,
				PaidTime:   paidTime,
				TxnID:      txnID,
			},
		)
	}

	if err := iter.Close(); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("Getting fund details failed")
			return nil, fmt.Errorf("failed to get fund details: %w", err)
		}
	}

	info.BillingRecords = billingRecords

	return info, nil
}

func (b *BillingRepo) GetMembershipTiers(_ context.Context) (map[string]map[string]interface{}, error) {
	tierInfo := make(map[string]map[string]interface{})

	m := make(map[string]interface{})
	for tier, val := range consts.NotificationCharacterCount {
		m[tier.String()] = val
	}
	tierInfo["notification_char_count"] = m

	m = make(map[string]interface{})
	for tier, val := range consts.NotificationCount {
		m[tier.String()] = val
	}
	tierInfo["notification_count"] = m

	m = make(map[string]interface{})
	for tier, val := range consts.ChannelCount {
		m[tier.String()] = val
	}
	tierInfo["channel_count"] = m

	m = make(map[string]interface{})
	for tier, charge := range consts.MembershipCharge {
		m[tier.String()] = charge
	}
	tierInfo["charge"] = m

	m = make(map[string]interface{})
	for tier, seconds := range consts.NotificationRetentionSecs {
		m[tier.String()] = seconds / 60 / 60 / 24
	}
	tierInfo["notification_retention"] = m

	m = make(map[string]interface{})
	for tier, seconds := range consts.NotificationMaxSchedule {
		m[tier.String()] = seconds / 60 / 60 / 24
	}
	tierInfo["notification_max_schedule_time"] = m

	m = make(map[string]interface{})
	for tier, isAllowed := range consts.ChannelRename {
		m[tier.String()] = isAllowed
	}
	tierInfo["channel_rename"] = m

	for statsName, val := range consts.Analytics {
		m = make(map[string]interface{})
		for tier, allowed := range val {
			m[tier.String()] = allowed
		}
		tierInfo[statsName] = m
	}

	return tierInfo, nil
}
