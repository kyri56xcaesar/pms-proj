create table if not exists teams (
    teamid bigint generated always as identity primary key,
    name text,
    description text,
    created_at timestamptz not null default now()
);

create table if not exists team_members (
  teamid   bigint not null references teams(teamid) on delete cascade,
  username text not null,
  role     text not null default 'member', -- 'owner' | 'leader' | 'member'
  joined_at timestamptz not null default now(),
  primary key (teamid, username)
);

create index if not exists idx_team_members_username on team_members(username);

CREATE UNIQUE INDEX IF NOT EXISTS team_one_leader_per_team
ON team_members(teamid)
WHERE role = 'leader';

CREATE UNIQUE INDEX IF NOT EXISTS team_members_unique
ON team_members(teamid, username);
