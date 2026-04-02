package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/droptodrop/droptodrop/pkg/sessiontoken"
)

type contextKey string

const (
	ShopIDKey     contextKey = "shop_id"
	ShopDomainKey contextKey = "shop_domain"
	ShopRoleKey   contextKey = "shop_role"
)

// ShopFromContext extracts the shop ID from context.
func ShopFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ShopIDKey).(string); ok {
		return v
	}
	return ""
}

// RoleFromContext extracts the shop role from context.
func RoleFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ShopRoleKey).(string); ok {
		return v
	}
	return ""
}

// SessionAuth validates the session token from the Authorization header
// and loads the shop context.
//
// It supports two authentication methods:
//  1. Shopify App Bridge session tokens (JWTs signed with HS256 using API secret)
//  2. Database-backed session tokens (issued during OAuth callback)
//
// App Bridge tokens are detected by structure (three dot-separated base64 segments).
// When a valid App Bridge JWT is received for the first time, a database session is
// created so subsequent requests can use the faster DB lookup path.
func SessionAuth(db *pgxpool.Pool, apiKey, apiSecret string, sessionMaxAge int, logger zerolog.Logger) gin.HandlerFunc {
	jwtConfig := sessiontoken.VerifyConfig{
		APIKey:    apiKey,
		APISecret: apiSecret,
		ClockSkew: 10 * time.Second,
	}

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}

		var shopID, shopDomain, shopRole string

		if isJWT(token) {
			// Path 1: App Bridge session token (JWT)
			claims, err := sessiontoken.Verify(token, jwtConfig)
			if err != nil {
				logger.Warn().Err(err).Msg("App Bridge JWT verification failed")
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid session token"})
				return
			}

			shopDomain = claims.ShopDomain()

			// Look up shop by domain
			err = db.QueryRow(c.Request.Context(), `
				SELECT id, role FROM shops WHERE shopify_domain = $1 AND status = 'active'
			`, shopDomain).Scan(&shopID, &shopRole)
			if err != nil {
				// Shop not in DB — auto-create it so OAuth callback can update it
				logger.Info().Str("shop", shopDomain).Msg("auto-creating shop from JWT")
				err = db.QueryRow(c.Request.Context(), `
					INSERT INTO shops (shopify_domain, status)
					VALUES ($1, 'active')
					ON CONFLICT (shopify_domain) DO UPDATE SET status = 'active', updated_at = NOW()
					RETURNING id, role
				`, shopDomain).Scan(&shopID, &shopRole)
				if err != nil {
					logger.Warn().Err(err).Str("shop", shopDomain).Msg("failed to auto-create shop")
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "shop not found or inactive"})
					return
				}
			}

			// Ensure a database session exists for this shop so that subsequent
			// requests (or non-JWT clients) can authenticate quickly.
			// Use the JWT's session ID (sid) as the session token for dedup.
			sessionKey := claims.Sid
			if sessionKey == "" {
				sessionKey = claims.Jti
			}
			if sessionKey != "" {
				expiresAt := time.Unix(claims.Exp, 0)
				// Use a longer expiry for the DB session than the JWT itself,
				// since the frontend will refresh the JWT before it expires.
				if expiresAt.Before(time.Now().Add(time.Duration(sessionMaxAge) * time.Second)) {
					expiresAt = time.Now().Add(time.Duration(sessionMaxAge) * time.Second)
				}
				_, err = db.Exec(c.Request.Context(), `
					INSERT INTO shop_sessions (id, shop_id, session_token, expires_at)
					VALUES ($1, $2, $3, $4)
					ON CONFLICT (session_token) DO UPDATE SET expires_at = EXCLUDED.expires_at
				`, uuid.New(), shopID, sessionKey, expiresAt)
				if err != nil {
					// Non-fatal: auth still succeeds, just won't cache the session
					logger.Warn().Err(err).Msg("failed to persist session from JWT")
				}
			}

			logger.Debug().Str("shop", shopDomain).Str("sub", claims.Sub).Msg("authenticated via App Bridge JWT")

		} else {
			// Path 2: Database session token (from OAuth callback or persisted JWT sid)
			err := db.QueryRow(c.Request.Context(), `
				SELECT s.id, s.shopify_domain, s.role
				FROM shop_sessions ss
				JOIN shops s ON s.id = ss.shop_id
				WHERE ss.session_token = $1 AND ss.expires_at > NOW()
			`, token).Scan(&shopID, &shopDomain, &shopRole)
			if err != nil {
				logger.Warn().Err(err).Msg("session lookup failed")
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired session"})
				return
			}
		}

		// Set context values for downstream handlers
		ctx := context.WithValue(c.Request.Context(), ShopIDKey, shopID)
		ctx = context.WithValue(ctx, ShopDomainKey, shopDomain)
		ctx = context.WithValue(ctx, ShopRoleKey, shopRole)
		c.Request = c.Request.WithContext(ctx)

		c.Set("shop_id", shopID)
		c.Set("shop_domain", shopDomain)
		c.Set("shop_role", shopRole)

		c.Next()
	}
}

// isJWT returns true if the token looks like a JWT (three base64url segments separated by dots).
func isJWT(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	// Each part should be non-empty and look like base64url
	for _, p := range parts {
		if len(p) == 0 {
			return false
		}
	}
	return true
}

// RequireRole ensures the authenticated shop has one of the allowed roles.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("shop_role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "no role found"})
			return
		}

		roleStr, ok := role.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid role"})
			return
		}

		for _, r := range roles {
			if r == roleStr {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient role"})
	}
}
