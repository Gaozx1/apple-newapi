package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// storeProductView is the wire shape of a product. price_amount is derived from
// the stored quota so the UI never has to know the quota rate.
type storeProductView struct {
	Id          int     `json:"id"`
	UserId      int     `json:"user_id"`
	Username    string  `json:"username"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       int     `json:"price"`
	PriceAmount float64 `json:"price_amount"`
	Status      int     `json:"status"`
	Stock       int     `json:"stock"`
	CdkCount    int     `json:"cdk_count"`
	SoldCount   int     `json:"sold_count"`
	IsOwner     bool    `json:"is_owner"`
	CreatedAt   int64   `json:"created_at"`
}

type storeOrderView struct {
	Id           int     `json:"id"`
	OrderNo      string  `json:"order_no"`
	ProductId    int     `json:"product_id"`
	ProductName  string  `json:"product_name"`
	SellerUserId int     `json:"seller_user_id"`
	BuyerUserId  int     `json:"buyer_user_id"`
	Price        int     `json:"price"`
	PriceAmount  float64 `json:"price_amount"`
	Fee          int     `json:"fee"`
	FeeAmount    float64 `json:"fee_amount"`
	SellerIncome int     `json:"seller_income"`
	IncomeAmount float64 `json:"income_amount"`
	Code         string  `json:"code"`
	CreatedAt    int64   `json:"created_at"`
}

type storeProductRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	PriceAmount float64 `json:"price_amount"`
	Status      *int    `json:"status"`
}

type storeCdkImportRequest struct {
	Codes string `json:"codes"`
}

func storeQuotaFromAmount(amount float64) int {
	if amount <= 0 {
		return 0
	}
	return common.QuotaRound(amount * common.QuotaPerUnit)
}

func storeAmountFromQuota(quota int) float64 {
	return float64(quota) / common.QuotaPerUnit
}

// toStoreProductView decorates one product for the requesting user.
func toStoreProductView(product *model.StoreProduct, viewerId int) storeProductView {
	username, _ := model.GetUsernameById(product.UserId, false)
	return storeProductView{
		Id:          product.Id,
		UserId:      product.UserId,
		Username:    username,
		Name:        product.Name,
		Description: product.Description,
		Price:       product.Price,
		PriceAmount: storeAmountFromQuota(product.Price),
		Status:      product.Status,
		Stock:       product.Stock(),
		CdkCount:    product.CdkCount,
		SoldCount:   product.SoldCount,
		IsOwner:     product.UserId == viewerId,
		CreatedAt:   product.CreatedAt,
	}
}

func toStoreProductViews(products []*model.StoreProduct, viewerId int) []storeProductView {
	views := make([]storeProductView, 0, len(products))
	for _, product := range products {
		views = append(views, toStoreProductView(product, viewerId))
	}
	return views
}

func toStoreOrderView(order *model.StoreOrder) storeOrderView {
	return storeOrderView{
		Id:           order.Id,
		OrderNo:      order.OrderNo,
		ProductId:    order.ProductId,
		ProductName:  order.ProductName,
		SellerUserId: order.SellerUserId,
		BuyerUserId:  order.BuyerUserId,
		Price:        order.Price,
		PriceAmount:  storeAmountFromQuota(order.Price),
		Fee:          order.Fee,
		FeeAmount:    storeAmountFromQuota(order.Fee),
		SellerIncome: order.SellerIncome,
		IncomeAmount: storeAmountFromQuota(order.SellerIncome),
		Code:         order.Code,
		CreatedAt:    order.CreatedAt,
	}
}

func toStoreOrderViews(orders []*model.StoreOrder) []storeOrderView {
	views := make([]storeOrderView, 0, len(orders))
	for _, order := range orders {
		views = append(views, toStoreOrderView(order))
	}
	return views
}

// storeRouteId reads the :id path parameter.
func storeRouteId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "商品编号无效")
		return 0, false
	}
	return id, true
}

// ListStoreProducts lists every product on sale.
func ListStoreProducts(c *gin.Context) {
	page := common.GetPageQuery(c)
	products, total, err := model.ListStoreProducts(c.Query("keyword"), page.GetPage(), page.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(toStoreProductViews(products, c.GetInt("id")))
	common.ApiSuccess(c, page)
}

// GetStoreProduct returns one product with its live stock.
func GetStoreProduct(c *gin.Context) {
	productId, ok := storeRouteId(c)
	if !ok {
		return
	}
	product, err := model.GetStoreProductById(productId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toStoreProductView(product, c.GetInt("id")))
}

// ListMyStoreProducts lists the products the caller published.
func ListMyStoreProducts(c *gin.Context) {
	page := common.GetPageQuery(c)
	products, total, err := model.ListUserStoreProducts(c.GetInt("id"), page.GetPage(), page.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(toStoreProductViews(products, c.GetInt("id")))
	common.ApiSuccess(c, page)
}

// CreateStoreProduct publishes a product for the caller.
func CreateStoreProduct(c *gin.Context) {
	request := storeProductRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	name, description, err := model.NormalizeStoreProductInput(request.Name, request.Description, storeQuotaFromAmount(request.PriceAmount))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	product := &model.StoreProduct{
		UserId:      c.GetInt("id"),
		Name:        name,
		Description: description,
		Price:       storeQuotaFromAmount(request.PriceAmount),
		Status:      model.StoreProductStatusOnSale,
	}
	if request.Status != nil && *request.Status != model.StoreProductStatusOnSale {
		product.Status = model.StoreProductStatusOffShelf
	}
	if err := model.CreateStoreProduct(product); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toStoreProductView(product, c.GetInt("id")))
}

// UpdateStoreProduct edits one product the caller owns.
func UpdateStoreProduct(c *gin.Context) {
	productId, ok := storeRouteId(c)
	if !ok {
		return
	}
	request := storeProductRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	product, err := model.GetStoreProductById(productId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if product.UserId != c.GetInt("id") {
		common.ApiError(c, model.ErrStoreProductNotOwned)
		return
	}
	name, description, err := model.NormalizeStoreProductInput(request.Name, request.Description, storeQuotaFromAmount(request.PriceAmount))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	product.Name = name
	product.Description = description
	product.Price = storeQuotaFromAmount(request.PriceAmount)
	if request.Status != nil {
		if *request.Status == model.StoreProductStatusOnSale {
			product.Status = model.StoreProductStatusOnSale
		} else {
			product.Status = model.StoreProductStatusOffShelf
		}
	}
	if err := model.UpdateStoreProduct(product); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toStoreProductView(product, c.GetInt("id")))
}

// DeleteStoreProduct removes a product and its unsold codes.
func DeleteStoreProduct(c *gin.Context) {
	productId, ok := storeRouteId(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	if c.GetInt("role") >= common.RoleAdminUser {
		// Administrators may remove any product; resolve its owner first so the
		// ownership check inside the model call still holds.
		owner, err := model.StoreProductOwnerId(productId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		userId = owner
	}
	if err := model.DeleteStoreProduct(productId, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// ImportStoreCdks appends a bulk batch of codes to one product.
func ImportStoreCdks(c *gin.Context) {
	productId, ok := storeRouteId(c)
	if !ok {
		return
	}
	request := storeCdkImportRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	imported, duplicated, err := model.ImportStoreCdks(productId, c.GetInt("id"), request.Codes)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	product, err := model.GetStoreProductById(productId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"imported":   imported,
		"duplicated": duplicated,
		"product":    toStoreProductView(product, c.GetInt("id")),
	})
}

// PurchaseStoreProduct buys one unit and returns the delivered code.
func PurchaseStoreProduct(c *gin.Context) {
	productId, ok := storeRouteId(c)
	if !ok {
		return
	}
	order, err := model.PurchaseStoreProduct(c.GetInt("id"), productId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"order": toStoreOrderView(order)})
}

// ListStoreOrders lists the caller's purchases, including the delivered codes.
func ListStoreOrders(c *gin.Context) {
	page := common.GetPageQuery(c)
	orders, total, err := model.ListStoreOrders(c.GetInt("id"), page.GetPage(), page.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(toStoreOrderViews(orders))
	common.ApiSuccess(c, page)
}

// ListStoreSales lists the caller's sales with the fee each one cost.
func ListStoreSales(c *gin.Context) {
	page := common.GetPageQuery(c)
	orders, total, err := model.ListStoreSales(c.GetInt("id"), page.GetPage(), page.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(toStoreOrderViews(orders))
	common.ApiSuccess(c, page)
}

// AdminListStoreProducts lists every product regardless of seller or status.
func AdminListStoreProducts(c *gin.Context) {
	page := common.GetPageQuery(c)
	products, total, err := model.ListAllStoreProducts(c.Query("keyword"), page.GetPage(), page.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(toStoreProductViews(products, c.GetInt("id")))
	common.ApiSuccess(c, page)
}

// StoreFeeRate reports the platform commission so the UI can show it verbatim.
func StoreFeeRate(c *gin.Context) {
	common.ApiSuccess(c, gin.H{
		"permille":             model.StoreFeePermille,
		"rate":                 float64(model.StoreFeePermille) / 1000,
		"max_codes_per_import": model.StoreMaxCodesPerImport,
		"max_price_amount":     storeAmountFromQuota(model.StoreMaxPriceQuota),
	})
}
