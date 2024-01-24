package consts

import "strings"

type MembershipTier int

const (
	FreeTier MembershipTier = 1 + iota
	SilverTier
	GoldTier
)

var membershipTierStrToEnum = map[string]MembershipTier{
	"free":   FreeTier,
	"silver": SilverTier,
	"gold":   GoldTier,
}

func (e MembershipTier) String() string {
	switch e {
	case FreeTier:
		return "free"
	case SilverTier:
		return "silver"
	case GoldTier:
		return "gold"
	default:
		return "free"
	}
}

func MembershipStringToEnum(tier string) MembershipTier {
	if strings.TrimSpace(tier) == "" {
		return FreeTier
	}
	return membershipTierStrToEnum[strings.ToLower(tier)]
}
