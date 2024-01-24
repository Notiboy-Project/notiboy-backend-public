package controllers

import (
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/pkg/middlewares"
	"notiboy/pkg/usecases"
	"notiboy/utilities"

	"github.com/gin-gonic/gin"
)

type ChannelController struct {
	router      *gin.RouterGroup
	useCases    usecases.ChannelUseCaseImply
	middleWares *middlewares.Middlewares
}

func NewChannelController(router *gin.RouterGroup, channelUseCase usecases.ChannelUseCaseImply, middleWare *middlewares.Middlewares) *ChannelController {
	return &ChannelController{
		router:      router,
		useCases:    channelUseCase,
		middleWares: middleWare,
	}
}

// InitRoutes initializes the routes for the ChannelController.
func (c *ChannelController) InitRoutes() {
	v1 := c.router.Group(config.GetConfig().Server.APIVersion)
	verifyToken := v1.Group("", c.middleWares.ValidateToken)
	onboarded := verifyToken.Group("", c.middleWares.VerifyUserOnboarded)
	{
		onboarded.POST("/chains/:chain/channels", c.CreateChannel)
		onboarded.PUT("/chains/:chain/channels/:app_id", c.ChannelUpdate)
		onboarded.DELETE("/chains/:chain/channels/:app_id", c.DeleteChannel)

		onboarded.GET("/chains/:chain/channels/:app_id/users", c.ChannelUsers)
		onboarded.GET("/chains/:chain/channels", c.ListChannels)
		onboarded.GET("/chains/:chain/channels/users/:address/owned", c.ListUserOwnedChannels)
		onboarded.GET("/chains/:chain/channels/users/:address/optins", c.ListOptedInChannels)

		onboarded.GET("/chains/:chain/stats/channels", c.ChannelStatistics)
		onboarded.GET("/chains/:chain/stats/channels/:app_id/notifications",
			c.ChannelReadSentStatistics)
		//onboarded.GET("/chains/:chain/stats/channels/:app_id/users/:address/notification",
		//	c.ChannelNotificationStatistics)
	}

	admin := onboarded.Group("", c.middleWares.IsAdminUser)
	{
		admin.PUT("/admin/chains/:chain/channels/:app_id/verify", c.VerifyChannel)
	}
}

// ChannelNotificationStatistics retrieves channel notification statistics based on the provided parameters.
func (c *ChannelController) ChannelNotificationStatistics(ctx *gin.Context) {
	chain := ctx.Param("chain")
	channel := ctx.Param("app_id")
	address := ctx.Param("address")
	typeStr := ctx.DefaultQuery("type", "all")
	startDate := ctx.DefaultQuery("start_date", "")
	endDate := ctx.DefaultQuery("end_date", utilities.DateNow())

	log := utilities.NewLogger("ChannelNotificationStatistics")

	log.Info("Hitting ChannelNotificationStatistics controller with chain:", chain, "app_id:", channel, "address:", address)

	if chain == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "parameter chain is required",
			Message:    "please provide chain parameter",
		})
		return
	}

	if channel == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "parameter app_id is required",
			Message:    "Please provide app_id parameter",
		})
		return
	}

	if address == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "parameter address is required",
			Message:    "Please provide address parameter",
		})
		return
	}

	// Parse pagination parameters from query string
	limitStr := ctx.DefaultQuery("limit", "10")
	offsetStr := ctx.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to parse limit parameter",
			Message:    err.Error(),
		})
		return
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to parse offset parameter",
			Message:    err.Error(),
		})
		return
	}

	// Get channel users from use cases with request object and pagination parameters
	tractions, err := c.useCases.ChannelNotificationStatistics(ctx, ctx.Request, chain, channel, address, typeStr, startDate, endDate, limit, offset)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to list channel tractions",
			Message:    err.Error(),
		})
		return
	}

	metaData := entities.MetaData{
		Total:       tractions.TotalCount,
		PerPage:     limit,
		CurrentPage: tractions.CurrentPage,
	}

	resp := entities.Response{
		StatusCode: 200,
		Message:    "channel notification stats retrieved successfully",
		MetaData:   &metaData,
		Data: gin.H{
			"mediums": tractions.Mediums,
			"data":    tractions.Data,
		},
	}

	ctx.JSON(http.StatusOK, resp)
}

