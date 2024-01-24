package config

type NotiboyConfModel struct {
	AutoOnboardUsers          bool      `mapstructure:"auto_onboard_users"`
	LogLevel                  string    `mapstructure:"log_level"`
	LoginTokenExpiry          string    `mapstructure:"login_token_expiry"`
	MembershipCheckerInterval string    `mapstructure:"membership_checker_interval"`
	Mode                      string    `mapstructure:"mode"`
	AdminUsers                []string  `mapstructure:"admin_users"`
	Server                    Server    `mapstructure:"server"`
	Chain                     Chain     `mapstructure:"chain"`
	Algorand                  Algorand  `mapstructure:"algorand"`
	Xrpl                      Xrpl      `mapstructure:"xrpl"`
	DB                        DB        `mapstructure:"db"`
	Migration                 Migration `mapstructure:"migration"`
	Logo                      Logo      `mapstructure:"logo"`
	TTL                       TTL       `mapstructure:"ttl"`
	Email                     Email     `mapstructure:"email"`
	Discord                   Discord   `mapstructure:"discord"`
	Firebase                  Firebase  `mapstructure:"firebase"`
	Chat                      Chat      `mapstructure:"chat"`
	Dns                       Dns       `mapstructure:"dns"`
}

type Algorand struct {
	BlockCreationPacePerMinute int         `mapstructure:"block_creation_pace_per_minute"`
	BlockLeewayTime            string      `mapstructure:"block_leeway_time"`
	Daemon                     ChainDaemon `mapstructure:"daemon"`
	Fund                       ChainFund   `mapstructure:"fund"`
}

type Xrpl struct {
	LedgerCreationPacePerMinute int         `mapstructure:"ledger_creation_pace_per_minute"`
	LedgerLeewayTime            string      `mapstructure:"ledger_leeway_time"`
	Daemon                      ChainDaemon `mapstructure:"daemon"`
	Fund                        ChainFund   `mapstructure:"fund"`
}

type ChainFund struct {
	Mainnet ChainNet `mapstructure:"mainnet"`
	Testnet ChainNet `mapstructure:"testnet"`
}

type ChainDaemon struct {
	Mainnet ChainNet `mapstructure:"mainnet"`
	Testnet ChainNet `mapstructure:"testnet"`
}

type ChainNet struct {
	Address string `mapstructure:"address"`
	Token   string `mapstructure:"token"`
	Asset   int64  `mapstructure:"asset"`
}

type Chain struct {
	Supported          []string `mapstructure:"supported"`
	BlockTimerInterval string   `mapstructure:"block_timer_interval"`
	BlockTTL           string   `mapstructure:"block_ttl"`
	PricingApi         string   `mapstructure:"pricing_api"`
}

type DB struct {
	Host     string `mapstructure:"host"`
	Keyspace string `mapstructure:"keyspace"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type Discord struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURI  string `mapstructure:"redirect_uri"`
	BotToken     string `mapstructure:"bot_token"`
	BotServerID  string `mapstructure:"bot_server_id"`
}

type Email struct {
	Username     string      `mapstructure:"username"`
	Password     string      `mapstructure:"password"`
	Region       string      `mapstructure:"region"`
	Host         string      `mapstructure:"host"`
	Port         int64       `mapstructure:"port"`
	Notification EmailFormat `mapstructure:"notification"`
	Verify       EmailFormat `mapstructure:"verify"`
}

type EmailFormat struct {
	From       string `mapstructure:"from"`
	SenderName string `mapstructure:"sender_name"`
}

type Logo struct {
	MaxSize        int      `mapstructure:"maxSize"`
	MaxX           int      `mapstructure:"maxX"`
	MaxY           int      `mapstructure:"maxY"`
	SupportedTypes []string `mapstructure:"supported_types"`
}

type Migration struct {
	Allowed []string `mapstructure:"allowed"`
}

type Server struct {
	Debug            string   `mapstructure:"debug"`
	AcceptedVersions []string `mapstructure:"accepted_versions"`
	APIScheme        string   `mapstructure:"api_scheme"`
	Host             string   `mapstructure:"host"`
	Port             int      `mapstructure:"port"`
	APIPrefix        string   `mapstructure:"api_prefix"`
	APIVersion       string   `mapstructure:"api_version"`
	RedirectPrefix   string   `mapstructure:"redirect_prefix"`
}

type TTL struct {
	Metrics       int64  `mapstructure:"metrics"`
	VerifyToken   int64  `mapstructure:"verify_token"`
	Notifications int64  `mapstructure:"notifications"`
	FCM           int64  `mapstructure:"fcm"`
	PAToken       string `mapstructure:"pat_token"`
	UserTotalSend int64  `mapstructure:"user_total_send"`
}

type Firebase struct {
	Path string `mapstructure:"path"`
}

type Chat struct {
	Personal PersonalChat `mapstructure:"personal"`
	Group    GroupChat    `mapstructure:"group"`
}

type PersonalChat struct {
	Ttl int `mapstructure:"ttl"`
}

type GroupChat struct {
	Ttl int `mapstructure:"ttl"`
}

type Dns struct {
	Algorand AlgorandDns `mapstructure:"algorand"`
	Xrpl     XrplDns     `mapstructure:"xrpl"`
}

type AlgorandDns struct {
	Mainnet AlgorandMainnetDns `mapstructure:"mainnet"`
	Testnet AlgorandTestnetDns `mapstructure:"testnet"`
}

type AlgorandMainnetDns struct {
	Nfd Nfdomains `mapstructure:"nfd"`
}

type AlgorandTestnetDns struct {
	Nfd Nfdomains `mapstructure:"nfd"`
}

type Nfdomains struct {
	Url  string `mapstructure:"url"`
	Path string `mapstructure:"path"`
}

type XrplDns struct {
	Mainnet XrplMainnetDns `mapstructure:"mainnet"`
	Testnet XrplTestnetDns `mapstructure:"testnet"`
}

type XrplMainnetDns struct {
	Xrpns XrpnsDomains `mapstructure:"xrpns"`
}

type XrplTestnetDns struct {
	Xrpns XrpnsDomains `mapstructure:"xrpns"`
}

type XrpnsDomains struct {
	Url   string `mapstructure:"url"`
	Path  string `mapstructure:"path"`
	Token string `mapstructure:"token"`
}
