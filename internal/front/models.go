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
	Username  string
	Firstname string
	Lastname  string
	Email     string
	IsAdmin   bool
	Roles     []string
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

type TeamTasksSummary struct {
	TeamID        int64
	Counts        map[string]int // TODO/IN_PROGRESS/DONE
	Total         int
	PreviewTitles []string // optional
}

type MyTeamRowVM struct {
	Team    Team
	Summary TeamTasksSummary
}

type MyTeamsVM struct {
	Title  string
	Active string
	User   UserVM

	IsAdmin   bool
	IsLeader  bool
	CanManage bool

	Rows []MyTeamRowVM
}

type MyTasksVM struct {
	Title  string
	Active string
	User   UserVM

	TotalTasks   int
	StatusCounts map[string]int // optional

	Tasks []Task // you can also make a TaskRowVM if you prefer
}
