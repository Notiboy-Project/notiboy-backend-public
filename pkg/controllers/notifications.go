package controllers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"

	"notiboy/config"
	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/pkg/middlewares"
	"notiboy/pkg/repo/driver/medium"
	"notiboy/pkg/usecases"
	"notiboy/utilities"

	"github.com/gin-gonic/gin"
)

type NotificationController struct {
	router      *gin.RouterGroup
	useCases    usecases.NotificationUsecaseImply
	middleWares *middlewares.Middlewares
	ws          *medium.Socket
}

// NewNotificationController
func NewNotificationController(
	router *gin.RouterGroup, notificationUseCases usecases.NotificationUsecaseImply,
	ws *medium.Socket, middleWare *middlewares.Middlewares,
) *NotificationController {
	return &NotificationController{
		router:      router,
		useCases:    notificationUseCases,
		middleWares: middleWare,
		ws:          ws,
	}
}

// InitRoutes
func (n *NotificationController) InitRoutes() {
	v1 := n.router.Group(config.GetConfig().Server.APIVersion)
	v1.GET("/ws", n.WebsocketHandler)

	verifyToken := v1.Group("", n.middleWares.ValidateToken)
	onboarded := verifyToken.Group("", n.middleWares.VerifyUserOnboarded)
	{
		onboarded.POST("/chains/:chain/channels/:app_id/notifications/:kind", n.SendNotifications)
		onboarded.GET("/chains/:chain/notifications", n.GetNotifications)
		onboarded.GET("/chains/:chain/scheduled_notifications", n.GetScheduledNotifications)
		onboarded.DELETE("/chains/:chain/scheduled_notifications/schedule/:schedule", n.DeleteScheduledNotification)
		onboarded.PUT("/chains/:chain/scheduled_notifications/schedule/:schedule", n.UpdateScheduledNotification)
		onboarded.GET("/chains/public-notification/:notification_id/count", n.NotificationReachCount)
	}
}

// SendNotifications is a handler function for sending notifications in the NotificationController.
func (n *NotificationController) SendNotifications(ctx *gin.Context) {

	chain, appID, kind := ctx.Param("chain"), ctx.Param("app_id"), ctx.Param("kind")
	sender, _ := ctx.Get(consts.UserAddress)
	request := entities.NotificationRequest{
		Chain:   chain,
		Channel: appID,
		Type:    kind,
	}
	request.Sender, _ = sender.(string)
	log := utilities.NewLogger("SendNotifications")
	request.SystemSupportedMediums = []string{"email", "discord"}

	if err := ctx.BindJSON(&request); err != nil {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed to send notification",
				Message:    fmt.Sprintf("binding failed: %s", err.Error()),
			},
		)
		return
	}

	log.Info("Received SendNotifications request for chain:", chain, " appID:", appID, " and kind:", kind)

	err := n.useCases.SendNotifications(ctx, request)
	if err != nil {
		ctx.JSON(
			http.StatusInternalServerError, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed to send notification",
				Message:    err.Error(),
			},
		)
		return
	}
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 200,
			Message:    "Successfully sent notifications",
		},
	)

}

// DeleteScheduledNotification is a handler function for retrieving scheduled notifications in the NotificationController.
func (n *NotificationController) DeleteScheduledNotification(ctx *gin.Context) {
	log := utilities.NewLogger("DeleteScheduledNotification")

	chain := ctx.Param("chain")
	schedule := ctx.Param("schedule")
	user, _ := ctx.Get(consts.UserAddress)

	log.Info("Received DeleteScheduledNotification request for chain:", chain)

	err := n.useCases.DeleteScheduledNotificationInfo(
		ctx, chain, user.(string), utilities.TimeStringToRFC3339Time(schedule),
	)
	if err != nil {
		ctx.JSON(
			http.StatusInternalServerError, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed deleting scheduled notifications",
				Message:    err.Error(),
			},
		)
		return
	}

	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 200,
			Message:    "Successfully deleted scheduled notifications",
		},
	)
}

