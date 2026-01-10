package front

import (
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
