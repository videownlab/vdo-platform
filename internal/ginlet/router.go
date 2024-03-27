package ginlet

import (
	"net/http"
	"strings"
	"time"

	"vdo-platform/internal/ginlet/api"
	"vdo-platform/internal/ginlet/middleware/auth"
	"vdo-platform/internal/ginlet/resp"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/logger"
	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	router := gin.New()
	router.Use(logger.SetLogger())
	router.Use(configedCors())
	router.Use(trimGetSuffix())
	router.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		resp.ErrorWithHttpStatus(c, err.(error), http.StatusInternalServerError)
		c.Abort()
	}))

	registerEndpointsForSys(router)
	registerEndpointsForVideo(router)
	registerEndpointsForNFT(router)
	router.StaticFile("/favicon.ico", "./static/favicon.ico")
	return router
}

func configedCors() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowCredentials = true
	return cors.New(config)
}

func trimGetSuffix() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			req := c.Request.RequestURI
			idx := strings.LastIndex(req, "&")
			if idx > 0 {
				c.Request.RequestURI = req[0:idx]
			}
		}
		c.Next()
	}
}

func registerEndpointsForVideo(router *gin.Engine) {
	v := api.NewVideoAPI()
	g := router.Group("/video")
	g.PUT("/list", v.QueryVideos)
	g.PUT("/search", v.SearchVideos)
	g.PUT("/cover", v.UploadVideoCoverImg)
	g.GET("/cover", v.DownloadVideoCoverImg)
	g.GET("/views", v.AddVideoViews)
}

func registerEndpointsForNFT(router *gin.Engine) {
	n := api.NewNftAPI()
	g := router.Group("/nft")
	g.Use(auth.AuthRequired)
	{
		g.PUT("/create", n.CreateVideoMetadata)
		g.PUT("/mint/:act", n.MintNFT)
		g.PUT("/purchase/:act", n.BuyNFT)
		g.PUT("/transfer/:act", n.TransferNFT)
		g.PUT("/change/status/:act", n.ChangeSellingStatus)
		g.PUT("/change/price/:act", n.ChangeSellingPrice)
		g.PUT("/activity/list", n.QueryActivities)
		g.PUT("/delete", n.DeleteVideoMetadata)
	}
}

func registerEndpointsForSys(router *gin.Engine) {
	s := api.NewAuthAPI()
	g := router.Group("/auth")
	{
		g.GET("/ts", func(c *gin.Context) { resp.Ok(c, time.Now().Unix()) })
		g.PUT("/apply-code", s.ApplyAuthCode)
		g.PUT("/login-by-email", s.LoginByEmail)
		g.PUT("/login-by-wallet", s.LoginByWallet)
		g.PUT("/sign-tx", s.SignTx)
	}
}
