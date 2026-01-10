package front

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Request struct {
	Username   string `form:"username" json:"username"`
	Password   string `form:"password" json:"password"`
	RepeatPass string `form:"repeat-password,omitempty" json:"repeat-password,omitempty"`
	Email      string `form:"email,omitempty" json:"email,omitempty"`
	Firstname  string `form:"firstname,omitempty" json:"firstname,omitempty"`
	Lastname   string `form:"lastname,omitempty" json:"lastname,omitempty"`
}

func (r Request) validateLogin() error {
	// check if the request fields are allowed
	if r.Username == "" || r.Password == "" {
		return fmt.Errorf("cannot have empty fields")
	}

	return nil
}

func (r Request) validateSignup() error {
	if r.Username == "" || r.Password == "" || r.RepeatPass == "" || r.Email == "" || r.Firstname == "" || r.Lastname == "" {
		return fmt.Errorf("cannot have empty fields")
	}

	if r.Password != r.RepeatPass {
		return fmt.Errorf("passwords don't match")
	}

	return nil
}

func handleLogin(c *gin.Context) {
	var r Request
	if err := c.ShouldBind(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	ctx := context.Background()

	jwt, err := kcService.LoginUser(ctx, r.Username, r.Password)
	if err != nil {
		// Keycloak errors are generic; don't leak details
		log.Printf("failed to login user: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid username or password",
		})

		return
	}

	// OPTIONAL: check user status / roles
	// user, _ := kcService.GetUserByUsername(ctx, jwt.AccessToken, r.Username)

	c.SetSameSite(http.SameSiteLaxMode)

	// Secure HTTP-only cookie
	c.SetCookie(
		"access_token",
		jwt.AccessToken,
		int(jwt.ExpiresIn),
		"/",
		"",
		false, // secure (HTTPS)
		true,  // httpOnly
	)

	// OPTION A: return tokens (SPA / API client)
	c.JSON(http.StatusOK, gin.H{
		"access_token":  jwt.AccessToken,
		"refresh_token": jwt.RefreshToken,
		"expires_in":    jwt.ExpiresIn,
	})
}

func handleRegister(c *gin.Context) {
	var r Request
	if err := c.ShouldBind(&r); err != nil {
		log.Printf("failed to bind: %v", err)
		c.JSON(400, gin.H{"error": "bad data"})
		return
	}

	ctx := context.Background()

	jwt, err := kcService.LoginAdmin(ctx)
	if err != nil {
		log.Printf("auth failed: %v", err)
		c.JSON(500, gin.H{"error": "auth failed"})

		return
	}

	if err := r.validateSignup(); err != nil {
		log.Printf("bad field: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}

	userID, err := kcService.CreateUser(
		ctx,
		jwt.AccessToken,
		r.Username,
		r.Email,
		r.Password,
		r.Firstname,
		r.Lastname,
	)
	if err != nil {
		log.Printf("failed to create a user: %v", err)
		c.JSON(400, gin.H{"error": err.Error()})

		return
	}

	_ = kcService.AddUserToGroup(ctx, jwt.AccessToken, userID, "user")

	c.JSON(201, gin.H{"success": true})
}

func dashboardHandler(c *gin.Context) {
	username, _ := c.Get("kc.username")
	email, _ := c.Get("kc.email")
	rolesAny, _ := c.Get("kc.roles")
	firstname, _ := c.Get("kc.firstname")
	lastname, _ := c.Get("kc.lastname")

	roles, _ := rolesAny.([]string)
	isAdmin := false
	for _, r := range roles {
		if r == "admin" {
			isAdmin = true
			break
		}
	}
	bearer := c.GetString("kc.access_token") // adapt to your storage
	if bearer == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access_token missing"})

		return
	}
	teamListResponse, err := ds.MyTeams(c.Request.Context(), bearer)
	if err != nil {
		log.Printf("failed to retrieve teams: %v", err)
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "TeamAPI: " + err.Error()})

		return
	}
	teams := teamListResponse.Items

	log.Printf("teams found: %+v", teams)

	type result struct {
		teamID int64
		tasks  []Task
		err    error
	}

	sem := make(chan struct{}, 6) // max 6 concurrent calls
	ch := make(chan result, len(teams))

	for _, t := range teams {
		teamID := t.TeamID
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			tasksResponse, e := ds.TeamTasks(context.Background(), bearer, teamID)
			log.Printf("tasks response: %+v", tasksResponse)
			ch <- result{teamID: teamID, tasks: tasksResponse.Items, err: e}
		}()
	}

	allTasks := make([]Task, 0, 128)
	for i := 0; i < len(teams); i++ {
		r := <-ch
		if r.err != nil {
			log.Printf("failed to retrieve tasks: %v", r.err)
			c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "TaskAPI: " + r.err.Error()})

			return
		}
		allTasks = append(allTasks, r.tasks...)
	}

	// Aggregate
	counts := map[string]int{"TODO": 0, "IN_PROGRESS": 0, "DONE": 0}
	assigned := make([]Task, 0, 32)
	created := make([]Task, 0, 32)

	for _, task := range allTasks {
		if _, ok := counts[task.Status]; ok {
			counts[task.Status]++
		} else {
			// unknown statuses: either ignore or track separately
		}

		if task.Assignee == username {
			assigned = append(assigned, task)
		}
		if task.Author == username {
			created = append(created, task)
		}
	}

	// Optional: cap lists to avoid huge page
	if len(assigned) > 10 {
		assigned = assigned[:10]
	}
	if len(created) > 10 {
		created = created[:10]
	}

	// Build VM
	var vm DashboardVM
	vm.Title = "Dashboard"
	vm.Active = "dashboard"
	vm.User.Username = username.(string)
	vm.User.Roles = roles
	vm.User.IsAdmin = isAdmin
	vm.User.Email = email.(string)
	vm.User.Firstname = firstname.(string)
	vm.User.Lastname = lastname.(string)

	vm.TotalTeams = len(teams)
	vm.TotalTasks = len(allTasks)
	vm.StatusCounts = counts
	vm.AssignedToMe = assigned
	vm.CreatedByMe = created
	vm.Teams = teams

	c.HTML(http.StatusOK, "layout.html", gin.H{
		"Title":  vm.Title,
		"Active": vm.Active,
		"User":   vm.User,
		"Page":   "pages/dashboard.html",
		"Data":   vm, // if you prefer: pass as ".Data"
		// or flatten: "TotalTeams":..., etc.
		"VM": vm,
	})

}

