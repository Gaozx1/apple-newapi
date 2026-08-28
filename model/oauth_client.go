package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// OAuthClient represents a registered OAuth2.0 client application. Admins
// create clients; end users authorize them via the authorization-code flow.
type OAuthClient struct {
	Id           int       `json:"id" gorm:"primaryKey"`
	ClientId     string    `json:"client_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	ClientSecret string    `json:"-" gorm:"type:varchar(128);not null"` // never serialized to JSON
	Name         string    `json:"name" gorm:"type:varchar(128);not null"`
	RedirectURIs string    `json:"redirect_uris" gorm:"type:text"`    // newline-separated
	Scopes       string    `json:"scopes" gorm:"type:varchar(512)"`    // space-separated
	Enabled      bool      `json:"enabled" gorm:"default:false"`      // set in code, not gorm tag
	UserId       int       `json:"user_id" gorm:"index"`              // owner (admin who created)
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// OAuthAuthorizationCode is a short-lived one-time code exchanged for an
// access token in the authorization-code grant flow.
type OAuthAuthorizationCode struct {
	Id          int64  `json:"id" gorm:"primaryKey"`
	Code        string `json:"-" gorm:"type:varchar(128);uniqueIndex;not null"`
	ClientId    string `json:"client_id" gorm:"type:varchar(64);index;not null"`
	UserId      int    `json:"user_id" gorm:"not null"`
	RedirectURI string `json:"redirect_uri" gorm:"type:varchar(512)"`
	Scopes      string `json:"scopes" gorm:"type:varchar(512)"`
	ExpiresAt   int64  `json:"expires_at" gorm:"not null"`
	Used        bool   `json:"used" gorm:"default:false"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime"`
}

var (
	ErrOAuthClientNotFound       = errors.New("oauth client not found")
	ErrOAuthClientDisabled       = errors.New("oauth client is disabled")
	ErrOAuthInvalidRedirectURI   = errors.New("invalid redirect uri")
	ErrOAuthCodeNotFound         = errors.New("authorization code not found")
	ErrOAuthCodeExpired          = errors.New("authorization code expired")
	ErrOAuthCodeAlreadyUsed      = errors.New("authorization code already used")
	ErrOAuthClientSecretMismatch = errors.New("client secret mismatch")
)

// RedirectURIList splits the stored newline-separated redirect URIs.
func (c *OAuthClient) RedirectURIList() []string {
	if c.RedirectURIs == "" {
		return nil
	}
	var result []string
	for _, uri := range strings.Split(c.RedirectURIs, "\n") {
		uri = strings.TrimSpace(uri)
		if uri != "" {
			result = append(result, uri)
		}
	}
	return result
}

// IsValidRedirectURI checks whether the given URI matches a registered redirect URI.
func (c *OAuthClient) IsValidRedirectURI(uri string) bool {
	if uri == "" {
		return false
	}
	for _, registered := range c.RedirectURIList() {
		if registered == uri {
			return true
		}
	}
	return false
}

// RedirectURIListDefault returns the first registered redirect URI, used when
// the authorize request omits redirect_uri.
func (c *OAuthClient) RedirectURIListDefault() string {
	list := c.RedirectURIList()
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

// HasScope checks whether the client is allowed to request the given scope.
func (c *OAuthClient) HasScope(scope string) bool {
	if c.Scopes == "" {
		return true // no scope restriction
	}
	for _, s := range strings.Fields(c.Scopes) {
		if s == scope {
			return true
		}
	}
	return false
}

// CreateOAuthClient inserts a new OAuth client.
func CreateOAuthClient(client *OAuthClient) error {
	return DB.Create(client).Error
}

// GetOAuthClientByClientId retrieves a client by its client_id.
func GetOAuthClientByClientId(clientId string) (*OAuthClient, error) {
	var client OAuthClient
	err := DB.Where("client_id = ?", clientId).First(&client).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOAuthClientNotFound
		}
		return nil, err
	}
	return &client, nil
}

// GetOAuthClientById retrieves a client by its primary key.
func GetOAuthClientById(id int) (*OAuthClient, error) {
	var client OAuthClient
	err := DB.First(&client, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOAuthClientNotFound
		}
		return nil, err
	}
	return &client, nil
}

// GetAllOAuthClients returns all registered OAuth clients.
func GetAllOAuthClients() ([]*OAuthClient, error) {
	var clients []*OAuthClient
	err := DB.Order("id DESC").Find(&clients).Error
	return clients, err
}

// GetOAuthClientsByUserId returns all OAuth clients owned by the given user.
func GetOAuthClientsByUserId(userId int) ([]*OAuthClient, error) {
	var clients []*OAuthClient
	err := DB.Where("user_id = ?", userId).Order("id DESC").Find(&clients).Error
	return clients, err
}

// UpdateOAuthClient updates a client's mutable fields.
func UpdateOAuthClient(client *OAuthClient) error {
	return DB.Save(client).Error
}

// DeleteOAuthClient removes a client by id.
func DeleteOAuthClient(id int) error {
	return DB.Delete(&OAuthClient{}, id).Error
}

