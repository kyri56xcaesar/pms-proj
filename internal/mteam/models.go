package mteam

import "time"

type Team struct {
	TeamID      int64     `json:"teamid"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`

	Leader      string       `json:"leader,omitempty"`
	MemberCount int          `json:"memberCount"`
	Members     []TeamMember `json:"members,omitempty"`
}

type TeamMember struct {
	TeamID   int64  `json:"teamid,omitempty"`
	Username string `json:"username"`
	Role     string `json:"role"` // owner/leader/member
}

type CreateTeamRequest struct {
	Name        string `json:"name" form:"name" binding:"required,min=2,max=64"`
	Description string `json:"description" form:"description" binding:"max=500"`
	Leader      string `json:"leader" form:"leader" binding:"required"`
}

type UpdateTeamRequest struct {
	Name        *string `json:"name" form:"name" binding:"required,min=2,max=64"`
	Description *string `json:"description" form:"description" binding:"max=500"`
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

type AddTeamMemberRequest struct {
	Username string `json:"username" binding:"required"`
	Role     string `json:"role"` // optional; default member
}
