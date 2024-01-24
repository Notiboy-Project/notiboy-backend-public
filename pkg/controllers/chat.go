package controllers

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/pkg/middlewares"
	"notiboy/pkg/repo/driver/medium"
	"notiboy/pkg/usecases"
	"notiboy/utilities"
)

type ChatController struct {
	router      *gin.RouterGroup
	useCases    usecases.ChatUseCaseImply
	middleWares *middlewares.Middlewares
	ws          *medium.Socket
}

func NewChatController(
	router *gin.RouterGroup, chatUseCase usecases.ChatUseCaseImply, ws *medium.Socket,
	middleWare *middlewares.Middlewares,
) *ChatController {
	return &ChatController{
		router:      router,
		useCases:    chatUseCase,
		middleWares: middleWare,
		ws:          ws,
	}
}

// InitRoutes initializes the routes for the ChannelController.
func (c *ChatController) InitRoutes(ctx context.Context) {
	v1 := c.router.Group(config.GetConfig().Server.APIVersion)
	v1.GET("/ws/chat", c.WebsocketHandler)

	verifyToken := v1.Group("", c.middleWares.ValidateToken)
	onboarded := verifyToken.Group("", c.middleWares.VerifyUserOnboarded)
	{
		onboarded.GET("/chains/:chain/chat/messages", c.GetPersonalChat)
		onboarded.GET("/chains/:chain/chat/user/:user/messages", c.GetPersonalChatByUser)
		onboarded.POST("/chains/:chain/chat/:user/block", c.BlockUser)
		onboarded.GET("/chains/:chain/chat/:user/block", c.IsUserBlocked)
		onboarded.POST("/chains/:chain/chat/:user/unblock", c.UnBlockUser)
		onboarded.GET("/chains/:chain/chat/dns/contacts", c.GetDNSContacts)

		if false {
			onboarded.POST("/chains/:chain/chat/group", c.CreateGroup)
			onboarded.PUT("/chains/:chain/chat/group/:gid", c.UpdateGroupInfo)
			onboarded.DELETE("/chains/:chain/chat/group/:gid", c.UpdateGroupInfo)
			onboarded.PUT("/chains/:chain/chat/group/:gid/join", c.JoinGroup)
			onboarded.PUT("/chains/:chain/chat/group/:gid/leave", c.LeaveGroup)
			onboarded.GET("/chains/:chain/chat/group/:gid/messages", c.GetGroupChatByGroup)
			onboarded.GET("/chains/:chain/chat/group/messages", c.GetGroupChats)
		}
	}

	go c.useCases.ChatProcessor(ctx)
	go c.useCases.NotificationProcessor(ctx)
}

func (c *ChatController) GetPersonalChat(ctx *gin.Context) {
	log := utilities.NewLogger("GetPersonalChat")
	log.Info("Received GetPersonalChat request")

	chain := ctx.Param("chain")
	user := ctx.GetString(consts.UserAddress)
	if user == "" || chain == "" {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "user and chain are required",
				Message:    "please provide chain/user parameter",
			},
		)
		return
	}

	from := cast.ToInt64(ctx.Query("from"))
	to := cast.ToInt64(ctx.Query("to"))

	if from == 0 {
		// default 1 week
		from = time.Now().AddDate(0, 0, -7).Unix()
	}

	res, err := c.useCases.GetPersonalChat(ctx, chain, user, from, to)
	if err != nil {
		ctx.JSON(
			http.StatusInternalServerError, entities.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Error:      "failed to get personal chat",
				Message:    err.Error(),
			},
		)
		return
	}
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 201,
			Message:    "Personal chat retrieved successfully.",
			Data:       res,
		},
	)
}

