package xrpns

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"

	"notiboy/config"
	"notiboy/utilities"
	"notiboy/utilities/http_client"
)

var Client *Xrpns

type Xrpns struct {
	cl *http.Client
}

type Response struct {
	Domain string `json:"domain"`
}

func InitClient(ctx context.Context) *Xrpns {
	log := utilities.NewLogger("InitClient")

	Client = new(Xrpns)

	xrpnsClient := http_client.GetClient()

	Client.cl = xrpnsClient

	log.Info("XRPNS client created")

	return Client
}

func getXrpnsAddrAndToken() (string, string) {
	dns := config.GetConfig().Dns.Xrpl

	var (
		address, token string
	)
	if config.GetConfig().Mode == "stage" || config.GetConfig().Mode == "local" {
		address = fmt.Sprintf("%s/%s", dns.Testnet.Xrpns.Url, dns.Testnet.Xrpns.Path)
		token = dns.Testnet.Xrpns.Token
	} else {
		address = fmt.Sprintf("%s/%s", dns.Mainnet.Xrpns.Url, dns.Mainnet.Xrpns.Path)
		token = dns.Mainnet.Xrpns.Token
	}

	return address, token
}

func (n *Xrpns) getDomainName(address string) (string, error) {
	endpoint, token := getXrpnsAddrAndToken()

	url := fmt.Sprintf(endpoint, address)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create a request: %w", err)
	}

	req.Header.Add("X-API-Key", token)
	resp, err := n.cl.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			logrus.WithError(err).Error("failed to close response body")
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received unexpected http status code: %d, status: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var res *Response
	err = json.Unmarshal(body, &res)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return res.Domain, nil
}

func (n *Xrpns) GetXrpnsNames(addresses []string) (map[string]string, error) {
	log := utilities.NewLogger("GetXrpnsNames")
	domains := make(map[string]string)

	for _, address := range addresses {
		domain, err := n.getDomainName(address)
		if err != nil {
			log.WithError(err).Errorf("failed to get domain name for %s", address)
			continue
		}

		domains[address] = domain
	}

	return domains, nil
}
