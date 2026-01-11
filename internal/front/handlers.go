package front

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Nerzal/gocloak/v13"
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
	isLeader := false
	for _, r := range roles {
		if r == "leader" {
			isLeader = true
		}
		if r == "admin" {
			isAdmin = true
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
	vm.CanCreate = isLeader || isAdmin
	vm.CanEdit = isLeader || isAdmin
	vm.CanStatus = true // since verified already ensures student/admin; keep true

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
	canCreate := isAdmin
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
		preview := make([]TaskPreviewItem, 0, 5)

		for _, task := range teamTasks {
			if _, ok := counts[task.Status]; ok {
				counts[task.Status]++
			}
			if len(preview) < 5 {
				preview = append(preview, TaskPreviewItem{
					TaskID: task.TaskID,
					Title:  task.Title,
				})
			}
		}

		rows = append(rows, MyTeamRowVM{
			TeamID: team.TeamID,
			Team:   team,
			Summary: TeamTasksSummary{
				TeamID:  team.TeamID,
				Counts:  counts,
				Total:   len(teamTasks),
				Preview: preview,
			},
		})

	}

	var users []UserPick
	if isAdmin {
		jwt, err := kcService.LoginAdmin(c.Request.Context())
		if err == nil {
			kcUsers, err := kcService.Client.GetUsers(c.Request.Context(), jwt.AccessToken, kcService.Realm, gocloak.GetUsersParams{
				Max: gocloak.IntP(200),
			})
			if err == nil {
				users = make([]UserPick, 0, len(kcUsers))
				for _, u := range kcUsers {
					if u == nil {
						continue
					}
					users = append(users, UserPick{
						ID:       derefStr(u.ID),
						Username: derefStr(u.Username),
						Email:    derefStr(u.Email),
					})
				}
				sort.Slice(users, func(i, j int) bool { return users[i].Username < users[j].Username })
			}
		}
	}

	// 4) Build VM
	var vm MyTeamsVM
	vm.Title = "My Teams"
	vm.Active = "teams"
	vm.IsAdmin = isAdmin
	vm.IsLeader = isLeader
	vm.CanCreate = canCreate
	vm.CanManage = canManage
	vm.Rows = rows

	vm.Users = users // add field to MyTeamsVM
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

func createTeamHandler(c *gin.Context) {
	bearer := c.GetString("kc.access_token")
	if bearer == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access_token missing"})
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	desc := strings.TrimSpace(c.PostForm("description"))
	leader := strings.TrimSpace(c.PostForm("leader"))

	if name == "" || leader == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "name and leader required"})
		return
	}

	req := gin.H{"name": name, "description": desc, "leader": leader}

	// Forward to TeamAPI: POST /admin/teams
	if err := ds.PostJSON(c.Request.Context(), bearer, ds.TeamBase+"/admin/teams", req, nil); err != nil {
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "TeamAPI: " + err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/api/v1/auth/myteams")
}

func editTeamHandler(c *gin.Context) {
	bearer := c.GetString("kc.access_token")
	if bearer == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access_token missing"})
		return
	}

	idStr := strings.TrimSpace(c.PostForm("teamid"))
	name := strings.TrimSpace(c.PostForm("name"))
	desc := strings.TrimSpace(c.PostForm("description"))

	teamID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || teamID <= 0 {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "invalid teamid"})
		return
	}
	if name == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "name required"})
		return
	}

	req := gin.H{"teamid": teamID, "name": name, "description": desc}

	// Forward to TeamAPI: PUT /admin/teams
	if err := ds.PutJSON(c.Request.Context(), bearer, ds.TeamBase+"/admin/teams?teamid="+idStr, req, nil); err != nil {
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "TeamAPI: " + err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/api/v1/auth/myteams")
}

func addMemberHandler(c *gin.Context) {
	bearer := c.GetString("kc.access_token")
	if bearer == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access_token missing"})
		return
	}

	idStr := strings.TrimSpace(c.PostForm("teamid"))
	username := strings.TrimSpace(c.PostForm("username"))

	teamID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || teamID <= 0 {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "invalid teamid"})
		return
	}
	if username == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "username required"})
		return
	}

	req := gin.H{"username": username}

	url := fmt.Sprintf("%s/leader/teams/%d/members", ds.TeamBase, teamID)
	if err := ds.PostJSON(c.Request.Context(), bearer, url, req, nil); err != nil {
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "TeamAPI: " + err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/api/v1/auth/myteams")
}

func removeMemberHandler(c *gin.Context) {
	bearer := c.GetString("kc.access_token")
	if bearer == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access_token missing"})
		return
	}

	idStr := strings.TrimSpace(c.PostForm("teamid"))
	username := strings.TrimSpace(c.PostForm("username"))

	log.Printf("id: %v, username: %v", idStr, username)

	teamID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || teamID <= 0 {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "invalid teamid"})
		return
	}
	if username == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "username required"})
		return
	}

	// Forward to TeamAPI: DELETE /admin/teams/:teamid/members/:username
	url := fmt.Sprintf("%s/leader/teams/%d/members/%s", ds.TeamBase, teamID, url.PathEscape(username))
	if err := ds.Delete(c.Request.Context(), bearer, url); err != nil {
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "TeamAPI: " + err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/api/v1/auth/myteams")
}

