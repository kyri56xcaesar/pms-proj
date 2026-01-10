package mtask

import "time"

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

type CreateTaskRequest struct {
	TeamID      int64      `json:"teamid" form:"teamid" binding:"required,gt=0"`
	Title       string     `json:"title" form:"title" binding:"required,min=2,max=120"`
	Description string     `json:"description" form:"description" binding:"max=2000"`
	Assignee    string     `json:"assignee" form:"assignee" binding:"max=128"`
	Status      string     `json:"status" form:"status" binding:"omitempty,oneof=TODO IN_PROGRESS DONE"`
	Deadline    *time.Time `json:"deadline" form:"deadline"`
	Priority    string     `json:"priority" form:"priority" binding:"omitempty,oneof=LOW MEDIUM HIGH"`
}

type UpdateTaskRequest struct {
	Title       *string    `json:"title" form:"title" binding:"omitempty,min=2,max=120"`
	Description *string    `json:"description" form:"description" binding:"omitempty,max=2000"`
	Assignee    *string    `json:"assignee" form:"assignee" binding:"omitempty,max=128"`
	Status      *string    `json:"status" form:"status" binding:"omitempty,oneof=TODO IN_PROGRESS DONE"`
	Deadline    *time.Time `json:"deadline" form:"deadline"`
	Priority    *string    `json:"priority" form:"priority" binding:"omitempty,oneof=LOW MEDIUM HIGH"`
}

func normalizeLimit(n int) int {
	if n <= 0 {
		return 20
	}
	if n > 100 {
		return 100
	}
	return n
}

func taskOrderClause(order string) string {
	switch order {
	case "created_asc":
		return "created_at ASC"
	case "deadline_asc":
		return "deadline ASC NULLS LAST"
	case "deadline_desc":
		return "deadline DESC NULLS LAST"
	case "priority_desc":
		return "priority DESC" // simplistic, better with enum later
	case "created_desc":
		fallthrough
	default:
		return "created_at DESC"
	}
}

type Comment struct {
	CommentID int64     `json:"commentid"`
	TaskID    int64     `json:"taskid"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateCommentRequest struct {
	TaskID int64  `json:"taskid" form:"taskid" binding:"required,gt=0"`
	Body   string `json:"body" form:"body" binding:"required,min=1,max=2000"`
}

func validStatus(status string) bool {
	return status == "IN_PROGRESS" || status == "TODO" || status == "DONE"
}
