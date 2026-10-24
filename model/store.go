package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store product status.
const (
	StoreProductStatusOffShelf = 0
	StoreProductStatusOnSale   = 1
)

// Store CDK status.
const (
	StoreCdkStatusUnsold = 0
	StoreCdkStatusSold   = 1
)

const (
	// StoreFeePermille is the platform commission taken from every sale, in
	// per-mille of the price. 1 per-mille = 0.1%.
	StoreFeePermille = 1
	// StoreOrderStatusPaid marks a delivered, paid order.
	StoreOrderStatusPaid = 1

	// StoreMaxPriceQuota bounds a merchant-set price so no single purchase can
	// approach the wallet ceiling. 5e9 quota is $10,000 at the default rate.
	StoreMaxPriceQuota = 5_000_000_000
	// StoreMaxNameRunes bounds the product name.
	StoreMaxNameRunes = 80
	// StoreMaxDescriptionRunes bounds the product description.
	StoreMaxDescriptionRunes = 1000
	// StoreMaxCodesPerImport bounds one bulk import batch.
	StoreMaxCodesPerImport = 500
	// StoreMaxCodeLength bounds a single CDK. Codes are stored as text and
	// deduplicated by hash, so long license blobs fit.
	StoreMaxCodeLength = 4096
)

var (
	ErrStoreProductNotFound     = errors.New("商品不存在")
	ErrStoreProductOffShelf     = errors.New("商品已下架")
	ErrStoreOutOfStock          = errors.New("商品库存不足")
	ErrStoreInsufficientQuota   = errors.New("余额不足，请先充值")
	ErrStoreSelfPurchase        = errors.New("不能购买自己发布的商品")
	ErrStorePriceInvalid        = errors.New("商品价格无效")
	ErrStoreSellerQuotaOverflow = errors.New("商家余额已达上限")
	ErrStoreProductNotOwned     = errors.New("只能管理自己发布的商品")
	ErrStoreNoInput             = errors.New("请填写商品信息")
	ErrStoreNoCode              = errors.New("请提供至少一个卡密")
	ErrStoreCodeTooLong         = errors.New("卡密过长")
)

// StoreProduct is a product a user publishes for sale. The seller sets the
// price; every sale delivers exactly one imported CDK to the buyer.
type StoreProduct struct {
	Id          int    `json:"id"`
	UserId      int    `json:"user_id" gorm:"index"`
	Name        string `json:"name" gorm:"type:varchar(80);not null"`
	Description string `json:"description" gorm:"type:text"`
	// Price is the per-unit price in quota units, so accounting never rounds.
	Price     int   `json:"price" gorm:"not null"`
	Status    int   `json:"status" gorm:"default:1;index"`
	CdkCount  int   `json:"cdk_count" gorm:"default:0"`
	SoldCount int   `json:"sold_count" gorm:"default:0"`
	CreatedAt int64 `json:"created_at" gorm:"bigint;index"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

// TableName pins the table name for every supported dialect.
func (StoreProduct) TableName() string {
	return "store_products"
}

// Stock is the number of unsold codes; it is derived, never stored.
func (p *StoreProduct) Stock() int {
	stock := p.CdkCount - p.SoldCount
	if stock < 0 {
		return 0
	}
	return stock
}

// StoreCdk is one imported code. The code itself is never serialized: a buyer
// reads it from the order that delivered it. Uniqueness is enforced on a hash,
// so a code of any length keeps a small, indexable column.
type StoreCdk struct {
	Id        int    `json:"id"`
	ProductId int    `json:"product_id" gorm:"index:idx_store_cdk_stock,priority:1"`
	Code      string `json:"-" gorm:"type:text;not null"`
	CodeHash  string `json:"-" gorm:"type:varchar(64);not null;uniqueIndex"`
	Status    int    `json:"status" gorm:"index:idx_store_cdk_stock,priority:2;default:0"`
	OrderId   int    `json:"order_id" gorm:"default:0"`
	CreatedAt int64  `json:"created_at" gorm:"bigint"`
	SoldAt    int64  `json:"sold_at" gorm:"bigint;default:0"`
}

func (StoreCdk) TableName() string {
	return "store_cdks"
}

// StoreOrder is one delivered purchase. It snapshots the product name and the
// code so both stay readable for the buyer after the product changes.
type StoreOrder struct {
	Id           int    `json:"id"`
	OrderNo      string `json:"order_no" gorm:"type:varchar(64);uniqueIndex"`
	ProductId    int    `json:"product_id" gorm:"index"`
	ProductName  string `json:"product_name" gorm:"type:varchar(80)"`
	SellerUserId int    `json:"seller_user_id" gorm:"index"`
	BuyerUserId  int    `json:"buyer_user_id" gorm:"index"`
	Price        int    `json:"price"`
	Fee          int    `json:"fee"`
	SellerIncome int    `json:"seller_income"`
	Code         string `json:"code" gorm:"type:text"`
	CdkId        int    `json:"cdk_id"`
	Status       int    `json:"status" gorm:"default:1"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint;index"`
}

