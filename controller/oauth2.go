package controller

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// OAuth 2.0 authorization server.
//
// This lets third-party applications (registered as OAuthClients) obtain
// access tokens on behalf of new-api users via the authorization-code grant,
// and lets first-party/trusted services exchange client credentials directly
// (client_credentials grant). Issued access tokens are accepted as bearer
// credentials at /oauth2/userinfo and can be validated by other services.
//
// Endpoints:
//   - GET  /oauth2/authorize  (user login + consent, then redirect with code)
//   - POST /oauth2/token      (exchange code / client_credentials for a token)
//   - GET  /oauth2/userinfo    (returns the authorized user's profile)

const oauthAuthorizationCodeTTL = 10 * time.Minute
const oauthAccessTokenTTL = 2 * time.Hour

// ----------------------------------------------------------------------------
// Authorization endpoint (authorization-code grant)
// ----------------------------------------------------------------------------

// OAuthAuthorize renders a minimal consent page for the logged-in user and
// (on POST) issues a one-time authorization code. The user must already be
// authenticated via a session cookie (browser flow).
func OAuthAuthorize(c *gin.Context) {
	clientId := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	scope := c.Query("scope")
	state := c.Query("state")

	client, err := model.GetOAuthClientByClientId(clientId)
	if err != nil {
		common.ApiErrorMsg(c, "无效的 OAuth 客户端")
		return
	}
	if !client.Enabled {
		common.ApiErrorMsg(c, "OAuth 客户端已被禁用")
		return
	}
	if redirectURI == "" {
		redirectURI = client.RedirectURIListDefault()
	}
	if !client.IsValidRedirectURI(redirectURI) {
		common.ApiErrorMsg(c, "非法的重定向地址")
		return
	}

	userId := c.GetInt("id")
	if userId == 0 {
		// Not logged in: bounce to the SPA login with a return hint. The SPA
		// handles login and sends the user back to /oauth2/authorize.
		c.Redirect(http.StatusFound, "/login?oauth=1")
		return
	}

	if c.Request.Method == http.MethodPost {
		decision := c.PostForm("decision")
		if decision != "allow" {
			redirectError(c, redirectURI, state, "access_denied", "用户拒绝授权")
			return
		}
		code := common.GetUUID()
		authCode := &model.OAuthAuthorizationCode{
			Code:        code,
			ClientId:    client.ClientId,
			UserId:      userId,
			RedirectURI: redirectURI,
			Scopes:      scope,
			ExpiresAt:   time.Now().Add(oauthAuthorizationCodeTTL).Unix(),
		}
		if err := model.CreateOAuthAuthorizationCode(authCode); err != nil {
			common.ApiError(c, err)
			return
		}
		q := url.Values{}
		q.Set("code", code)
		if state != "" {
			q.Set("state", state)
		}
		c.Redirect(http.StatusFound, redirectURI+"?"+q.Encode())
		return
	}

	// GET: show a simple consent page.
	c.Data(http.StatusOK, "text/html; charset=utf-8", renderOAuthConsent(client, scope, state, redirectURI))
}

// ----------------------------------------------------------------------------
// Token endpoint
// ----------------------------------------------------------------------------

// OAuthToken exchanges an authorization code (or client credentials) for an
// access token. Implements RFC 6749 §4.1.3 and §4.4.
func OAuthToken(c *gin.Context) {
	// Support both Basic auth (client_id:client_secret) and body params.
	clientId, clientSecret := clientCredentialsFromRequest(c)
	if clientId == "" {
		clientId = c.PostForm("client_id")
		clientSecret = c.PostForm("client_secret")
	}

	client, err := model.GetOAuthClientByClientId(clientId)
	if err != nil || !client.Enabled {
		oauthTokenError(c, http.StatusUnauthorized, "invalid_client", "客户端验证失败")
		return
	}
	if client.ClientSecret != clientSecret {
		oauthTokenError(c, http.StatusUnauthorized, "invalid_client", "客户端密钥错误")
		return
	}

	grantType := c.PostForm("grant_type")
	switch grantType {
	case "authorization_code":
		handleAuthorizationCodeGrant(c, client)
	case "client_credentials":
		handleClientCredentialsGrant(c, client)
	default:
		oauthTokenError(c, http.StatusBadRequest, "unsupported_grant_type", "不支持的授权类型")
	}
}