func (c *ChatController) GetPersonalChatByUser(ctx *gin.Context) {
	log := utilities.NewLogger("GetPersonalChat")
	log.Info("Received GetPersonalChat request")

	chain := ctx.Param("chain")
	rcvr := ctx.Param("user")
	user := ctx.GetString(consts.UserAddress)
	if user == "" || chain == "" {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "user and chain are required",
				Message:    "please provide chain/user parameter",
			},
		)
		return
	}

	pageSize, pageState := ctx.DefaultQuery("page_size", consts.DefaultPageSize), ctx.Query("page_state")
	// decoding base64 encoded page state to []byte
	currPageState, err := base64.URLEncoding.DecodeString(pageState)
	if err != nil {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "incorrect page state format",
				Message:    err.Error(),
			},
		)
		return
	}

	numPageSize, err := strconv.Atoi(pageSize)
	if err != nil {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed to convert page number to an integer",
				Message:    err.Error(),
			},
		)
		return
	}

	res, nextPageState, err := c.useCases.GetPersonalChatByUser(ctx, chain, user, rcvr, numPageSize, currPageState)
	if err != nil {
		ctx.JSON(
			http.StatusInternalServerError, entities.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Error:      "failed to get personal chat by user",
				Message:    err.Error(),
			},
		)
		return
	}
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 201,
			Message:    "Personal chat by user retrieved successfully.",
			Data:       res,
			PaginationMetaData: &entities.PaginationMetaData{
				Size:     len(res),
				PageSize: numPageSize,
				Next:     base64.URLEncoding.EncodeToString(nextPageState),
				Prev:     pageState,
			},
		},
	)
}

func (c *ChatController) BlockUser(ctx *gin.Context) {
	log := utilities.NewLogger("BlockUser")
	log.Info("Received BlockUser request")

	chain := ctx.Param("chain")
	blockedUser := ctx.Param("user")
	user := ctx.GetString(consts.UserAddress)

	if blockedUser == "" || chain == "" {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "user and chain are required",
				Message:    "please provide chain/user parameter",
			},
		)
		return
	}

	res, err := c.useCases.BlockUser(ctx, chain, user, blockedUser)
	if err != nil {
		ctx.JSON(
			res.StatusCode, entities.ErrorResponse{
				StatusCode: res.StatusCode,
				Error:      "failed to block user",
				Message:    err.Error(),
			},
		)
		return
	}
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 201,
			Message:    "User blocked successfully.",
			Data:       res.Data,
		},
	)
}

func (c *ChatController) UnBlockUser(ctx *gin.Context) {
	log := utilities.NewLogger("UnBlockUser")
	log.Info("Received UnBlockUser request")

	chain := ctx.Param("chain")
	blockedUser := ctx.Param("user")
	user := ctx.GetString(consts.UserAddress)

	if blockedUser == "" || chain == "" {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "user and chain are required",
				Message:    "please provide chain/user parameter",
			},
		)
		return
	}

	res, err := c.useCases.UnblockUser(ctx, chain, user, blockedUser)
	if err != nil {
		ctx.JSON(
			res.StatusCode, entities.ErrorResponse{
				StatusCode: res.StatusCode,
				Error:      "failed to block user",
				Message:    err.Error(),
			},
		)
		return
	}
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 201,
			Message:    "User unblocked successfully.",
			Data:       res.Data,
		},
	)
}

func (c *ChatController) IsUserBlocked(ctx *gin.Context) {
	log := utilities.NewLogger("IsUserBlocked")
	log.Info("Received IsUserBlocked request")

	chain := ctx.Param("chain")
	blockedUser := ctx.Param("user")
	user := ctx.GetString(consts.UserAddress)

	if blockedUser == "" || chain == "" {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "user and chain are required",
				Message:    "please provide chain/user parameter",
			},
		)
		return
	}

	isBlocked, err := c.useCases.IsBlockedUser(ctx, chain, user, blockedUser)
	if err != nil {
		ctx.JSON(
			http.StatusInternalServerError, entities.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Error:      "failed to get user block status",
				Message:    err.Error(),
			},
		)
		return
	}
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 201,
			Message:    "User block status fetched successfully.",
			Data:       isBlocked,
		},
	)
}

func (c *ChatController) GetDNSContacts(ctx *gin.Context) {
	log := utilities.NewLogger("GetDNSContacts")
	log.Info("Received GetDNSContacts request")

	chain := ctx.Param("chain")
	user := ctx.GetString(consts.UserAddress)
	if user == "" || chain == "" {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "user and chain are required",
				Message:    "please provide chain/user parameter",
			},
		)
		return
	}

	lookupAddr := ctx.Query("lookup")

	res, err := c.useCases.GetDNSContactsList(ctx, chain, user, lookupAddr)
	if err != nil {
		ctx.JSON(
			http.StatusInternalServerError, entities.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Error:      "failed to get contacts list",
				Message:    err.Error(),
			},
		)
		return
	}
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 201,
			Message:    "Contacts list of user retrieved successfully.",
			Data:       res,
		},
	)
}

