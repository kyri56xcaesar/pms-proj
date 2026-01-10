package authmw

import (
	"context"
	"fmt"
	"log"
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
