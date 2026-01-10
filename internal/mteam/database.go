package mteam

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

func CreateTeam(ctx context.Context, name, desc, ownerUsername string) (int64, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var id int64
	err = tx.QueryRow(ctx,
		`insert into teams(name, description) values($1, $2) returning teamid`,
		name, desc,
	).Scan(&id)
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx, `
		insert into team_members (teamid, username, role)
		values ($1, $2, 'owner')
	`, id, ownerUsername)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return id, nil
}

func DeleteTeam(ctx context.Context, id int64) error {
	res, err := pool.Exec(ctx, `delete from teams where teamid = $1`, id)
	if err != nil {
		return err
	}

	n := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func orderClause(order string) string {
	switch order {
	case "created_asc":
		return "created_at ASC"
	case "name_asc":
		return "name ASC"
	case "name_desc":
		return "name DESC"
	case "created_desc":
		fallthrough
	default:
		return "created_at DESC"
	}
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func ListTeams(
	ctx context.Context,
	teamID *int64,
	name *string,
	limit int,
	order string,
) ([]Team, error) {

	limit = normalizeLimit(limit)
	orderSQL := orderClause(order) // keep your existing

	var (
		where  string
		args   []any
		argIdx = 1
	)

	if teamID != nil {
		where = fmt.Sprintf("WHERE t.teamid = $%d", argIdx)
		args = append(args, *teamID)
		argIdx++
	} else if name != nil && strings.TrimSpace(*name) != "" {
		where = fmt.Sprintf("WHERE t.name ILIKE $%d", argIdx)
		args = append(args, "%"+strings.TrimSpace(*name)+"%")
		argIdx++
	}

	// LIMIT placeholder is argIdx
	query := fmt.Sprintf(`
        SELECT
          t.teamid,
          t.name,
          COALESCE(t.description,'') AS description,
          t.created_at,

          COALESCE((
            SELECT tm.username
            FROM team_members tm
            WHERE tm.teamid = t.teamid AND tm.role = 'leader'
            LIMIT 1
          ), '') AS leader,

          COALESCE(COUNT(m.username), 0) AS member_count,

          COALESCE(
            json_agg(
              json_build_object('username', m.username, 'role', m.role)
              ORDER BY (m.role = 'leader') DESC, m.username
            ) FILTER (WHERE m.username IS NOT NULL),
            '[]'::json
          ) AS members_json



        FROM teams t
        LEFT JOIN team_members m ON m.teamid = t.teamid
        %s
        GROUP BY t.teamid
        ORDER BY %s
        LIMIT $%d
    `, where, orderSQL, argIdx)

	args = append(args, limit)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Team, 0, limit)
	for rows.Next() {
		var (
			t           Team
			membersJSON []byte
		)
		if err := rows.Scan(
			&t.TeamID,
			&t.Name,
			&t.Description,
			&t.CreatedAt,
			&t.Leader,
			&t.MemberCount,
			&membersJSON,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(membersJSON, &t.Members); err != nil {
			return nil, fmt.Errorf("unmarshal members_json: %w", err)
		}

		out = append(out, t)
	}
	return out, rows.Err()
}

func UpdateTeam(ctx context.Context, teamID int64, req UpdateTeamRequest) error {
	sets := make([]string, 0, 2)
	args := make([]any, 0, 3)
	i := 1

	if req.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", i))
		args = append(args, strings.TrimSpace(*req.Name))
		i++
	}
	if req.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", i))
		args = append(args, *req.Description)
		i++
	}

	if len(sets) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// WHERE teamid = $i
	args = append(args, teamID)
	q := fmt.Sprintf("UPDATE teams SET %s WHERE teamid = $%d", strings.Join(sets, ", "), i)

	ct, err := pool.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func AddMember(ctx context.Context, teamID int64, username, role string) error {
	if role == "" {
		role = "member"
	}

	_, err := pool.Exec(ctx, `
        INSERT INTO team_members (teamid, username, role)
        VALUES ($1, $2, $3)
        ON CONFLICT (teamid, username) DO UPDATE SET role = EXCLUDED.role
    `, teamID, username, role)
	return err
}

func RemoveMember(ctx context.Context, teamID int64, username string) error {
	ct, err := pool.Exec(ctx, `
        DELETE FROM team_members
        WHERE teamid = $1 AND username = $2
    `, teamID, username)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func ListTeamsForUser(ctx context.Context, username string, limit int) ([]Team, error) {
	limit = normalizeLimit(limit)

	rows, err := pool.Query(ctx, `
        SELECT
          t.teamid,
          t.name,
          COALESCE(t.description,'') AS description,
          t.created_at,

          COALESCE((
            SELECT tm.username
            FROM team_members tm
            WHERE tm.teamid = t.teamid AND tm.role = 'leader'
            LIMIT 1
          ), '') AS leader,

          COALESCE(COUNT(m.username), 0) AS member_count,

          COALESCE(
            json_agg(
              json_build_object('username', m.username, 'role', m.role)
              ORDER BY (m.role = 'leader') DESC, m.username
            ) FILTER (WHERE m.username IS NOT NULL),
            '[]'::json
          ) AS members_json

        FROM teams t
        -- restrict to teams that THIS user belongs to
        JOIN team_members me
          ON me.teamid = t.teamid AND me.username = $1

        -- aggregate ALL members for those teams
        LEFT JOIN team_members m
          ON m.teamid = t.teamid

        GROUP BY t.teamid
        ORDER BY t.created_at DESC
        LIMIT $2
    `, username, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Team, 0, limit)
	for rows.Next() {
		var t Team
		if err := rows.Scan(
			&t.TeamID,
			&t.Name,
			&t.Description,
			&t.CreatedAt,
			&t.Leader,
			&t.MemberCount,
			&t.Members,
		); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func IsMember(ctx context.Context, teamID int64, username string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
        SELECT EXISTS(
            SELECT 1 FROM team_members
            WHERE teamid = $1 AND username = $2
        )
    `, teamID, username).Scan(&exists)
	return exists, err
}
