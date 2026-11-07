package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func restorePaymentChannelsForTest(t *testing.T) {
	t.Helper()
	channels, _ := common.Marshal(operation_setting.GetPaymentChannels())
	methods := operation_setting.PayMethods
	t.Cleanup(func() {
		if string(channels) == "null" {
			channels = []byte("[]")
		}
		require.NoError(t, operation_setting.UpdatePaymentChannels(string(channels)))
		operation_setting.PayMethods = methods
	})
}

func TestPaymentChannelConfigurationAndRouting(t *testing.T) {
	restorePaymentChannelsForTest(t)
	operation_setting.PayMethods = nil
	config := `[{"id":"gmpay","name":"USDT","address":"http://gateway.example/payments/epay/v1/order/create-transaction","merchant_id":"1001","key":"original-secret","enabled":true}]`
	require.NoError(t, operation_setting.UpdatePaymentChannels(config))
	method := map[string]string{"name": "TRON", "type": "usdt-tron", "gateway_type": "usdt.tron", "channel_id": "gmpay"}
	operation_setting.PayMethods = []map[string]string{method}
	snapshot, encoded, err := operation_setting.NewEpayOrderSnapshot(method)
	require.NoError(t, err)
	assert.Equal(t, "usdt.tron", snapshot.GatewayType)
	masked := operation_setting.MaskPaymentChannels(config)
	assert.NotContains(t, masked, "original-secret")
	assert.Contains(t, masked, `"key_configured":true`)
	normalized, err := operation_setting.NormalizePaymentChannels(masked)
	require.NoError(t, err)
	assert.Contains(t, normalized, "original-secret")
	_, err = operation_setting.NormalizePaymentChannels("[]")
	require.Error(t, err, "bound channels cannot be removed")
	require.NoError(t, operation_setting.UpdatePaymentChannels(strings.ReplaceAll(strings.ReplaceAll(config, "original-secret", "replacement-secret"), `"enabled":true`, `"enabled":false`)))
	_, err = operation_setting.ResolveEpayChannel(method)
	require.Error(t, err, "disabled binding cannot fall back to legacy credentials")
	loaded, err := operation_setting.ReadEpayOrderSnapshot(encoded, method["type"])
	require.NoError(t, err)
	assert.Equal(t, "original-secret", loaded.Channel.Key, "pending order keeps original credentials")
	require.Error(t, operation_setting.ValidatePaymentMethods(`[{"name":"A","type":"alipay"},{"name":"B","type":"alipay"}]`))
	require.Error(t, operation_setting.ValidatePaymentMethods(`[{"name":"A","type":"alipay","channel_id":"missing"}]`))
	require.Error(t, operation_setting.ValidatePaymentMethods(`[{"name":"A","type":"stripe","channel_id":"gmpay"}]`))
	require.Error(t, operation_setting.ValidatePaymentMethods(`[{"name":"A","type":"alipay","min_topup":"NaN"}]`))
	_, err = operation_setting.NormalizePaymentChannels(strings.ReplaceAll(config, "http://gateway.example/payments/epay/v1/order/create-transaction", "file:///etc/passwd"))
	require.Error(t, err)
}

func signedPaymentCallback(params url.Values, key string) string {
	fields := make([]string, 0, len(params))
	for _, name := range []string{"money", "name", "out_trade_no", "pid", "trade_status", "type"} {
		if value := params.Get(name); value != "" {
			fields = append(fields, name+"="+value)
		}
	}
	params.Set("sign", epay.MD5String(strings.Join(fields, "&"), key))
	params.Set("sign_type", "MD5")
	return params.Encode()
}

// The released order schema before payment snapshots were introduced.
type legacyChannelTopUp struct {
	Id              int `json:"id"`
	UserId          int `gorm:"index"`
	Amount          int64
	Money           float64
	TradeNo         string `gorm:"unique;type:varchar(255);index"`
	PaymentMethod   string `gorm:"type:varchar(50)"`
	PaymentProvider string `gorm:"type:varchar(50);default:''"`
	CreateTime      int64
	CompleteTime    int64
	Status          string
}

type legacyChannelSubscriptionOrder struct {
	Id              int
	UserId          int `gorm:"index"`
	PlanId          int `gorm:"index"`
	Money           float64
	TradeNo         string `gorm:"unique;type:varchar(255);index"`
	PaymentMethod   string `gorm:"type:varchar(50)"`
	PaymentProvider string `gorm:"type:varchar(50);default:''"`
	Status          string
	CreateTime      int64
	CompleteTime    int64
	ProviderPayload string `gorm:"type:text"`
}

func TestPaymentChannelsDatabaseMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restorePaymentChannelsForTest(t)
	confirmPaymentComplianceForTest(t)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	t.Cleanup(func() { model.DB, model.LOG_DB = previousDB, previousLogDB })
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				// Schema recreation happens in one process here; avoid pgx reusing
				// SELECT * plans from the previous schema after a column is added.
				driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "payment_channel_test_"}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			defer sqlDB.Close()
			var version string
			versionSQL := "SELECT version()"
			if dialect == "sqlite" {
				versionSQL = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(versionSQL).Scan(&version).Error)
			t.Logf("database version: %s", version)
			model.DB = db
			for _, upgrade := range []bool{false, true} {
				t.Run(fmt.Sprintf("upgrade=%v", upgrade), func(t *testing.T) {
					require.NoError(t, db.Migrator().DropTable(&model.TopUp{}, &model.SubscriptionOrder{}))
					if upgrade {
						require.NoError(t, db.Table("payment_channel_test_top_ups").AutoMigrate(&legacyChannelTopUp{}))
						require.NoError(t, db.Table("payment_channel_test_subscription_orders").AutoMigrate(&legacyChannelSubscriptionOrder{}))
						require.NoError(t, db.Table("payment_channel_test_top_ups").Create(&legacyChannelTopUp{TradeNo: "legacy", Money: 1, Status: common.TopUpStatusPending, PaymentProvider: model.PaymentProviderEpay}).Error)
						require.NoError(t, db.Table("payment_channel_test_subscription_orders").Create(&legacyChannelSubscriptionOrder{TradeNo: "legacy-sub", Money: 1, Status: common.TopUpStatusPending, PaymentProvider: model.PaymentProviderEpay}).Error)
					}
					for range 2 {
						require.NoError(t, db.AutoMigrate(&model.TopUp{}, &model.SubscriptionOrder{}))
					}
					if upgrade {
						legacy := model.GetTopUpByTradeNo("legacy")
						require.NotNil(t, legacy)
						assert.Equal(t, 1.0, legacy.Money)
						assert.Empty(t, legacy.PaymentSnapshot)
						assert.NotNil(t, model.GetSubscriptionOrderByTradeNo("legacy-sub"))
					}
					channel := operation_setting.PaymentChannel{ID: "channel-a", Name: "A", Address: "https://pay.example", MerchantID: "1001", Key: "channel-a-secret", Enabled: true}
					data, _ := common.Marshal([]operation_setting.PaymentChannel{channel})
					require.NoError(t, operation_setting.UpdatePaymentChannels(string(data)))
					method := map[string]string{"type": "usdt-tron", "channel_id": "channel-a", "gateway_type": "usdt.tron", "name": "TRON"}
					operation_setting.PayMethods = []map[string]string{method}
					_, snapshot, err := operation_setting.NewEpayOrderSnapshot(method)
					require.NoError(t, err)
					topUp := model.TopUp{TradeNo: "channel-order", Money: 6.8, PaymentMethod: "usdt-tron", PaymentProvider: model.PaymentProviderEpay, PaymentSnapshot: snapshot, Status: common.TopUpStatusPending}
					require.NoError(t, topUp.Insert())
					require.NoError(t, (&model.SubscriptionOrder{TradeNo: "channel-sub", Money: 6.8, PaymentMethod: "usdt-tron", PaymentProvider: model.PaymentProviderEpay, PaymentSnapshot: snapshot, Status: common.TopUpStatusPending}).Insert())
					loaded := model.GetTopUpByTradeNo(topUp.TradeNo)
					require.NotNil(t, loaded)
					assert.Equal(t, snapshot, loaded.PaymentSnapshot)
					serialized, _ := common.Marshal(loaded)
					assert.NotContains(t, string(serialized), "channel-a-secret")
					assert.NotContains(t, string(serialized), "payment_snapshot")
					// Current binding/key/availability must not override pending orders.
					require.NoError(t, operation_setting.UpdatePaymentChannels("[]"))
					operation_setting.PayMethods = nil
					for _, subscription := range []bool{false, true} {
						tradeNo := "channel-order"
						if subscription {
							tradeNo = "channel-sub"
						}
						for _, tc := range []struct {
							name, key, money, pid, kind string
							duplicate                   bool
							valid                       bool
						}{
							{"original channel", "channel-a-secret", "6.80", "1001", "usdt.tron", false, true},
							{"other channel signature", "other-secret", "6.80", "1001", "usdt.tron", false, false},
							{"underpayment", "channel-a-secret", "0.01", "1001", "usdt.tron", false, false},
							{"overpayment", "channel-a-secret", "6.81", "1001", "usdt.tron", false, false},
							{"wrong merchant", "channel-a-secret", "6.80", "1002", "usdt.tron", false, false},
							{"wrong method", "channel-a-secret", "6.80", "1001", "alipay", false, false},
							{"duplicate parameter", "channel-a-secret", "6.80", "1001", "usdt.tron", true, false},
						} {
							params := url.Values{"money": {tc.money}, "name": {"item"}, "out_trade_no": {tradeNo}, "pid": {tc.pid}, "trade_status": {"WAIT_BUYER_PAY"}, "type": {tc.kind}}
							body := signedPaymentCallback(params, tc.key)
							if tc.duplicate {
								body += "&money=6.80"
							}
							ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
							ctx.Request = httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader(body))
							ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
							_, method, err := verifyEpayOrderCallback(ctx, subscription)
							if tc.valid {
								require.NoError(t, err, tc.name)
								assert.Equal(t, "usdt-tron", method)
							} else {
								require.Error(t, err, tc.name)
							}
						}
						assert.Equal(t, common.TopUpStatusPending, model.GetTopUpByTradeNo("channel-order").Status)
					}
				})
			}
			require.NoError(t, db.Migrator().DropTable(&model.TopUp{}, &model.SubscriptionOrder{}))
		})
	}
}
