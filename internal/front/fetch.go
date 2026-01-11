package front

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Downstream struct {
	TeamBase string
	TaskBase string
	Client   *http.Client
}

func (d *Downstream) doJSON(ctx context.Context, method, url, bearer string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)

	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("downstream %s %s -> %d: %s", method, url, resp.StatusCode, string(b))
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func (d *Downstream) PatchJSON(ctx context.Context, bearer, url string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bb, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s -> %d: %s", url, resp.StatusCode, string(bb))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (d *Downstream) PostJSON(ctx context.Context, bearer, url string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bb, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s -> %d: %s", url, resp.StatusCode, string(bb))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (d *Downstream) PutJSON(ctx context.Context, bearer, url string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bb, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PUT %s -> %d: %s", url, resp.StatusCode, string(bb))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (d *Downstream) Delete(ctx context.Context, bearer, url string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Accept", "application/json")

	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bb, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DELETE %s -> %d: %s", url, resp.StatusCode, string(bb))
	}
	return nil
}

func (d *Downstream) MyTeams(ctx context.Context, bearer string) (TeamListResponse, error) {
	var teams TeamListResponse
	err := d.doJSON(
		ctx,
		"GET",
		d.TeamBase+"/auth/my-teams",
		bearer,
		&teams)
	return teams, err
}

// expects TaskAPI: GET /auth/tasks?teamId=...
func (d *Downstream) TeamTasks(ctx context.Context, bearer string, teamID int64) (TaskListResponse, error) {
	var tasks TaskListResponse
	url := fmt.Sprintf("%s/auth/tasks?teamid=%v", d.TaskBase, teamID)
	err := d.doJSON(ctx, "GET", url, bearer, &tasks)
	return tasks, err
}

type TasksByTeamsReq struct {
	TeamIDs []int64 `json:"teamids"`
	Limit   int     `json:"limit"`
}

type ItemsResponse[T any] struct {
	Items []T `json:"items"`
	Limit int `json:"limit,omitempty"`
}

func (d *Downstream) AdminTeams(ctx context.Context, bearer string) (ItemsResponse[Team], error) {
	var resp ItemsResponse[Team]
	err := d.doJSON(ctx, "GET", d.TeamBase+"/admin/teams", bearer, &resp)
	return resp, err
}

func (d *Downstream) TaskByID(ctx context.Context, bearer string, taskID int64) (Task, error) {
	var out Task
	url := fmt.Sprintf("%s/auth/tasks/%d", d.TaskBase, taskID)
	err := d.doJSON(ctx, "GET", url, bearer, &out)
	return out, err
}

func (d *Downstream) CommentsByTaskID(ctx context.Context, bearer string, taskID int64) (ItemsResponse[Comment], error) {
	var out ItemsResponse[Comment]
	url := fmt.Sprintf("%s/auth/comments?taskid=%d&limit=200", d.TaskBase, taskID)
	err := d.doJSON(ctx, "GET", url, bearer, &out)
	return out, err
}
