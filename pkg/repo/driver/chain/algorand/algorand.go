package algorand

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/spf13/cast"

	"notiboy/utilities"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/v2/encoding/msgpack"
	"github.com/algorand/go-algorand-sdk/v2/transaction"
	"github.com/algorand/go-algorand-sdk/v2/types"

	"notiboy/config"
	"notiboy/pkg/entities"
)

type Algorand struct {
	client    *algod.Client
	blockInfo entities.BlockModel
}

// VerifyPayment verifies payment txn and returns dollars received
func (alg *Algorand) VerifyPayment(ctx context.Context, senderAddress string, signedTxnStr string, _ string) (
	float64, string, error,
) {
	log := utilities.NewLogger("VerifyPayment")

	mode := config.GetConfig().Mode

	fundReceiver := config.GetConfig().Algorand.Fund.Mainnet.Address
	fundAsset := config.GetConfig().Algorand.Fund.Mainnet.Asset
	if mode == "stage" || mode == "local" {
		fundAsset = config.GetConfig().Algorand.Fund.Testnet.Asset
		fundReceiver = config.GetConfig().Algorand.Fund.Testnet.Address
	}

	var signedTxnModel types.SignedTxn

	signedTxnBytes, err := base64.StdEncoding.DecodeString(signedTxnStr)
	if err != nil {
		return 0, "", fmt.Errorf("failed to b64 decode signedtxn: %w", err)
	}

	err = msgpack.Decode(signedTxnBytes, &signedTxnModel)
	if err != nil {
		return 0, "", fmt.Errorf("failed to decode msgpack, signedTxnStr=%s: %w", signedTxnStr, err)
	}

	fromAddr := signedTxnModel.Txn.Sender.String()
	if fromAddr != senderAddress {
		return 0, "", fmt.Errorf(
			"txn sender is different from txn signer, found: %s, expected: %s", fromAddr, senderAddress,
		)
	}

	toAddr := signedTxnModel.Txn.AssetReceiver.String()
	log.Debugf("SenderAddress: %s, ToAddress: %s", fromAddr, toAddr)

	if toAddr != fundReceiver {
		return 0, "", fmt.Errorf("fund sent to %s instead of %s", toAddr, fundReceiver)
	}

	assetIdx := signedTxnModel.Txn.XferAsset
	if int64(assetIdx) != fundAsset {
		log.Errorf("asset idx %d is different from expected %d", assetIdx, fundAsset)
		return 0, "", fmt.Errorf("asset idx %d is different from expected %d", assetIdx, fundAsset)
	}

	assetCloseTo := signedTxnModel.Txn.AssetCloseTo
	if !assetCloseTo.IsZero() {
		return 0, "", fmt.Errorf("closeTo is set")
	}

	assetCount := signedTxnModel.Txn.AssetAmount

	txnID, err := alg.client.SendRawTransaction(signedTxnBytes).Do(ctx)
	if err != nil {
		log.WithError(err).Error("failed to send transaction")
		return 0, "", fmt.Errorf("failed to send transaction: %w", err)
	}
	log.Infof("Submitted transaction with ID: %s, amount: %d", txnID, assetCount)

	_, err = transaction.WaitForConfirmation(alg.client, txnID, 10, ctx)
	if err != nil {
		return 0, "", fmt.Errorf("waiting for txn timed out: %w", err)
	}

	log.Infof("Received asset of %f", float64(assetCount/1000000.0))

	return float64(assetCount / 1000000.0), txnID, nil
}

func New() *Algorand {
	log := utilities.NewLogger("New")

	conf := config.GetConfig()
	alg := new(Algorand)

	var address, token string
	if config.GetConfig().Mode == "stage" || config.GetConfig().Mode == "local" {
		address = conf.Algorand.Daemon.Testnet.Address
		token = conf.Algorand.Daemon.Testnet.Token
	} else {
		address = conf.Algorand.Daemon.Mainnet.Address
		token = conf.Algorand.Daemon.Mainnet.Token
	}

	algodClient, err := algod.MakeClient(address, token)
	if err != nil {
		log.WithError(err).Fatal("failed to get algorand client")
	}

	alg.client = algodClient

	return alg
}

