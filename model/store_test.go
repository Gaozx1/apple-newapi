package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// storeFeeCases covers the per-mille commission, including the rounding that
// keeps a one-quota sale worth something.
func TestStoreFeeForPrice(t *testing.T) {
	cases := []struct {
		price int
		want  int
	}{
		{price: 0, want: 0},
		{price: 1, want: 1},
		{price: 999, want: 1},
		{price: 1000, want: 1},
		{price: 500_000, want: 500},
		{price: 1_000_000, want: 1000},
		{price: StoreMaxPriceQuota, want: 5_000_000},
	}
	for _, testCase := range cases {
		assert.Equal(t, testCase.want, StoreFeeForPrice(testCase.price), "price %d", testCase.price)
	}
}

func TestParseStoreCodes(t *testing.T) {
	codes, err := ParseStoreCodes(" a \n\nb\r\n a \nb\n")
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, codes)

	_, err = ParseStoreCodes(" \n\t\n")
	assert.ErrorIs(t, err, ErrStoreNoCode)

	_, err = ParseStoreCodes(strings.Repeat("x", StoreMaxCodeLength+1))
	assert.ErrorIs(t, err, ErrStoreCodeTooLong)
}

func TestNormalizeStoreProductInput(t *testing.T) {
	name, description, err := NormalizeStoreProductInput("  卡密一号  ", "  用途说明  ", 500_000)
	require.NoError(t, err)
	assert.Equal(t, "卡密一号", name)
	assert.Equal(t, "用途说明", description)

	_, _, err = NormalizeStoreProductInput("  ", "", 500_000)
	assert.ErrorIs(t, err, ErrStoreNoInput)

	for _, price := range []int{0, -1, StoreMaxPriceQuota + 1} {
		_, _, err = NormalizeStoreProductInput("卡密", "", price)
		assert.ErrorIs(t, err, ErrStorePriceInvalid, "price %d", price)
	}
}

// resetStoreTables gives each case a clean store schema.
func resetStoreTables(t *testing.T) {
	t.Helper()
	// The option map is process global; registering a group writes into it.
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	require.NoError(t, DB.AutoMigrate(&StoreProduct{}, &StoreCdk{}, &StoreOrder{}, &User{}, &Log{},
		&StoreChannel{}, &StoreChannelOrder{}, &StoreChannelReport{}, &StoreChannelEarnings{}, &UserChannelGroup{}, &Channel{}, &Ability{}))
	for _, model := range []any{&StoreOrder{}, &StoreCdk{}, &StoreProduct{}, &User{},
		&StoreChannelReport{}, &StoreChannelOrder{}, &StoreChannel{}, &StoreChannelEarnings{}, &Channel{}, &Ability{}} {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(model).Error)
	}
	t.Cleanup(func() {
		for _, model := range []any{&StoreOrder{}, &StoreCdk{}, &StoreProduct{}, &User{},
			&StoreChannelReport{}, &StoreChannelOrder{}, &StoreChannel{}, &StoreChannelEarnings{}, &Channel{}, &Ability{}} {
			require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(model).Error)
		}
	})
}

func mustStoreUser(t *testing.T, id int, quota int) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id: id, Username: "store-user-" + string(rune('0'+id)), DisplayName: "store-user",
		AffCode: "store-aff-" + string(rune('0'+id)), Group: "default",
		Quota: quota, Status: common.UserStatusEnabled,
	}).Error)
}

func storeQuotaOf(t *testing.T, userId int) int {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("id", "quota").First(&user, userId).Error)
	return user.Quota
}

// mustStoreProduct publishes one product with the given codes for seller 2.
func mustStoreProduct(t *testing.T, price int, codes ...string) *StoreProduct {
	t.Helper()
	product := &StoreProduct{UserId: 2, Name: "卡密商品", Description: "desc", Price: price, Status: StoreProductStatusOnSale}
	require.NoError(t, CreateStoreProduct(product))
	if len(codes) == 0 {
		return product
	}
	imported, duplicated, err := ImportStoreCdks(product.Id, 2, strings.Join(codes, "\n"))
	require.NoError(t, err)
	require.Equal(t, len(codes), imported)
	require.Equal(t, 0, duplicated)
	reloaded, err := GetStoreProductById(product.Id)
	require.NoError(t, err)
	return reloaded
}

