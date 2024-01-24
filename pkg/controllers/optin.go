package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"notiboy/config"
	"notiboy/pkg/entities"
	"notiboy/pkg/middlewares"
	"notiboy/pkg/usecases"
	"notiboy/utilities"
)

type OptinController struct {
	router      *gin.RouterGroup
	useCases    usecases.OptinUseCaseImply
	middleWares *middlewares.Middlewares
}

// NewOptinController
func NewOptinController(router *gin.RouterGroup, OptinUseCase usecases.OptinUseCaseImply, middleWare *middlewares.Middlewares) *OptinController {
	return &OptinController{
		router:      router,
		useCases:    OptinUseCase,
		middleWares: middleWare,
	}
}

// InitRoutes initializes the routes for the OptinController.
func (Optin *OptinController) InitRoutes() {
	v1 := Optin.router.Group(config.GetConfig().Server.APIVersion)

	validateToken := v1.Group("", Optin.middleWares.ValidateToken)

	onboarded := validateToken.Group("", Optin.middleWares.VerifyUserOnboarded)
	{
		onboarded.POST("chains/:chain/channels/:app_id/users/:address/optin", Optin.Optin)
		onboarded.DELETE("chains/:chain/channels/:app_id/users/:address/optout", Optin.Optout)
		onboarded.GET("chains/:chain/stats/channels/:app_id/optinout", Optin.OptinoutStatistics)
	}
}

// Optin is an API endpoint for channel opt-in.
func (Optin *OptinController) Optin(ctx *gin.Context) {

	chain := ctx.Param("chain")
	appId := ctx.Param("app_id")
	userAddr := ctx.Param("address")
	log := utilities.NewLogger("Optin")

	if chain == "" || appId == "" || userAddr == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "optin failed",
			Message:    "missing or invalid parameters",
		})
		return
	}
	log.Info("Received Optin request for chain:", chain, ", app_id:", appId, ", address:", userAddr)

	err := Optin.useCases.Optin(ctx, chain, appId, userAddr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "optin failed",
			Message:    err.Error(),
		})

		return
	}

	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: 200,
		Message:    "channel optin successfully",
	})
}

// Optout is an API endpoint for channel opt-out.
func (Optin *OptinController) Optout(ctx *gin.Context) {

	chain := ctx.Param("chain")
	appId := ctx.Param("app_id")
	userAddr := ctx.Param("address")
	log := utilities.NewLogger("Optout")

	if chain == "" || appId == "" || userAddr == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "optout failed",
			Message:    "missing or invalid parameters",
		})
		return
	}
	log.Info("Received Optout request for chain:", chain, ", app_id:", appId, ", address:", userAddr)

	err := Optin.useCases.Optout(ctx, chain, appId, userAddr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "optout failed",
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: 200,
		Message:    "channel optout successfully",
	})
}

// OptinoutStatistics is an API endpoint for retrieving opt-in and opt-out statistics of a channel.
func (Optin *OptinController) OptinoutStatistics(ctx *gin.Context) {
	chain := ctx.Param("chain")
	appId := ctx.Param("app_id")
	statType := ctx.DefaultQuery("type", "all")
	startDate := ctx.DefaultQuery("start_date", "")
	endDate := ctx.DefaultQuery("end_date", utilities.DateNow())
	log := utilities.NewLogger("OptinoutStatistics")

	if chain == "" || appId == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to fetch optinout statistics",
			Message:    "missing or invalid parameters",
		})
		return
	}
	log.Info("Received OptinoutStatistics request for chain:", chain, ", app_id:", appId)

	res, err := Optin.useCases.OptinoutStatistics(ctx, chain, appId, statType, startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to fetch optinout statistics",
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: 200,
		Message:    "channel optin optout stats retrieved successfully",
		Data:       res,
	})
}