// ChannelUsers retrieves the list of users for a specific channel based on the provided parameters.
func (c *ChannelController) ChannelUsers(ctx *gin.Context) {
	chain := ctx.Param("chain")
	appId := ctx.Param("app_id")
	address := ctx.GetString(consts.UserAddress)

	log := utilities.NewLogger("ChannelUsers")

	log.Info("Hitting ChannelUsers controller with chain:", chain, "app_id:", appId, "address:", address)

	if chain == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "parameter chain is required",
			Message:    "please provide chain parameter",
		})
		return
	}

	if appId == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "parameter app_id is required",
			Message:    "please provide app_id parameter",
		})
		return
	}

	//pageStr := ctx.DefaultQuery("page", "1")
	withLogoStr := ctx.DefaultQuery("logo", "false")
	addrOnlyStr := ctx.DefaultQuery("address_only", "true")

	withLogo := cast.ToBool(withLogoStr)
	addrOnly := cast.ToBool(addrOnlyStr)

	channelUsersRequest := &entities.ListChannelUsersRequest{
		Pagination:  entities.Pagination{},
		AppId:       appId,
		Chain:       chain,
		Address:     address,
		WithLogo:    withLogo,
		AddressOnly: addrOnly,
	}

	// Get channel users from use cases with request object and pagination parameters
	response, err := c.useCases.ChannelUsers(ctx, ctx.Request, channelUsersRequest)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to list channel users",
			Message:    err.Error(),
		})
		return
	}

	resp := entities.Response{
		StatusCode: 200,
		Message:    "channel users retrieved successfully",
		Data:       response.Data,
	}

	ctx.JSON(http.StatusOK, resp)

}

// CreateChannel creates a new channel based on the provided channel information.
func (c *ChannelController) CreateChannel(ctx *gin.Context) {

	log := utilities.NewLogger("CreateChannel")
	log.Info("Received CreateChannel request with payload:")

	var req entities.ChannelInfo
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "error",
			Message:    "please provide proper paylod",
		})
		return
	}

	chain := ctx.Param("chain")
	if chain == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "chain is required",
			Message:    "please provide chain parameter",
		})
		return
	}
	req.Address = ctx.GetString(consts.UserAddress)

	res, err := c.useCases.ChannelCreate(ctx, req, chain)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to create channel",
			Message:    err.Error(),
		})
		return
	}
	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: 201,
		Message:    "Channel Created successfully.",
		Data:       res.Data,
	})

}

// ChannelUpdate updates an existing channel with the provided channel information.
func (c *ChannelController) ChannelUpdate(ctx *gin.Context) {
	log := utilities.NewLogger("ChannelUpdate")

	sender, _ := ctx.Get(consts.UserAddress)

	var req entities.ChannelInfo
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "error",
			Message:    err.Error(),
		})
		return
	}

	log.Info("Received ChannelUpdate request with payload:", req)

	req.Chain = ctx.Param("chain")
	req.AppID = ctx.Param("app_id")
	req.Address, _ = sender.(string)

	if req.Chain == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "chain is required",
			Message:    "please provide chain parameter",
		})
		return
	}
	if req.AppID == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "app_id is required",
			Message:    "please provide app_id parameter",
		})
		return
	}

	err := c.useCases.ChannelUpdate(ctx, req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to update channel",
			Message:    err.Error(),
		})
		return
	}
	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: 200,
		Message:    "Channel updated successfully.",
	})
}

