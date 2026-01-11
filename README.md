# pms proj
> PLH513 project

# GCP ADDRESS: 34.118.60.49:5045

PMS (project manager service)! 

# Run 

you have to setup keycloak... if you want to run this system...
use the deployment/pms-proj-realm.json exported realm, to import and setup fast!

> running
make sure to user the appropriate .env file
container.env for containers
.env for baremetal

! change at cmd/.../main.go -> the path of the config env


- Makefile, make front, make team, make task 
to compile and run each service

- deployments/compose/compose.yml, to deploy keycloak, kc-db, and api-db

> deployment

use deployment/compose/deploy-compose.yml

to build all images, deploy everything

*make sure to use the correct environment config*









# Details

**BFF + microservices** architecture and supports teams, tasks, comments, and role-based access control.

---

## Architecture Overview

 **three backend services** and external dependencies:
### Services
- **Front API (BFF)**  
  Serves HTML pages, handles user interactions, and aggregates data from other services.

- **Team Service (`mteam`)**  
  Manages teams, team members, and leadership roles.

- **Task Service (`mtask`)**  
  Manages tasks, task status, and comments.

### External Dependencies
- **Keycloak** – Authentication & authorization (OIDC, JWT, roles)
- **PostgreSQL**
  - One database for application data (teams, tasks, comments)
  - One separate database for Keycloak

---

## Key Features

### Authentication & Authorization
- Keycloak-based login and registration
- Role-based access control:
  - `student`
  - `leader`
  - `admin`

### Teams
- Users belong to one or more teams
- Admins can create and delete teams
- Admins and leaders can manage team members
- Teams display task summaries and previews

### Tasks
- Tasks belong to teams
- Fields include title, description, assignee, author, status, priority, deadline
- Task status lifecycle:
  - `TODO`
  - `IN_PROGRESS`
  - `DONE`
- Tasks can be previewed and opened in a modal from:
  - My Tasks
  - My Teams
  - Admin Teams

### Comments
- Tasks support threaded comments
- Comments are displayed and added dynamically (no page reload)

### UI
- Server-rendered HTML (Gin templates)
- Modals for task details and creation
- Role-sensitive actions (edit, create, delete)

---

## Project Structure

```text
.
├── cmd/
│   ├── front/        # Front API (BFF)
│   ├── mteam/        # Team service
│   └── mtask/        # Task service
│
├── internal/
│   ├── front/        # Frontend handlers & views
│   ├── mteam/        # Team domain logic
│   ├── mtask/        # Task domain logic
│   ├── authmw/       # Keycloak auth middleware
│   └── utils/        # Shared utilities
│
├── templates/        # HTML templates
├── static/           # CSS / JS assets
├── config/
│   ├── .env          # Front service config
│   ├── team.env      # Team service config
│   └── task.env      # Task service config
│
├── build/
│   └── Dockerfile    # Multi-service Dockerfile
│
├── docker-compose.yml
└── README.md
