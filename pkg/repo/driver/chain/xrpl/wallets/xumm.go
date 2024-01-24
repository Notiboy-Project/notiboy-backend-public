package wallets

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	
	"notiboy/config"
	"notiboy/pkg/entities"
	"notiboy/utilities"
	"notiboy/utilities/http_client"
)

const xummEndpoint = "https://xumm.app"

type XummWallet struct {
	client    *http.Client
	blockInfo entities.BlockModel
}

type PingResponse struct {
	Pong            bool   `json:"pong"`
	ClientId        string `json:"client_id"`
	State           string `json:"state"`
	Scope           string `json:"scope"`
	Aud             string `json:"aud"`
	Sub             string `json:"sub"`
	Email           string `json:"email"`
	AppUuidv4       string `json:"app_uuidv4"`
	AppName         string `json:"app_name"`
	PayloadUuidv4   string `json:"payload_uuidv4"`
	UsertokenUuidv4 string `json:"usertoken_uuidv4"`
	NetworkType     string `json:"network_type"`
	NetworkEndpoint string `json:"network_endpoint"`
	NetworkId       string `json:"network_id"`
	Iat             int    `json:"iat"`
	Exp             int    `json:"exp"`
	Iss             string `json:"iss"`
}

func NewXummWallet() *XummWallet {
	log := utilities.NewLogger("New")

	x := new(XummWallet)

	xrplClient := http_client.GetClient()

	x.client = xrplClient

	log.Info("Xumm Wallet client created")

	return x
}

func (xw *XummWallet) VerifyTransaction(_ context.Context, senderAddress string, encodedJWT string) error {
	log := utilities.NewLoggerWithFields(
		"xumm.VerifyTransaction", map[string]interface{}{
			"address": senderAddress,
		},
	)

	requestURL, _ := url.JoinPath(xummEndpoint, "api/v1/jwt/ping")

	jwt, err := base64.StdEncoding.DecodeString(encodedJWT)
	if err != nil {
		return fmt.Errorf("failed to decode b64 jwt: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create GET request to %s: %w", requestURL, err)
	}
	req.Header.Add("Authorization", "Bearer "+string(jwt))

	resp, err := xw.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make GET call to %s: %w", requestURL, err)
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

	var res *PingResponse
	err = json.Unmarshal(body, &res)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	if res.Sub != senderAddress {
		log.Errorf("sender %s != subject %s", senderAddress, res.Sub)
		return fmt.Errorf("malformed JWT token received, incorrect sender")
	}

	xummAppName := "notiboy"
	if config.GetConfig().Mode == "stage" {
		xummAppName = "notiboy-staging"
	} else if config.GetConfig().Mode == "local" {
		xummAppName = "notiboy-local"
	}

	if !strings.EqualFold(res.AppName, xummAppName) {
		log.Errorf("appname %s != expected %s", res.AppName, xummAppName)
		return fmt.Errorf("malformed JWT token received, incorrect appname %s", res.AppName)
	}

	return nil
}
