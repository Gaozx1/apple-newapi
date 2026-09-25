package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupScopedLimitTest forces the in-memory limiter path and enables the
// critical rate limit switch the scoped limiters check.
func setupScopedLimitTest(t *testing.T) {
	t.Helper()
	prevRedis := common.RedisEnabled
	prevLoginEnable := common.LoginRateLimitEnable
	prevLoginNum := common.LoginRateLimitNum
	prevLoginDur := common.LoginRateLimitDuration
	prevMailEnable := common.EmailTargetRateLimitEnable
	prevMailNum := common.EmailTargetRateLimitNum
	prevMailDur := common.EmailTargetRateLimitDuration
	prevTTL := common.RateLimitKeyExpirationDuration

	common.RedisEnabled = false
	common.LoginRateLimitEnable = true
	common.LoginRateLimitNum = 10
	common.LoginRateLimitDuration = 15 * 60
	common.EmailTargetRateLimitEnable = true
	common.EmailTargetRateLimitNum = 1
	common.EmailTargetRateLimitDuration = 60
	if common.RateLimitKeyExpirationDuration <= 0 {
		common.RateLimitKeyExpirationDuration = 60
	}
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() {
		common.RedisEnabled = prevRedis
		common.LoginRateLimitEnable = prevLoginEnable
		common.LoginRateLimitNum = prevLoginNum
		common.LoginRateLimitDuration = prevLoginDur
		common.EmailTargetRateLimitEnable = prevMailEnable
		common.EmailTargetRateLimitNum = prevMailNum
		common.EmailTargetRateLimitDuration = prevMailDur
		common.RateLimitKeyExpirationDuration = prevTTL
	})
}

// TestAccountLoginRateLimitBlocksPerAccount verifies that attempts against one
// account are throttled once the per-account budget is exhausted.
func TestAccountLoginRateLimitBlocksPerAccount(t *testing.T) {
	setupScopedLimitTest(t)

	router := gin.New()
	router.POST("/login", AccountLoginRateLimit(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "bad credentials"})
	})

	blocked := 0
	budget := common.LoginRateLimitNum
	for i := 0; i < budget+5; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/login",
			strings.NewReader(`{"username":"victim","password":"guess"}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			blocked++
		}
	}
	t.Logf("同一账号尝试 %d 次，被限流 %d 次（预算 %d）",
		common.LoginRateLimitNum+5, blocked, common.LoginRateLimitNum)
	assert.Positive(t, blocked, "同一账号超预算后必须被限流")
}

// TestAccountLoginRateLimitIsolatesAccounts is the regression test for the CDN
// problem: users behind the SAME source IP but using DIFFERENT accounts must not
// be throttled because of each other.
func TestAccountLoginRateLimitIsolatesAccounts(t *testing.T) {
	setupScopedLimitTest(t)

	router := gin.New()
	router.POST("/login", AccountLoginRateLimit(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": false})
	})

	// Exhaust the budget for account A.
	budget := common.LoginRateLimitNum
	for i := 0; i < budget+3; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/login",
			strings.NewReader(`{"username":"accountA","password":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		// 所有请求来自同一个 IP —— 模拟 CDN 共享出口
		req.RemoteAddr = "203.0.113.7:1234"
		router.ServeHTTP(rec, req)
	}

	// Account A is now throttled.
	recA := httptest.NewRecorder()
	reqA := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"username":"accountA","password":"x"}`))
	reqA.Header.Set("Content-Type", "application/json")
	reqA.RemoteAddr = "203.0.113.7:1234"
	router.ServeHTTP(recA, reqA)

	// Account B, same IP, must still be allowed.
	recB := httptest.NewRecorder()
	reqB := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"username":"accountB","password":"x"}`))
	reqB.Header.Set("Content-Type", "application/json")
	reqB.RemoteAddr = "203.0.113.7:1234" // 同一个出口 IP
	router.ServeHTTP(recB, reqB)

	t.Logf("共享同一出口 IP：账号A=%d（应 429）, 账号B=%d（应 200）", recA.Code, recB.Code)
	assert.Equal(t, http.StatusTooManyRequests, recA.Code, "超预算账号应被限流")
	assert.Equal(t, http.StatusOK, recB.Code, "同 IP 的其他账号不得被牵连（CDN 场景回归）")
}

