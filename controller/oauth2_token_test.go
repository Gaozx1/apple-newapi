package controller

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupOAuthTokenTest wires an isolated database for the token endpoint and
// returns a registered, usable client owned by user 140.
func setupOAuthTokenTest(t *testing.T) *model.OAuthClient {
	t.Helper()
	gin.SetMode(gin.TestMode)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.OAuthClient{}, &model.OAuthAuthorizationCode{}, &model.OAuthAccessToken{}))
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMain, previousLog)
		common.RedisEnabled = previousRedis
		connection, err := db.DB()
		if err == nil {
			_ = connection.Close()
		}
	})
	client := &model.OAuthClient{
		ClientId:     "cid_owner",
		ClientSecret: "secret",
		Name:         "app",
		Enabled:      true,
		Status:       model.OAuthClientStatusApproved,
		UserId:       140,
	}
	require.NoError(t, db.Create(client).Error)
	return client
}

func postOAuthToken(t *testing.T, form url.Values, basicAuth string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/oauth2/token", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basicAuth != "" {
		request.Header.Set("Authorization", "Basic "+basicAuth)
	}
	response := httptest.NewRecorder()
	router := gin.New()
	router.POST("/api/oauth2/token", OAuthToken)
	router.ServeHTTP(response, request)
	return response
}

// TestOAuthTokenRejectsClientCredentialsGrant is the regression test for the
// grant that let anyone holding a client_secret mint a token for the client
// owner, bypassing the owner's password, second factor and consent.
func TestOAuthTokenRejectsClientCredentialsGrant(t *testing.T) {
	client := setupOAuthTokenTest(t)
	basic := base64.StdEncoding.EncodeToString([]byte(client.ClientId + ":" + client.ClientSecret))

	cases := []struct {
		name      string
		form      url.Values
		basicAuth string
	}{
		{
			name: "body credentials",
			form: url.Values{
				"grant_type":    {"client_credentials"},
				"client_id":     {client.ClientId},
				"client_secret": {client.ClientSecret},
				"scope":         {"api"},
			},
		},
		{
			name:      "basic auth credentials",
			form:      url.Values{"grant_type": {"client_credentials"}},
			basicAuth: basic,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := postOAuthToken(t, tc.form, tc.basicAuth)
			assert.Equal(t, http.StatusBadRequest, response.Code)
			assert.Contains(t, response.Body.String(), "unsupported_grant_type")
		})
	}

	// A valid secret must never yield a token impersonating its owner.
	var issued int64
	require.NoError(t, model.DB.Model(&model.OAuthAccessToken{}).Count(&issued).Error)
	assert.Zero(t, issued)
}

// TestOAuthTokenAuthorizationCodeGrantIssuesUserBoundToken keeps the remaining
// grant on contract: the token represents the user who consented, never the
// client's owner.
func TestOAuthTokenAuthorizationCodeGrantIssuesUserBoundToken(t *testing.T) {
	client := setupOAuthTokenTest(t)
	require.NoError(t, model.DB.Create(&model.OAuthAuthorizationCode{
		Code:        "code-1",
		ClientId:    client.ClientId,
		UserId:      7,
		RedirectURI: "https://app.example.com/callback",
		Scopes:      "api",
		ExpiresAt:   time.Now().Add(time.Minute).Unix(),
	}).Error)

	response := postOAuthToken(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"code-1"},
		"redirect_uri":  {"https://app.example.com/callback"},
		"client_id":     {client.ClientId},
		"client_secret": {client.ClientSecret},
	}, "")
	require.Equal(t, http.StatusOK, response.Code)

	var body struct {
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	require.NotEmpty(t, body.AccessToken)

	token, err := model.GetOAuthAccessTokenByToken(body.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, 7, token.UserId)
	assert.Equal(t, "api", token.Scopes)
}
