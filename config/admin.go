package config

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

var adminUsers map[string]struct{}

func loadAdminUsers() {
	adminUsers = make(map[string]struct{})
	for _, user := range GetConfig().AdminUsers {
		if len(strings.Split(user, ":")) != 2 {
			logrus.Fatalf("admin_users value %s has invalid format", user)
		}
		adminUsers[user] = struct{}{}
	}
}

// IsAdminUser checks if the user in a given chain is admin
func IsAdminUser(chain, user string) bool {
	_, present := adminUsers[fmt.Sprintf("%s:%s", chain, user)]
	return present
}
