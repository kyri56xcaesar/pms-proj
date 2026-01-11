package front

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	auth "kyri56xcaesar/pms-proj/internal/authmw"
	"kyri56xcaesar/pms-proj/internal/utils"
)

const (
	apiVersion = "/api/v1"
)

var (
	templatesPath = "./internal/front/web/templates"
	staticsPath   = "internal/front/web/static"
	config        Config
	engine        *gin.Engine
	kcService     *auth.Service
	tpl           *template.Template
	ds            Downstream
)

func setCors() {
	corsconfig := cors.DefaultConfig()
	corsconfig.AllowOrigins = config.AllowedOrigins
	corsconfig.AllowMethods = config.AllowedMethods
	corsconfig.AllowHeaders = config.AllowedHeaders
	engine.Use(cors.New(corsconfig))
}

func setTemplateEngine() {
	funcMap := template.FuncMap{
		"add": func(a, b any) float64 {
			return utils.ToFloat64(a) + utils.ToFloat64(b)
		},
		"sub": func(a, b any) float64 {
			return utils.ToFloat64(a) - utils.ToFloat64(b)
		},
		"mul": func(a, b any) float64 {
			return utils.ToFloat64(a) * utils.ToFloat64(b)
		},
		"div": func(a, b any) float64 {
			if utils.ToFloat64(b) == 0 {
				return 0
			}

			return utils.ToFloat64(a) / utils.ToFloat64(b)
		},
		"typeIs": func(value any, t string) bool {
			return reflect.TypeOf(value).Kind().String() == t
		},
		"hasKey": func(value map[string]any, key string) bool {
			_, exists := value[key]

			return exists
		},
		"lt": func(a, b any) bool {
			return utils.ToFloat64(a) < utils.ToFloat64(b)
		},
		"gr": func(a, b any) bool {
			return utils.ToFloat64(a) > utils.ToFloat64(b)
		},
		"index": func(m map[string]int, key string) any {
			if val, ok := m[key]; ok {
				return val
			}

			return nil // Return nil if key does not exist
		},
		"toJSON": func(v any) string {
			b, err := json.Marshal(v)
			if err != nil {
				return "{}"
			}
			return template.HTMLEscapeString(string(b))
		},
		"lower":     strings.ToLower,
		"bytesToMB": bytesToMB,
		"ago":       ago,
		"include": func(name string, data any) template.HTML {
			// name is the template name, e.g. "pages/dashboard.html"
			var buf bytes.Buffer
			if err := tpl.ExecuteTemplate(&buf, name, data); err != nil {
				// show error in-page (dev friendly)
				return template.HTML(fmt.Sprintf(`<pre style="color:#fb7185">include error: %v</pre>`, err))
			}
			return template.HTML(buf.String())
		},
		"joinUsernames": joinUsernames,
		"joinTitles":    joinTitles,
		"joinStrings": func(ss []string) string {
			return strings.Join(ss, ",")
		},
	}
	t := template.New("").Funcs(funcMap)

	t = template.Must(t.ParseGlob(templatesPath + "/*.html"))
	t = template.Must(t.ParseGlob(templatesPath + "/partials/*.html"))
	t = template.Must(t.ParseGlob(templatesPath + "/pages/*.html"))
	t = template.Must(t.ParseGlob(templatesPath + "/pages_admin/*.html"))

	// add more folders as needed:
	// t = template.Must(t.ParseGlob(templatesPath + "/pages_admin/*.html"))

	tpl = t
	engine.SetHTMLTemplate(tpl)

	// Optional: list loaded templates once
	for _, tt := range tpl.Templates() {
		log.Println("loaded template:", tt.Name())
	}
}

