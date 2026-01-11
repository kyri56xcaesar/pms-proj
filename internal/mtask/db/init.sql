create table if not exists tasks (
    taskid bigint generated always as identity primary key,
    teamid bigint references teams(teamid) on delete cascade,
    title text,
    description text,
    author text,
    assignee text,
    status text,
    deadline timestamptz,
    priority text,
    created_at timestamptz not null default now()
);

create table if not exists task_comments (
    commentid bigint generated always as identity primary key,
    taskid bigint  references tasks(taskid) on delete cascade,
    author text,
    body       text not null,
    created_at timestamptz not null default now()
);


create index if not exists idx_tasks_teamid_created on tasks(teamid, created_at desc);
create index if not exists idx_tasks_assignee on tasks(assignee);
create index if not exists idx_tasks_status on tasks(status);

create index if not exists idx_task_comments_taskid_created on task_comments(taskid, created_at asc);