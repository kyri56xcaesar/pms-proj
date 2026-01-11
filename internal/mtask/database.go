package mtask

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func CreateTask(ctx context.Context, author string, req CreateTaskRequest) (int64, error) {
	status := req.Status
	if status == "" {
		status = "TODO"
	}
	priority := req.Priority
	if priority == "" {
		priority = "MEDIUM"
	}

	var id int64
	err := pool.QueryRow(ctx, `
		INSERT INTO tasks (teamid, title, description, author, assignee, status, deadline, priority)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING taskid
	`, req.TeamID, req.Title, req.Description, author, req.Assignee, status, req.Deadline, priority).Scan(&id)
	return id, err
}

func DeleteTask(ctx context.Context, taskID int64) error {
	ct, err := pool.Exec(ctx, `DELETE FROM tasks WHERE taskid = $1`, taskID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func UpdateTask(ctx context.Context, taskID int64, req UpdateTaskRequest) error {
	sets := make([]string, 0, 6)
	args := make([]any, 0, 7)
	i := 1

	if req.Title != nil {
		sets = append(sets, fmt.Sprintf("title = $%d", i))
		args = append(args, *req.Title)
		i++
	}
	if req.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", i))
		args = append(args, *req.Description)
		i++
	}
	if req.Assignee != nil {
		sets = append(sets, fmt.Sprintf("assignee = $%d", i))
		args = append(args, *req.Assignee)
		i++
	}
	if req.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", i))
		args = append(args, *req.Status)
		i++
	}
	if req.Deadline != nil {
		sets = append(sets, fmt.Sprintf("deadline = $%d", i))
		args = append(args, *req.Deadline)
		i++
	}
	if req.Priority != nil {
		sets = append(sets, fmt.Sprintf("priority = $%d", i))
		args = append(args, *req.Priority)
		i++
	}

	if len(sets) == 0 {
		return fmt.Errorf("no fields to update")
	}

	args = append(args, taskID)
	q := fmt.Sprintf("UPDATE tasks SET %s WHERE taskid = $%d", strings.Join(sets, ", "), i)

	ct, err := pool.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

type ListTasksFilter struct {
	TeamID   int64
	Assignee string
	Status   string
	Limit    int
	Order    string
}

func ListTasks(ctx context.Context, f ListTasksFilter) ([]Task, error) {
	if f.TeamID <= 0 {
		return nil, fmt.Errorf("teamid required")
	}
	limit := normalizeLimit(f.Limit)
	orderSQL := taskOrderClause(f.Order)

	where := []string{"teamid = $1"}
	args := []any{f.TeamID}
	i := 2

	if strings.TrimSpace(f.Assignee) != "" {
		where = append(where, fmt.Sprintf("assignee = $%d", i))
		args = append(args, strings.TrimSpace(f.Assignee))
		i++
	}
	if strings.TrimSpace(f.Status) != "" {
		where = append(where, fmt.Sprintf("status = $%d", i))
		args = append(args, strings.TrimSpace(f.Status))
		i++
	}

	q := fmt.Sprintf(`
		SELECT taskid, teamid, title, COALESCE(description,''), author, COALESCE(assignee,''), status,
		       deadline, priority, created_at
		FROM tasks
		WHERE %s
		ORDER BY %s
		LIMIT $%d
	`, strings.Join(where, " AND "), orderSQL, i)

	args = append(args, limit)

	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Task, 0, limit)
	for rows.Next() {
		var t Task
		var deadline *time.Time
		if err := rows.Scan(&t.TaskID, &t.TeamID, &t.Title, &t.Description, &t.Author, &t.Assignee,
			&t.Status, &deadline, &t.Priority, &t.CreatedAt); err != nil {
			return nil, err
		}
		if deadline != nil {
			t.Deadline = *deadline
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func CreateComment(ctx context.Context, taskID int64, author, body string) (int64, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return 0, fmt.Errorf("empty body")
	}

	var id int64
	err := pool.QueryRow(ctx, `
		INSERT INTO task_comments (taskid, author, body)
		VALUES ($1, $2, $3)
		RETURNING commentid
	`, taskID, author, body).Scan(&id)
	return id, err
}

func DeleteComment(ctx context.Context, commentID int64) error {
	ct, err := pool.Exec(ctx, `DELETE FROM task_comments WHERE commentid = $1`, commentID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func ListComments(ctx context.Context, taskID int64, limit int) ([]Comment, error) {
	if taskID <= 0 {
		return nil, fmt.Errorf("taskid required")
	}
	limit = normalizeLimit(limit)

	rows, err := pool.Query(ctx, `
		SELECT commentid, taskid, author, body, created_at
		FROM task_comments
		WHERE taskid = $1
		ORDER BY created_at ASC
		LIMIT $2
	`, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Comment, 0, limit)
	for rows.Next() {
		var cmt Comment
		if err := rows.Scan(&cmt.CommentID, &cmt.TaskID, &cmt.Author, &cmt.Body, &cmt.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, cmt)
	}
	return out, rows.Err()
}

func GetTaskByID(ctx context.Context, taskID int64) (*Task, error) {
	row := pool.QueryRow(ctx, `
		SELECT taskid, teamid, COALESCE(title,''), COALESCE(description,''),
		       COALESCE(author,''), COALESCE(assignee,''), COALESCE(status,''),
		       deadline, COALESCE(priority,''), created_at
		FROM tasks
		WHERE taskid = $1
	`, taskID)

	var t Task
	var deadline sql.NullTime

	if err := row.Scan(
		&t.TaskID,
		&t.TeamID,
		&t.Title,
		&t.Description,
		&t.Author,
		&t.Assignee,
		&t.Status,
		&deadline,
		&t.Priority,
		&t.CreatedAt,
	); err != nil {
		return nil, err
	}

	if deadline.Valid {
		t.Deadline = deadline.Time
	}

	return &t, nil
}

func ListCommentsByTaskID(ctx context.Context, taskID int64, limit int, order string) ([]Comment, error) {
	orderSQL := "created_at ASC"
	if order == "created_desc" {
		orderSQL = "created_at DESC"
	}

	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT commentid, taskid, COALESCE(author,''), body, created_at
		FROM task_comments
		WHERE taskid = $1
		ORDER BY %s
		LIMIT $2
	`, orderSQL), taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Comment, 0, limit)
	for rows.Next() {
		var cmt Comment
		if err := rows.Scan(&cmt.CommentID, &cmt.TaskID, &cmt.Author, &cmt.Body, &cmt.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, cmt)
	}
	return out, rows.Err()
}
