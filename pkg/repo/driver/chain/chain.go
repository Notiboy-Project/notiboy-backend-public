package chain

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cast"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/repo/driver/chain/algorand"
	"notiboy/pkg/repo/driver/chain/xrpl"
	"notiboy/utilities"
)

var chainStore *Store

type Store struct {
	store           map[string]Chain
	supportedChains map[string]struct{}
}

type Chain interface {
	VerifyTransaction(ctx context.Context, senderAddress string, signedTxn string) error
	FetchValidBlock(ctx context.Context) error
	VerifyPayment(ctx context.Context, senderAddress string, signedTxnStr string, txnID string) (float64, string, error)
}

// LoadChains initialises clients of blockchain networks
func LoadChains(ctx context.Context) {
	chainStore = new(Store)

	chainStore.store = make(map[string]Chain)

	chainStore.supportedChains = make(map[string]struct{})
	for _, chain := range config.GetConfig().Chain.Supported {
		chain = strings.ToLower(chain)
		chainStore.supportedChains[chain] = struct{}{}

		switch chain {
		case consts.Algorand:
			chainStore.store[consts.Algorand] = algorand.New()
		case consts.Xrpl:
			chainStore.store[consts.Xrpl] = xrpl.New()
		}
	}

	if config.GetConfig().Mode == "stage" || config.GetConfig().Mode == "local" {
		go chainBlockTimer(ctx)
	} else {
		chainBlockTimer(ctx)
	}
}

func chainBlockTimer(ctx context.Context) {
	log := utilities.NewLogger("chainBlockTimer")

	ticker := time.NewTicker(cast.ToDuration(config.GetConfig().Chain.BlockTimerInterval))

	fn := func() {
		for chainName, chain := range chainStore.store {
			err := chain.FetchValidBlock(ctx)
			if err != nil {
				log.WithError(err).Errorf("failed to fetch block info of %s", chainName)
			}
		}
	}
	fn()

	log.Info("Starting ticker")
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Info("Terminating...")
				ticker.Stop()
				return
			case t := <-ticker.C:
				log.Debugf("Tick at %s", t)
				fn()
			}
		}
	}()
}

// GetBlockchainClient returns client of specified blockchain network
func GetBlockchainClient(network string) Chain {
	return chainStore.store[network]
}

// IsChainSupported checks if the given chain is supported by Notiboy
func IsChainSupported(chain string) bool {
	_, present := chainStore.store[chain]
	return present
}