func TestPurchaseStoreProductChargesBuyerAndPaysSellerMinusFee(t *testing.T) {
	resetStoreTables(t)
	mustStoreUser(t, 1, 1_000_000)
	mustStoreUser(t, 2, 0)
	product := mustStoreProduct(t, 500_000, "CODE-A", "CODE-B")

	order, err := PurchaseStoreProduct(1, product.Id)
	require.NoError(t, err)
	assert.Equal(t, 500_000, order.Price)
	assert.Equal(t, 500, order.Fee)
	assert.Equal(t, 499_500, order.SellerIncome)
	assert.Equal(t, "CODE-A", order.Code)
	assert.Equal(t, StoreOrderStatusPaid, order.Status)

	// Buyer paid the price, seller received the price minus the per-mille fee.
	assert.Equal(t, 500_000, storeQuotaOf(t, 1))
	assert.Equal(t, 499_500, storeQuotaOf(t, 2))

	reloaded, err := GetStoreProductById(product.Id)
	require.NoError(t, err)
	assert.Equal(t, 1, reloaded.SoldCount)
	assert.Equal(t, 1, reloaded.Stock())

	var sold StoreCdk
	require.NoError(t, DB.Where("code = ?", "CODE-A").First(&sold).Error)
	assert.Equal(t, StoreCdkStatusSold, sold.Status)
	assert.Equal(t, order.Id, sold.OrderId)

	orders, total, err := ListStoreOrders(1, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, orders, 1)
	assert.Equal(t, order.OrderNo, orders[0].OrderNo)
}

func TestPurchaseStoreProductRejectsWhenBuyerCannotPay(t *testing.T) {
	resetStoreTables(t)
	mustStoreUser(t, 1, 400_000)
	mustStoreUser(t, 2, 0)
	product := mustStoreProduct(t, 500_000, "CODE-A")

	_, err := PurchaseStoreProduct(1, product.Id)
	assert.ErrorIs(t, err, ErrStoreInsufficientQuota)

	// Nothing moved: no code left stock, no credit, no order.
	assert.Equal(t, 400_000, storeQuotaOf(t, 1))
	assert.Equal(t, 0, storeQuotaOf(t, 2))
	reloaded, err := GetStoreProductById(product.Id)
	require.NoError(t, err)
	assert.Equal(t, 0, reloaded.SoldCount)
	assert.Equal(t, 1, reloaded.Stock())
	var unsold StoreCdk
	require.NoError(t, DB.Where("code = ?", "CODE-A").First(&unsold).Error)
	assert.Equal(t, StoreCdkStatusUnsold, unsold.Status)
	var orders int64
	require.NoError(t, DB.Model(&StoreOrder{}).Count(&orders).Error)
	assert.Equal(t, int64(0), orders)
}

func TestPurchaseStoreProductRequiresStock(t *testing.T) {
	resetStoreTables(t)
	mustStoreUser(t, 1, 1_000_000)
	mustStoreUser(t, 2, 0)
	product := mustStoreProduct(t, 500_000)

	_, err := PurchaseStoreProduct(1, product.Id)
	assert.ErrorIs(t, err, ErrStoreOutOfStock)
	assert.Equal(t, 1_000_000, storeQuotaOf(t, 1))
	assert.Equal(t, 0, storeQuotaOf(t, 2))
}

func TestPurchaseStoreProductRejectsSelfPurchase(t *testing.T) {
	resetStoreTables(t)
	mustStoreUser(t, 1, 0)
	mustStoreUser(t, 2, 1_000_000)
	product := mustStoreProduct(t, 500_000, "CODE-A")

	_, err := PurchaseStoreProduct(2, product.Id)
	assert.ErrorIs(t, err, ErrStoreSelfPurchase)
	assert.Equal(t, 0, storeQuotaOf(t, 1))
	assert.Equal(t, 1_000_000, storeQuotaOf(t, 2))
}

func TestImportStoreCdksSkipsStoredCodesAndCountsTheRest(t *testing.T) {
	resetStoreTables(t)
	mustStoreUser(t, 2, 0)
	product := mustStoreProduct(t, 500_000, "CODE-A", "CODE-B")

	imported, duplicated, err := ImportStoreCdks(product.Id, 2, "CODE-B\nCODE-C\nCODE-C\n")
	require.NoError(t, err)
	assert.Equal(t, 1, imported)
	assert.Equal(t, 1, duplicated)

	reloaded, err := GetStoreProductById(product.Id)
	require.NoError(t, err)
	assert.Equal(t, 3, reloaded.CdkCount)
	assert.Equal(t, 3, reloaded.Stock())
}