func handleAuthorizationCodeGrant(c *gin.Context, client *model.OAuthClient) {
	code := c.PostForm("code")
	redirectURI := c.PostForm("redirect_uri")
	if code == "" {
		oauthTokenError(c, http.StatusBadRequest, "invalid_request", "缺少授权码")
		return
	}
	authCode, err := model.ConsumeOAuthAuthorizationCode(code, client.ClientId)
	if err != nil {
		oauthTokenError(c, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}
	if redirectURI != "" && authCode.RedirectURI != redirectURI {
		oauthTokenError(c, http.StatusBadRequest, "invalid_grant", "重定向地址不匹配")
		return
	}
	issueAccessToken(c, client, authCode.UserId, authCode.Scopes)
}

func handleClientCredentialsGrant(c *gin.Context, client *model.OAuthClient) {
	// Client credentials act on behalf of the client owner (the admin who
	// registered the client). This is intended for first-party services.
	issueAccessToken(c, client, client.UserId, c.PostForm("scope"))
}

func issueAccessToken(c *gin.Context, client *model.OAuthClient, userId int, scopes string) {
	if client.Scopes != "" {
		for _, requested := range strings.Fields(scopes) {
			if requested == "" {
				continue
			}
			if !client.HasScope(requested) {
				oauthTokenError(c, http.StatusBadRequest, "invalid_scope", "请求的权限超出客户端允许范围")
				return
			}
		}
	}
	token := &model.OAuthAccessToken{
		AccessToken: common.GetUUID(),
		ClientId:    client.ClientId,
		UserId:      userId,
		Scopes:      scopes,
		ExpiresAt:   time.Now().Add(oauthAccessTokenTTL).Unix(),
	}
	if err := model.CreateOAuthAccessToken(token); err != nil {
		oauthTokenError(c, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token": token.AccessToken,
		"token_type":   "Bearer",
		"expires_in":   int64(oauthAccessTokenTTL.Seconds()),
		"scope":        scopes,
	})
}

// ----------------------------------------------------------------------------
// UserInfo endpoint
// ----------------------------------------------------------------------------

// OAuthUserInfo returns the profile of the user that owns a bearer access
// token. Mirrors the OpenID Connect userinfo contract.
//
// Tiered billing: the first 50 calls per day are free; calls 51-100 cost
// $0.001, 101-200 cost $0.002, and 201+ cost $0.003. Quota is deducted from
// the user's wallet each call according to the tier.
func OAuthUserInfo(c *gin.Context) {
	token := bearerToken(c)
	if token == "" {
		oauthTokenError(c, http.StatusUnauthorized, "invalid_token", "缺少访问令牌")
		return
	}
	accessToken, err := model.GetOAuthAccessTokenByToken(token)
	if err != nil {
		oauthTokenError(c, http.StatusUnauthorized, "invalid_token", "访问令牌无效或已过期")
		return
	}
	if accessToken.ExpiresAt < time.Now().Unix() {
		oauthTokenError(c, http.StatusUnauthorized, "invalid_token", "访问令牌已过期")
		return
	}
	user, err := model.GetUserById(accessToken.UserId, false)
	if err != nil || user.Status != common.UserStatusEnabled {
		oauthTokenError(c, http.StatusUnauthorized, "invalid_token", "用户不可用")
		return
	}

	// Tiered billing — compute price for this call, deduct quota, then count.
	today := time.Now().Format("2006-01-02")
	usage, err := model.GetOAuthDailyUsage(user.Id, today)
	if err != nil {
		oauthTokenError(c, http.StatusInternalServerError, "server_error", "获取用量失败")
		return
	}
	price := oauthTierPrice(usage.CallCount) // price in USD for the NEXT call
	if price > 0 {
		quota := common.QuotaFromFloat(price * common.QuotaPerUnit)
		if quota <= 0 {
			quota = 1
		}
		// Check balance first
		balance, quotaErr := model.GetUserQuota(user.Id, false)
		if quotaErr != nil {
			oauthTokenError(c, http.StatusInternalServerError, "server_error", "获取额度失败")
			return
		}
		if balance < quota {
			oauthTokenError(c, http.StatusPaymentRequired, "insufficient_quota", "额度不足，请充值")
			return
		}
		if err := model.DecreaseUserQuota(user.Id, quota, true); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("OAuth2 userinfo 扣费失败 user_id=%d quota=%d error=%q", user.Id, quota, err.Error()))
			oauthTokenError(c, http.StatusInternalServerError, "server_error", "扣费失败")
			return
		}
	}
	// Increment the daily counter after billing so the tier is correct.
	if _, err := model.IncrementOAuthDailyUsage(user.Id, today); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("OAuth2 userinfo 用量计数失败 user_id=%d error=%q", user.Id, err.Error()))
	}

	c.JSON(http.StatusOK, gin.H{
		"sub":      user.Id,
		"username": user.Username,
		"email":    user.Email,
		"role":     user.Role,
	})
}

