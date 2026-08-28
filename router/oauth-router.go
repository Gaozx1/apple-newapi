package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

// SetOAuthRouter registers the browser-facing OAuth 2.0 authorization server
// endpoints at the site root. They must live outside the /api group: the
// consent page is opened by top-level browser navigation from third-party
// apps, which carries the session cookie but never an Authorization header,
// so the Bearer-only UserAuth middleware cannot apply.
func SetOAuthRouter(router *gin.Engine) {
	authorize := router.Group("/oauth2/authorize")
	authorize.Use(middleware.TryBrowserSessionAuth())
	authorize.Use(middleware.DisableCache())
	authorize.GET("", controller.OAuthAuthorize)
	authorize.POST("", middleware.SessionCookieOriginGuard(), controller.OAuthAuthorize)
}
