package cache

func Init() {
	_ = GetChannelNameCache()
	_ = GetChannelVerifyCache()
	_ = InitBlockedUserCache()
	_ = InitUserContactsCache()
	_ = InitUserGroupsCache()
}
