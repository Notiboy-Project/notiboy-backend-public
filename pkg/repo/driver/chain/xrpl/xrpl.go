package xrpl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gocql/gocql"
	"github.com/spf13/cast"

	"notiboy/pkg/consts"
	"notiboy/pkg/repo/driver/chain/xrpl/wallets"
	"notiboy/pkg/repo/driver/db"
	"notiboy/utilities"
	"notiboy/utilities/http_client"

	_ "github.com/xyield/xrpl-go/model/client/common"
	_ "github.com/xyield/xrpl-go/model/client/transactions"

	"notiboy/config"
	"notiboy/pkg/entities"
)

type Xrpl struct {
	client    *http.Client
	blockInfo entities.BlockModel
	wallets   map[string]Wallets
}

type Wallets interface {
	VerifyTransaction(ctx context.Context, senderAddress string, signedTxn string) error
}

var _ Wallets = new(wallets.XummWallet)

func getAddr() string {
	conf := config.GetConfig()

	var address string
	if config.GetConfig().Mode == "stage" || config.GetConfig().Mode == "local" {
		address = conf.Xrpl.Daemon.Testnet.Address
	} else {
		address = conf.Xrpl.Daemon.Mainnet.Address
	}

	return address
}

// VerifyPayment verifies payment txn and returns dollars received
func (x *Xrpl) VerifyPayment(_ context.Context, senderAddress string, _ string, txnID string) (float64, string, error) {
	log := utilities.NewLogger("VerifyPayment")

	if txnID == "" {
		return 0, "", fmt.Errorf("received empty txn_id")
	}

	mode := config.GetConfig().Mode

	fundReceiver := config.GetConfig().Xrpl.Fund.Mainnet.Address
	if mode == "stage" || mode == "local" {
		fundReceiver = config.GetConfig().Xrpl.Fund.Testnet.Address
	}

	req := TxnReferenceRequest{
		Method: "tx",
		Params: []TxnReferenceRequestParams{
			{
				Transaction: txnID,
				Binary:      false,
			},
		},
	}
	reqBody, err := json.Marshal(req)
	if err != nil {
		return 0, "", fmt.Errorf("failed to marshal txn ref request: %w", err)
	}

	resp, err := x.client.Post(getAddr(), "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return 0, "", fmt.Errorf("failed to make POST call to %s: %w", getAddr(), err)
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.WithError(err).Error("failed to close response body")
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("received unexpected http status code: %d, status: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("failed to read response body: %w", err)
	}

	var res *TxnReferenceResponse
	err = json.Unmarshal(body, &res)
	if err != nil {
		return 0, "", fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	fromAddr := res.Result.Account
	toAddr := res.Result.Destination
	lastLedger := res.Result.LastLedgerSequence
	txnType := res.Result.TransactionType
	// 86400 is to minus 24 hours from txn date
	txnTime := int64(res.Result.Date) + 946684800 - 86400
	txnStatus := res.Result.Meta.TransactionResult
	deliveredAmount := res.Result.Meta.DeliveredAmount

	log.Debugf("Dest: %s, Src: %s, LastLedger: %d", toAddr, fromAddr, lastLedger)

	if fromAddr != senderAddress {
		return 0, "", fmt.Errorf(
			"txn sender is different from txn src addr, found: %s, expected: %s", fromAddr, senderAddress,
		)
	}

	if toAddr != fundReceiver {
		return 0, "", fmt.Errorf("fund sent to %s instead of %s", toAddr, fundReceiver)
	}

	if txnStatus != "tesSUCCESS" {
		return 0, "", fmt.Errorf("payment txn submission failed. status: %s", txnStatus)
	}

	if txnType != "Payment" {
		return 0, "", fmt.Errorf("non-payment txn received: %s", txnType)
	}

	//i, err := strconv.ParseInt(txnTime, 10, 64)
	//if err != nil {
	//	panic(err)
	//}
	utcTxnTime := time.Unix(txnTime, 0).UTC()

	var txnIDFromDB string
	query := fmt.Sprintf(
		`SELECT txn_id FROM %s.%s WHERE chain = ? AND address = ? AND paid_time > ?`,
		config.GetConfig().DB.Keyspace, consts.BillingHistoryTable,
	)

	iter := db.GetCassandraSession().Query(query, consts.Xrpl, fromAddr, utcTxnTime).Iter()
	for iter.Scan(&txnIDFromDB) {
		if txnIDFromDB == txnID {
			return 0, "", fmt.Errorf("duplicate txn found: %s", txnID)
		}
	}

	if err = iter.Close(); err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			log.WithError(err).Error("getting billing details failed")
			return 0, "", fmt.Errorf("failed to get billing details: %w", err)
		}
	}

	amount := cast.ToFloat64(deliveredAmount) / 1000000.0
	//convert to USD
	usdAmount := amount * x.blockInfo.Block

	log.Infof("Received asset of %f -> USD: %f", amount, usdAmount)

	return usdAmount, txnID, nil
}

func New() *Xrpl {
	log := utilities.NewLogger("New")

	x := new(Xrpl)

	xrplClient := http_client.GetClient()

	x.client = xrplClient

	x.wallets = make(map[string]Wallets)
	x.wallets[consts.XummWallet] = wallets.NewXummWallet()

	log.Info("Xrpl client created")

	return x
}

func (x *Xrpl) VerifyTransaction(ctx context.Context, senderAddress string, signedTxnStr string) error {
	log := utilities.NewLoggerWithFields(
		"VerifyTransaction", map[string]interface{}{
			"address": senderAddress,
		},
	)

	log.Info("Verifying transaction using xumm wallet")

	return x.wallets[consts.XummWallet].VerifyTransaction(ctx, senderAddress, signedTxnStr)
}

func (x *Xrpl) FetchValidBlock(_ context.Context) error {
	log := utilities.NewLogger("FetchValidBlock")

	// run this only every 24 hours
	if !x.blockInfo.Time.IsZero() {
		if x.blockInfo.Time.Add(time.Hour * 24).After(time.Now()) {
			fmt.Println("skipping since time less than 24h")
			return nil
		}
	}

	pricingAPI := fmt.Sprintf(config.GetConfig().Chain.PricingApi, "ripple", "usd")

	resp, err := x.client.Get(pricingAPI)
	if err != nil {
		return fmt.Errorf("failed to make GET call to %s: %w", pricingAPI, err)
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.WithError(err).Error("failed to close response body")
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received unexpected http status code: %d, status: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var res *PricingAPIResponse
	err = json.Unmarshal(body, &res)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	x.blockInfo = entities.BlockModel{
		Block: res.Ripple.Usd,
		Time:  time.Now(),
	}

	log.Debugf("Fetched current price %f", x.blockInfo.Block)

	return nil
}
