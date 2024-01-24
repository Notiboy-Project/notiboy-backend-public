package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"notiboy/config"
	"notiboy/migrations"
	"notiboy/pkg/cache"
	controllersLib "notiboy/pkg/controllers"
	"notiboy/pkg/middlewares"
	repoLib "notiboy/pkg/repo"
	chainLib "notiboy/pkg/repo/driver/chain"
	"notiboy/pkg/repo/driver/db"
	"notiboy/pkg/repo/driver/dns"
	"notiboy/pkg/repo/driver/medium"
	"notiboy/pkg/usecases"
	"notiboy/utilities"
)

func initMediums(ctx context.Context) (*medium.DiscordMessenger, *medium.EmailClient) {
	log := utilities.NewLogger("initMediums")

	log.Info("Initialising Discord")
	discordMsgr, err := medium.NewDiscordMessenger(config.GetConfig().Discord.BotToken)
	if err != nil {
		logrus.WithError(err).Fatal("unable to initialize discord")
	}
	go discordMsgr.SpawnSender(ctx)

	log.Info("Initialising Discord Complete")

	log.Info("Initialising Email")
	emailClient, err := medium.NewEmailClient()
	if err != nil {
		logrus.WithError(err).Fatal("unable to initialize email")
	}
	go emailClient.SpawnSender(ctx)
	log.Info("Initialising Email Complete")

	return discordMsgr, emailClient
}

func Run() {
	ctx := context.Background()
	ctx, cancelFn := context.WithCancel(ctx)

	// init the env config
	conf, err := config.LoadConfig()
	if err != nil {
		logrus.Fatalf("unable to initialize environment variables %s", err.Error())
	}

	// Initialise the logger
	utilities.InitLogger(conf.LogLevel)
	log := utilities.NewLogger("run")

	if conf.Mode != "local" {
		discordMsgr, emailClient := initMediums(ctx)
		defer func() {
			discordMsgr.Close()
			emailClient.Close()
		}()
	}

	log.Info("Initialising firebase")
	err = medium.InitFirebase(ctx, conf)
	if err != nil {
		log.WithError(err).Fatal("failed to initialise firebase")
	}

	log.Info("Initialising DB")
	session, err := db.NewCassandraSession(conf.DB)
	defer session.Close()
	if err != nil {
		log.Fatal("unable to create cassandra session ", err.Error())
	}

	log.Info("Initialising NFD client")
	dns.InitDNSClient(ctx)

	log.Info("Initialising migrations")
	migrations.Init()

	log.Info("Initialising cache")
	cache.Init()

	// initialise the blockchain network clients
	log.Info("Initialising Chains")
	chainLib.LoadChains(ctx)

	log.Info("Initialising membership checker")
	repoLib.MembershipChecker(ctx)

	// here initalizing the router
	router := initRouter(conf)
	if conf.LogLevel != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	path, err := url.JoinPath(config.GetConfig().Server.APIPrefix, config.GetConfig().Mode)
	if err != nil {
		log.Panic(err)
	}

	api := router.Group(path)

	notificationWS := medium.NewWebSocket(false)
	chatWS := medium.NewWebSocket(true)

	{
		// repo initialization
		billingRepo := repoLib.NewBillingRepo(session, conf)
		notificationRepo := repoLib.NewNotificationRepo(session, conf)
		channelRepo := repoLib.NewChannelRepo(session, conf)
		chatRepo := repoLib.NewChatRepo(session, conf)
		UserRepo := repoLib.NewUserRepo(session, conf)
		OptinRepo := repoLib.NewOptinRepo(session, conf, channelRepo)
		verifyRepo := repoLib.NewVerifyRepo(session, conf)
		repo := repoLib.NewRepo(session, conf)

		// initializing usecases
		billingUsecases := usecases.NewBillingUsecases(billingRepo, UserRepo)
		notificationUsecases := usecases.NewNotificationUsecases(
			notificationRepo, UserRepo, verifyRepo, channelRepo, OptinRepo, notificationWS,
		)
		channelUseCases := usecases.NewChannelUseCases(channelRepo, UserRepo)
		chatUseCases := usecases.NewChatUseCases(chatRepo, UserRepo, chatWS)
		UserUseCases := usecases.NewUserUseCases(UserRepo)
		OptinUseCases := usecases.NewOptinUseCases(OptinRepo)
		verifyUseCases := usecases.NewVerifyUseCases(verifyRepo)
		useCases := usecases.NewUseCases(repo)

		log.Info("Initialising notification scheduler")
		usecases.NotificationSchedulerStub(ctx, usecases.GetNotificationUsecases())

		// initializing middleware
		m := middlewares.NewMiddlewares(useCases)

		// initializing controllersLib
		billingControllers := controllersLib.NewBillingController(api, billingUsecases, m)
		notificationControllers := controllersLib.NewNotificationController(
			api, notificationUsecases,
			notificationWS, m,
		)
		channelControllers := controllersLib.NewChannelController(api, channelUseCases, m)
		chatControllers := controllersLib.NewChatController(api, chatUseCases, chatWS, m)
		UserControllers := controllersLib.NewUserController(api, UserUseCases, m)
		OptinControllers := controllersLib.NewOptinController(api, OptinUseCases, m)
		verifyControllers := controllersLib.NewVerifyController(api, verifyUseCases, m)
		controllers := controllersLib.NewController(api, useCases, m)

		// init the routes
		billingControllers.InitRoutes()
		notificationControllers.InitRoutes()
		channelControllers.InitRoutes()
		chatControllers.InitRoutes(ctx)
		UserControllers.InitRoutes()
		OptinControllers.InitRoutes()
		verifyControllers.InitRoutes()
		controllers.InitRoutes()
	}

	// run the app
	launch(ctx, cancelFn, router)
}

func initRouter(conf *config.NotiboyConfModel) *gin.Engine {

	router := gin.Default()
	gin.SetMode(gin.DebugMode)

	router.Use(
		cors.New(
			cors.Config{
				//AllowOrigins:     []string{"https://app.notiboy.com", "https://notiboy.com"},
				AllowOrigins: []string{"*"},
				AllowMethods: []string{"PUT", "PATCH", "POST", "DELETE", "GET", "OPTIONS"},
				AllowHeaders: []string{
					"Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "accept",
					"origin", "Cache-Control", "X-USER-ADDRESS", "X-SIGNED-TXN", "HOST",
				},
				AllowCredentials: true,
				MaxAge:           12 * time.Hour,
			},
		),
	)

	mode := config.GetConfig().Mode
	if mode == "stage" || mode == "local" {
		router.GET("/debug/pprof/*profile", gin.WrapF(pprof.Index))
	}

	router.Use(gzip.Gzip(gzip.DefaultCompression))

	return router
}

// launch
func launch(ctx context.Context, cancelFn context.CancelFunc, router *gin.Engine) {
	log := utilities.NewLogger("launch")
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.GetConfig().Server.Port),
		Handler: router,
	}

	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	fmt.Println("Server listening in...", config.GetConfig().Server.Port)
	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal)
	// kill (no param) default send syscanll.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall. SIGKILL but can"t be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown Server ...")
	cancelFn()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	// catching ctx.Done(). timeout of 5 seconds.
	select {
	case <-ctx.Done():
		log.Println("timeout of 5 seconds.")
	}
	log.Println("Server exiting")
}