func TestImportStoreCdksKeepsLongCodesIntact(t *testing.T) {
	resetStoreTables(t)
	mustStoreUser(t, 1, 1_000_000)
	mustStoreUser(t, 2, 0)
	product := mustStoreProduct(t, 500_000)
	longCode := strings.Repeat("L", 512) + "-CARD-KEY"

	imported, duplicated, err := ImportStoreCdks(product.Id, 2, longCode+"\nSHORT-1\n")
	require.NoError(t, err)
	assert.Equal(t, 2, imported)
	assert.Equal(t, 0, duplicated)

	order, err := PurchaseStoreProduct(1, product.Id)
	require.NoError(t, err)
	assert.Equal(t, longCode, order.Code)
}

func TestImportStoreCdksRejectsForeignKeyMerchant(t *testing.T) {
	resetStoreTables(t)
	mustStoreUser(t, 1, 0)
	mustStoreUser(t, 2, 0)
	product := mustStoreProduct(t, 500_000, "CODE-A")

	_, _, err := ImportStoreCdks(product.Id, 1, "CODE-B")
	assert.ErrorIs(t, err, ErrStoreProductNotOwned)
}

func TestNormalizeStoreChannelOffer(t *testing.T) {
	offer, err := NormalizeStoreChannelOffer("  我的渠道  ", "  自建  ", 0, "https://up.example.com/", "sk-key", "gpt-4o, gpt-4o\nclaude-3")
	require.NoError(t, err)
	assert.Equal(t, "我的渠道", offer.Name)
	assert.Equal(t, "https://up.example.com", offer.BaseURL)
	assert.Equal(t, "gpt-4o,claude-3", offer.Models)
	assert.Equal(t, 1.0, offer.Ratio)
	assert.Equal(t, StoreChannelStatusOffShelf, offer.Status)

	_, err = NormalizeStoreChannelOffer("x", "", 0, "", "k", "m")
	assert.ErrorIs(t, err, ErrStoreChannelBadInput)

	_, err = NormalizeStoreChannelOffer("x", "", 0, "https://u", "k", "  ")
	assert.ErrorIs(t, err, ErrStoreChannelNoModels)

	_, err = NormalizeStoreChannelOffer("x", "", len(constant.ChannelBaseURLs)+5, "https://u", "k", "m")
	assert.ErrorIs(t, err, ErrStoreChannelTypeInvalid)

}

func TestCreateStoreChannelStaysPrivateUntilListed(t *testing.T) {
	resetStoreTables(t)
	mustStoreUser(t, 2, 0)
	offer, err := NormalizeStoreChannelOffer("上游 A", "", 0, "https://up.example.com", "sk-upstream", "gpt-4o,claude-3")
	require.NoError(t, err)
	offer.UserId = 2
	require.NoError(t, CreateStoreChannel(offer))

	// One channel, owned by the merchant, reachable from the private group only.
	publicGroup := UserChannelGroupPublicName(UserChannelGroupName(2))
	require.NotZero(t, offer.ChannelId)
	assert.Equal(t, publicGroup, offer.GroupName)
	var channel Channel
	require.NoError(t, DB.First(&channel, offer.ChannelId).Error)
	assert.Equal(t, publicGroup, channel.Group)
	assert.Equal(t, int64(2), countAbilities(t, channel.Id, publicGroup), "private group carries both models")
	assert.Equal(t, int64(0), countAbilities(t, channel.Id, StoreChannelPurchaseGroup), "market group is untouched until listing")

	// Listing exposes the same channel to everyone, still without duplicating it.
	require.NoError(t, SetStoreChannelListing(offer.Id, 2, true))
	var listed Channel
	require.NoError(t, DB.First(&listed, offer.ChannelId).Error)
	assert.Equal(t, publicGroup+","+StoreChannelPurchaseGroup, listed.Group)
	assert.Equal(t, int64(2), countAbilities(t, channel.Id, StoreChannelPurchaseGroup))

	// Unlisting hides it again.
	require.NoError(t, SetStoreChannelListing(offer.Id, 2, false))
	assert.Equal(t, int64(0), countAbilities(t, channel.Id, StoreChannelPurchaseGroup))
}

