package dns

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/repo/driver/db"
	"notiboy/pkg/repo/driver/dns/nfd"
	"notiboy/pkg/repo/driver/dns/xrpns"
	"notiboy/utilities"
)

type Client struct {
	Nfd   *nfd.Nfd
	Xrpns *xrpns.Xrpns
}

func InitDNSClient(ctx context.Context) {
	log := utilities.NewLogger("InitDNSClient")

	nfdClient := nfd.InitClient(ctx)
	xrpnsClient := xrpns.InitClient(ctx)
	dnsClient := &Client{
		Nfd:   nfdClient,
		Xrpns: xrpnsClient,
	}

	fn := func() {
		ticker := time.NewTicker(6 * time.Hour)

		for {
			if err := dnsClient.Sync(ctx); err != nil {
				log.WithError(err).Error("dns sync failed")
			}

			select {
			case <-ctx.Done():
				log.Info("Terminating dns client...")
				ticker.Stop()
				return
			case <-ticker.C:
				continue
			}
		}
	}

	go fn()
	log.Info("DNS client created")

	return
}

func getAddresses(pageSize int, pageState []byte) (map[string][]string, []byte, error) {
	query := fmt.Sprintf("SELECT address, chain FROM %s.%s", config.GetConfig().DB.Keyspace, consts.UserTable)

	iter := db.GetCassandraSession().Query(query).PageSize(pageSize).PageState(pageState).Iter()
	currentPageState := iter.PageState()

	var address, chain string

	addresses := make(map[string][]string)
	for _, supportedChain := range config.GetConfig().Chain.Supported {
		addresses[strings.ToLower(supportedChain)] = make([]string, 0)
	}

	for iter.Scan(&address, &chain) {
		addresses[chain] = append(addresses[chain], address)
	}

	if err := iter.Close(); err != nil {
		return addresses, currentPageState, fmt.Errorf("failed to get addresses: %w", err)
	}

	return addresses, currentPageState, nil
}

func storeInDB(ctx context.Context, chain string, domains map[string]string) error {
	query := fmt.Sprintf(
		"INSERT INTO %s.%s (chain, user, dns) VALUES %s", config.GetConfig().DB.Keyspace, consts.UserDNSTable,
		utilities.DBMultiValuePlaceholders(3),
	)
	for addr, dns := range domains {
		if err := db.GetCassandraSession().Query(query, chain, addr, dns).Exec(); err != nil {
			logrus.WithError(err).Error("failed to store dns entry")
		}
	}

	return nil
}

func (dnsClient *Client) fetchAndStoreNFDomains(ctx context.Context, addresses []string) error {
	if len(addresses) == 0 {
		return nil
	}

	domains, err := dnsClient.Nfd.GetNfdNames(addresses)
	if err != nil {
		return fmt.Errorf("failed to fetch nfd domains: %w", err)
	}

	err = storeInDB(ctx, consts.Algorand, domains)
	if err != nil {
		return fmt.Errorf("failed to store nfd domains: %w", err)
	}

	return nil
}

func (dnsClient *Client) fetchAndStoreXRPDomains(ctx context.Context, addresses []string) error {
	if len(addresses) == 0 {
		return nil
	}

	domains, err := dnsClient.Xrpns.GetXrpnsNames(addresses)
	if err != nil {
		return fmt.Errorf("failed to fetch xrp domains: %w", err)
	}

	err = storeInDB(ctx, consts.Xrpl, domains)
	if err != nil {
		return fmt.Errorf("failed to store xrp domains: %w", err)
	}

	return nil
}

func (dnsClient *Client) Sync(ctx context.Context) error {
	t1 := time.Now()
	defer func() {
		logrus.Infof("Finished syncing DNS names in %s", time.Now().Sub(t1))
	}()

	var pageState []byte
	pageSize := 20

	for {
		addresses, currPageState, err := getAddresses(pageSize, pageState)
		if err != nil {
			return fmt.Errorf("failed to fetch addresses: %w", err)
		}

		addrCount := 0
		for _, chainAddresses := range addresses {
			addrCount += len(chainAddresses)
		}

		if addrCount == 0 {
			logrus.Debugf("no addresses returned")
			return nil
		}

		pageState = currPageState

		if err = dnsClient.fetchAndStoreNFDomains(ctx, addresses[consts.Algorand]); err != nil {
			logrus.WithError(err).Error("failed to fetchAndStoreNFDomains")
		}

		if err = dnsClient.fetchAndStoreXRPDomains(ctx, addresses[consts.Xrpl]); err != nil {
			logrus.WithError(err).Error("failed to fetchAndStoreXRPDomains")
		}

		if addrCount < pageSize {
			logrus.Debugf("no more addresses left, got: %d", addrCount)
			return nil
		}
	}
}
