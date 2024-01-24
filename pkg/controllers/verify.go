package controllers

import (
	"fmt"
	"net/http"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/pkg/middlewares"
	"notiboy/pkg/usecases"
	"notiboy/utilities"

	"github.com/gin-gonic/gin"
)

type VerifyController struct {
	router      *gin.RouterGroup
	useCases    usecases.VerifyUseCaseImply
	middleWares *middlewares.Middlewares
}

// NewController
func NewVerifyController(router *gin.RouterGroup, useCases usecases.VerifyUseCaseImply, middleWare *middlewares.Middlewares) *VerifyController {
	return &VerifyController{
		router:      router,
		useCases:    useCases,
		middleWares: middleWare,
	}
}

// InitRoutes initializes the routes for the VerifyController.
func (verify *VerifyController) InitRoutes() {

	v1 := verify.router.Group(config.GetConfig().Server.APIVersion)
	v1.GET("chains/:chain/user/:address/verification/:token/mediums/:medium", verify.Callback)
	v1.GET("verify/discord", verify.CallbackDiscord)

	validateToken := v1.Group("", verify.middleWares.ValidateToken)
	onboarded := validateToken.Group("", verify.middleWares.VerifyUserOnboarded)
	onboarded.POST("chains/:chain/user/:address/verification/mediums/:medium", verify.Verify)
}

// Verify is an API endpoint for initiating the verification process.
func (verify *VerifyController) Verify(ctx *gin.Context) {

	var user entities.UserIdentifier

	user.Address = ctx.Param("address")
	user.Chain = ctx.GetString(consts.UserChain)
	medium := ctx.Param("medium")
	log := utilities.NewLogger("Verify")

	var mediumAddress entities.VerifyMedium
	// var response entities.Response
	if err := ctx.BindJSON(&mediumAddress); err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "verification failed",
			Message:    "binding error",
		})
		return
	}
	log.Info("Received Verify request for address:", user.Address, ", chain:", user.Chain, ", medium:", medium)

	err := verify.useCases.Verify(ctx, medium, user, mediumAddress)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "verification failed",
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: 200,
		Message:    "verify request sent successfully",
	})

}

// Callback is an API endpoint for handling the verification callback.
func (verify *VerifyController) Callback(ctx *gin.Context) {

	var user entities.UserIdentifier

	user.Address = ctx.Param("address")
	user.Chain = ctx.Param("chain")
	token := ctx.Param("token")
	medium := ctx.Param("medium")
	log := utilities.NewLogger("Callback")
	log.Info("Received Callback request for address:", user.Address, ", chain:", user.Chain, ", token:", token, ", medium:", medium)

	err := verify.useCases.Callback(ctx, user, token, medium)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      fmt.Sprintf("%s verification failed", medium),
			Message:    err.Error(),
		})
		return
	}

	ctx.Redirect(http.StatusPermanentRedirect, "/settings")
}

// CallbackDiscord is an API endpoint for handling the callback from the Discord verification process.
func (verify *VerifyController) CallbackDiscord(ctx *gin.Context) {
	var userData entities.UserIdentifier
	token := ctx.Query("code")
	userData.Address = ctx.Query("state")
	log := utilities.NewLogger("CallbackDiscord")
	log.Info("Received CallbackDiscord request with code:", token, ", state:", userData.Address)

	err := verify.useCases.Callback(ctx, userData, token, "discord")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Message:    err.Error(),
		})
		return
	}
	ctx.Redirect(http.StatusPermanentRedirect, "/settings")
}