// ListOptedInChannels retrieves a list of channels based on the provided chain parameter.
func (c *ChannelController) ListOptedInChannels(ctx *gin.Context) {
	chain := ctx.Param("chain")
	address := ctx.GetString(consts.UserAddress)

	log := utilities.NewLogger("ListOptedInChannels").WithFields(
		logrus.Fields{
			"chain":   chain,
			"address": address,
		})

	if chain == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "parameter chain is required",
			Message:    "please provide chain parameter",
		})
		return
	}

	withLogoStr := ctx.DefaultQuery("logo", "false")
	withLogo := cast.ToBool(withLogoStr)

	channelData, err := c.useCases.ListOptedInChannels(ctx, chain, address, withLogo)
	if err != nil {
		log.WithError(err).Error("listing channels failed")
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to list channels",
			Message:    err.Error(),
		})
		return
	}

	resp := entities.Response{
		StatusCode: 200,
		Message:    "Channels listed successfully",
		Data:       channelData.Data,
	}

	ctx.JSON(http.StatusOK, resp)

}

// ListChannels retrieves a list of channels based on the provided chain parameter.
func (c *ChannelController) ListChannels(ctx *gin.Context) {

	chain := ctx.Param("chain")
	address := ctx.GetString(consts.UserAddress)

	log := utilities.NewLogger("ListChannels").WithFields(
		logrus.Fields{
			"chain":   chain,
			"address": address,
		})

	if chain == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "parameter chain is required",
			Message:    "please provide chain parameter",
		})
		return
	}

	pageSize, pageState := ctx.DefaultQuery("page_size", consts.DefaultPageSize), ctx.Query("page_state")
	withLogoStr := ctx.DefaultQuery("logo", "false")
	verifiedStr := ctx.DefaultQuery("verified", "true")
	nameSearchStr := ctx.DefaultQuery("name", "")

	currPageState, err := base64.URLEncoding.DecodeString(pageState)
	if err != nil {
		log.WithError(err).Errorf("incorrect page state %s", pageState)
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "incorrect page state",
			Message:    err.Error(),
		})
		return
	}

	numPageSize, err := strconv.Atoi(pageSize)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "incorrect page size",
			Message:    err.Error(),
		})
		return
	}

	withLogo := cast.ToBool(withLogoStr)
	verified := cast.ToBool(verifiedStr)

	req := &entities.ListChannelRequest{
		Pagination: entities.Pagination{
			PageSize:  numPageSize,
			NextToken: currPageState,
		},
		Chain:      chain,
		WithLogo:   withLogo,
		NameSearch: nameSearchStr,
		Verified:   verified,
	}

	channelData, err := c.useCases.ListChannels(ctx, req)
	if err != nil {
		log.WithError(err).Error("listing channels failed")
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to list channels",
			Message:    err.Error(),
		})
		return
	}

	resp := entities.Response{
		StatusCode:         200,
		Message:            "Channels listed successfully",
		Data:               channelData.Data,
		PaginationMetaData: channelData.PaginationMetaData,
	}
	resp.PaginationMetaData.Prev = pageState

	ctx.JSON(http.StatusOK, resp)

}

// ListUserOwnedChannels retrieves a list of channels associated with a specific user based on the provided chain parameter and user address.
func (c *ChannelController) ListUserOwnedChannels(ctx *gin.Context) {
	chain := ctx.Param("chain")
	address := ctx.GetString(consts.UserAddress)

	log := utilities.NewLogger("ListUserOwnedChannels").WithFields(
		logrus.Fields{
			"chain":   chain,
			"address": address,
		})

	if chain == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "parameter chain is required",
			Message:    "please provide chain parameter",
		})
		return
	}

	if address == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "address is missing",
			Message:    "please provide address",
		})
		return
	}

	withLogoStr := ctx.DefaultQuery("logo", "false")

	withLogo := cast.ToBool(withLogoStr)

	channelData, err := c.useCases.ListUserOwnedChannels(ctx, ctx.Request, chain, address, withLogo)
	if err != nil {
		log.WithError(err).Error("listing channels failed")
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to list channels",
			Message:    err.Error(),
		})
		return
	}

	resp := entities.Response{
		StatusCode: 200,
		Message:    "Channels listed successfully",
		Data:       channelData.Data,
	}

	ctx.JSON(http.StatusOK, resp)
}

