package controllers

import (
	"net/http"
	"strings"

	"notiboy/utilities"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/pkg/middlewares"
	"notiboy/pkg/usecases"

	"github.com/gin-gonic/gin"
)

type UserController struct {
	router      *gin.RouterGroup
	useCases    usecases.UserUseCaseImply
	middleWares *middlewares.Middlewares
}

// NewUserController
func NewUserController(router *gin.RouterGroup, userUseCase usecases.UserUseCaseImply, middleWare *middlewares.Middlewares) *UserController {
	return &UserController{
		router:      router,
		useCases:    userUseCase,
		middleWares: middleWare,
	}
}

// InitRoutes initializes the routes for the UserController.
func (user *UserController) InitRoutes() {
	v1 := user.router.Group(config.GetConfig().Server.APIVersion)
	v1.GET("/stats/global", user.GlobalSatistics)

	verifyAddress := v1.Group("", user.middleWares.ValidateUserAddress)
	{
		verifyAddress.POST("/chains/:chain/users/:address", user.Onboarding)

		if config.GetConfig().AutoOnboardUsers {
			verifyAddress.POST("/chains/:chain/users/:address/login", user.Login)
		} else {
			onboarded := verifyAddress.Group("", user.middleWares.VerifyUserOnboarded)
			onboarded.POST("/chains/:chain/users/:address/login", user.Login)
		}
	}

	validToken := v1.Group("", user.middleWares.ValidateToken)
	validTokenUser := validToken.Group("", user.middleWares.VerifyUserOnboarded)
	{
		validTokenUser.DELETE("/chains/:chain/users/:address/logout", user.Logout)
		validTokenUser.GET("/chains/:chain/users/:address", user.GetUser)
		validTokenUser.PUT("/chains/:chain/users/:address", user.ProfileUpdate)
		validTokenUser.DELETE("/chains/:chain/users/:address", user.Offboarding)
		validTokenUser.GET("/chains/:chain/stats/users", user.UserStatistics)
		validTokenUser.POST("/chains/:chain/users/:address/pat/kind/:kind/:name", user.GeneratePAT)
		validTokenUser.GET("/chains/:chain/users/:address/pat/kind/:kind", user.GetPAT)
		validTokenUser.DELETE("/chains/:chain/users/:address/pat/kind/:kind/:uuid", user.RevokePAT)
		validTokenUser.POST("/chains/:chain/users/:address/fcm", user.StoreFCM)
	}
}

// ProfileUpdate is an API endpoint for updating a user's profile information.
func (user *UserController) ProfileUpdate(ctx *gin.Context) {
	log := utilities.NewLogger("ProfileUpdate()")

	var req entities.UserInfo

	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "profile updation failed",
			Message:    err.Error(),
		})
		return
	}

	req.Address = ctx.Param("address")
	req.Chain = ctx.Param("chain")

	if req.Address == "" || req.Chain == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "profile updation failed",
			Message:    "address and chain are required",
		})
		return
	}

	err := user.useCases.ProfileUpdate(ctx, req)
	if err != nil {
		log.WithError(err).Errorf("profile update failed for user %s", req.Address)
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "profile updation failed",
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: 200,
		Message:    "profile updated successfully.",
	})
}

// Onboarding is an API endpoint for handling user onboarding.
func (user *UserController) Onboarding(ctx *gin.Context) {
	var req entities.OnboardingRequest
	log := utilities.NewLogger("Onboarding")
	log.Info("Received Onboarding request")

	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "binding error",
			Message:    err.Error(),
		})
		return
	}

	req.Address = ctx.Param("address")
	req.Chain = ctx.Param("chain")

	if req.Address == "" || req.Chain == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "insufficient data",
			Message:    "address and chain are required",
		})
		return
	}

	onBoarded := ctx.GetBool(consts.UserOnboarded)

	if onBoarded {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status_code": 400,
			"message":     "you are already onboarded",
		})
	}

	err := user.useCases.Onboarding(ctx, req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "user onboarding failed",
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: 200,
		Message:    "profile onboarded successfully",
	})
}

// Offboarding is an API endpoint for handling user offboarding.
func (user *UserController) Offboarding(ctx *gin.Context) {

	chain := ctx.Param("chain")
	address := ctx.Param("address")
	log := utilities.NewLogger("Offboarding")

	if chain == "" || address == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "offboarding failed",
			Message:    "address and chain are required",
		})
		return
	}
	log.Info("Received Offboarding request")

	err := user.useCases.Offboarding(ctx, address, chain)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "offboarding failed",
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: 200,
		Message:    "profile offboarded successfully",
	})
}

// UserStatistics is an API endpoint for retrieving user statistics.
func (user *UserController) UserStatistics(ctx *gin.Context) {
	chain := ctx.Param("chain")
	log := utilities.NewLogger("UserStatistics")

	if chain == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to fetch user data",
			Message:    "chain is required",
		})
		return
	}
	log.Info("Received UserStatistics request")

	statType := ctx.DefaultQuery("type", "all")
	startDate := ctx.DefaultQuery("start_date", "")
	endDate := ctx.DefaultQuery("end_date", utilities.DateNow())

	data, err := user.useCases.UserStatistics(ctx, chain, statType, startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "failed to fetch user data",
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: 200,
		Message:    "user statistics retrieved successfully",
		Data:       data,
	})
}

