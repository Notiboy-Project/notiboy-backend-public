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

type BillingController struct {
	router      *gin.RouterGroup
	useCases    usecases.BillingUsecasesImply
	middleWares *middlewares.Middlewares
}

// NewBillingController
func NewBillingController(router *gin.RouterGroup, notificationUseCases usecases.BillingUsecasesImply, middleWare *middlewares.Middlewares) *BillingController {
	return &BillingController{
		router:      router,
		useCases:    notificationUseCases,
		middleWares: middleWare,
	}
}

// InitRoutes
func (n *BillingController) InitRoutes() {
	v1 := n.router.Group(config.GetConfig().Server.APIVersion)

	verifyToken := v1.Group("", n.middleWares.ValidateToken)
	onboarded := verifyToken.Group("", n.middleWares.VerifyUserOnboarded)
	{
		onboarded.POST("/chains/:chain/users/:address/billing/fund", n.AddFund)
		onboarded.PUT("/chains/:chain/users/:address/billing/membership", n.ChangeMembership)
		onboarded.GET("/chains/:chain/users/:address/billing", n.GetBillingDetails)
		onboarded.GET("/billing", n.GetMembershipTiers)
	}

	admin := onboarded.Group("", n.middleWares.IsAdminUser)
	{
		admin.PUT("/admin/chains/:chain/users/:address/billing/membership", n.AdminChangeMembership)
	}
}

func (n *BillingController) AddFund(ctx *gin.Context) {
	log := utilities.NewLogger("AddFund")

	var req entities.BillingRequest
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      err.Error(),
			Message:    "failed to parse request body",
		})
		return
	}

	log.Info("Received request with payload:", req)

	req.Chain = ctx.Param("chain")
	req.Address = ctx.Param("address")

	if req.Chain == "" || req.Address == "" || (req.SignedTxn == "" && req.TxnID == "") {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "chain, address and signed txn/txn ID are required",
			Message:    "Please provide chain, address and signed txn/txn ID",
		})
		return
	}

	err := n.useCases.AddFund(ctx, req)
	if err != nil {
		log.WithError(err).Error("adding fund failed")
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      err.Error(),
			Message:    "Adding fund failed",
		})
		return
	}
	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: 200,
		Message:    "Fund added successfully.",
	})
}

func (n *BillingController) AdminChangeMembership(ctx *gin.Context) {
	log := utilities.NewLogger("AdminChangeMembership")

	var req entities.BillingRequest
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      err.Error(),
			Message:    "failed to parse request body",
		})
		return
	}

	log.Info("Received request with payload:", req)

	req.Chain = ctx.Param("chain")
	req.Address = ctx.Param("address")

	if req.Chain == "" || req.Address == "" || req.Membership == "" || req.Days == 0 {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "chain, address, days and membership are required",
			Message:    "Please provide chain, address, days and membership",
		})
		return
	}

	err := n.useCases.AdminChangeMembership(ctx, req)
	if err != nil {
		log.WithError(err).Error("membership change failed")
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      err.Error(),
			Message:    "Membership change failed",
		})
		return
	}
	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: 200,
		Message:    "Membership changed successfully by admin.",
	})
}

func (n *BillingController) ChangeMembership(ctx *gin.Context) {
	log := utilities.NewLogger("ChangeMembership")

	var req entities.BillingRequest
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      err.Error(),
			Message:    "failed to parse request body",
		})
		return
	}

	log.Info("Received request with payload:", req)

	req.Chain = ctx.Param("chain")
	req.Address = ctx.Param("address")

	if req.Chain == "" || req.Address == "" || req.Membership == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "chain, address and membership are required",
			Message:    "Please provide chain, address and membership",
		})
		return
	}

	err := n.useCases.ChangeMembership(ctx, req)
	if err != nil {
		log.WithError(err).Error("membership change failed")
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      err.Error(),
			Message:    "Membership change failed",
		})
		return
	}
	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: 200,
		Message:    "Membership changed successfully.",
	})
}

func (n *BillingController) GetBillingDetails(ctx *gin.Context) {
	log := utilities.NewLogger("GetBillingDetails")

	var req entities.BillingRequest

	log.Info("Received request with payload:", req)

	req.Chain = ctx.Param("chain")
	req.Address = ctx.Param("address")

	if req.Chain == "" || req.Address == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "chain, address are required",
			Message:    "Please provide chain, address",
		})
		return
	}

	data, err := n.useCases.GetBillingDetails(ctx, req)
	if err != nil {
		log.WithError(err).Error("get billing failed")
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      err.Error(),
			Message:    "Get billing failed",
		})
		return
	}

	resp := entities.Response{
		StatusCode: 200,
		Message:    "Billing info retrieved successfully",
		Data:       data,
	}

	ctx.JSON(http.StatusOK, resp)
}

func (n *BillingController) GetMembershipTiers(ctx *gin.Context) {
	log := utilities.NewLogger("GetMembershipTiers")

	data, err := n.useCases.GetMemershipTiers(ctx)
	if err != nil {
		log.WithError(err).Error("get membership tiers failed")
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      err.Error(),
			Message:    "Get membership tiers failed",
		})
		return
	}

	resp := entities.Response{
		StatusCode: 200,
		Message:    "Membership tiers retrieved successfully",
		Data:       data,
	}

	ctx.JSON(http.StatusOK, resp)
}