func TestReviewStoreChannelReportTakesOfferDown(t *testing.T) {
	resetStoreTables(t)
	mustStoreUser(t, 1, 1_000_000)
	mustStoreUser(t, 2, 0)
	mustStoreUser(t, 9, 0)
	offer, err := NormalizeStoreChannelOffer("上游 B", "", 0, "https://up.example.com", "sk-b", "gpt-4o")
	require.NoError(t, err)
	offer.UserId = 2
	require.NoError(t, CreateStoreChannel(offer))

	require.NoError(t, CreateStoreChannelReport(offer.Id, offer.ChannelId, 1, "接口全是 500"))
	reports, total, err := ListStoreChannelReports(StoreReportStatusPending, 1, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, reports, 1)

	require.NoError(t, ReviewStoreChannelReport(reports[0].Id, 9, true, "核实属实"))

	reloaded, err := GetStoreChannelById(offer.Id)
	require.NoError(t, err)
	assert.Equal(t, StoreChannelStatusTakenDown, reloaded.Status)

	var disabled Channel
	require.NoError(t, DB.First(&disabled, offer.ChannelId).Error)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, disabled.Status)
}

func TestRefreshStoreChannelHealthDisablesHighErrorRate(t *testing.T) {
	resetStoreTables(t)
	mustStoreUser(t, 1, 1_000_000)
	mustStoreUser(t, 2, 0)
	offer, err := NormalizeStoreChannelOffer("上游 C", "", 0, "https://up.example.com", "sk-c", "gpt-4o")
	require.NoError(t, err)
	offer.UserId = 2
	require.NoError(t, CreateStoreChannel(offer))
	require.NoError(t, SetStoreChannelListing(offer.Id, 2, true))
	channel := struct{ Id int }{Id: offer.ChannelId}

	now := common.GetTimestamp()
	logs := make([]Log, 0, StoreChannelHealthWindow)
	for i := 0; i < 90; i++ {
		logs = append(logs, Log{UserId: 1, ChannelId: channel.Id, Type: LogTypeError, CreatedAt: now, RequestId: common.NewRequestId()})
	}
	for i := 0; i < 10; i++ {
		logs = append(logs, Log{UserId: 1, ChannelId: channel.Id, Type: LogTypeConsume, CreatedAt: now, RequestId: common.NewRequestId()})
	}
	require.NoError(t, DB.CreateInBatches(logs, 50).Error)

	disabled, err := RefreshStoreChannelHealth()
	require.NoError(t, err)
	assert.Equal(t, []int{offer.Id}, disabled)

	reloaded, err := GetStoreChannelById(offer.Id)
	require.NoError(t, err)
	assert.Equal(t, StoreChannelStatusTakenDown, reloaded.Status)
	assert.Equal(t, 100, reloaded.RequestCount)
	assert.Equal(t, 90, reloaded.ErrorCount)
}

func TestCreditStoreChannelRevenuePaysSellerMinusFeeOnce(t *testing.T) {
	resetStoreTables(t)
	mustStoreUser(t, 1, 1_000_000)
	mustStoreUser(t, 2, 0)
	group, err := CreateUserChannelGroup(2, "wb", 1.2, "测试分组")
	require.NoError(t, err)
	offer, err := NormalizeStoreChannelOffer("上游 D", "", 0, "https://up.example.com", "sk-d", "gpt-4o")
	require.NoError(t, err)
	offer.UserId = 2
	offer.GroupId = group.Id
	require.NoError(t, CreateStoreChannel(offer))

	require.NoError(t, CreditStoreChannelRevenue(offer.ChannelId, 1, 100_000, "req-earn-1", "gpt-4o"))
	// 1% commission, rounded up: the merchant keeps 99_000.
	assert.Equal(t, 99_000, storeQuotaOf(t, 2))

	// The same settled request never pays twice.
	require.NoError(t, CreditStoreChannelRevenue(offer.ChannelId, 1, 100_000, "req-earn-1", "gpt-4o"))
	assert.Equal(t, 99_000, storeQuotaOf(t, 2))

	earnings, total, err := ListStoreChannelEarnings(2, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, earnings, 1)
	assert.Equal(t, 1_000, earnings[0].Fee)
	assert.Equal(t, 1.2, earnings[0].Ratio)
}

func TestCreditStoreChannelRevenueIgnoresPlainChannels(t *testing.T) {
	resetStoreTables(t)
	mustStoreUser(t, 1, 1_000_000)
	mustStoreUser(t, 2, 0)

	require.NoError(t, CreditStoreChannelRevenue(4242, 1, 100_000, "req-plain-1", "gpt-4o"))
	assert.Equal(t, 0, storeQuotaOf(t, 2))
}

func countAbilities(t *testing.T, channelId int, group string) int64 {
	t.Helper()
	count := int64(0)
	require.NoError(t, DB.Model(&Ability{}).Where("channel_id = ? AND "+commonGroupCol+" = ?", channelId, group).Count(&count).Error)
	return count
}