func (StoreOrder) TableName() string {
	return "store_orders"
}

// StoreFeeForPrice returns the platform commission in quota units. Integer
// arithmetic rounds the commission up, so a tiny price never yields a free sale
// and no float rounding can drift the split, and it never exceeds the price.
func StoreFeeForPrice(price int) int {
	if price <= 0 {
		return 0
	}
	fee := (price*StoreFeePermille + 999) / 1000
	if fee > price {
		fee = price
	}
	return fee
}

// NormalizeStoreProductInput validates and trims merchant input. price is in
// quota units, already converted from the amount the merchant typed.
func NormalizeStoreProductInput(name string, description string, price int) (string, string, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return "", "", ErrStoreNoInput
	}
	if len([]rune(name)) > StoreMaxNameRunes {
		return "", "", fmt.Errorf("商品名称不能超过 %d 个字", StoreMaxNameRunes)
	}
	if len([]rune(description)) > StoreMaxDescriptionRunes {
		return "", "", fmt.Errorf("商品描述不能超过 %d 个字", StoreMaxDescriptionRunes)
	}
	if price <= 0 || price > StoreMaxPriceQuota {
		return "", "", ErrStorePriceInvalid
	}
	return name, description, nil
}

// ParseStoreCodes splits a bulk import into unique, trimmed codes.
func ParseStoreCodes(raw string) ([]string, error) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	seen := make(map[string]struct{}, len(lines))
	codes := make([]string, 0, len(lines))
	for _, line := range lines {
		code := strings.TrimSpace(line)
		if code == "" {
			continue
		}
		if len([]rune(code)) > StoreMaxCodeLength {
			return nil, ErrStoreCodeTooLong
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
		// Accept one batch of codes; the caller re-imports for more.
		if len(codes) >= StoreMaxCodesPerImport {
			break
		}
	}
	if len(codes) == 0 {
		return nil, ErrStoreNoCode
	}
	return codes, nil
}

func CreateStoreProduct(product *StoreProduct) error {
	now := common.GetTimestamp()
	product.CreatedAt = now
	product.UpdatedAt = now
	if product.Status != StoreProductStatusOnSale {
		product.Status = StoreProductStatusOffShelf
	}
	return DB.Create(product).Error
}

func GetStoreProductById(id int) (*StoreProduct, error) {
	product := &StoreProduct{}
	if err := DB.Where("id = ?", id).First(product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStoreProductNotFound
		}
		return nil, err
	}
	return product, nil
}

// ListStoreProducts lists products on sale, newest first.
func ListStoreProducts(keyword string, page int, pageSize int) ([]*StoreProduct, int64, error) {
	query := DB.Model(&StoreProduct{}).Where("status = ?", StoreProductStatusOnSale)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	products := make([]*StoreProduct, 0, pageSize)
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

// ListUserStoreProducts lists the products one merchant published.
func ListUserStoreProducts(userId int, page int, pageSize int) ([]*StoreProduct, int64, error) {
	query := DB.Model(&StoreProduct{}).Where("user_id = ?", userId)
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	products := make([]*StoreProduct, 0, pageSize)
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

// UpdateStoreProduct saves the editable fields of an owned product.
func UpdateStoreProduct(product *StoreProduct) error {
	product.UpdatedAt = common.GetTimestamp()
	return DB.Model(&StoreProduct{}).Where("id = ? AND user_id = ?", product.Id, product.UserId).
		Updates(map[string]any{
			"name":        product.Name,
			"description": product.Description,
			"price":       product.Price,
			"status":      product.Status,
			"updated_at":  product.UpdatedAt,
		}).Error
}

// DeleteStoreProduct removes an owned product and its unsold codes. Delivered
// orders keep their own snapshot, so buyers never lose what they paid for.
func DeleteStoreProduct(productId int, userId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		product := &StoreProduct{}
		if err := lockForUpdate(tx).Where("id = ?", productId).First(product).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStoreProductNotFound
			}
			return err
		}
		if product.UserId != userId {
			return ErrStoreProductNotOwned
		}
		if err := tx.Where("product_id = ? AND status = ?", productId, StoreCdkStatusUnsold).Delete(&StoreCdk{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", productId).Delete(&StoreProduct{}).Error
	})
}

