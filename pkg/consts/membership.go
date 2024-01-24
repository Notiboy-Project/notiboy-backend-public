package consts

var MembershipCharge = map[MembershipTier]int{
	GoldTier:   29,
	SilverTier: 9,
	FreeTier:   0,
}

var NotificationCharacterCount = map[MembershipTier]int{
	GoldTier:   500,
	SilverTier: 200,
	FreeTier:   120,
}

var NotificationCount = map[MembershipTier]int{
	GoldTier:   100000,
	SilverTier: 30000,
	FreeTier:   1000,
}

var ChannelCount = map[MembershipTier]int{
	GoldTier:   10,
	SilverTier: 3,
	FreeTier:   1,
}

var NotificationRetentionSecs = map[MembershipTier]int{
	GoldTier:   2592000,
	SilverTier: 1296000,
	FreeTier:   604800,
}

var NotificationMaxSchedule = map[MembershipTier]int{
	GoldTier:   2592000,
	SilverTier: 1296000,
	FreeTier:   604800,
}

var ChannelRename = map[MembershipTier]bool{
	GoldTier:   true,
	SilverTier: true,
	FreeTier:   false,
}

var Analytics = map[string]map[MembershipTier]bool{
	OPTIN_OPTOUT_STATS: {
		GoldTier:   true,
		SilverTier: true,
		FreeTier:   true,
	},
	CHANNEL_READ_SENT_STATS: {
		GoldTier:   true,
		SilverTier: true,
		FreeTier:   true,
	},
}