func setRoutes() {
	// set statics
	engine.Static(apiVersion+"/static", staticsPath)

	// apply middleware
	root := engine.Group("/")
	{
		root.GET("/healthz", func(c *gin.Context) {
			respondInFormat(c, gin.H{
				"status": "alive",
			}, "health.html")
		})
	}

	apiV1 := engine.Group(apiVersion)
	{
		apiV1.GET("/", func(c *gin.Context) {
			c.HTML(http.StatusOK, "login.html", c.Request.UserAgent())
		})
		apiV1.GET("/login", func(c *gin.Context) {
			c.HTML(http.StatusOK, "login.html", c.Request.UserAgent())
		})
		apiV1.GET("/register", func(c *gin.Context) {
			c.HTML(http.StatusOK, "register.html", nil)
		})

		apiV1.POST("/logout", logoutHandler)

		// handle post requests
		apiV1.POST("/login", handleLogin)
		apiV1.POST("/register", handleRegister)

	}

	kcAuth := mustInitKcAuth()
	verified := apiV1.Group("/auth")
	verified.Use(kcAuth.RequireRoles("student", "leader", "admin"))
	verified.Use(auth.RequireEmailVerified())
	// use middleware to check for authentication...
	{
		verified.GET("/dashboard", dashboardHandler)
		verified.GET("/myteams", myTeamsHandler)
		verified.GET("/mytasks", myTasksHandler)

		verified.GET("/tasks/:id/json", taskDetailJSONHandler)
		verified.POST("/tasks/:id/status", taskStatusHandler)
		verified.POST("/tasks/:id/comment", addCommentHandler)

		leader := verified.Group("/leader")
		leader.Use(kcAuth.RequireRoles("leader", "admin"))
		{
			leader.POST("/teams/edit", editTeamHandler)
			leader.POST("/teams/member/add", addMemberHandler)
			leader.POST("/teams/member/remove", removeMemberHandler)

			leader.POST("/tasks/create", kcAuth.RequireRoles("leader", "admin"), createTaskHandler)
		}

		admin := verified.Group("/admin")
		admin.Use(kcAuth.RequireRoles("admin"))
		{
			admin.GET("/users", adminUsersHandlers)
			admin.GET("/teams", adminTeamsHandler)
			admin.POST("/teams/create", createTeamHandler)
			admin.POST("/teams/:teamid/delete", deleteTeamHandler)

			admin.GET("/users/:id", handleAdminGetUserByID)
			admin.POST("/users/:id/roles", handleAdminSetUserRoles)
			admin.POST("/users/:id/active", handleAdminVerifyEmail)
			admin.POST("/users/:id/delete", handleAdminDeleteUser)
		}
	}

	engine.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "bad path"})
	})

}

func mustInitKcAuth() *auth.KeycloakAuth {
	issuer := fmt.Sprintf("http://%s/realms/%s", config.AuthAddress, config.Realm)
	jwksURL := fmt.Sprintf("http://%s/realms/%s/protocol/openid-connect/certs", config.AuthAddress, config.Realm)

	a, err := auth.NewKeycloakAuth(jwksURL, issuer, config.Audience, config.ClientID)
	if err != nil {
		panic(err)
	}
	return a
}

func InitAndServe(confPath string) {
	// load configuration
	config = loadConfig(confPath)

	templatesPath = config.TemplatesPath
	staticsPath = config.StaticsPath
	// create a keycloak client service adapter (init)
	var err error
	kcService, err = auth.NewService(
		config.AuthAddress,
		config.Realm,
		config.ClientID,
		config.Issuer,
		config.Audience,
		config.ClientSecret,
	)
	if err != nil {
		log.Fatalf("failed to connect to KC: %v", err)
	}

	// set a downstream
	ds = Downstream{
		TeamBase: "http://" + config.TeamServiceAddress,
		TaskBase: "http://" + config.TaskServiceAddress,
		Client:   http.DefaultClient,
	}

	log.Printf("ds: %+v", ds)

	// instantiate a new http server
	engine = gin.Default()
	setGinMode(config.ApiGinMode)

	// -> setup cors
	setCors()
	// -> setup funcMaps & setup templateEngine
	setTemplateEngine()
	// -> setupRoutes
	setRoutes()

	// serve http
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", config.Port),
		Handler:           engine,
		ReadHeaderTimeout: time.Second * 5,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	<-ctx.Done()

	stop()
	log.Println("shutting down gracefully, press Ctrl+C again to force")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}

func setGinMode(mode string) {
	switch strings.ToLower(mode) {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "debug":
		gin.SetMode(gin.DebugMode)
	case "envgin":
		gin.SetMode(gin.EnvGinMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}
}

func respondInFormat(c *gin.Context, data any, templateName string) {
	// 1. Determine format: Query param takes priority, then Accept header
	format := c.DefaultQuery("format", "html")

	// 2. Handle based on format
	switch strings.ToLower(format) {
	case "json":
		c.JSON(http.StatusOK, data)

	case "xml":
		c.XML(http.StatusOK, data)

	case "html":
		// Ensure we don't crash if templateName is empty for data-only requests
		if templateName == "" {
			c.JSON(http.StatusNotAcceptable, gin.H{"error": "HTML format not supported for this endpoint"})
			return
		}
		c.HTML(http.StatusOK, templateName, data)

	default:
		// Fallback to JSON or a 406 Not Acceptable
		c.JSON(http.StatusOK, data)
	}
}

func joinUsernames(m []TeamMember) string {
	out := make([]string, 0, len(m))
	for _, x := range m {
		out = append(out, x.Username)
	}
	return strings.Join(out, ", ")
}

func joinTitles(t []string) string {
	return strings.Join(t, "|")
}