// CreateOAuthAuthorizationCode creates a one-time authorization code.
func CreateOAuthAuthorizationCode(code *OAuthAuthorizationCode) error {
	return DB.Create(code).Error
}

// ConsumeOAuthAuthorizationCode atomically marks a code as used and returns
// the associated data. It uses a compare-and-swap on the `used` column so
// concurrent or replayed exchanges of the same code fail safely.
func ConsumeOAuthAuthorizationCode(code string, clientId string) (*OAuthAuthorizationCode, error) {
	var authCode OAuthAuthorizationCode
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("code = ?", code).First(&authCode).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrOAuthCodeNotFound
			}
			return err
		}
		if authCode.ClientId != clientId {
			return ErrOAuthClientNotFound
		}
		if authCode.Used {
			return ErrOAuthCodeAlreadyUsed
		}
		if authCode.ExpiresAt < time.Now().Unix() {
			return ErrOAuthCodeExpired
		}
		// CAS: only the transaction that flips used=false → true may proceed.
		result := tx.Model(&OAuthAuthorizationCode{}).
			Where("id = ? AND used = ?", authCode.Id, false).
			Update("used", true)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrOAuthCodeAlreadyUsed
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &authCode, nil
}

// GenerateOAuthClientSecret generates a random client secret.
func GenerateOAuthClientSecret() string {
	return common.GetUUID() + common.GetUUID()
}

// OAuthAccessToken is a bearer access token issued by the OAuth2.0 server.
// Access tokens are bearer credentials: anyone holding the token can act as
// the authorized user until the token expires.
type OAuthAccessToken struct {
	Id        int64     `json:"id" gorm:"primaryKey"`
	AccessToken string  `json:"-" gorm:"type:varchar(64);uniqueIndex;not null"`
	ClientId  string    `json:"client_id" gorm:"type:varchar(64);index;not null"`
	UserId    int       `json:"user_id" gorm:"not null"`
	Scopes    string    `json:"scopes" gorm:"type:varchar(512)"`
	ExpiresAt int64     `json:"expires_at" gorm:"not null"`
	CreatedAt int64     `json:"created_at" gorm:"autoCreateTime"`
}

// CreateOAuthAccessToken persists a newly issued access token.
func CreateOAuthAccessToken(token *OAuthAccessToken) error {
	return DB.Create(token).Error
}

// GetOAuthAccessTokenByToken looks up a (non-expired) access token.
func GetOAuthAccessTokenByToken(accessToken string) (*OAuthAccessToken, error) {
	var token OAuthAccessToken
	err := DB.Where("access_token = ?", accessToken).First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOAuthAccessTokenNotFound
		}
		return nil, err
	}
	return &token, nil
}

// DeleteOAuthAccessTokensByUser removes all access tokens for a user. Used when
// a user's credentials rotate so issued tokens can no longer be used.
func DeleteOAuthAccessTokensByUser(userId int) error {
	return DB.Where("user_id = ?", userId).Delete(&OAuthAccessToken{}).Error
}

// OAuthDailyUsage tracks the number of OAuth2.0 userinfo calls made by a user
// on a given day. It drives the tiered billing: first 50 calls/day free, then
// $0.001/call up to 100, $0.002 up to 200, $0.003 beyond.
type OAuthDailyUsage struct {
	Id        int64  `json:"id" gorm:"primaryKey"`
	UserId    int    `json:"user_id" gorm:"index:idx_oauth_usage_user_date,unique;not null"`
	Date      string `json:"date" gorm:"type:varchar(10);index:idx_oauth_usage_user_date,unique;not null"` // YYYY-MM-DD
	CallCount int    `json:"call_count" gorm:"default:0"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

// GetOAuthDailyUsage returns the usage row for the given user and date,
// creating one if it does not exist yet.
func GetOAuthDailyUsage(userId int, date string) (*OAuthDailyUsage, error) {
	var usage OAuthDailyUsage
	err := DB.Where("user_id = ? AND date = ?", userId, date).First(&usage).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			usage = OAuthDailyUsage{UserId: userId, Date: date, CallCount: 0}
			if createErr := DB.Create(&usage).Error; createErr != nil {
				return nil, createErr
			}
			return &usage, nil
		}
		return nil, err
	}
	return &usage, nil
}

// IncrementOAuthDailyUsage atomically increments the call count for the given
// user/date and returns the new count.
func IncrementOAuthDailyUsage(userId int, date string) (int, error) {
	usage, err := GetOAuthDailyUsage(userId, date)
	if err != nil {
		return 0, err
	}
	result := DB.Model(&OAuthDailyUsage{}).
		Where("id = ?", usage.Id).
		Update("call_count", gorm.Expr("call_count + ?", 1))
	if result.Error != nil {
		return 0, result.Error
	}
	return usage.CallCount + 1, nil
}

// var holding the not-found sentinel for access tokens.
var ErrOAuthAccessTokenNotFound = errors.New("oauth access token not found")