// GlobalSatistics is an API endpoint for retrieving global user statistics.
func (user *UserController) GlobalSatistics(ctx *gin.Context) {

	log := utilities.NewLogger("GlobalSatistics")
	log.Info("Received GlobalSatistics request")

	data, err := user.useCases.GlobalStatistics(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":        data,
		"status_code": 200,
		"message":     "user statistics retrieved successfully",
	})
}

// GetUser is an API endpoint for retrieving user profile information.
func (user *UserController) GetUser(ctx *gin.Context) {
	var req entities.UserIdentifier
	log := utilities.NewLogger("GetUser")
	log.Info("Received GetUser request")

	req.Address = ctx.Param("address")
	req.Chain = ctx.Param("chain")

	if req.Address == "" || req.Chain == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "address and chain are required",
			Message:    "Incorrect request",
		})
		return
	}

	getUserResp, err := user.useCases.GetUser(ctx, req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := entities.Response{
		StatusCode: 200,
		Message:    "User profile retrieved successfully",
		Data:       getUserResp.Data,
	}

	ctx.JSON(http.StatusOK, resp)
}

// Login is an API endpoint for user login.
func (user *UserController) Login(ctx *gin.Context) {
	var req entities.UserIdentifier

	req.Address = ctx.Param("address")
	req.Chain = ctx.Param("chain")
	log := utilities.NewLogger("Login")

	if req.Address == "" || req.Chain == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "address and chain are required",
			Message:    "Incorrect request",
		})
		return
	}
	log.Info("Received Login request")

	token, err := user.useCases.Login(ctx, req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, entities.ErrorResponse{
			StatusCode: 500,
			Error:      err.Error(),
			Message:    "Login Failed",
		})
		return
	}

	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: http.StatusOK,
		Message:    "Login successful",
		Data: map[string]string{
			"token": token,
		},
	})
}

// Logout is an API endpoint for user logout.
func (user *UserController) Logout(ctx *gin.Context) {
	var req entities.UserIdentifier

	req.Address = ctx.Param("address")
	req.Chain = ctx.Param("chain")
	log := utilities.NewLogger("Logout")

	if req.Address == "" || req.Chain == "" {
		msg := "Address and chain are required"
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      msg,
			Message:    msg,
		})
		return
	}
	log.Info("Received Logout request")

	err := user.useCases.Logout(ctx, req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, entities.Response{StatusCode: 200, Message: "Logout successful"})
}

func (user *UserController) GeneratePAT(ctx *gin.Context) {
	log := utilities.NewLogger("GeneratePAT")

	var req entities.PATTokens

	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "binding error",
			Message:    err.Error(),
		})
		return
	}

	name := ctx.Param("name")
	if strings.TrimSpace(name) == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Error:      "name parameter is empty",
			Message:    "PAT Generation failed",
		})
		return
	}

	kind := ctx.Param("kind")
	if strings.TrimSpace(kind) == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Error:      "kind parameter is empty",
			Message:    "PAT Generation failed",
		})
		return
	}

	log.Info("Received GeneratePAT request")

	token, err := user.useCases.GeneratePAT(ctx, name, kind, req.Description)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, entities.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      err.Error(),
			Message:    "PAT Generation failed",
		})
		return
	}

	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: http.StatusOK,
		Message:    "PAT generated successfully",
		Data: map[string]string{
			"token": token,
			"name":  name,
		},
	})
}

func (user *UserController) GetPAT(ctx *gin.Context) {
	log := utilities.NewLogger("GetPAT")

	log.Info("Received request")

	kind := ctx.Param("kind")
	if strings.TrimSpace(kind) == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Error:      "kind parameter is empty",
			Message:    "PAT fetch failed",
		})
		return
	}

	data, err := user.useCases.GetPAT(ctx, kind)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, entities.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      err.Error(),
			Message:    "PAT retrieval failed",
		})
		return
	}

	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: http.StatusOK,
		Message:    "PAT retrieved successfully",
		Data:       data,
	})
}

func (user *UserController) RevokePAT(ctx *gin.Context) {
	log := utilities.NewLogger("RevokePAT")

	log.Info("Received request")

	kind := ctx.Param("kind")
	if strings.TrimSpace(kind) == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Error:      "kind parameter is empty",
			Message:    "PAT revocation failed",
		})
		return
	}

	err := user.useCases.RevokePAT(ctx, ctx.Param("uuid"), kind)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, entities.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      err.Error(),
			Message:    "PAT Revocation failed",
		})
		return
	}

	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: http.StatusOK,
		Message:    "PAT revoked successfully",
	})
}

func (user *UserController) StoreFCM(ctx *gin.Context) {
	log := utilities.NewLogger("StoreFCM")
	log.Info("Received request")

	var req entities.FCM

	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: 400,
			Error:      "binding error",
			Message:    err.Error(),
		})
		return
	}

	req.Address = ctx.Param("address")
	req.Chain = ctx.Param("chain")

	if strings.TrimSpace(req.DeviceID) == "" {
		ctx.JSON(http.StatusBadRequest, entities.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Error:      "device_id is empty",
			Message:    "FCM device id cannot be stored",
		})
		return
	}

	err := user.useCases.StoreFCMToken(ctx, req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, entities.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      err.Error(),
			Message:    "FCM device id storage failed",
		})
		return
	}

	ctx.JSON(http.StatusOK, entities.Response{
		StatusCode: http.StatusOK,
		Message:    "FCM device id stored successfully",
	})
}
