package mteam

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	ConfigPath  string
	Profile     string
	Verbose     bool
	ApiGinMode  string
	InitSQLPath string

	Ip                 string
	Port               string
	AuthAddress        string
	TeamServiceAddress string
	TaskServiceAddress string

	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string

	//kc
	Issuer       string
	Audience     string
	Realm        string
	ClientID     string
	ClientSecret string

	// database
	DBAddress  string
	DBUser     string
	DBPassword string
	DBName     string
}

func loadConfig(path string) Config {
	if err := godotenv.Load(path); err != nil {
		log.Printf("Failed to load the config file at %s, using default ones...", path)
	}

	s := strings.Split(path, "/")
	config := Config{
		ConfigPath:  s[len(s)-1],
		Profile:     getEnv("PROFILE", "baremetal"),
		Verbose:     getBoolEnv("VERBOSE", "true"),
		ApiGinMode:  getEnv("GIN_MODE", "debug"),
		InitSQLPath: getEnv("INIT_SQL_PATH", "./internal/mteam/db/init.sql"),

		Ip:                 getEnv("IP", "localhost"),
		Port:               getEnv("PORT", "5045"),
		AuthAddress:        getEnv("AUTH_ADDRESS", "localhost:5555"),
		TeamServiceAddress: getEnv("TEAMSERVICE_ADDRESS", "localhost:5015"),
		TaskServiceAddress: getEnv("TASKSERVICE_ADDRESS", "localhost:5030"),
		AllowedOrigins:     getEnvFields("ALLOW_ORIGINS", []string{"*"}),
		AllowedMethods:     getEnvFields("ALLOW_METHDODS", []string{"*"}),
		AllowedHeaders:     getEnvFields("ALLOW_HEADERS", []string{"*"}),

		Issuer:       getEnv("KC_ISSUER", "http://localhost:5555"),
		Audience:     getEnv("KC_AUDIENCE", "pms-front"),
		Realm:        getEnv("KC_REALM", "pms-myproj"),
		ClientID:     getEnv("KC_CLIENT", "admin"),
		ClientSecret: getEnv("KC_CLIENT_SECRET", ""),

		DBAddress:  getEnv("DB_ADDRESS", "api-db:5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "pms"),
	}

	log.Print(config.toString())

	return config
}

func getSecret(env string, fail bool) []byte {
	secret := os.Getenv(env)
	if len(secret) == 0 {
		log.Fatalf("Secret was empty. Aborting... -> %s", env)
	}

	return []byte(secret)
}

func getEnv(env, fallback string) string {
	if value, exists := os.LookupEnv(env); exists {
		return value
	}

	return fallback
}

func getEnvFields(env string, fallback []string) []string {
	if value, exists := os.LookupEnv(env); exists {
		fields := strings.Split(strings.TrimSpace(value), ",")

		return fields
	}

	return fallback
}

func getBoolEnv(env, fallback string) bool {
	if value, exists := os.LookupEnv(env); exists {
		return strings.ToLower(value) == "true"
	}

	return strings.ToLower(fallback) == "true"
}

func getIntEnv(env string, fallback int) int {
	if value, exists := os.LookupEnv(env); exists {
		int_value, err := strconv.Atoi(value)
		if err == nil {
			return int_value
		}
	}

	return fallback
}

func (cfg *Config) toString() string {
	var strBuilder strings.Builder

	reflectedValues := reflect.ValueOf(cfg).Elem()
	reflectedTypes := reflect.TypeOf(cfg).Elem()

	strBuilder.WriteString(fmt.Sprintf("[CFG]CONFIGURATION: %s\n", cfg.ConfigPath))

	for i := range reflectedValues.NumField() {
		fieldName := reflectedTypes.Field(i).Name
		fieldValue := reflectedValues.Field(i).Interface()

		if byteSlice, ok := fieldValue.([]byte); ok {
			fieldValue = string(byteSlice)
		}

		strBuilder.WriteString("[CFG]")
		if i < 9 {
			strBuilder.WriteString(fmt.Sprintf("%d.  ", i+1))
		} else {
			strBuilder.WriteString(fmt.Sprintf("%d. ", i+1))
		}
		if len(fieldName) <= 6 {
			strBuilder.WriteString(fmt.Sprintf("%v\t\t\t\t\t-> %v\n", fieldName, fieldValue))
		} else if len(fieldName) <= 14 {
			strBuilder.WriteString(fmt.Sprintf("%v\t\t\t\t-> %v\n", fieldName, fieldValue))
		} else if len(fieldName) <= 25 {
			strBuilder.WriteString(fmt.Sprintf("%v\t\t\t-> %v\n", fieldName, fieldValue))
		} else {
			strBuilder.WriteString(fmt.Sprintf("%v\t\t-> %v\n", fieldName, fieldValue))
		}
	}

	return strBuilder.String()
}

func bytesToMB(bytes any) string {
	switch v := bytes.(type) {
	case int64:

		return fmt.Sprintf("%.1f", float64(v)/1024.0/1024.0)
	case float64:

		return fmt.Sprintf("%.1f", v/1024.0/1024.0)
	case string:
		if num, err := strconv.ParseInt(v, 10, 64); err == nil {
			return fmt.Sprintf("%.1f", float64(num)/1024.0/1024.0)
		}
	}

	return "N/A"
}

func ago(t any) string {
	var parsed time.Time
	switch v := t.(type) {
	case time.Time:
		parsed = v
	case string:
		var err error
		parsed, err = time.Parse(time.RFC3339, v)
		if err != nil {
			return "invalid time"
		}
	default:

		return "unknown time"
	}

	diff := time.Since(parsed)
	switch {
	case diff < time.Minute:

		return "just now"
	case diff < time.Hour:

		return fmt.Sprintf("%d min ago", int(diff.Minutes()))
	case diff < 24*time.Hour:

		return fmt.Sprintf("%d hr ago", int(diff.Hours()))
	default:

		return fmt.Sprintf("%d days ago", int(diff.Hours()/24))
	}
}
