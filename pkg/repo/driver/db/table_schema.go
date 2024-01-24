package db

import "notiboy/pkg/consts"

var dbTableSchemas = map[string]string{
	consts.ChannelInfo:                         channelInfoSchema,
	consts.VerifiedChannelInfo:                 verifiedChannelInfoSchema,
	consts.UnverifiedChannelInfo:               unverifiedChannelInfoSchema,
	consts.ChannelName:                         channelNameSchema,
	consts.ChannelActivityMetrics:              channeActivityMetricsSchema,
	consts.ChannelTractionMetrics:              channelTractionMetricsSchema,
	consts.ChannelUsers:                        channelUsersMetricsSchema,
	consts.NotificationInfo:                    notificationInfoSchema,
	consts.ScheduledNotificationInfo:           scheduledNotificationInfoSchema,
	consts.UserActivityMetrics:                 userActivityMetricsSchema,
	consts.UserInfo:                            userInfoSchema,
	consts.VerifyInfo:                          verifyInfoSchema,
	consts.GlobalStatistics:                    globalStatsSchema,
	consts.LoginInfo:                           loginInfoSchema,
	consts.PATInfo:                             patSchema,
	consts.NotificationChannelCounter:          notificationChannelCounterSchema,
	consts.NotificationTotalSendPerUserMetrics: notificationTotalSendPerUserMetricsSchema,
	consts.NotificationChannelMetrics:          notificationChannelMetricsSchema,
	consts.ChannelSentReadMetrics:              channelSentReadMetricsSchema,
	consts.NotificationTotalSent:               notificationTotalSentSchema,
	consts.NotificationDiscordMediumReach:      notificationDiscordMediumReachSchema,
	consts.NotificationEmailMediumReach:        notificationEmailMediumReachSchema,
	consts.NotificationAppMediumReach:          notificationAppMediumReachSchema,
	consts.NotificationReadStatus:              notificationReadStatusSchema,
	consts.BillingHistoryTable:                 billingHistorySchema,
	consts.BillingTable:                        billingSchema,
	consts.FcmTable:                            fcmSchema,
	consts.ChatUserTable:                       userChatSchema,
	consts.ChatUserBlockTable:                  userChatBlockSchema,
	consts.ChatUserContactsTable:               userChatContactsSchema,
	consts.ChatGroupInfoTable:                  groupChatInfoSchema,
	consts.ChatGroupTable:                      groupChatSchema,
	consts.ChatUserGroupTable:                  userChatGroupSchema,
	consts.UserDNSTable:                        userDNSSchema,
}

var channeActivityMetricsSchema = `
CREATE TABLE IF NOT EXISTS  %s.channel_activity_metrics (
chain varchar,
event_date text,
event_time timestamp,
"create" int,
"delete" int,
PRIMARY KEY (chain, event_date, event_time)
) WITH CLUSTERING ORDER BY (event_date desc, event_time desc)
`

var channelInfoSchema = `
CREATE TABLE IF NOT EXISTS  %s.channel_info (
chain varchar,
app_id varchar,
created timestamp,
description varchar,
logo varchar,
name varchar,
owner varchar,
status varchar,
verified BOOLEAN,
PRIMARY KEY (chain, app_id)
)
`

var verifiedChannelInfoSchema = `
CREATE TABLE IF NOT EXISTS  %s.verified_channel_info (
chain varchar,
app_id varchar,
description varchar,
logo varchar,
name varchar,
owner varchar,
status varchar,
verified BOOLEAN,
created timestamp,
PRIMARY KEY (chain, app_id)
)
`

var unverifiedChannelInfoSchema = `
CREATE TABLE IF NOT EXISTS  %s.unverified_channel_info (
chain varchar,
app_id varchar,
description varchar,
logo varchar,
name varchar,
owner varchar,
status varchar,
verified BOOLEAN,
created timestamp,
PRIMARY KEY (chain, app_id)
)
`

var channelNameSchema = `
CREATE TABLE IF NOT EXISTS  %s.channel_name (
chain varchar,
name varchar,
app_id set<TEXT>,
PRIMARY KEY (chain, name)
)
`

var channelTractionMetricsSchema = `
CREATE TABLE IF NOT EXISTS  %s.channel_traction_metrics (
chain varchar,
channel varchar,
event_date text,
event_time timestamp,
optin int,
optout int,
PRIMARY KEY ((chain, channel), event_date, event_time)
) WITH CLUSTERING ORDER BY (event_date desc, event_time desc)
`