// storeCodeHash is the dedupe key of a code: the unique index lives on the
// hash so codes of any length stay insertable and comparable in one column.
func storeCodeHash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// ImportStoreCdks appends codes to an owned product and returns how many were
// stored and how many were already present (in the input or in the database).
func ImportStoreCdks(productId int, userId int, rawCodes string) (int, int, error) {
	codes, err := ParseStoreCodes(rawCodes)
	if err != nil {
		return 0, 0, err
	}
	product := &StoreProduct{}
	if err := DB.Where("id = ?", productId).First(product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, 0, ErrStoreProductNotFound
		}
		return 0, 0, err
	}
	if product.UserId != userId {
		return 0, 0, ErrStoreProductNotOwned
	}

	now := common.GetTimestamp()
	rows := make([]StoreCdk, 0, len(codes))
	for _, code := range codes {
		rows = append(rows, StoreCdk{
			ProductId: productId,
			Code:      code,
			CodeHash:  storeCodeHash(code),
			Status:    StoreCdkStatusUnsold,
			CreatedAt: now,
		})
	}
	// The unique index makes the insert idempotent: a code already stored for
	// any product is skipped instead of sold twice.
	result := DB.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 100)
	if result.Error != nil {
		return 0, 0, result.Error
	}
	imported := int(result.RowsAffected)
	duplicated := len(codes) - imported
	if duplicated < 0 {
		duplicated = 0
	}
	if imported > 0 {
		if err := DB.Model(&StoreProduct{}).Where("id = ?", productId).
			Update("cdk_count", gorm.Expr("cdk_count + ?", imported)).Error; err != nil {
			return imported, duplicated, err
		}
	}
	return imported, duplicated, nil
}

// PurchaseStoreProduct charges the buyer, delivers one code and credits the
// seller minus the platform fee, all inside one transaction: either the buyer
// is charged and inventory leaves with an order, or nothing changes.
func PurchaseStoreProduct(buyerUserId int, productId int) (*StoreOrder, error) {
	var order *StoreOrder
	err := DB.Transaction(func(tx *gorm.DB) error {
		product := &StoreProduct{}
		if err := lockForUpdate(tx).Where("id = ?", productId).First(product).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStoreProductNotFound
			}
			return err
		}
		if product.Status != StoreProductStatusOnSale {
			return ErrStoreProductOffShelf
		}
		if product.UserId == buyerUserId {
			return ErrStoreSelfPurchase
		}
		if product.Price <= 0 || product.Price > StoreMaxPriceQuota {
			return ErrStorePriceInvalid
		}

		cdk := &StoreCdk{}
		if err := lockForUpdate(tx).
			Where("product_id = ? AND status = ?", product.Id, StoreCdkStatusUnsold).
			Order("id").First(cdk).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStoreOutOfStock
			}
			return err
		}

		fee := StoreFeeForPrice(product.Price)
		income := product.Price - fee

		// The guard in the WHERE clause keeps the wallet non-negative without a
		// read-then-write race.
		debit := tx.Model(&User{}).Where("id = ? AND quota >= ?", buyerUserId, product.Price).
			Update("quota", gorm.Expr("quota - ?", product.Price))
		if debit.Error != nil {
			return debit.Error
		}
		if debit.RowsAffected == 0 {
			return ErrStoreInsufficientQuota
		}
		credit := tx.Model(&User{}).Where("id = ? AND quota <= ?", product.UserId, common.MaxWalletQuota-income).
			Update("quota", gorm.Expr("quota + ?", income))
		if credit.Error != nil {
			return credit.Error
		}
		if credit.RowsAffected == 0 {
			return ErrStoreSellerQuotaOverflow
		}

		now := common.GetTimestamp()
		if err := tx.Model(&StoreCdk{}).Where("id = ? AND status = ?", cdk.Id, StoreCdkStatusUnsold).
			Updates(map[string]any{"status": StoreCdkStatusSold, "sold_at": now}).Error; err != nil {
			return err
		}

		order = &StoreOrder{
			OrderNo:      "SO" + strings.ReplaceAll(uuid.NewString(), "-", ""),
			ProductId:    product.Id,
			ProductName:  product.Name,
			SellerUserId: product.UserId,
			BuyerUserId:  buyerUserId,
			Price:        product.Price,
			Fee:          fee,
			SellerIncome: income,
			Code:         cdk.Code,
			CdkId:        cdk.Id,
			Status:       StoreOrderStatusPaid,
			CreatedAt:    now,
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		if err := tx.Model(&StoreCdk{}).Where("id = ?", cdk.Id).Update("order_id", order.Id).Error; err != nil {
			return err
		}
		return tx.Model(&StoreProduct{}).Where("id = ?", product.Id).
			Update("sold_count", gorm.Expr("sold_count + 1")).Error
	})
	if err != nil {
		return nil, err
	}

	// Cache and audit trail after the commit: a stale cached balance would make
	// the purchase invisible until the cache expires.
	if cacheErr := cacheDecrUserQuota(buyerUserId, int64(order.Price)); cacheErr != nil {
		common.SysLog("failed to sync store purchase debit to user quota cache: " + cacheErr.Error())
	}
	syncCreditUserQuotaCache(order.SellerUserId, order.SellerIncome, "store_sale")
	recordStoreOrderLogs(order)
	return order, nil
}