// oauthTierPrice returns the USD price for the call at the given 0-indexed
// call count (i.e. the count BEFORE this call). Tier boundaries:
//   0-49  → free ($0)
//  50-99  → $0.001
// 100-199 → $0.002
// 200+    → $0.003
func oauthTierPrice(callCount int) float64 {
	switch {
	case callCount < 50:
		return 0
	case callCount < 100:
		return 0.001
	case callCount < 200:
		return 0.002
	default:
		return 0.003
	}
}

// ----------------------------------------------------------------------------
// Admin: OAuth client management
// ----------------------------------------------------------------------------

// OAuthClientList returns all registered OAuth clients (admin only).
func OAuthClientList(c *gin.Context) {
	clients, err := model.GetAllOAuthClients()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, clients)
}

type OAuthClientCreateRequest struct {
	Name         string `json:"name"`
	RedirectURIs string `json:"redirect_uris"`
	Scopes       string `json:"scopes"`
}

// OAuthClientCreate registers a new OAuth client and returns its generated
// client_id / client_secret (the secret is shown only once).
func OAuthClientCreate(c *gin.Context) {
	var req OAuthClientCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	client := &model.OAuthClient{
		ClientId:     common.GetRandomString(16),
		ClientSecret: model.GenerateOAuthClientSecret(),
		Name:         req.Name,
		RedirectURIs: req.RedirectURIs,
		Scopes:       req.Scopes,
		Enabled:      true,
		UserId:       c.GetInt("id"),
	}
	if err := model.CreateOAuthClient(client); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "oauth_client.create", map[string]interface{}{
		"client_id": client.ClientId,
		"name":      client.Name,
	})
	common.ApiSuccess(c, client)
}

// OAuthClientUpdate updates a client's mutable fields (admin only). The secret
// is never overwritten here; rotate it via a dedicated endpoint if needed.
func OAuthClientUpdate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	client, err := model.GetOAuthClientById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req OAuthClientCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	client.Name = req.Name
	client.RedirectURIs = req.RedirectURIs
	client.Scopes = req.Scopes
	if err := model.UpdateOAuthClient(client); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "oauth_client.update", map[string]interface{}{
		"client_id": client.ClientId,
	})
	common.ApiSuccess(c, client)
}

// OAuthClientDelete removes a registered OAuth client (admin only).
func OAuthClientDelete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	client, err := model.GetOAuthClientById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteOAuthClient(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "oauth_client.delete", map[string]interface{}{
		"client_id": client.ClientId,
	})
	common.ApiSuccess(c, nil)
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func clientCredentialsFromRequest(c *gin.Context) (string, string) {
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Basic ") {
		return "", ""
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func bearerToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return c.Query("access_token")
}

func oauthTokenError(c *gin.Context, status int, code, description string) {
	c.JSON(status, gin.H{
		"error":             code,
		"error_description": description,
	})
}

