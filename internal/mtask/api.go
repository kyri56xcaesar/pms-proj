package mtask

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	auth "kyri56xcaesar/pms-proj/internal/authmw"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	apiVersion = "/api/v1"
)

var (
	config Config
	engine *gin.Engine
	pool   *pgxpool.Pool
)

func initDBConn() {
	var err error
	pool, err = pgxpool.New(
		context.Background(),
		fmt.Sprintf(
			"postgres://%s:%s@%s/%s",
			config.DBUser,
			config.DBPassword,
			config.DBAddress,
			config.DBName,
		),
	)
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	err = pool.Ping(context.Background())
	if err != nil {
		log.Fatalf("failed to ping the db: %v", err)
	}

	b, err := os.ReadFile("internal/mtask/db/init.sql")
	if err != nil {
		log.Fatalf("failed to open and read the init sql file: %v", err)
	}
	sql := string(b)
	// apply init sql script
	_, err = pool.Exec(context.Background(), sql)
	if err != nil {
		log.Fatalf("failed to execute init sql: %v", err)
	}

}

func setCors() {
	corsconfig := cors.DefaultConfig()
	corsconfig.AllowOrigins = config.AllowedOrigins
	corsconfig.AllowMethods = config.AllowedMethods
	corsconfig.AllowHeaders = config.AllowedHeaders
	engine.Use(cors.New(corsconfig))
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

func setRoutes() {
	root := engine.Group("/")
	{
		root.GET("/healthz", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "alive"})
		})
	}

	kcAuth := mustInitKcAuth()
	// need to enforce middleware check for authz
	secure := engine.Group("/auth")
	secure.Use(kcAuth.RequireRoles("leader", "student", "admin"))
	{
		secure.GET("/mytask", handlePersonalTask)
		secure.GET("/tasks", handleListTasks)
		secure.POST("/tasks", handleTaskCreate)
		secure.PUT("/tasks", handleTaskUpdate)
		secure.DELETE("/tasks", handleTaskDelete)

		secure.PATCH("/change-status", handleTaskPatch)

		secure.POST("/comments", handleCommentCreate)
		secure.DELETE("/comments", handleCommentDelete)
		secure.GET("/comments", handleCommentList)
	}
}

func InitAndServe(confPath string) {
	config = loadConfig(confPath)

	engine = gin.Default()
	setGinMode(config.ApiGinMode)

	setCors()
	setRoutes()

	initDBConn()

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

	// close db conn
	pool.Close()

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
