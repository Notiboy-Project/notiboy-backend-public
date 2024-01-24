package nfd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"notiboy/config"
	"notiboy/utilities"
	"notiboy/utilities/http_client"
)

var Client *Nfd

type Nfd struct {
	cl *http.Client
}

type Response map[string]Item

type Item struct {
	CaAlgo           []string `json:"caAlgo"`
	Name             string   `json:"name"`
	Owner            string   `json:"owner"`
	UnverifiedCaAlgo []string `json:"unverifiedCaAlgo"`
}

func InitClient(ctx context.Context) *Nfd {
	log := utilities.NewLogger("InitClient")

	Client = new(Nfd)

	nfdClient := http_client.GetClient()

	Client.cl = nfdClient

	log.Info("NFD client created")

	return Client
}

func getNfdAddr() string {
	dns := config.GetConfig().Dns.Algorand

	var (
		address string
	)
	if config.GetConfig().Mode == "stage" || config.GetConfig().Mode == "local" {
		address = fmt.Sprintf("%s/%s", dns.Testnet.Nfd.Url, dns.Testnet.Nfd.Path)
	} else {
		address = fmt.Sprintf("%s/%s", dns.Mainnet.Nfd.Url, dns.Mainnet.Nfd.Path)
	}

	return address
}

func (n *Nfd) GetNfdNames(addresses []string) (map[string]string, error) {
	log := utilities.NewLogger("GetNfdNames")
	domains := make(map[string]string)

	endpoint := getNfdAddr()

	var queryParams []string
	for _, address := range addresses {
		queryParams = append(queryParams, fmt.Sprintf("address=%s", address))
	}

	endpoint = fmt.Sprintf("%s&%s", endpoint, strings.Join(queryParams, "&"))

	resp, err := n.cl.Get(endpoint)
	if err != nil {
		return domains, fmt.Errorf("failed to make GET call to %s: %w", endpoint, err)
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.WithError(err).Error("failed to close response body")
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return domains, fmt.Errorf("received unexpected http status code: %d, status: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return domains, fmt.Errorf("failed to read response body: %w", err)
	}

	var res *Response
	err = json.Unmarshal(body, &res)
	if err != nil {
		return domains, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	for addr, item := range *res {
		domains[addr] = item.Name
	}

	return domains, nil
}
