package middlewares

import (
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"

	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/utilities"

	"github.com/gin-gonic/gin"
)

// VerifyUserOnboarded checks if a user is onboarded and
// sets a flag in the context accordingly
func (m *Middlewares) VerifyUserOnboarded(ctx *gin.Context) {
	log := utilities.NewLogger("VerifyUserOnboarded")

	// Get the user's address from the context
	userAddress, exist := ctx.Get(consts.UserAddress)
	if !exist {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Message:    "user address is missing",
		})
		return
	}

	// Get the user's chain from the context
	userChain, exist := ctx.Get(consts.UserChain)
	if !exist {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Message:    "user chain is missing",
		})
		return
	}

	log = log.WithFields(logrus.Fields{
		"address": userAddress,
		"chain":   userChain,
	})

	// Check if the user is onboarded
	err := m.useCases.VerifyUserOnboarded(ctx, cast.ToString(userAddress), cast.ToString(userChain))
	if err != nil {
		log.WithError(err).Error("user onboarding failed")
		ctx.Set(consts.UserOnboarded, false)
		ctx.AbortWithStatusJSON(http.StatusNotFound, entities.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "user is not onboarded",
		})
		return
	}

	log.Debugf("User '%s' onboard validation done", userAddress)

	// Set the user onboarded flag in the context
	ctx.Set(consts.UserOnboarded, true)
	ctx.Next()
}