// DeleteChannel deletes a channel based on the provided chain, app_id, and user address parameters.
func (c *ChannelController) DeleteChannel(ctx *gin.Context) {

	log := utilities.NewLogger("DeleteChannel")

	chain := ctx.Param("chain")
	if chain == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "parameter chain is required",
			Message:    "please provide chain parameter",
		})
		return
	}

	appID := ctx.Param("app_id")
	if appID == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "parameter app_id is required",
			Message:    "please provide app_id parameter",
		})
		return
	}

	log.Info("Received DeleteChannel request for chain:", chain, "and app_id:", appID)

	address := ctx.GetString(consts.UserAddress)
	err := c.useCases.DeleteChannel(ctx, chain, appID, address)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to delete channel",
			Message:    err.Error(),
		})
		return
	}
	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: 200,
		Message:    "Channel deleted successfully.",
	})
}

func (c *ChannelController) VerifyChannel(ctx *gin.Context) {

	log := utilities.NewLogger("VerifyChannel")

	chain := ctx.Param("chain")
	if chain == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "parameter chain is required",
			Message:    "please provide chain parameter",
		})
		return
	}

	appID := ctx.Param("app_id")
	if appID == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "parameter app_id is required",
			Message:    "please provide app_id parameter",
		})
		return
	}

	log.Info("Received VerifyChannel request for chain:", chain, "and app_id:", appID)

	err := c.useCases.VerifyChannel(ctx, chain, appID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to verify channel",
			Message:    err.Error(),
		})
		return
	}
	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: 200,
		Message:    "Channel verified successfully.",
	})
}

// ChannelStatistics retrieves statistics for a channel based on the provided chain parameter.
func (c *ChannelController) ChannelStatistics(ctx *gin.Context) {

	chain := ctx.Param("chain")
	log := utilities.NewLogger("ChannelStatistics")

	if chain == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "parameter chain is required",
			Message:    "please provide chain parameter",
		})
		return
	}

	typeStr := ctx.DefaultQuery("type", "all")
	startDate := ctx.DefaultQuery("start_date", "")
	endDate := ctx.DefaultQuery("end_date", utilities.DateNow())
	log.Infof("Received ChannelStatistics request for chain: %s, type: %s, startDate: %s", chain, typeStr, startDate)

	data, err := c.useCases.ChannelStatistics(ctx, ctx.Request, chain, typeStr, startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to list channel users",
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, entities.ChannelStatsResponse{
		Data:       data,
		StatusCode: 200,
		Message:    "channel stats retrieved successfully",
	})
}

func (c *ChannelController) ChannelReadSentStatistics(ctx *gin.Context) {
	log := utilities.NewLogger("ChannelReadSentStatistics")

	chain := ctx.Param("chain")
	appId := ctx.Param("app_id")
	statType := ctx.DefaultQuery("type", "all")
	startDate := ctx.DefaultQuery("start_date", "")
	endDate := ctx.DefaultQuery("end_date", utilities.DateNow())

	if chain == "" || appId == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Error:      "failed to fetch channel notification traction statistics",
			Message:    "missing or invalid parameters",
		})
		return
	}
	log.Info("Received ChannelReadSentStatistics request for chain:", chain, ", app_id:", appId)

	res, err := c.useCases.ChannelReadSentStatistics(ctx, ctx.Request, chain, appId, statType, startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, entities.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      "failed to fetch channel notification traction statistics",
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: http.StatusOK,
		Message:    "channel notification traction retrieved successfully",
		Data:       res,
	})
}