func (alg *Algorand) VerifyTransaction(_ context.Context, senderAddress string, signedTxnStr string) error {
	log := utilities.NewLoggerWithFields(
		"VerifyTransaction", map[string]interface{}{
			"address": senderAddress,
		},
	)

	var signedTxnModel types.SignedTxn

	signedTxnBytes, err := base64.StdEncoding.DecodeString(signedTxnStr)
	if err != nil {
		return fmt.Errorf("failed to b64 decode signedtxn: %w", err)
	}

	err = msgpack.Decode(signedTxnBytes, &signedTxnModel)
	if err != nil {
		return fmt.Errorf("failed to decode msgpack, signedTxnStr=%s: %w", signedTxnStr, err)
	}

	from := signedTxnModel.Txn.Sender[:]
	authAddr := signedTxnModel.AuthAddr[:]
	fromAddr := signedTxnModel.Txn.Sender.String()
	log.Debugf("SenderAddress: %s, AuthAddress: %s", from, authAddr)

	if fromAddr != senderAddress {
		return fmt.Errorf("txn sender is different from txn signer, found: %s, expected: %s", fromAddr, senderAddress)
	}

	encodedTx := msgpack.Encode(signedTxnModel.Txn)

	msgParts := [][]byte{[]byte("TX"), encodedTx}
	msg := bytes.Join(msgParts, nil)

	emptyAddrVar := make([]byte, 32)
	if !bytes.Equal(emptyAddrVar, authAddr) {
		from = authAddr
	}

	if !ed25519.Verify(from, msg, signedTxnModel.Sig[:]) {
		return fmt.Errorf("txn signature validation failed")
	}

	txnBlock := uint64(signedTxnModel.Txn.FirstValid)

	ttl := cast.ToDuration(config.GetConfig().Chain.BlockTTL)
	interval := cast.ToDuration(config.GetConfig().Chain.BlockTimerInterval)
	leewayTime := cast.ToDuration(config.GetConfig().Algorand.BlockLeewayTime)
	cachedBlockTime := alg.blockInfo.Time
	cachedFirstBlockRound := uint64(alg.blockInfo.Block)
	if cachedBlockTime.Add(ttl).Before(time.Now()) {
		return fmt.Errorf("stale cached block found, block: %d at %s", cachedFirstBlockRound, cachedBlockTime)
	}

	blockCreationPacePerMinute := config.GetConfig().Algorand.BlockCreationPacePerMinute
	// We fetch block at t1, next fetch is at t2 where t2 - t1 = block_timer_interval
	// If txn comes with block at (t2 - 1 second), we need to find max
	maxValidBlock := cachedFirstBlockRound + uint64(int(interval.Minutes())*blockCreationPacePerMinute)
	// providing a 10 minutes leeway to account for the foll. case
	// We fetch block at t2, user fetches block at t1 - since block creation is fast, we accept 10 minutes old blocks too
	minValidBlock := cachedFirstBlockRound - uint64(int(leewayTime.Minutes())*blockCreationPacePerMinute)
	log.Debugf("MaxValidBlock=%d, MinValidBlock=%d, usr block: %d", maxValidBlock, minValidBlock, txnBlock)

	if txnBlock > maxValidBlock {
		return fmt.Errorf("found block %d, max valid block is %d", txnBlock, maxValidBlock)
	} else if txnBlock < minValidBlock {
		return fmt.Errorf("found block %d, min valid block is %d", txnBlock, minValidBlock)
	}

	return nil
}

func (alg *Algorand) FetchValidBlock(_ context.Context) error {
	log := utilities.NewLogger("FetchValidBlock")

	// implement retry backoff
	sp, err := alg.client.SuggestedParams().Do(context.Background())
	if err != nil {
		log.WithError(err).Error("failed to get suggested params")
		return err
	}

	firstValidBlock := float64(sp.FirstRoundValid)
	alg.blockInfo = entities.BlockModel{
		Block: firstValidBlock,
		Time:  time.Now(),
	}

	log.Debugf("Fetched block %f", firstValidBlock)

	return nil
}