// recordStoreOrderLogs writes the buyer's charge and the seller's income into
// the log table; failures never roll back a completed purchase.
func recordStoreOrderLogs(order *StoreOrder) {
	buyerName, _ := GetUsernameById(order.BuyerUserId, false)
	if err := createLog(&Log{
		UserId:    order.BuyerUserId,
		Username:  buyerName,
		CreatedAt: order.CreatedAt,
		Type:      LogTypeConsume,
		Content:   fmt.Sprintf("商店购买「%s」（订单 %s）", order.ProductName, order.OrderNo),
		Quota:     order.Price,
		ModelName: "store",
	}); err != nil {
		common.SysLog("failed to record store purchase log: " + err.Error())
	}
	sellerName, _ := GetUsernameById(order.SellerUserId, false)
	if err := createLog(&Log{
		UserId:    order.SellerUserId,
		Username:  sellerName,
		CreatedAt: order.CreatedAt,
		Type:      LogTypeTopup,
		Content:   fmt.Sprintf("商店售出「%s」，手续费 %s", order.ProductName, logger.FormatQuota(order.Fee)),
		Quota:     order.SellerIncome,
		ModelName: "store",
	}); err != nil {
		common.SysLog("failed to record store sale log: " + err.Error())
	}
}

// ListStoreOrders lists the purchases of one buyer, newest first.
func ListStoreOrders(buyerUserId int, page int, pageSize int) ([]*StoreOrder, int64, error) {
	query := DB.Model(&StoreOrder{}).Where("buyer_user_id = ?", buyerUserId)
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	orders := make([]*StoreOrder, 0, pageSize)
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// ListStoreSales lists the sales of one merchant, newest first.
func ListStoreSales(sellerUserId int, page int, pageSize int) ([]*StoreOrder, int64, error) {
	query := DB.Model(&StoreOrder{}).Where("seller_user_id = ?", sellerUserId)
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	orders := make([]*StoreOrder, 0, pageSize)
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// StoreProductOwnerId resolves the merchant that owns a product; administrators
// use it to remove any listing without knowing who published it.
func StoreProductOwnerId(productId int) (int, error) {
	product := &StoreProduct{}
	if err := DB.Select("id", "user_id").Where("id = ?", productId).First(product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrStoreProductNotFound
		}
		return 0, err
	}
	return product.UserId, nil
}

// ListAllStoreProducts is the admin inventory view across every merchant.
func ListAllStoreProducts(keyword string, page int, pageSize int) ([]*StoreProduct, int64, error) {
	query := DB.Model(&StoreProduct{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	products := make([]*StoreProduct, 0, pageSize)
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, total, nil
}