var channelUsersMetricsSchema = `
CREATE TABLE IF NOT EXISTS  %s.channel_users (
chain varchar,
app_id varchar,
users set<TEXT>,
PRIMARY KEY (app_id, chain)
)
`

var userActivityMetricsSchema = `
CREATE TABLE IF NOT EXISTS  %s.user_activity_metrics (
chain varchar,
event_date text,
event_time timestamp,
offboard int,
onboard int,
PRIMARY KEY (chain, event_date, event_time)
) WITH CLUSTERING ORDER BY (event_date desc, event_time desc)
`

var userInfoSchema = `
CREATE TABLE IF NOT EXISTS  %s.user_info (
address varchar,
chain varchar,
allowed_mediums set<TEXT>,
channels set<TEXT>,
created timestamp,
logo text,
medium_metadata varchar,
membership varchar,
modified timestamp,
optins set<TEXT>,
status varchar,
supported_mediums set<TEXT>,
PRIMARY KEY (address, chain)
) WITH CLUSTERING ORDER BY (chain asc)
`

var notificationReadStatusSchema = `
CREATE TABLE IF NOT EXISTS  %s.notification_read_status (
address text,
chain text,
last_read timestamp,
PRIMARY KEY (address, chain)
)
`

var verifyInfoSchema = `
CREATE TABLE IF NOT EXISTS  %s.verify_info (
"token" text,
address text,
chain text,
metadata text,
expiry text,
medium text,
verified boolean,
sent boolean,
deleted boolean,
PRIMARY KEY (("token", address, chain))
)
`

var globalStatsSchema = `
CREATE TABLE IF NOT EXISTS %s.global_stats (
chain TEXT,
users_onboarded counter,
notifications_sent counter,
PRIMARY KEY (chain)
)
`

var loginInfoSchema = `
CREATE TABLE IF NOT EXISTS %s.login_info
(
address TEXT,
chain TEXT,
jwt TEXT,
"created" TIMESTAMP,
PRIMARY KEY ((address,chain,jwt))
)
`

var patSchema = `
CREATE TABLE IF NOT EXISTS %s.pa_token
(
address TEXT,
chain TEXT,
uuid TEXT,
name TEXT,
jwt TEXT,
"created" TIMESTAMP,
description TEXT,
kind TEXT,
PRIMARY KEY ((chain,address), kind, uuid)
)
`

var notificationChannelCounterSchema = `
CREATE TABLE IF NOT EXISTS %s.channel_notification_counter (
chain text,
channel text,
total_sent int,
PRIMARY KEY ((chain, channel))
)
`

var notificationChannelMetricsSchema = `
CREATE TABLE IF NOT EXISTS %s.channel_notification_metrics (
chain text,
channel text,
event_time timestamp,
event_date text,
medium text,
read int,
sent int,
PRIMARY KEY ((chain, channel), event_date, event_time)
) WITH CLUSTERING ORDER BY (event_date DESC, event_time DESC)
`

var channelSentReadMetricsSchema = `
CREATE TABLE IF NOT EXISTS %s.channel_sent_read_metrics (
chain text,
channel text,
event_time timestamp,
event_date text,
medium text,
read int,
sent int,
PRIMARY KEY ((chain, channel), event_date, event_time)
) WITH CLUSTERING ORDER BY (event_date DESC, event_time DESC)
`

// Tracks the number of notifications sent over a period of 1 month
// This will be exposed in stats page as well as used for restricting
// notifications sent in billing section
var notificationTotalSendPerUserMetricsSchema = `
CREATE TABLE IF NOT EXISTS %s.user_notification_send_metrics (
chain text,
address text,
event_time timestamp,
event_date text,
sent int,
PRIMARY KEY ((chain, address), event_date, event_time)
) WITH CLUSTERING ORDER BY (event_date desc, event_time desc)
`

var notificationInfoSchema = `
CREATE TABLE IF NOT EXISTS %s.notification_info (
chain text,
receiver text,
uuid text,
app_id text,
channel_name text,
created_time timestamp,
hash text,
link text,
medium_published text,
message text,
scheduled_time timestamp,
seen boolean,
verified boolean,
type text,
updated_time timestamp,
logo text,
PRIMARY KEY ((chain, receiver), created_time, uuid)
) WITH CLUSTERING ORDER BY (created_time DESC, uuid ASC)
`

