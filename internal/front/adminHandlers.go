package front

import (
	"context"
	"net/http"
	"strings"

	auth "kyri56xcaesar/pms-proj/internal/authmw"

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

var managedRealmRoles = []string{"student", "leader", "admin"}

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
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "missing user id"})
		return
	}

	// 1) read roles from either JSON or form field roles_csv
	var roles []string

	rolesCSV := strings.TrimSpace(c.PostForm("roles_csv"))
	if rolesCSV != "" {
		for _, r := range strings.Split(rolesCSV, ",") {
			rr := strings.TrimSpace(r)
			if rr != "" {
				roles = append(roles, rr)
			}
		}
	} else {
		var req SetRolesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "invalid input"})
			return
		}
		roles = req.Roles
	}

	// Validate roles
	allowed := map[string]struct{}{}
	for _, r := range managedRealmRoles {
		allowed[r] = struct{}{}
	}
	for _, r := range roles {
		if _, ok := allowed[r]; !ok {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "unsupported role: " + r})
			return
		}
	}

	adminJWT, err := kcService.LoginAdmin(ctx)
	if err != nil {
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "failed to get admin token"})
		return
	}
	token := adminJWT.AccessToken

	// Fetch current roles
	currentRoles, err := kcService.Client.GetRealmRolesByUserID(ctx, token, kcService.Realm, userID)
	if err != nil {
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "failed to get user's roles"})
		return
	}

	desired := map[string]struct{}{}
	for _, r := range roles {
		desired[r] = struct{}{}
	}

	// Remove managed roles that are not desired
	toRemove := make([]gocloak.Role, 0)
	currentSet := map[string]struct{}{}
	for _, cr := range currentRoles {
		if cr.Name == nil {
			continue
		}
		name := *cr.Name
		currentSet[name] = struct{}{}

		if _, isManaged := allowed[name]; isManaged {
			if _, keep := desired[name]; !keep {
				toRemove = append(toRemove, *cr)
			}
		}
	}

	// Add desired roles not already present
	toAdd := make([]gocloak.Role, 0)
	for roleName := range desired {
		if _, already := currentSet[roleName]; already {
			continue
		}
		rr, err := kcService.Client.GetRealmRole(ctx, token, kcService.Realm, roleName)
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "role not found: " + roleName})
			return
		}
		toAdd = append(toAdd, *rr)
	}

	if len(toRemove) > 0 {
		if err := kcService.Client.DeleteRealmRoleFromUser(ctx, token, kcService.Realm, userID, toRemove); err != nil {
			c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "failed to remove roles"})
			return
		}
	}
	if len(toAdd) > 0 {
		if err := kcService.Client.AddRealmRoleToUser(ctx, token, kcService.Realm, userID, toAdd); err != nil {
			c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "failed to add roles"})
			return
		}
	}

	c.Redirect(http.StatusSeeOther, "/api/v1/auth/admin/users")
}

func handleAdminDeleteUser(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.Param("id")
	if userID == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "missing user id"})
		return
	}

	jwt, err := kcService.LoginAdmin(ctx)
	if err != nil {
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "failed to get admin token"})
		return
	}

	if err := kcService.DeleteUser(ctx, jwt.AccessToken, userID); err != nil {
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/api/v1/auth/admin/users")
}

func adminUsersHandlers(c *gin.Context) {
	// Page user info (logged-in admin)
	username, _ := c.Get("kc.username")
	email, _ := c.Get("kc.email")
	rolesAny, _ := c.Get("kc.roles")
	firstname, _ := c.Get("kc.firstname")
	lastname, _ := c.Get("kc.lastname")
	roles, _ := rolesAny.([]string)

	rows, err := kcService.ListRealmUsersWithRoles(c.Request.Context(), 200)
	if err != nil {
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "Keycloak: " + err.Error()})
		return
	}

	// VM (match your layout usage)
	var vm struct {
		Title        string
		Active       string
		User         UserVM
		Users        []auth.AdminUserRow
		ManagedRoles []string
	}
	vm.Title = "Admin Users"
	vm.Active = "admin-users"
	vm.Users = rows
	vm.ManagedRoles = managedRealmRoles

	vm.User.Username = username.(string)
	vm.User.Email = email.(string)
	vm.User.Roles = roles
	vm.User.IsAdmin = true
	vm.User.Firstname = firstname.(string)
	vm.User.Lastname = lastname.(string)

	c.HTML(http.StatusOK, "layout.html", gin.H{
		"Title":  vm.Title,
		"Active": vm.Active,
		"User":   vm.User,
		"Page":   "pages_admin/admin_users.html",
		"VM":     vm,
	})
}

func adminTeamsHandler(c *gin.Context) {
	username, _ := c.Get("kc.username")
	email, _ := c.Get("kc.email")
	rolesAny, _ := c.Get("kc.roles")
	firstname, _ := c.Get("kc.firstname")
	lastname, _ := c.Get("kc.lastname")
	roles, _ := rolesAny.([]string)

	bearer := c.GetString("kc.access_token")
	if bearer == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access_token missing"})
		return
	}

	// 1) fetch all teams as admin
	teamsResp, err := ds.AdminTeams(c.Request.Context(), bearer)
	if err != nil {
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "TeamAPI: " + err.Error()})
		return
	}
	teams := teamsResp.Items

	// 2) fetch tasks for each team concurrently
	type result struct {
		teamID int64
		tasks  []Task
		err    error
	}

	sem := make(chan struct{}, 6)
	ch := make(chan result, len(teams))

	for _, t := range teams {
		teamID := t.TeamID
		sem <- struct{}{}
		go func(teamID int64) {
			defer func() { <-sem }()
			tasksResp, e := ds.TeamTasks(c.Request.Context(), bearer, teamID)
			ch <- result{teamID: teamID, tasks: tasksResp.Items, err: e}
		}(teamID)
	}

	tasksByTeam := make(map[int64][]Task, len(teams))
	for i := 0; i < len(teams); i++ {
		r := <-ch
		if r.err != nil {
			c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "TaskAPI: " + r.err.Error()})
			return
		}
		tasksByTeam[r.teamID] = r.tasks
	}

	// 3) build rows with task summary
	rows := make([]AdminTeamRowVM, 0, len(teams))
	for _, t := range teams {
		ts := tasksByTeam[t.TeamID]

		counts := map[string]int{"TODO": 0, "IN_PROGRESS": 0, "DONE": 0}
		preview := make([]string, 0, 5)
		for _, task := range ts {
			if _, ok := counts[task.Status]; ok {
				counts[task.Status]++
			}
			if len(preview) < 5 {
				preview = append(preview, task.Title)
			}
		}

		rows = append(rows, AdminTeamRowVM{
			Team:          t,
			TotalTasks:    len(ts),
			StatusCounts:  counts,
			PreviewTitles: preview,
		})
	}

	// 4) build VM for layout
	var vm AdminTeamsVM
	vm.Title = "Admin Teams"
	vm.Active = "admin-teams"
	vm.Rows = rows

	vm.User.Username = username.(string)
	vm.User.Email = email.(string)
	vm.User.Roles = roles
	vm.User.IsAdmin = true
	vm.User.Firstname = firstname.(string)
	vm.User.Lastname = lastname.(string)

	c.HTML(http.StatusOK, "layout.html", gin.H{
		"Title":  vm.Title,
		"Active": vm.Active,
		"User":   vm.User,
		"Page":   "pages_admin/admin_teams.html",
		"VM":     vm,
	})
}
