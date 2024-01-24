package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"notiboy/config"
	"notiboy/pkg/entities"
	"notiboy/pkg/middlewares"
	"notiboy/pkg/usecases"
)

type Controller struct {
	router      *gin.RouterGroup
	useCases    usecases.UseCaseImply
	middleWares *middlewares.Middlewares
}

// NewController
func NewController(
	router *gin.RouterGroup, useCases usecases.UseCaseImply, middleWare *middlewares.Middlewares,
) *Controller {
	return &Controller{
		router:      router,
		useCases:    useCases,
		middleWares: middleWare,
	}
}

// InitRoutes
func (c *Controller) InitRoutes() {

	v1 := c.router.Group(config.GetConfig().Server.APIVersion)
	{
		v1.GET("/", c.RootHandler)
		v1.GET("/health", c.HealthHandler)
		v1.GET("/db/health", c.DatabaseHealthHandler)
	}

}

func (c *Controller) RootHandler(ctx *gin.Context) {
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 200,
			Message:    "Welcome to the Notiboy API! Please refer to the documentation for information on available endpoints.",
		},
	)
}

// HealthHandler
func (c *Controller) HealthHandler(ctx *gin.Context) {
	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 200,
			Message:    "Heath check ok",
		},
	)
}

func (c *Controller) DatabaseHealthHandler(ctx *gin.Context) {
	err := c.useCases.DBHealthHandler(ctx)
	if err != nil {
		ctx.JSON(
			http.StatusOK, entities.ErrorResponse{
				StatusCode: 400,
				Message:    "unhealthy database",
			},
		)
		return
	}

	ctx.JSON(
		http.StatusOK, entities.Response{
			StatusCode: 200,
			Message:    "database health is okay",
		},
	)
}