// TestEmailTargetRateLimitBlocksPerRecipient verifies mail bombing is stopped at
// the recipient, regardless of how many distinct source IPs the attacker uses.
func TestEmailTargetRateLimitBlocksPerRecipient(t *testing.T) {
	setupScopedLimitTest(t)

	router := gin.New()
	router.GET("/verification", EmailTargetRateLimit(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	blocked := 0
	const attempts = 10
	for i := 0; i < attempts; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet,
			"/verification?email=target@example.com", nil)
		// 每次都换一个来源 IP —— 模拟攻击者轮换 IP 绕过 IP 限流
		req.RemoteAddr = fmt.Sprintf("198.51.100.%d:1234", i+1)
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			blocked++
		}
	}
	t.Logf("轮换 %d 个来源 IP 轰炸同一邮箱，被拦截 %d 次", attempts, blocked)
	assert.Positive(t, blocked, "同一收件人超预算后必须被限流，且不依赖来源 IP")
}

// TestEmailTargetRateLimitAllowsDifferentRecipients ensures the recipient-scoped
// limit does not become a global cap: distinct recipients stay independent.
func TestEmailTargetRateLimitAllowsDifferentRecipients(t *testing.T) {
	setupScopedLimitTest(t)

	router := gin.New()
	router.GET("/verification", EmailTargetRateLimit(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// Same source IP, different recipients — must all pass.
	codes := make([]int, 0, 3)
	for _, addr := range []string{"a@example.com", "b@example.com", "c@example.com"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/verification?email="+addr, nil)
		req.RemoteAddr = "203.0.113.9:5555" // 同一来源
		router.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}
	t.Logf("同一来源、不同收件人的状态码: %v（应全为 200）", codes)
	for _, code := range codes {
		assert.Equal(t, http.StatusOK, code, "不同收件人之间不得互相牵连")
	}
}

// TestEmailTargetRateLimitScopeIsNormalized ensures case/whitespace variants of
// the same address share one bucket and cannot be used to bypass the limit.
func TestEmailTargetRateLimitScopeIsNormalized(t *testing.T) {
	setupScopedLimitTest(t)
	require.Equal(t, "user@example.com", normalizeRateLimitScope("  User@Example.COM "))
}

// TestScopedLimitSkipsWhenScopeMissing documents the fail-open behavior for
// requests that carry no usable scope.
func TestScopedLimitSkipsWhenScopeMissing(t *testing.T) {
	setupScopedLimitTest(t)

	router := gin.New()
	router.POST("/login", AccountLoginRateLimit(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": false})
	})

	budget := common.LoginRateLimitNum
	for i := 0; i < budget+5; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)
		require.NotEqual(t, http.StatusTooManyRequests, rec.Code,
			"无可用的账号作用域时应放行，由端点自身校验返回错误")
	}
}

// TestScopedLimitLeavesRequestBodyReadable is the regression test for the login
// outage: the limiter reads the JSON body to find the account, and every later
// middleware and the endpoint handler must still be able to read it.
func TestScopedLimitLeavesRequestBodyReadable(t *testing.T) {
	setupScopedLimitTest(t)

	router := gin.New()
	router.POST("/login", AccountLoginRateLimit(), func(c *gin.Context) {
		var payload struct {
			Username string `json:"username"`
		}
		if err := common.DecodeJson(c.Request.Body, &payload); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		c.JSON(http.StatusOK, gin.H{"username": payload.Username})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"username":"body-probe-user","password":"guess"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "限流中间件读体后，端点仍须能读到请求体")
	assert.Contains(t, rec.Body.String(), "body-probe-user")
}