func logoutHandler(c *gin.Context) {
	c.SetCookie(
		"access_token",
		"",
		-1,
		"/",
		"",
		false,
		true,
	)

	c.Redirect(http.StatusSeeOther, "/api/v1/login")
}

func userInfoHandler(c *gin.Context) {
	username, _ := c.Get("kc.username")
	email, _ := c.Get("kc.email")
	active, _ := c.Get("kc.email_verified")
	sub, _ := c.Get("kc.sub")
	rolesAny, _ := c.Get("kc.roles")
	roles, _ := rolesAny.([]string)

	isAdmin := false
	for _, r := range roles {
		if r == "admin" {
			isAdmin = true
			break
		}
	}

	payload := gin.H{
		"username":  username,
		"email":     email,
		"sub":       sub,
		"roles":     roles,
		"isAdmin":   isAdmin,
		"activated": active,
	}

	respondInFormat(c, payload, "user_info.html")
}

func myTasksHandler(c *gin.Context) {
	usernameAny, _ := c.Get("kc.username")
	emailAny, _ := c.Get("kc.email")
	rolesAny, _ := c.Get("kc.roles")
	firstnameAny, _ := c.Get("kc.firstname")
	lastnameAny, _ := c.Get("kc.lastname")

	username := usernameAny.(string)
	email := emailAny.(string)
	firstname := firstnameAny.(string)
	lastname := lastnameAny.(string)

	roles, _ := rolesAny.([]string)
	isAdmin := false
	for _, r := range roles {
		if r == "admin" {
			isAdmin = true
			break
		}
	}

	bearer := c.GetString("kc.access_token")
	if bearer == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access_token missing"})
		return
	}

	// 1) Get my teams (so we know which team tasks to pull)
	teamListResponse, err := ds.MyTeams(c.Request.Context(), bearer)
	if err != nil {
		log.Printf("failed to retrieve teams: %v", err)
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "TeamAPI: " + err.Error()})
		return
	}
	teams := teamListResponse.Items

	// 2) Fetch tasks for each team concurrently
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
			tasksResponse, e := ds.TeamTasks(c.Request.Context(), bearer, teamID)
			ch <- result{teamID: teamID, tasks: tasksResponse.Items, err: e}
		}(teamID)
	}

	// 3) Merge + filter only tasks assigned to me
	myTasks := make([]Task, 0, 64)
	for i := 0; i < len(teams); i++ {
		r := <-ch
		if r.err != nil {
			log.Printf("failed to retrieve tasks: %v", r.err)
			c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "TaskAPI: " + r.err.Error()})
			return
		}
		for _, task := range r.tasks {
			if task.Assignee == username {
				myTasks = append(myTasks, task)
			}
		}
	}

	// Optional: counts per status (nice for header summary)
	counts := map[string]int{"TODO": 0, "IN_PROGRESS": 0, "DONE": 0}
	for _, t := range myTasks {
		if _, ok := counts[t.Status]; ok {
			counts[t.Status]++
		}
	}

	// 4) Build VM
	var vm MyTasksVM
	vm.Title = "My Tasks"
	vm.Active = "mytasks"
	vm.TotalTasks = len(myTasks)
	vm.StatusCounts = counts
	vm.Tasks = myTasks

	vm.User.Username = username
	vm.User.Roles = roles
	vm.User.IsAdmin = isAdmin
	vm.User.Email = email
	vm.User.Firstname = firstname
	vm.User.Lastname = lastname

	c.HTML(http.StatusOK, "layout.html", gin.H{
		"Title":  vm.Title,
		"Active": vm.Active,
		"User":   vm.User,
		"Page":   "pages/mytasks.html",
		"VM":     vm,
	})
}