func (c *ChatController) WebsocketHandler(ctx *gin.Context) {
	chain := ctx.Query("chain")
	address := ctx.Query("address")
	token := ctx.Query("token")

	if err := c.middleWares.VerifyWebsocketRequest(ctx, chain, address, token); err != nil {
		ctx.JSON(
			http.StatusUnauthorized, entities.Response{
				StatusCode: http.StatusUnauthorized,
				Message:    err.Error(),
			},
		)
		return
	}

	upgrader := medium.Upgrade()
	wsConn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		logrus.WithError(err).Error("failed to upgrade websocket connection")
		return
	}

	c.ws.Add(chain, address, wsConn)
}

func (c *ChatController) GetGroupChats(ctx *gin.Context) {
	log := utilities.NewLogger("GetGroupChats")
	log.Info("Received GetGroupChats request")

	chain := ctx.Param("chain")
	user := ctx.GetString(consts.UserAddress)
	if user == "" || chain == "" {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "user and chain are required",
				Message:    "please provide chain/user parameter",
			},
		)
		return
	}

	from := cast.ToInt64(ctx.Query("from"))
	to := cast.ToInt64(ctx.Query("to"))
	limit := cast.ToInt(ctx.Query("limit"))

	if from == 0 {
		// default 1 week
		from = time.Now().AddDate(0, 0, -7).Unix()
	}
	if limit == 0 {
		limit = 5
	}

	res, err := c.useCases.GetGroupChats(ctx, chain, user, from, to, limit)
	if err != nil {
		ctx.JSON(
			http.StatusInternalServerError, entities.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Error:      "failed to get group chat",
				Message:    err.Error(),
			},
		)
		return
	}
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 201,
			Message:    "Group chat retrieved successfully.",
			Data:       res,
		},
	)
}

func (c *ChatController) GetGroupChatByGroup(ctx *gin.Context) {
	log := utilities.NewLogger("GetGroupChatByGroup")
	log.Info("Received GetGroupChatByGroup request")

	chain := ctx.Param("chain")
	gid := ctx.Param("gid")
	user := ctx.GetString(consts.UserAddress)
	if gid == "" || chain == "" {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "gid and chain are required",
				Message:    "please provide chain/gid parameter",
			},
		)
		return
	}

	pageSize, pageState := ctx.DefaultQuery("page_size", consts.DefaultPageSize), ctx.Query("page_state")
	// decoding base64 encoded page state to []byte
	currPageState, err := base64.URLEncoding.DecodeString(pageState)
	if err != nil {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "incorrect page state format",
				Message:    err.Error(),
			},
		)
		return
	}

	numPageSize, err := strconv.Atoi(pageSize)
	if err != nil {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed to convert page number to an integer",
				Message:    err.Error(),
			},
		)
		return
	}

	res, nextPageState, err := c.useCases.GetGroupChatByGroup(ctx, chain, user, gid, numPageSize, currPageState)
	if err != nil {
		ctx.JSON(
			http.StatusInternalServerError, entities.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Error:      "failed to get group chat by user",
				Message:    err.Error(),
			},
		)
		return
	}
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 201,
			Message:    "Chat by group retrieved successfully.",
			Data:       res,
			PaginationMetaData: &entities.PaginationMetaData{
				Size:     len(res),
				PageSize: numPageSize,
				Next:     base64.URLEncoding.EncodeToString(nextPageState),
				Prev:     pageState,
			},
		},
	)
}

func (c *ChatController) CreateGroup(ctx *gin.Context) {
	log := utilities.NewLogger("CreateGroup")
	log.Info("Received CreateGroup request")

	chain := ctx.Param("chain")
	user := ctx.GetString(consts.UserAddress)

	if user == "" || chain == "" {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "user and chain are required",
				Message:    "please provide chain/user parameter",
			},
		)
		return
	}

	var request *entities.GroupChatInfo
	if err := ctx.BindJSON(&request); err != nil {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed to create group",
				Message:    fmt.Sprintf("binding failed: %s", err.Error()),
			},
		)
		return
	}

	request.Chain = chain
	request.Owner = user
	request.Admins = []string{user}
	request.Users = []string{user}

	res, err := c.useCases.StoreGroupInfo(ctx, request)
	if err != nil {
		ctx.JSON(
			res.StatusCode, entities.ErrorResponse{
				StatusCode: res.StatusCode,
				Error:      "failed to create group",
				Message:    err.Error(),
			},
		)
		return
	}
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 201,
			Message:    "Group created successfully.",
			Data:       res.Data,
		},
	)
}

