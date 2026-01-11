package mteam

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func createHandler(c *gin.Context) {
	var req CreateTeamRequest
	if err := c.ShouldBind(&req); err != nil {
		log.Printf("failed to bind input: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	authorAny, _ := c.Get("kc.username")
	createdBy := authorAny.(string)

	teamID, err := CreateTeam(c.Request.Context(), req.Name, req.Description, createdBy)
	if err != nil {
		log.Printf("failed to create the entity: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// Assign selected leader as member(role=leader)
	if err := AddMember(c.Request.Context(), teamID, req.Leader, "leader"); err != nil {
		log.Printf("failed to add leader member: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error (add leader)"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "ok", "teamid": teamID})
}

func updateHandler(c *gin.Context) {
	idStr := c.Query("teamid")
	teamID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || teamID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing/invalid teamid"})
		return
	}

	var req UpdateTeamRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	err = UpdateTeam(c.Request.Context(), teamID, req)
	if err != nil {
		if strings.Contains(err.Error(), "no fields to update") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "provide name and/or description"})
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func deleteHandler(c *gin.Context) {
	idStr := c.Query("teamid")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing/invalid id"})

		return
	}

	err = DeleteTeam(c.Request.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})

		return
	}
	if err != nil {
		log.Printf("failed to delete entity: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})

		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func getHandler(c *gin.Context) {
	ctx := c.Request.Context()

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	order := c.DefaultQuery("order", "created_desc")

	// Optional filters
	var (
		teamID *int64
		name   *string
	)

	if idStr := c.Query("teamid"); idStr != "" {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil && id > 0 {
			teamID = &id
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid teamid"})

			return
		}
	} else if nameStr := c.Query("name"); nameStr != "" {
		name = &nameStr
	}

	teams, err := ListTeams(ctx, teamID, name, limit, order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})

		return
	}

	payload := gin.H{
		"items": teams,
		"limit": normalizeLimit(limit),
		"order": order,
	}

	if teamID != nil {
		payload["teamid"] = *teamID
	}
	if name != nil {
		payload["name"] = *name
	}

	c.JSON(http.StatusOK, payload)
}

func mustUsername(c *gin.Context) (string, bool) {
	v, ok := c.Get("kc.username")
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}

func handleMyTeams(c *gin.Context) {
	username, ok := mustUsername(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})

		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	teams, err := ListTeamsForUser(c.Request.Context(), username, limit)
	if err != nil {
		log.Printf("failed to retrieve data: %v", err)
		c.JSON(500, gin.H{"error": "db error"})

		return
	}

	payload := gin.H{"items": teams, "limit": limit}
	c.JSON(http.StatusOK, payload)
}

func addTeamMemberHandler(c *gin.Context) {
	teamIDStr := c.Param("teamid")
	teamID, err := strconv.ParseInt(teamIDStr, 10, 64)
	if err != nil || teamID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid teamid"})
		return
	}

	var req AddTeamMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "member"
	}

	// Optional: validate role
	switch role {
	case "member", "leader":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}

	// Optional but recommended: leader-only can manage only their teams
	if err := ensureCanManageTeam(c, teamID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if err := AddMember(c.Request.Context(), teamID, req.Username, role); err != nil {
		log.Printf("add member failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func removeTeamMemberHandler(c *gin.Context) {
	teamIDStr := c.Param("teamid")
	teamID, err := strconv.ParseInt(teamIDStr, 10, 64)
	if err != nil || teamID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid teamid"})
		return
	}

	username := strings.TrimSpace(c.Param("username"))
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username required"})
		return
	}

	if err := ensureCanManageTeam(c, teamID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if err := RemoveMember(c.Request.Context(), teamID, username); err != nil {
		log.Printf("remove member failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func ensureCanManageTeam(c *gin.Context, teamID int64) error {
	// admin can manage all
	rolesAny, _ := c.Get("kc.roles")
	roles, _ := rolesAny.([]string)
	for _, r := range roles {
		if r == "admin" {
			return nil
		}
	}

	// leader must be leader of this team
	usernameAny, _ := c.Get("kc.username")
	username := usernameAny.(string)

	ok, err := IsLeaderOfTeam(c.Request.Context(), teamID, username)
	if err != nil {
		log.Printf("IsLeaderOfTeam failed: %v", err)
		return fmt.Errorf("db error")
	}
	if !ok {
		return fmt.Errorf("not allowed")
	}
	return nil
}

func IsLeaderOfTeam(ctx context.Context, teamID int64, username string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM team_members
            WHERE teamid = $1 AND username = $2 AND role = 'leader'
        )
    `, teamID, username).Scan(&exists)
	return exists, err
}
