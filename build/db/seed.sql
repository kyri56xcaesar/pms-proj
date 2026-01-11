-- =========================================================
-- TaskBoard demo seed data
-- =========================================================

BEGIN;

-- ---------------------------------------------------------
-- Teams
-- ---------------------------------------------------------
INSERT INTO teams (name, description)
VALUES
  ('Platform', 'Platform and infrastructure work'),
  ('Frontend', 'UI and user experience'),
  ('Backend', 'Core APIs and services');

-- ---------------------------------------------------------
-- Team members
-- Assumes users exist in Keycloak:
-- alice, bob, charlie, diana, admin
-- ---------------------------------------------------------
INSERT INTO team_members (teamid, username, role)
VALUES
  (1, 'alice',   'leader'),
  (1, 'bob',     'member'),
  (1, 'charlie', 'member'),

  (2, 'diana',   'leader'),
  (2, 'alice',   'member'),

  (3, 'admin',   'leader'),
  (3, 'bob',     'member'),
  (3, 'charlie', 'member');

-- ---------------------------------------------------------
-- Tasks
-- ---------------------------------------------------------
INSERT INTO tasks
  (teamid, title, description, author, assignee, status, priority, deadline)
VALUES
  (1, 'Setup CI pipeline',
      'Configure CI for all services',
      'alice', 'bob', 'IN_PROGRESS', 'HIGH', now() + interval '5 days'),

  (1, 'Dockerize services',
      'Ensure all services build via Docker',
      'alice', 'charlie', 'TODO', 'MEDIUM', now() + interval '7 days'),

  (2, 'Design login page',
      'Create responsive login page',
      'diana', 'alice', 'DONE', 'LOW', now() - interval '1 day'),

  (2, 'Improve UX',
      'Enhance dashboard UX',
      'diana', 'alice', 'IN_PROGRESS', 'MEDIUM', now() + interval '3 days'),

  (3, 'Task API refactor',
      'Clean up task handlers and validation',
      'admin', 'bob', 'TODO', 'HIGH', now() + interval '10 days');

-- ---------------------------------------------------------
-- Comments
-- ---------------------------------------------------------
INSERT INTO task_comments (taskid, author, body)
VALUES
  (1, 'alice', 'Please prioritize this.'),
  (1, 'bob',   'Working on it now.'),

  (3, 'diana', 'Looks good to me.'),

  (4, 'alice', 'I will push changes later today.'),

  (5, 'admin', 'This is blocking the release.');

COMMIT;
