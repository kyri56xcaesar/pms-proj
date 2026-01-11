package authmw

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/Nerzal/gocloak/v13"
)

type Service struct {
	Client       *gocloak.GoCloak
	Realm        string
	clientID     string
	clientSecret string

	KCAuth *KeycloakAuth
}

func NewService(baseURL, Realm, clientID, issuer, aud, clientSecret string) (*Service, error) {
	client := gocloak.NewClient("http://" + baseURL)

	// the middleware authenticatior
	kcAuth, err := NewKeycloakAuth(
		fmt.Sprintf(
			"http://%s/realms/%s/protocol/openid-connect/certs",
			baseURL,
			Realm,
		),
		issuer,
		aud,
		clientID,
	)
	if err != nil {
		log.Printf("failed to instantiate the kc authenticator middleware: %v", err)

		return nil, err
	}

	s := &Service{
		Client:       client,
		Realm:        Realm,
		clientID:     clientID,
		clientSecret: clientSecret,
	}

	s.KCAuth = kcAuth

	if err := s.selfTest(); err != nil {
		log.Printf("self test error: %v", err)

		return nil, err
	}

	return s, nil
}

func (s *Service) selfTest() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jwt, err := s.Client.LoginClient(
		ctx,
		s.clientID,
		s.clientSecret,
		s.Realm,
	)
	if err != nil {
		return fmt.Errorf("keycloak auth failed: %w", err)
	}

	// Minimal permission check (safe & cheap)
	_, err = s.Client.GetRealm(ctx, jwt.AccessToken, s.Realm)
	if err != nil {
		return fmt.Errorf("keycloak permission check failed: %w", err)
	}

	return nil
}

func (s *Service) LoginAdmin(ctx context.Context) (*gocloak.JWT, error) {
	return s.Client.LoginClient(
		ctx,
		s.clientID,
		s.clientSecret,
		s.Realm,
	)
}

func (s *Service) LoginUser(
	ctx context.Context,
	username, password string,
) (*gocloak.JWT, error) {

	return s.Client.Login(
		ctx,
		s.clientID,
		s.clientSecret,
		s.Realm,
		username,
		password,
	)
}

func (s *Service) DeleteUser(
	ctx context.Context,
	token, userID string,
) error {

	return s.Client.DeleteUser(ctx, token, s.Realm, userID)
}

func (s *Service) CreateUser(
	ctx context.Context,
	token string,
	username, email, password, firstname, lastname string,
) (string, error) {

	user := gocloak.User{
		Username:  gocloak.StringP(username),
		Email:     gocloak.StringP(email),
		Enabled:   gocloak.BoolP(true),
		FirstName: gocloak.StringP(firstname),
		LastName:  gocloak.StringP(lastname),
		Credentials: &[]gocloak.CredentialRepresentation{
			{
				Type:      gocloak.StringP("password"),
				Value:     gocloak.StringP(password),
				Temporary: gocloak.BoolP(false),
			},
		},
	}

	return s.Client.CreateUser(ctx, token, s.Realm, user)
}

func (s *Service) AddUserToGroup(
	ctx context.Context,
	token, userID, groupName string,
) error {

	groups, err := s.Client.GetGroups(ctx, token, s.Realm, gocloak.GetGroupsParams{})
	if err != nil {
		return err
	}

	var groupID string
	for _, g := range groups {
		if g.Name != nil && *g.Name == groupName {
			groupID = *g.ID
			break
		}
	}

	if groupID == "" {
		return fmt.Errorf("group not found: %s", groupName)
	}

	return s.Client.AddUserToGroup(ctx, token, s.Realm, userID, groupID)
}

func (s *Service) getRealmRole(
	ctx context.Context,
	token, roleName string,
) (*gocloak.Role, error) {

	return s.Client.GetRealmRole(ctx, token, s.Realm, roleName)
}

func (s *Service) SetUserEnabled(
	ctx context.Context,
	token, userID string,
	enabled bool,
) error {

	user, err := s.Client.GetUserByID(ctx, token, s.Realm, userID)
	if err != nil {
		return err
	}

	user.Enabled = gocloak.BoolP(enabled)

	return s.Client.UpdateUser(ctx, token, s.Realm, *user)
}

func (s *Service) GetUserByUsername(ctx context.Context, token, username string) (*gocloak.User, error) {
	users, err := s.Client.GetUsers(ctx, token, s.Realm, gocloak.GetUsersParams{
		Username: gocloak.StringP(username),
		Exact:    gocloak.BoolP(true),
		Max:      gocloak.IntP(2),
	})
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("user not found")
	}
	if len(users) > 1 {
		return nil, fmt.Errorf("multiple users matched username")
	}
	return users[0], nil
}

func (s *Service) ListUsersSimple(ctx context.Context, token string, max int) ([]*gocloak.User, error) {
	if max <= 0 {
		max = 50
	}
	if max > 200 {
		max = 200
	}

	return s.Client.GetUsers(ctx, token, s.Realm, gocloak.GetUsersParams{
		Max: gocloak.IntP(max),
	})
}

type AdminUserRow struct {
	ID            string   `json:"id"`
	Username      string   `json:"username"`
	Email         string   `json:"email"`
	FirstName     string   `json:"firstName"`
	LastName      string   `json:"lastName"`
	Enabled       bool     `json:"enabled"`
	EmailVerified bool     `json:"emailVerified"`
	Roles         []string `json:"roles"`
}

func (s *Service) ListRealmUsersWithRoles(ctx context.Context, max int) ([]AdminUserRow, error) {
	if max <= 0 {
		max = 50
	}
	if max > 200 {
		max = 200
	}

	// 1) service account token
	jwt, err := s.LoginAdmin(ctx)
	if err != nil {
		return nil, fmt.Errorf("login admin: %w", err)
	}
	token := jwt.AccessToken

	// 2) list users
	users, err := s.Client.GetUsers(ctx, token, s.Realm, gocloak.GetUsersParams{
		Max: gocloak.IntP(max),
	})
	if err != nil {
		return nil, fmt.Errorf("get users: %w", err)
	}

	// 3) for each user, fetch effective realm roles (composites included)
	out := make([]AdminUserRow, 0, len(users))

	// bounded concurrency (prevents hammering KC)
	sem := make(chan struct{}, 6)
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup

	for _, u := range users {
		if u == nil || u.ID == nil {
			continue
		}
		userID := *u.ID

		wg.Add(1)
		sem <- struct{}{}
		go func(u *gocloak.User, userID string) {
			defer wg.Done()
			defer func() { <-sem }()

			roles, err := s.Client.GetCompositeRealmRolesByUserID(ctx, token, s.Realm, userID)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("get roles for user %s: %w", userID, err)
				}
				mu.Unlock()
				return
			}

			roleNames := make([]string, 0, len(roles))
			for _, r := range roles {
				if r != nil && r.Name != nil {
					roleNames = append(roleNames, *r.Name)
				}
			}
			sort.Strings(roleNames)

			row := AdminUserRow{
				ID:            derefStr(u.ID),
				Username:      derefStr(u.Username),
				Email:         derefStr(u.Email),
				FirstName:     derefStr(u.FirstName),
				LastName:      derefStr(u.LastName),
				Enabled:       derefBool(u.Enabled),
				EmailVerified: derefBool(u.EmailVerified),
				Roles:         roleNames,
			}

			mu.Lock()
			out = append(out, row)
			mu.Unlock()
		}(u, userID)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	// optional: stable ordering for UI
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })

	return out, nil
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func derefBool(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}
