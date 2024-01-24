package consts

const (
	AppName = "notiboy"
)

const (
	UserAddress   = "USER_ADDRESS"
	UserToken     = "USER_TOKEN"
	AdminUser     = "ADMIN_USER"
	UserChain     = "USER_CHAIN"
	UserOnboarded = "USER_ON_BOARDED"
)

const (
	STATUS_CHANNEL_LIMIT_EXCEEDED = "CHANNEL_LIMIT_EXCEEDED"
	STATUS_ACTIVE                 = "ACTIVE"
	STATUS_CHANNEL_ORPHANED       = "CHANNEL_ORPHANED"
)

const (
	OPTIN_OPTOUT_STATS      = "optin_optout_analytics"
	CHANNEL_READ_SENT_STATS = "channel_read_sent_analytics"
)

const (
	Inapp   = "inapp"
	Email   = "email"
	Discord = "discord"
)

const (
	DiscordGetToken       = "https://discord.com/api/oauth2/token"
	DiscordGetCurrentUser = "https://discord.com/api/v9/users/@me"
	DiscordAddMember      = "https://discord.com/api/v9/guilds"
	GetChannelIdUrl       = "https://discordapp.com/api/users/@me/channels"
)

const (
	UserInfo              = "user_info"
	ChannelInfo           = "channel_info"
	VerifiedChannelInfo   = "verified_channel_info"
	UnverifiedChannelInfo = "unverified_channel_info"
	ChannelName           = "channel_name"
	ChannelUsers          = "channel_users"

	UserNotificationChannelMetrics      = "user_notification_channel_metrics"
	NotificationChannelMetrics          = "channel_notification_metrics"
	ChannelSentReadMetrics              = "channel_sent_read_metrics"
	NotificationTotalSendPerUserMetrics = "user_notification_send_metrics"
	ChannelActivityMetrics              = "channel_activity_metrics"
	ChannelTractionMetrics              = "channel_traction_metrics"
	UserActivityMetrics                 = "user_activity_metrics"
	GlobalStatistics                    = "global_stats"
	NotificationTotalSent               = "notification_total_sent"
	NotificationChannelCounter          = "channel_notification_counter"
	NotificationEmailMediumReach        = "notification_email_reach"
	NotificationAppMediumReach          = "notification_app_reach"
	NotificationDiscordMediumReach      = "notification_discord_reach"

	VerifyInfo = "verify_info"
	LoginInfo  = "login_info"
	PATInfo    = "pa_token"

	NotificationInfo          = "notification_info"
	ScheduledNotificationInfo = "scheduled_notification_info"
	NotificationReadStatus    = "notification_read_status"

	BillingHistoryTable = "billing_history"
	BillingTable        = "billing"

	FcmTable = "fcm"

	ChatUserTable         = "user_chat"
	ChatUserBlockTable    = "user_chat_block"
	ChatUserContactsTable = "user_chat_contacts"
	ChatGroupTable        = "group_chat"
	ChatGroupInfoTable    = "group_chat_info"
	ChatUserGroupTable    = "user_chat_groups"

	UserDNSTable = "user_dns"
)

// DB
const (
	LoginTable = "login_info"
	UserTable  = "user_info"
)
const (
	Algorand = "algorand"
	Xrpl     = "xrpl"

	XummWallet = "xumm"
)

const (
	DefaultPageSize = "100"
)
