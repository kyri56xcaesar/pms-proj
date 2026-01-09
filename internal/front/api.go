package front

import (
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

	"kyri56xcaesar/pms-proj/internal/utils"
)

const (
	apiVersion    = "/api/v1"
	templatesPath = "./internal/front/web/templates"
	staticsPath   = "internal/front/web/static"
)

var (
	config Config
	engine *gin.Engine
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
		"index": func(m map[int]any, key int) any {
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
	}
	engine.SetHTMLTemplate(template.Must(template.New("").Funcs(funcMap).ParseGlob(templatesPath + "/*.html")))
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

		// handle post requests
		apiV1.POST("/login", handleLogin)
		apiV1.POST("/register", handleRegister)

	}

	verified := apiV1.Group("/authenticated")
	// use middleware to check for authentication...
	{
		verified.GET("/user-info")
		verified.GET("/dashboard")
		verified.GET("/teams")
		verified.GET("/mytasks")

		admin := verified.Group("/admin")
		{
			admin.GET("/users")
			admin.GET("/teams")

		}
	}

	engine.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "bad path"})
	})

}

func InitAndServe(confPath string) {
	// load configuration
	config = loadConfig(confPath)
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