func redirectError(c *gin.Context, redirectURI, state, code, description string) {
	q := url.Values{}
	q.Set("error", code)
	q.Set("error_description", description)
	if state != "" {
		q.Set("state", state)
	}
	c.Redirect(http.StatusFound, redirectURI+"?"+q.Encode())
}

func renderOAuthConsent(client *model.OAuthClient, scope, state, redirectURI string) []byte {
	scopeText := scope
	if scopeText == "" {
		scopeText = "默认权限"
	}
	html := `<!DOCTYPE html>
<html lang="zh">
<head><meta charset="utf-8"><title>授权确认</title>
<style>body{font-family:system-ui,sans-serif;background:#f5f5f7;display:flex;min-height:100vh;align-items:center;justify-content:center}
.card{background:#fff;padding:32px;border-radius:12px;max-width:420px;box-shadow:0 2px 12px rgba(0,0,0,.08)}
h2{margin-top:0}.scopes{color:#555;margin:16px 0}.actions{display:flex;gap:12px;justify-content:flex-end}
button{padding:8px 18px;border:0;border-radius:8px;cursor:pointer}
.allow{background:#2563eb;color:#fff}.deny{background:#eee;color:#333}</style></head>
<body><div class="card">
<h2>授权请求</h2>
<p>应用 <strong>` + htmlEscape(client.Name) + `</strong> 请求访问您的账号。</p>
<p class="scopes">权限范围：` + htmlEscape(scopeText) + `</p>
<form method="post">
<input type="hidden" name="decision" value="allow">
<input type="hidden" name="state" value="` + htmlEscape(state) + `">
<input type="hidden" name="redirect_uri" value="` + htmlEscape(redirectURI) + `">
<div class="actions">
<button type="submit" name="decision" value="deny" formnovalidate class="deny">拒绝</button>
<button type="submit" class="allow">允许</button>
</div>
</form></div></body></html>`
	return []byte(html)
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}

// ----------------------------------------------------------------------------
// User-level OAuth2.0 client management
// ----------------------------------------------------------------------------

// UserOAuthClientList returns the OAuth clients owned by the current user.
func UserOAuthClientList(c *gin.Context) {
	userId := c.GetInt("id")
	clients, err := model.GetOAuthClientsByUserId(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, clients)
}

// UserOAuthClientCreate registers a new OAuth client for the current user.
// The client_secret is returned in the response body only once.
func UserOAuthClientCreate(c *gin.Context) {
	var req OAuthClientCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userId := c.GetInt("id")
	client := &model.OAuthClient{
		ClientId:     common.GetRandomString(16),
		ClientSecret: model.GenerateOAuthClientSecret(),
		Name:         req.Name,
		RedirectURIs: req.RedirectURIs,
		Scopes:       req.Scopes,
		Enabled:      true,
		UserId:       userId,
	}
	if err := model.CreateOAuthClient(client); err != nil {
		common.ApiError(c, err)
		return
	}
	// Return the client with its secret visible (only time the secret is shown)
	common.ApiSuccess(c, gin.H{
		"id":             client.Id,
		"client_id":      client.ClientId,
		"client_secret":  client.ClientSecret,
		"name":           client.Name,
		"redirect_uris":  client.RedirectURIs,
		"scopes":         client.Scopes,
		"enabled":         client.Enabled,
		"user_id":         client.UserId,
		"created_at":      client.CreatedAt,
	})
}

// UserOAuthClientUpdate updates a client owned by the current user.
func UserOAuthClientUpdate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userId := c.GetInt("id")
	client, err := model.GetOAuthClientById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if client.UserId != userId {
		common.ApiErrorMsg(c, "无权修改此客户端")
		return
	}
	var req OAuthClientCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	client.Name = req.Name
	client.RedirectURIs = req.RedirectURIs
	client.Scopes = req.Scopes
	if err := model.UpdateOAuthClient(client); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, client)
}

// UserOAuthClientDelete removes a client owned by the current user.
func UserOAuthClientDelete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userId := c.GetInt("id")
	client, err := model.GetOAuthClientById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if client.UserId != userId {
		common.ApiErrorMsg(c, "无权删除此客户端")
		return
	}
	if err := model.DeleteOAuthClient(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
