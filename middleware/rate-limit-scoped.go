package middleware

import (
	"fmt"
	"io"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

// Rate limiting dimensions that do not depend on the client IP.
//
// The default limiters key on c.ClientIP(). Behind a CDN whose origin-return
// range is not listed in TRUSTED_PROXIES, ClientIP() resolves to the CDN node
// address, so every user behind that node shares a single bucket and unrelated
// traffic exhausts it — the 429 storm that led this deployment to remove rate
// limiting altogether. The limiters below key on the *account* or on the
// *target mailbox* instead: values that do not change with how the request
// reached the origin.
//
// Each dimension matches the object the endpoint actually protects:
//
//   - AccountLoginRateLimit: the protected object is the account, so the key is
//     the username. Throttles credential stuffing and password brute force.
//   - EmailTargetRateLimit:  the protected object is the recipient, so the key is
//     the destination address. Throttles mail bombing regardless of the sender's
//     origin, and cannot punish users who merely share a CDN egress address.

const (
	// LoginRateLimitMark namespaces account-scoped authentication limits.
	LoginRateLimitMark = "login"
	// EmailTargetRateLimitMark namespaces recipient-scoped mail limits.
	EmailTargetRateLimitMark = "mailto"
)

// Per-account authentication budget. Ten attempts per fifteen minutes is
// generous for a human mistyping a password, and makes online brute force
// impractical (≈960 attempts/day per account).
const (
	accountLoginMaxRequests = 10
	accountLoginDuration    = 15 * 60
)

// Per-recipient mail budget. One mail per minute to a given address: enough for
// a legitimate resend after a typo, far too slow to exhaust a sending quota.
const (
	emailTargetMaxRequests = 1
	emailTargetDuration    = 60
)

// redisScopedRateLimitKey builds a namespaced key for a caller-supplied scope
// value (username or target mailbox) instead of an IP or numeric user id.
func redisScopedRateLimitKey(mark, scope string) string {
	return fmt.Sprintf("%s:%s:%s", redisRateLimitNamespace, mark, scope)
}

// normalizeRateLimitScope canonicalizes a scope value so that
// "User@Example.com " and "user@example.com" share one bucket. Callers treat an
// empty result as "no usable scope".
func normalizeRateLimitScope(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// scopedRateLimiter performs one fixed-window check for an already resolved
// scope value, writing the 429 response when the limit is exceeded.
//
// When scope is empty there is nothing to key on, so the request is allowed
// through: reaching the limiter already required passing the endpoint's own
// input validation, and failing closed here would turn malformed requests into
// confusing 429s.
func scopedRateLimiter(c *gin.Context, maxRequestNum int, duration int64, mark, scope string) {
	if scope == "" {
		c.Next()
		return
	}
	if common.RedisEnabled {
		allowed, _, ttlSeconds, err := redisFixedWindowTake(
			c.Request.Context(),
			redisScopedRateLimitKey(mark, scope),
			maxRequestNum,
			duration,
		)
		if err != nil {
			// Redis unavailable: fall back to the in-memory limiter rather than
			// rejecting the request, mirroring EmailVerificationRateLimit.
			memoryScopedRateLimiter(c, maxRequestNum, duration, mark, scope)
			return
		}
		if !allowed {
			writeRateLimited(c, ttlSeconds)
			return
		}
		c.Next()
		return
	}
	memoryScopedRateLimiter(c, maxRequestNum, duration, mark, scope)
}

func memoryScopedRateLimiter(c *gin.Context, maxRequestNum int, duration int64, mark, scope string) {
	key := fmt.Sprintf("%s:%s", mark, scope)
	if !inMemoryRateLimiter.Request(key, maxRequestNum, duration) {
		writeRateLimited(c, duration)
		return
	}
	c.Next()
}

// AccountLoginRateLimit throttles authentication attempts per account name
// instead of per client IP.
//
// The username is read from the JSON body via common.GetRequestBody, which
// caches and rewinds the body, so the login handler still reads it normally.
// When the body carries no usable username the check is skipped and the
// endpoint's own validation produces the error response.
//
// Intended for unauthenticated endpoints such as /api/user/login and
// /api/user/register. Order it before the handler; it does not require auth.
func AccountLoginRateLimit() gin.HandlerFunc {
	if !common.CriticalRateLimitEnable {
		return defNext
	}
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
	return func(c *gin.Context) {
		scope := normalizeRateLimitScope(accountScopeFromBody(c))
		scopedRateLimiter(c, accountLoginMaxRequests, accountLoginDuration, LoginRateLimitMark, scope)
	}
}

// accountScopeFromBody extracts the account identifier from a JSON request body.
// It accepts either "username" (login, register) or "email" (login by address),
// both of which identify the account being attacked.
func accountScopeFromBody(c *gin.Context) string {
	if c.Request == nil || c.Request.Body == nil {
		return ""
	}
	body, err := common.GetRequestBody(c)
	if err != nil {
		return ""
	}
	reader, ok := body.(io.Reader)
	if !ok {
		return ""
	}
	var payload struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	if err := common.DecodeJson(reader, &payload); err != nil {
		return ""
	}
	if scope := normalizeRateLimitScope(payload.Username); scope != "" {
		return scope
	}
	return normalizeRateLimitScope(payload.Email)
}

// EmailTargetRateLimit limits how many mails may be sent to one recipient, keyed
// on the "email" query parameter.
//
// This is the correct dimension for mail-bombing protection: the harm falls on
// the recipient's inbox and on the site's sending quota, so the limit must
// follow the recipient. It cannot be bypassed by rotating source IPs, and it
// cannot punish users who merely share a CDN egress address.
func EmailTargetRateLimit() gin.HandlerFunc {
	if !common.CriticalRateLimitEnable {
		return defNext
	}
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
	return func(c *gin.Context) {
		scopedRateLimiter(c, emailTargetMaxRequests, emailTargetDuration,
			EmailTargetRateLimitMark, normalizeRateLimitScope(c.Query("email")))
	}
}

// EmailTargetFromBodyRateLimit is the JSON-body counterpart of
// EmailTargetRateLimit, for endpoints that take the address in the body.
func EmailTargetFromBodyRateLimit() gin.HandlerFunc {
	if !common.CriticalRateLimitEnable {
		return defNext
	}
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
	return func(c *gin.Context) {
		var payload struct {
			Email string `json:"email"`
		}
		scope := ""
		if body, err := common.GetRequestBody(c); err == nil {
			if reader, ok := body.(io.Reader); ok {
				if err := common.DecodeJson(reader, &payload); err == nil {
					scope = normalizeRateLimitScope(payload.Email)
				}
			}
		}
		scopedRateLimiter(c, emailTargetMaxRequests, emailTargetDuration, EmailTargetRateLimitMark, scope)
	}
}