func deleteTeamHandler(c *gin.Context) {
	bearer := c.GetString("kc.access_token")
	if bearer == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access_token missing"})
		return
	}

	teamIDStr := c.Param("teamid")
	teamID, err := strconv.ParseInt(teamIDStr, 10, 64)
	if err != nil || teamID <= 0 {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "invalid teamid"})
		return
	}

	// example: DELETE /admin/teams?teamid=123
	url := fmt.Sprintf("%s/admin/teams?teamid=%d", ds.TeamBase, teamID)
	if err := ds.Delete(c.Request.Context(), bearer, url); err != nil {
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "TeamAPI: " + err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/api/v1/auth/myteams")
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func derefBool(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}

func taskDetailJSONHandler(c *gin.Context) {
	bearer := c.GetString("kc.access_token")
	if bearer == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access_token missing"})
		return
	}

	idStr := c.Param("id")
	taskID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	// fetch task + comments (parallel)
	type resT struct {
		task Task
		err  error
	}
	type resC struct {
		items []Comment
		err   error
	}

	chT := make(chan resT, 1)
	chC := make(chan resC, 1)

	go func() {
		t, e := ds.TaskByID(c.Request.Context(), bearer, taskID)
		chT <- resT{task: t, err: e}
	}()
	go func() {
		cr, e := ds.CommentsByTaskID(c.Request.Context(), bearer, taskID)
		chC <- resC{items: cr.Items, err: e}
	}()

	rt := <-chT
	if rt.err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "TaskAPI: " + rt.err.Error()})
		return
	}
	rc := <-chC
	if rc.err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "TaskAPI: " + rc.err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task":     rt.task,
		"comments": rc.items,
	})
}

func taskStatusHandler(c *gin.Context) {
	bearer := c.GetString("kc.access_token")
	if bearer == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access_token missing"})
		return
	}

	idStr := c.Param("id")
	taskID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	status := strings.TrimSpace(c.PostForm("status"))
	if status == "" {
		// allow JSON too
		var body struct {
			Status string `json:"status"`
		}
		if err := c.ShouldBindJSON(&body); err == nil {
			status = strings.TrimSpace(body.Status)
		}
	}

	switch status {
	case "TODO", "IN_PROGRESS", "DONE":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}

	req := gin.H{"taskid": taskID, "status": status}
	log.Printf("sending req to update stauts: %+v", req)
	url := fmt.Sprintf("%s/auth/change-status?taskid=%v&status=%v", ds.TaskBase, taskID, status)

	if err := ds.PatchJSON(c.Request.Context(), bearer, url, req, nil); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "TaskAPI: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type CreateTaskForm struct {
	TeamID      string
	Title       string
	Description string
	Assignee    string
	Priority    string
	Deadline    string // yyyy-mm-dd from <input type="date">
}

func createTaskHandler(c *gin.Context) {
	bearer := c.GetString("kc.access_token")
	if bearer == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access_token missing"})
		return
	}

	// Read form fields
	f := CreateTaskForm{
		TeamID:      strings.TrimSpace(c.PostForm("teamid")),
		Title:       strings.TrimSpace(c.PostForm("title")),
		Description: strings.TrimSpace(c.PostForm("description")),
		Assignee:    strings.TrimSpace(c.PostForm("assignee")),
		Priority:    strings.TrimSpace(c.PostForm("priority")),
		Deadline:    strings.TrimSpace(c.PostForm("deadline")),
	}

	// Validate teamid
	teamID, err := strconv.ParseInt(f.TeamID, 10, 64)
	if err != nil || teamID <= 0 {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "invalid teamid"})
		return
	}

	if f.Title == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "title required"})
		return
	}
	if f.Assignee == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "assignee required"})
		return
	}

	// Normalize priority
	pr := strings.ToUpper(f.Priority)
	if pr == "" {
		pr = "MEDIUM"
	}
	switch pr {
	case "LOW", "MEDIUM", "HIGH":
	default:
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "invalid priority"})
		return
	}

	// Deadline: optional
	// You can pass it as string to TaskAPI and let it parse, or parse here.
	// We'll parse here into RFC3339 to be consistent.
	var deadlineRFC3339 *string
	if f.Deadline != "" {
		// HTML date is yyyy-mm-dd
		d, err := time.Parse("2006-01-02", f.Deadline)
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "invalid deadline"})
			return
		}
		// choose midnight UTC; or local. UTC is simplest for APIs.
		v := d.UTC().Format(time.RFC3339)
		deadlineRFC3339 = &v
	}

	// Build JSON request for TaskAPI
	req := gin.H{
		"teamid":      teamID,
		"title":       f.Title,
		"description": f.Description,
		"assignee":    f.Assignee,
		"priority":    pr,
	}
	if deadlineRFC3339 != nil {
		req["deadline"] = *deadlineRFC3339
	}

	// Forward to TaskAPI
	url := fmt.Sprintf("%s/auth/tasks", ds.TaskBase)
	if err := ds.PostJSON(c.Request.Context(), bearer, url, req, nil); err != nil {
		c.HTML(http.StatusBadGateway, "error.html", gin.H{"error": "TaskAPI: " + err.Error()})
		return
	}

	c.Redirect(http.StatusSeeOther, "/api/v1/auth/mytasks")
}

func addCommentHandler(c *gin.Context) {
	bearer := c.GetString("kc.access_token")
	if bearer == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access_token missing"})
		return
	}

	idStr := c.Param("id")
	taskID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	body := strings.TrimSpace(c.PostForm("body"))
	if body == "" {
		// allow JSON too
		var j struct {
			Body string `json:"body"`
		}
		if err := c.ShouldBindJSON(&j); err == nil {
			body = strings.TrimSpace(j.Body)
		}
	}
	if body == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body required"})
		return
	}

	req := gin.H{"taskid": taskID, "body": body}
	url := fmt.Sprintf("%s/auth/comments", ds.TaskBase)

	var resp any
	if err := ds.PostJSON(c.Request.Context(), bearer, url, req, &resp); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "TaskAPI: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "ok"})
}