var scheduledNotificationInfoSchema = `
CREATE TABLE IF NOT EXISTS %s.scheduled_notification_info (
chain text,
sender text,
receivers set<text>,
message text,
link text,
app_id text,
type text,
schedule timestamp,
PRIMARY KEY (chain, schedule, sender)
) WITH CLUSTERING ORDER BY (schedule DESC, sender DESC)
`

var notificationTotalSentSchema = `
CREATE TABLE IF NOT EXISTS %s.notification_total_sent (
hash text,
sent int,
event_time timestamp,
PRIMARY KEY (hash, event_time)
)  WITH CLUSTERING ORDER BY (event_time DESC)
`

var notificationEmailMediumReachSchema = `
CREATE TABLE IF NOT EXISTS %s.notification_email_reach (
hash text,
read int,
event_time timestamp,
PRIMARY KEY (hash, event_time)
)  WITH CLUSTERING ORDER BY (event_time DESC)
`

var notificationAppMediumReachSchema = `
CREATE TABLE IF NOT EXISTS %s.notification_app_reach (
hash text,
read int,
event_time timestamp,
PRIMARY KEY (hash, event_time)
)  WITH CLUSTERING ORDER BY (event_time DESC)
`

var notificationDiscordMediumReachSchema = `
CREATE TABLE IF NOT EXISTS %s.notification_discord_reach (
hash text,
read int,
event_time timestamp,
PRIMARY KEY (hash, event_time)
)  WITH CLUSTERING ORDER BY (event_time DESC)
`

var billingHistorySchema = `
CREATE TABLE IF NOT EXISTS %s.billing_history (
chain text,
address text,
txn_id text,
paid_time timestamp,
paid_amt double,
PRIMARY KEY ((chain, address), paid_time)
)  WITH CLUSTERING ORDER BY (paid_time DESC)
`

var billingSchema = `
CREATE TABLE IF NOT EXISTS %s.billing (
chain text,
address text,
membership text,
expiry timestamp,
updated timestamp,
balance double,
charge int,
PRIMARY KEY (chain, address)
)  WITH CLUSTERING ORDER BY (address DESC)
`

var fcmSchema = `
CREATE TABLE IF NOT EXISTS %s.fcm (
chain text,
address text,
device_id text,
updated timestamp,
PRIMARY KEY ((chain, address), device_id)
)
`

var userChatSchema = `
CREATE TABLE IF NOT EXISTS %s.user_chat (
chain text,
user_a text,
user_b text,
sender text,
message text,
uuid text,
status text,
sent_time bigint,
PRIMARY KEY ((chain, user_a), user_b, sent_time)
) WITH CLUSTERING ORDER BY (user_b desc, sent_time desc)
`

var userChatBlockSchema = `
CREATE TABLE IF NOT EXISTS %s.user_chat_block (
chain text,
user text,
blocked_users set<text>,
PRIMARY KEY ((chain, user))
)
`

var userChatContactsSchema = `
CREATE TABLE IF NOT EXISTS %s.user_chat_contacts (
chain text,
user text,
contacts set<text>,
PRIMARY KEY ((chain, user))
)
`

var groupChatInfoSchema = `
CREATE TABLE IF NOT EXISTS %s.group_chat_info (
chain text,
gid text,
name text,
description text,
owner text,
admins set<text>,
users set<text>,
created_at timestamp,
blocked_users set<text>,
PRIMARY KEY ((chain, gid), created_at)
)
WITH CLUSTERING ORDER BY (created_at desc)
`
var groupChatSchema = `
CREATE TABLE IF NOT EXISTS %s.group_chat (
chain text,
gid text,
sender text,
message text,
uuid text,
status text,
sent_time bigint,
PRIMARY KEY ((chain, gid), sent_time)
)
WITH CLUSTERING ORDER BY (sent_time desc)
`

var userChatGroupSchema = `
CREATE TABLE IF NOT EXISTS %s.user_chat_groups (
chain text,
user text,
gids set<text>,
PRIMARY KEY ((chain, user))
)
`

var userDNSSchema = `
CREATE TABLE IF NOT EXISTS %s.user_dns (
chain text,
user text,
dns text,
PRIMARY KEY (chain, user)
)
`
