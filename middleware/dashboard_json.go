package middleware

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// IsSameOriginDashboardWrite provides defense in depth for sensitive dashboard
// writes, which also require a valid session bearer token.
func IsSameOriginDashboardWrite(c *gin.Context) bool {
	if site := c.GetHeader("Sec-Fetch-Site"); site != "" {
		return site == "same-origin"
	}
	origin := c.GetHeader("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && strings.EqualFold(parsed.Host, c.Request.Host)
}

func ReadDashboardJSON(c *gin.Context, dst any) error {
	media, _, _ := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if media != "application/json" {
		return errors.New("expected application/json")
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1024))
	if err != nil {
		return err
	}
	return common.Unmarshal(body, dst)
}
