package config

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/viper"

	"notiboy/pkg/consts"
)

const configFilePath = "/etc/notiboy/config.yaml"

var (
	notiboyConf   *NotiboyConfModel
	ServerBaseURL string
	PathPrefix    string
)

func LoadConfig() (*NotiboyConfModel, error) {
	if err := loadViperConfig(configFilePath); err != nil {
		return nil, err
	}

	return notiboyConf, nil
}

func loadViperConfig(filePath string) error {
	viper.SetConfigFile(filePath)
	err := viper.ReadInConfig()
	if err != nil {
		return fmt.Errorf("error reading viper config: %w", err)
	}

	setEnvConf()
	setDefault()

	viper.WatchConfig()

	err = viper.Unmarshal(&notiboyConf)
	if err != nil {
		return fmt.Errorf("error loading viper config to struct: %w", err)
	}

	val, err := json.MarshalIndent(*notiboyConf, "", "  ")
	if err == nil {
		fmt.Println(string(val))
	}

	loadAdminUsers()

	// https://app.notiboy.com/api/stage/v1
	ServerBaseURL, err = url.JoinPath(notiboyConf.Server.RedirectPrefix, notiboyConf.Server.APIPrefix, notiboyConf.Mode, notiboyConf.Server.APIVersion)
	if err != nil {
		return err
	}

	// /api/stage/v1
	PathPrefix, err = url.JoinPath(notiboyConf.Server.APIPrefix, notiboyConf.Mode, notiboyConf.Server.APIVersion)
	if err != nil {
		return err
	}

	return nil
}

func setEnvConf() {
	viper.BindEnv("db.username", "NOTIBOY_DB_USERNAME")
	viper.BindEnv("db.password", "NOTIBOY_DB_PASSWORD")
}

func setDefault() {
	viper.SetDefault("supported", []string{consts.Algorand})
	viper.SetDefault("mode", "stage")
	viper.SetDefault("auto_onboard_users", true)
	viper.SetDefault("chat.group.ttl", 604800)
	viper.SetDefault("chat.personal.ttl", 604800)
}

// GetConfig returns env config
func GetConfig() *NotiboyConfModel {
	return notiboyConf
}