func myTeamsHandler(c *gin.Context) {
	username, _ := c.Get("kc.username")
	email, _ := c.Get("kc.email")
	rolesAny, _ := c.Get("kc.roles")
	firstname, _ := c.Get("kc.firstname")
	lastname, _ := c.Get("kc.lastname")

	roles, _ := rolesAny.([]string)

	isAdmin := false
	isLeader := false
	for _, r := range roles {
		if r == "admin" {
			isAdmin = true
		}
		if r == "leader" {
			isLeader = true
		}
	}
	canManage := isAdmin || isLeader

	bearer := c.GetString("kc.access_token")
	if bearer == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access_token missing"})
		return
	}

	// 1) Get my teams
	teamListResponse, err := ds.MyTeams(c.Request.Context(), bearer)
	if err != nil {
		log.Printf("failed to retrieve teams: %v", err)
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "TeamAPI: " + err.Error()})
		return
	}
	teams := teamListResponse.Items

	type result struct {
		teamID int64
		tasks  []Task
		err    error
	}

	// 2) Get tasks per team concurrently (same pattern as dashboard)
	sem := make(chan struct{}, 6)
	ch := make(chan result, len(teams))

	for _, t := range teams {
		teamID := t.TeamID
		sem <- struct{}{}
		go func(teamID int64) {
			defer func() { <-sem }()

			// IMPORTANT: use request context, not context.Background()
			tasksResponse, e := ds.TeamTasks(c.Request.Context(), bearer, teamID)

			ch <- result{teamID: teamID, tasks: tasksResponse.Items, err: e}
		}(teamID)
	}

	tasksByTeam := make(map[int64][]Task, len(teams))
	for i := 0; i < len(teams); i++ {
		r := <-ch
		if r.err != nil {
			log.Printf("failed to retrieve tasks: %v", r.err)
			c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "TaskAPI: " + r.err.Error()})
			return
		}
		tasksByTeam[r.teamID] = r.tasks
	}

	// 3) Build per-team summaries
	rows := make([]MyTeamRowVM, 0, len(teams))
	for _, team := range teams {
		teamTasks := tasksByTeam[team.TeamID]

		counts := map[string]int{"TODO": 0, "IN_PROGRESS": 0, "DONE": 0}
		preview := make([]string, 0, 5)

		for _, task := range teamTasks {
			if _, ok := counts[task.Status]; ok {
				counts[task.Status]++
			}
			if len(preview) < 5 {
				preview = append(preview, task.Title)
			}
		}

		rows = append(rows, MyTeamRowVM{
			Team: team,
			Summary: TeamTasksSummary{
				TeamID:        team.TeamID,
				Counts:        counts,
				Total:         len(teamTasks),
				PreviewTitles: preview,
			},
		})
	}

	// 4) Build VM
	var vm MyTeamsVM
	vm.Title = "My Teams"
	vm.Active = "teams"
	vm.IsAdmin = isAdmin
	vm.IsLeader = isLeader
	vm.CanManage = canManage
	vm.Rows = rows

	vm.User.Username = username.(string)
	vm.User.Roles = roles
	vm.User.IsAdmin = isAdmin
	vm.User.Email = email.(string)
	vm.User.Firstname = firstname.(string)
	vm.User.Lastname = lastname.(string)

	c.HTML(http.StatusOK, "layout.html", gin.H{
		"Title":  vm.Title,
		"Active": vm.Active,
		"User":   vm.User,
		"Page":   "pages/myteams.html",
		"VM":     vm,
	})
}

func adminTeamsHandler(c *gin.Context) {
	// Page user info (logged-in admin)
	username, _ := c.Get("kc.username")
	email, _ := c.Get("kc.email")
	rolesAny, _ := c.Get("kc.roles")
	firstname, _ := c.Get("kc.firstname")
	lastname, _ := c.Get("kc.lastname")
	roles, _ := rolesAny.([]string)

	// Use service account token to call KC Admin API
	jwt, err := kcService.LoginAdmin(c.Request.Context())
	if err != nil {
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "Keycloak login (service): " + err.Error()})
		return
	}

	users, err := kcService.ListAdminUsers(c.Request.Context(), jwt.AccessToken, 200)
	if err != nil {
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "Keycloak list users: " + err.Error()})
		return
	}

	// VM (match your layout usage)
	var vm struct {
		Title  string
		Active string
		User   UserVM
		Users  []AdminUser
	}
	vm.Title = "Admin Users"
	vm.Active = "admin-users"
	vm.Users = users

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
		"Page":   "pages/admin_users.html",
		"VM":     vm,
	})
}
