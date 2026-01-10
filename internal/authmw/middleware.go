package authmw

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type KeycloakAuth struct {
	Issuer   string // e.g. http://localhost:8080/realms/myrealm
	Audience string // usually your client-id (if you validate aud)
	ClientID string // for client roles under resource_access[ClientID].roles

	JWKS *keyfunc.JWKS
	// optional clock skew
	Leeway time.Duration
}

// Build once at startup (don’t fetch JWKS on every request)
func NewKeycloakAuth(jwksURL, issuer, audience, clientID string) (*KeycloakAuth, error) {
	jwks, err := keyfunc.Get(jwksURL, keyfunc.Options{
		RefreshInterval:  time.Hour,
		RefreshRateLimit: time.Minute * 5,
		RefreshTimeout:   time.Second * 10,
	})
	if err != nil {
		return nil, err
	}

	return &KeycloakAuth{
		Issuer:   issuer,
		Audience: audience,
		ClientID: clientID,
		JWKS:     jwks,
		Leeway:   30 * time.Second,
	}, nil
}

type KCClaims struct {
	jwt.RegisteredClaims

	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	Name              string `json:"name"`
	Firstname         string `json:"given_name"`
	Lastname          string `json:"family_name"`

	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`

	ResourceAccess map[string]struct {
		Roles []string `json:"roles"`
	} `json:"resource_access"`
}

func (a *KeycloakAuth) RequireRoles(anyOf ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := extractAccessToken(c)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			// c.Redirect(http.StatusSeeOther, "/api/v1/login")

			return
		}

		claims := &KCClaims{}
		_, err = jwt.ParseWithClaims(tokenStr, claims, a.JWKS.Keyfunc,
			jwt.WithIssuer(a.Issuer),
			// If your tokens do NOT include "aud" reliably, remove this line.
			jwt.WithAudience(a.Audience),
			jwt.WithLeeway(a.Leeway),
			jwt.WithValidMethods([]string{"RS256"}),
		)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})

			return
		}

		roles := collectRoles(claims, a.ClientID)

		// Put identity into context for handlers
		c.Set("kc.access_token", tokenStr)
		c.Set("kc.username", claims.PreferredUsername)
		c.Set("kc.email", claims.Email)
		c.Set("kc.email_verified", claims.EmailVerified)
		c.Set("kc.roles", roles)
		c.Set("kc.sub", claims.Subject)
		c.Set("kc.firstname", claims.Firstname)
		c.Set("kc.lastname", claims.Lastname)

		if !hasAnyRole(roles, anyOf...) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient role"})
			return
		}

		c.Next()
	}
}

func RequireEmailVerified() gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get("kc.email_verified")
		if !ok || v == false {
			// For HTML pages, redirect; for API, return JSON
			// accept := c.GetHeader("Accept")
			// if strings.Contains(accept, "text/html") {
			// 	c.Redirect(http.StatusSeeOther, "/api/v1/auth/verify-email")
			// } else {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "email not verified",
			})
			// }
			c.Abort()
			return
		}
		c.Next()
	}
}

// --- helpers ---

func extractAccessToken(c *gin.Context) (string, error) {
	// 1) Authorization: Bearer <token>
	authz := c.GetHeader("Authorization")
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[7:]), nil
	}

	// 2) Optional: cookie fallback (if you store token in cookie)
	if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
		return cookie, nil
	}

	return "", errors.New("missing access token")
}

func collectRoles(claims *KCClaims, clientID string) []string {
	out := make([]string, 0, 16)

	// realm roles
	out = append(out, claims.RealmAccess.Roles...)

	// client roles (resource_access)
	if clientID != "" && claims.ResourceAccess != nil {
		if ra, ok := claims.ResourceAccess[clientID]; ok {
			out = append(out, ra.Roles...)
		}
	}

	return uniq(out)
}

func hasAnyRole(userRoles []string, anyOf ...string) bool {
	roleSet := make(map[string]struct{}, len(userRoles))
	for _, r := range userRoles {
		roleSet[r] = struct{}{}
	}
	for _, required := range anyOf {
		if _, ok := roleSet[required]; ok {
			return true
		}
	}
	return false
}

func uniq(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
