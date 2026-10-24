package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

// SetStoreRouter registers the user-to-user store: any signed-in user may
// publish products and sell imported codes, and any user may buy them. Money
// moves through the wallet, so every endpoint requires a browser session.
func SetStoreRouter(router *gin.Engine) {
	storeRouter := router.Group("/api/store")
	storeRouter.Use(middleware.UserAuth())
	{
		storeRouter.GET("/meta", controller.StoreFeeRate)
		storeRouter.GET("/products", controller.ListStoreProducts)
		storeRouter.GET("/products/:id", controller.GetStoreProduct)
		storeRouter.POST("/products", controller.CreateStoreProduct)
		storeRouter.PUT("/products/:id", controller.UpdateStoreProduct)
		storeRouter.DELETE("/products/:id", controller.DeleteStoreProduct)
		storeRouter.POST("/products/:id/cdks", controller.ImportStoreCdks)
		storeRouter.POST("/products/:id/buy", controller.PurchaseStoreProduct)
		storeRouter.GET("/mine/products", controller.ListMyStoreProducts)
		storeRouter.GET("/mine/orders", controller.ListStoreOrders)
		storeRouter.GET("/mine/sales", controller.ListStoreSales)

		// Channel offers: a merchant uploads an upstream, a buyer receives a
		// live channel inside the 用户渠道 group.
		storeRouter.GET("/channels", controller.ListStoreChannels)
		storeRouter.GET("/market", controller.ListMarketStoreGroups)
		storeRouter.POST("/claims", controller.ClaimStoreGroup)
		storeRouter.DELETE("/claims", controller.UnclaimStoreGroup)
		storeRouter.POST("/channels", controller.CreateStoreChannel)
		storeRouter.GET("/my-channels", controller.ListMyStoreChannels)
		storeRouter.GET("/mine/earnings", controller.ListMyStoreChannelEarnings)
		storeRouter.PUT("/channels/:id", controller.UpdateStoreChannel)
		storeRouter.DELETE("/channels/:id", controller.DeleteStoreChannel)
		storeRouter.POST("/channels/:id/listing", controller.SetStoreChannelListing)
		storeRouter.POST("/fetch-models", controller.FetchStoreChannelModels)
		storeRouter.GET("/groups", controller.ListStoreGroups)
		storeRouter.POST("/groups", controller.CreateStoreGroup)
		storeRouter.PUT("/groups/:id", controller.UpdateStoreGroup)
		storeRouter.DELETE("/groups/:id", controller.DeleteStoreGroup)
		storeRouter.POST("/groups/:id/listing", controller.SetStoreGroupListing)
		storeRouter.POST("/reports", controller.ReportStoreChannel)
		storeRouter.POST("/group-reports", controller.ReportStoreGroup)
		storeRouter.GET("/assistant/context", controller.GetStoreAssistantContext)
		storeRouter.POST("/assistant/action", controller.RunStoreAssistantAction)
		storeRouter.POST("/assistant/chat", controller.AssistantChat)
		storeRouter.POST("/tickets", controller.CreateStoreTicket)
		storeRouter.GET("/tickets/mine", controller.ListMyStoreTickets)
		storeRouter.GET("/tickets/sales", controller.ListStoreSalesTickets)
		storeRouter.GET("/tickets/:id", controller.GetStoreTicket)
		storeRouter.POST("/tickets/:id/messages", controller.AddStoreTicketMessage)
		storeRouter.POST("/tickets/:id/refund", controller.RefundStoreTicket)
		storeRouter.POST("/tickets/:id/complain", controller.ComplainStoreTicket)

	}

	storeAdminRouter := router.Group("/api/store/admin")
	storeAdminRouter.Use(middleware.AdminAuth())
	{
		storeAdminRouter.GET("/products", controller.AdminListStoreProducts)
		storeAdminRouter.GET("/reports", controller.ListStoreChannelReports)
		storeAdminRouter.POST("/reports/:id/review", controller.ReviewStoreChannelReport)
		storeAdminRouter.GET("/tickets", controller.AdminListStoreComplaints)
		storeAdminRouter.POST("/tickets/:id/resolve", controller.AdminResolveStoreTicket)
	}
}
