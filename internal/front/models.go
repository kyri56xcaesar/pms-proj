package front

import "time"

type Team struct {
	TeamID      int64        `json:"teamid"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	CreatedAt   string       `json:"created_at"`
	Leader      string       `json:"leader"` // optional
	MemberCount int          `json:"memberCount"`
	Members     []TeamMember `json:"members"`
}

type TeamMember struct {
	TeamID   int64  `json:"teamid,omitempty"`
	Username string `json:"username"`
	Role     string `json:"role"` // owner/leader/member
}

type Task struct {
	TaskID      int64     `json:"taskid"`
	TeamID      int64     `json:"teamid"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Author      string    `json:"author"`
	Assignee    string    `json:"assignee"`
	Status      string    `json:"status"`
	Deadline    time.Time `json:"deadline"`
	Priority    string    `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
}

type TeamListResponse struct {
	Items []Team `json:"items"`
	Limit int    `json:"limit"`
}

type TaskListResponse struct {
	Items  []Task `json:"items"`
	Limit  int    `json:"limit"`
	Order  string `json:"order"`
	Status string `json:"status"`
}

type UserVM struct {
	ID            string `json:"id"`
	Username      string
	Firstname     string
	Lastname      string
	Email         string
	IsAdmin       bool
	Enabled       bool `json:"enabled"`
	EmailVerified bool `json:"emailVerified"`
	Roles         []string
}

type DashboardVM struct {
	Title  string
	Active string
	User   UserVM

	TotalTeams int
	TotalTasks int

	StatusCounts map[string]int // TODO, IN_PROGRESS, DONE

	AssignedToMe []Task
	CreatedByMe  []Task

	// optional: list team names too
	Teams []Team
}

type TaskPreviewItem struct {
	TaskID int64
	Title  string
}

type TeamTasksSummary struct {
	TeamID int64
	Counts map[string]int // TODO/IN_PROGRESS/DONE
	Total  int

	Preview []TaskPreviewItem // NEW (replaces PreviewTitles)}
}

type MyTeamRowVM struct {
	TeamID  int64
	Team    Team
	Summary TeamTasksSummary

	Preview []TaskPreviewItem // NEW (replaces PreviewTitles)
}

type MyTeamsVM struct {
	Title  string
	Active string
	User   UserVM

	IsAdmin   bool
	IsLeader  bool
	CanCreate bool // admin only
	CanManage bool

	Rows  []MyTeamRowVM
	Users []UserPick
}
type MyTasksVM struct {
	Title  string
	Active string
	User   UserVM

	CanCreate bool // leader/admin
	CanEdit   bool // leader/admin (for more fields inside modal)
	CanStatus bool // student/leader/admin

	TotalTasks   int
	StatusCounts map[string]int
	Tasks        []Task
}

type AdminTeamRowVM struct {
	Team Team

	TotalTasks    int
	StatusCounts  map[string]int
	PreviewTitles []string
}

type AdminTeamsVM struct {
	Title  string
	Active string
	User   UserVM

	Rows []AdminTeamRowVM
}

type UserPick struct {
	ID       string
	Username string
	Email    string
}

type Comment struct {
	CommentID int64     `json:"commentid"`
	TaskID    int64     `json:"taskid"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
