package middlewares

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"notiboy/pkg/consts"
	"notiboy/pkg/entities"
	"notiboy/pkg/usecases"
	"notiboy/utilities"
	"notiboy/utilities/jwt"

	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	"github.com/spf13/cast"

	"notiboy/config"
	chainDriver "notiboy/pkg/repo/driver/chain"
	"notiboy/pkg/repo/driver/db"
)

type Middlewares struct {
	Cache    *cache.Cache
	useCases usecases.UseCaseImply
}

// NewMiddlewares
func NewMiddlewares(useCases usecases.UseCaseImply) *Middlewares {
	return &Middlewares{
		Cache:    cache.New(5*time.Minute, 10*time.Minute),
		useCases: useCases,
	}
}

func (m *Middlewares) ValidateUserAddress(ctx *gin.Context) {
	log := utilities.NewLogger("ValidateUserAddress")

	chain := ctx.Param("chain")
	addrParam := ctx.Param("address")

	if !chainDriver.IsChainSupported(chain) {
		ctx.AbortWithStatusJSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Message:    "Unsupported chain",
			},
		)
		return
	}

	address := ctx.GetHeader("X-USER-ADDRESS")
	if len(address) == 0 {
		ctx.AbortWithStatusJSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Message:    "Missing X-USER-ADDRESS in API header",
			},
		)
		return
	}

	if addrParam != address {
		ctx.AbortWithStatusJSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Message:    "Address parameter doesn't match with X-USER-ADDRESS in API header",
			},
		)
		return
	}

	signedTxn := ctx.GetHeader("X-SIGNED-TXN")
	if len(signedTxn) == 0 {
		ctx.AbortWithStatusJSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Message:    "Missing X-SIGNED-TXN in API header",
			},
		)
		return
	}

	if err := chainDriver.GetBlockchainClient(strings.ToLower(chain)).VerifyTransaction(
		ctx, address, signedTxn,
	); err != nil {
		log.WithError(err).Errorf("VerifyTransaction failed for user %s", address)
		ctx.AbortWithStatusJSON(
			http.StatusUnauthorized, entities.ErrorResponse{
				StatusCode: http.StatusUnauthorized,
				Message:    "Bad Signed Transaction",
			},
		)
		return
	}

	log.Debugf("Chain validation done- user %s validated", address)

	ctx.Set(consts.UserAddress, address)
	ctx.Set(consts.UserChain, chain)

	ctx.Next()
}

func (m *Middlewares) ValidateToken(ctx *gin.Context) {
	log := utilities.NewLogger("ValidateUserAddress")

	tokenValue := ctx.GetHeader("Authorization")
	if len(tokenValue) == 0 || len(strings.Split(tokenValue, " ")) != 2 {
		ctx.AbortWithStatusJSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Message:    "Missing Authorization in API header",
			},
		)
		return
	}

	token := strings.Split(tokenValue, " ")[1]

	address := ctx.GetHeader("X-USER-ADDRESS")
	if len(address) == 0 {
		ctx.AbortWithStatusJSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Message:    "Missing X-USER-ADDRESS in API header",
			},
		)
		return
	}

	// if it is not an admin API
	if strings.HasPrefix(
		strings.TrimPrefix(ctx.FullPath(), "/"),
		strings.TrimPrefix(
			fmt.Sprintf("%s/admin/", config.PathPrefix),
			"/",
		),
	) == false {
		if addrParam := ctx.Param("address"); addrParam != "" && addrParam != address {
			ctx.AbortWithStatusJSON(
				http.StatusBadRequest, entities.ErrorResponse{
					StatusCode: http.StatusBadRequest,
					Message:    "Address parameter doesn't match with X-USER-ADDRESS in API header",
				},
			)
			return
		}
	}

	claims, err := jwt.VerifyJWT(address, token)
	if err != nil {
		log.WithError(err).Errorf("jwt verification failed for user %s", address)
		ctx.AbortWithStatusJSON(
			http.StatusUnauthorized, entities.ErrorResponse{
				StatusCode: http.StatusUnauthorized,
				Message:    "Authentication failed",
			},
		)
		return
	}

	if chainParam := ctx.Param("chain"); chainParam != "" && chainParam != claims["chain"] {
		ctx.AbortWithStatusJSON(
			http.StatusBadRequest, entities.ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Message:    "Token not meant for this chain",
			},
		)
		return
	}

	// if token was removed due to a sign-out
	if present, _ := db.IsTokenPresent(ctx, address, claims["chain"], token, claims["kind"], claims["uuid"]); !present {
		ctx.AbortWithStatusJSON(
			http.StatusUnauthorized, entities.ErrorResponse{
				StatusCode: http.StatusUnauthorized,
				Message:    "Authentication failed",
			},
		)
		return
	}

	log.Debugf("User %s validated", address)

	ctx.Set(consts.UserChain, claims["chain"])
	ctx.Set(consts.UserAddress, claims["address"])
	ctx.Set(consts.UserToken, token)

	ctx.Next()
}

func (m *Middlewares) IsAdminUser(ctx *gin.Context) {
	log := utilities.NewLogger("IsAdminUser")

	chain, _ := ctx.Get(consts.UserChain)
	address, _ := ctx.Get(consts.UserAddress)
	if !config.IsAdminUser(cast.ToString(chain), cast.ToString(address)) {
		log.Errorf("user %s is not privileged", address)
		ctx.AbortWithStatusJSON(
			http.StatusForbidden, entities.ErrorResponse{
				StatusCode: http.StatusForbidden,
				Message:    "Authentication failed",
			},
		)
		return
	}

	ctx.Set(consts.AdminUser, true)

	ctx.Next()
}
func (m *Middlewares) VerifyWebsocketRequest(ctx *gin.Context, chain, address, token string) error {
	claims, err := jwt.VerifyJWT(address, token)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if present, _ := db.IsTokenPresent(ctx, address, claims["chain"], token, claims["kind"], claims["uuid"]); !present {
		return fmt.Errorf("authentication failed: token not found")
	}

	if claims["chain"] != chain {
		return fmt.Errorf("token does not represnt the chain")
	}

	ctx.Set(consts.UserChain, claims["chain"])
	ctx.Set(consts.UserAddress, claims["address"])
	ctx.Set(consts.UserToken, token)

	return nil
}