// UpdateScheduledNotification is a handler function for retrieving scheduled notifications in the NotificationController.
func (n *NotificationController) UpdateScheduledNotification(ctx *gin.Context) {
	log := utilities.NewLogger("UpdateScheduledNotification")

	chain := ctx.Param("chain")
	schedule := ctx.Param("schedule")

	sender, _ := ctx.Get(consts.UserAddress)
	request := &entities.ScheduleNotificationRequest{
		Chain:    chain,
		Sender:   sender.(string),
		Schedule: utilities.TimeStringToRFC3339Time(schedule),
	}

	if err := ctx.BindJSON(&request); err != nil {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed to update scheduled notification",
				Message:    err.Error(),
			},
		)
		return
	}

	log.Info("Received UpdateScheduledNotification request for chain:", chain)

	err := n.useCases.UpdateScheduledNotificationInfo(ctx, request)
	if err != nil {
		ctx.JSON(
			http.StatusInternalServerError, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed updating scheduled notifications",
				Message:    err.Error(),
			},
		)
		return
	}

	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 200,
			Message:    "Successfully updated scheduled notifications",
		},
	)
}

// GetScheduledNotifications is a handler function for retrieving scheduled notifications in the NotificationController.
func (n *NotificationController) GetScheduledNotifications(ctx *gin.Context) {
	log := utilities.NewLogger("GetScheduledNotifications")

	chain := ctx.Param("chain")
	user, _ := ctx.Get(consts.UserAddress)

	log.Info("Received GetScheduledNotifications request for chain:", chain)

	data, err := n.useCases.GetScheduledNotificationsBySender(ctx, chain, user.(string))
	if err != nil {
		ctx.JSON(
			http.StatusInternalServerError, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed fetching scheduled notifications",
				Message:    err.Error(),
			},
		)
		return
	}

	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 200,
			Message:    "Successfully fetched scheduled notifications",
			Data:       data,
		},
	)
}

// GetNotifications is a handler function for retrieving notifications in the NotificationController.
func (n *NotificationController) GetNotifications(ctx *gin.Context) {

	chain := ctx.Param("chain")
	pageSize, pageState := ctx.DefaultQuery("page_size", consts.DefaultPageSize), ctx.Query("page_state")
	user, _ := ctx.Get(consts.UserAddress)
	log := utilities.NewLogger("GetNotifications")
	request := entities.RequestNotification{
		Chain: chain,
	}
	request.User, _ = user.(string)
	log.Info("Received GetNotifications request for chain:", chain)

	// decoding base64 encoded page state to []byte
	currPageState, err := base64.URLEncoding.DecodeString(pageState)
	if err != nil {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed to retreve channelinfo page state",
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

	data, nextPageState, err := n.useCases.GetNotifications(ctx, request, numPageSize, currPageState)
	if err != nil {
		ctx.JSON(
			http.StatusInternalServerError, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed fetching notifications",
				Message:    err.Error(),
			},
		)
		return
	}
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 200,
			Message:    "Successfully fetched notifications",
			PaginationMetaData: &entities.PaginationMetaData{
				Size:     len(data),
				PageSize: numPageSize,
				Next:     base64.URLEncoding.EncodeToString(nextPageState),
				Prev:     pageState,
			},
			Data: data,
		},
	)

}

// NotificationReachCount is a handler function for fetching the reach count of a notification in the NotificationController.
func (n *NotificationController) NotificationReachCount(ctx *gin.Context) {

	notiId := ctx.Param("notification_id")
	log := utilities.NewLogger("NotificationReachCount")
	pageSize, pageState := ctx.DefaultQuery("page_size", consts.DefaultPageSize), ctx.Query("page_state")
	if notiId == "" {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed to fetch notification reach count",
				Message:    "please provide notification id",
			},
		)
		return
	}

	// docoding base64 encoded page state to []byte
	currPageState, err := base64.URLEncoding.DecodeString(pageState)
	if err != nil {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed to retreve channelinfo page state",
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
	log.Info("Received NotificationReachCount request for notification_id:", notiId)

	data, nextPageState, err := n.useCases.NotificationReachCount(ctx, notiId, numPageSize, currPageState)
	if err != nil {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed to fetch notification reach count",
				Message:    err.Error(),
			},
		)
		return
	}
	if data == nil {
		ctx.JSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: 400,
				Error:      "failed to fetch notification reach count",
				Message:    "unable to find notification reach count",
			},
		)
		return
	}

	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 200,
			Message:    "notification reach count fetched successfully",
			PaginationMetaData: &entities.PaginationMetaData{
				PageSize: numPageSize,
				Next:     base64.URLEncoding.EncodeToString(nextPageState),
				Prev:     pageState,
			},
			Data: data,
		},
	)

}

func (n *NotificationController) WebsocketHandler(ctx *gin.Context) {
	chain := ctx.Query("chain")
	address := ctx.Query("address")
	token := ctx.Query("token")

	if err := n.middleWares.VerifyWebsocketRequest(ctx, chain, address, token); err != nil {
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

	n.ws.Add(chain, address, wsConn)
}
