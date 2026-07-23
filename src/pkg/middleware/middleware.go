// Package middleware holds HTTP middleware for the management API. The device
// (iclock) endpoints are intentionally NOT wrapped by these — they are called by
// the devices on the trusted LAN and speak the plain-text ADMS protocol.
package middleware

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	milastructs "ismila/erp/zkbio/mila_structs"

	"github.com/gin-gonic/gin"
)

// APIKey guards the management API with a shared token, supplied as either
// "X-API-Key: <token>" or "Authorization: Bearer <token>". If token is empty the
// guard is disabled (open on the LAN) and a warning is logged once at startup —
// set MGMT_API_KEY in any real deployment.
func APIKey(token string) gin.HandlerFunc {
	if token == "" {
		slog.Warn("management API is UNAUTHENTICATED (MGMT_API_KEY is not set) — protect the gateway on the LAN")
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		presented := c.GetHeader("X-API-Key")
		if presented == "" {
			if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
				presented = strings.TrimPrefix(h, "Bearer ")
			}
		}
		if subtle.ConstantTimeCompare([]byte(presented), []byte(token)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, milastructs.Fail("invalid or missing API key"))
			return
		}
		c.Next()
	}
}