func (c *ChatController) UpdateGroupInfo(ctx *gin.Context) {
	log := utilities.NewLogger("UpdateGroupInfo")
	log.Info("Received UpdateGroupInfo request")

	chain := ctx.Param("chain")
	gid := ctx.Param("gid")

	if gid == "" || chain == "" {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "gid and chain are required",
				Message:    "please provide chain/gid parameter",
			},
		)
		return
	}

	var request *entities.GroupChatInfo
	if err := ctx.BindJSON(&request); err != nil {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed to update group",
				Message:    fmt.Sprintf("binding failed: %s", err.Error()),
			},
		)
		return
	}

	request.Chain = chain
	request.GID = gid

	res, err := c.useCases.UpdateGroupInfo(ctx, request)
	if err != nil {
		ctx.JSON(
			res.StatusCode, entities.ErrorResponse{
				StatusCode: res.StatusCode,
				Error:      "failed to update group",
				Message:    err.Error(),
			},
		)
		return
	}
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 201,
			Message:    "Group deleted successfully.",
			Data:       res.Data,
		},
	)
}

func (c *ChatController) DeleteGroup(ctx *gin.Context) {
	log := utilities.NewLogger("DeleteGroup")
	log.Info("Received DeleteGroup request")

	chain := ctx.Param("chain")
	gid := ctx.Param("gid")

	if gid == "" || chain == "" {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "gid and chain are required",
				Message:    "please provide chain/gid parameter",
			},
		)
		return
	}

	res, err := c.useCases.DeleteGroup(ctx, chain, gid)
	if err != nil {
		ctx.JSON(
			res.StatusCode, entities.ErrorResponse{
				StatusCode: res.StatusCode,
				Error:      "failed to delete group",
				Message:    err.Error(),
			},
		)
		return
	}
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 201,
			Message:    "Group deleted successfully.",
			Data:       res.Data,
		},
	)
}

func (c *ChatController) JoinGroup(ctx *gin.Context) {
	log := utilities.NewLogger("JoinGroup")
	log.Info("Received JoinGroup request")

	chain := ctx.Param("chain")
	gid := ctx.Param("gid")
	user := ctx.GetString(consts.UserAddress)

	if user == "" || chain == "" {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "user and chain are required",
				Message:    "please provide chain/user parameter",
			},
		)
		return
	}

	var request *entities.GroupChatInfo
	if err := ctx.BindJSON(&request); err != nil {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed to join group",
				Message:    fmt.Sprintf("binding failed: %s", err.Error()),
			},
		)
		return
	}

	res, err := c.useCases.JoinGroup(ctx, chain, gid, request.Users)
	if err != nil {
		ctx.JSON(
			res.StatusCode, entities.ErrorResponse{
				StatusCode: res.StatusCode,
				Error:      "failed to join group",
				Message:    err.Error(),
			},
		)
		return
	}
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 201,
			Message:    "Joined group successfully.",
			Data:       res.Data,
		},
	)
}

func (c *ChatController) LeaveGroup(ctx *gin.Context) {
	log := utilities.NewLogger("LeaveGroup")
	log.Info("Received LeaveGroup request")

	chain := ctx.Param("chain")
	gid := ctx.Param("gid")
	user := ctx.GetString(consts.UserAddress)

	if user == "" || chain == "" {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "user and chain are required",
				Message:    "please provide chain/user parameter",
			},
		)
		return
	}

	var request *entities.GroupChatInfo
	if err := ctx.BindJSON(&request); err != nil {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed to leave group",
				Message:    fmt.Sprintf("binding failed: %s", err.Error()),
			},
		)
		return
	}

	res, err := c.useCases.LeaveGroup(ctx, chain, gid, request.Users)
	if err != nil {
		ctx.JSON(
			res.StatusCode, entities.ErrorResponse{
				StatusCode: res.StatusCode,
				Error:      "failed to leave group",
				Message:    err.Error(),
			},
		)
		return
	}
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 201,
			Message:    "Left group successfully.",
			Data:       res.Data,
		},
	)
}
