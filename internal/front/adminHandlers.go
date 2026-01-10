package front

import (
	"context"
	"net/http"

	"github.com/Nerzal/gocloak/v13"
	"github.com/gin-gonic/gin"
)

func handleAdminGetUserByID(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing user id"})
		return
	}

	// 1) get an admin token (service account)
	adminToken, err := kcService.LoginAdmin(ctx) // you implement this
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get admin token"})
		return
	}

	// 2) query KC
	u, err := kcService.Client.GetUserByID(ctx, adminToken.AccessToken, kcService.Realm, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	payload := gin.H{
		"id":            u.ID,
		"username":      u.Username,
		"email":         u.Email,
		"emailVerified": u.EmailVerified,
		"firstName":     u.FirstName,
		"lastName":      u.LastName,
		"enabled":       u.Enabled,
		"createdAt":     u.CreatedTimestamp, // ms since epoch
	}

	respondInFormat(c, payload, "admin_user_detail.html")
}

func handleAdminDeleteUser(c *gin.Context) {
	userID := c.Param("id")

	jwt, _ := kcService.LoginAdmin(context.Background())
	err := kcService.DeleteUser(
		context.Background(),
		jwt.AccessToken,
		userID,
	)

	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.Status(204)
}

func handleAdminEnableUser(c *gin.Context) {
	userID := c.Param("id")
	enabled := c.Query("enabled") == "true"

	jwt, _ := kcService.LoginAdmin(context.Background())
	err := kcService.SetUserEnabled(
		context.Background(),
		jwt.AccessToken,
		userID,
		enabled,
	)

	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

type SetRolesRequest struct {
	Roles []string `json:"roles" binding:"required"`
}

var managedRealmRoles = []string{"student", "teamleader", "admin"}

func handleAdminVerifyEmail(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing user id"})
		return
	}

	adminJWT, err := kcService.LoginAdmin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get admin token"})
		return
	}
	token := adminJWT.AccessToken

	u, err := kcService.Client.GetUserByID(ctx, token, kcService.Realm, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Set emailVerified = true
	u.EmailVerified = gocloak.BoolP(true)

	if err := kcService.Client.UpdateUser(ctx, token, kcService.Realm, *u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"userId":        userID,
		"emailVerified": true,
	})
}

func handleAdminGetUserByUsername(c *gin.Context) {
	ctx := c.Request.Context()
	username := c.Param("username")
	if username == "" {
		c.JSON(400, gin.H{"error": "missing username"})
		return
	}

	adminJWT, err := kcService.LoginAdmin(ctx)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get admin token"})
		return
	}
	token := adminJWT.AccessToken

	u, err := kcService.GetUserByUsername(ctx, token, username)
	if err != nil {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}

	payload := gin.H{
		"id":            u.ID, // still useful to show
		"username":      u.Username,
		"email":         u.Email,
		"emailVerified": u.EmailVerified,
		"firstName":     u.FirstName,
		"lastName":      u.LastName,
		"enabled":       u.Enabled,
	}
	respondInFormat(c, payload, "admin_user_detail.html")
}

func handleAdminSetUserRoles(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing user id"})
		return
	}

	var req SetRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	// Validate roles are in allowed set
	allowed := map[string]struct{}{}
	for _, r := range managedRealmRoles {
		allowed[r] = struct{}{}
	}
	for _, r := range req.Roles {
		if _, ok := allowed[r]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported role: " + r})
			return
		}
	}

	// Use service-account admin token (recommended)
	adminJWT, err := kcService.LoginAdmin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get admin token"})
		return
	}
	token := adminJWT.AccessToken

	// 1) Fetch current realm roles for the user
	currentRoles, err := kcService.Client.GetRealmRolesByUserID(ctx, token, kcService.Realm, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get user's roles"})
		return
	}

	// Build sets
	desired := map[string]struct{}{}
	for _, r := range req.Roles {
		desired[r] = struct{}{}
	}

	// Compute which managed roles to remove
	toRemove := make([]gocloak.Role, 0)
	for _, cr := range currentRoles {
		name := ""
		if cr.Name != nil {
			name = *cr.Name
		}
		// remove only roles we manage, and only if not desired
		if _, isManaged := allowed[name]; isManaged {
			if _, keep := desired[name]; !keep {
				toRemove = append(toRemove, *cr)
			}
		}
	}

	// Compute which roles to add (fetch full role representations)
	toAdd := make([]gocloak.Role, 0)
	// Quick set of current role names
	currentSet := map[string]struct{}{}
	for _, cr := range currentRoles {
		if cr.Name != nil {
			currentSet[*cr.Name] = struct{}{}
		}
	}
	for roleName := range desired {
		if _, already := currentSet[roleName]; already {
			continue
		}
		rr, err := kcService.Client.GetRealmRole(ctx, token, kcService.Realm, roleName)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "role not found: " + roleName})
			return
		}
		toAdd = append(toAdd, *rr)
	}

	// 2) Apply changes
	if len(toRemove) > 0 {
		if err := kcService.Client.DeleteRealmRoleFromUser(ctx, token, kcService.Realm, userID, toRemove); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove roles"})
			return
		}
	}
	if len(toAdd) > 0 {
		if err := kcService.Client.AddRealmRoleToUser(ctx, token, kcService.Realm, userID, toAdd); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add roles"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"userId": userID,
		"roles":  req.Roles,
	})
}
