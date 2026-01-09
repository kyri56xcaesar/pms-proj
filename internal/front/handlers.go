package front

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Request struct {
	Username   string `form:"username"`
	Password   string `form:"password"`
	RepeatPass string `form:"repeat-password,omitempty"`
	Email      string `form:"email,omitempty"`
	Firstname  string `form:"firstname,omitempty"`
	Lastname   string `form:"lastname,omitempty"`
}

func (r Request) validateLogin() error {
	// check if the request fields are allowed
	if r.Username == "" || r.Password == "" {
		return fmt.Errorf("cannot have empty fields")
	}

	return nil
}

func (r Request) validateSignup() error {
	if r.Username == "" || r.Password == "" || r.RepeatPass == "" || r.Email == "" || r.Firstname == "" || r.Lastname == "" {
		return fmt.Errorf("cannot have empty fields")
	}

	if r.Password != r.RepeatPass {
		return fmt.Errorf("passwords don't match")
	}

	return nil
}

func handleLogin(c *gin.Context) {
	var r Request
	if err := c.ShouldBind(&r); err != nil {
		log.Printf("Failed to bind request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad data"})

		return
	}

	if err := r.validateLogin(); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})

		return
	}

	log.Printf("r: %+v", r)

	// forward to kc, for validation
	url := fmt.Sprintf("http://%s/auth/realms/%s/protocol/openid-connect/token", config.AuthAddress, config.Realm)
	payload := strings.NewReader(fmt.Sprintf("username=%s&password=%s&client_secret=%s&grant_type=password&client_id=%s", r.Username, r.Password, config.ClientSecret, config.ClientID))
	request, err := http.NewRequest("POST", url, payload)
	if err != nil {
		log.Printf("failed to create a new request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})

		return
	}

	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Printf("failed to send the request :%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})

		return
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("failed to read the response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})

		return
	}

	var json_data map[string]any
	err = json.Unmarshal(responseBody, &json_data)
	if err != nil {
		log.Printf("failed to unmarshal the response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}

	log.Printf("json-data: %+v", json_data)

	c.JSON(response.StatusCode, json_data)
}

func handleRegister(c *gin.Context) {
	var r Request
	if err := c.ShouldBind(&r); err != nil {
		log.Printf("Failed to bind request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad data"})

		return
	}

	if err := r.validateSignup(); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})

		return
	}

	log.Printf("r: %+v", r)

	url := fmt.Sprintf("http://%s/auth/realms/master/protocol/openid-connect/token", config.AuthAddress)
	payload := strings.NewReader(fmt.Sprintf("client_secret=%s&grant_type=client_credentials&client_id=%s", config.MasterSecret, config.MasterClient))
	request, err := http.NewRequest("POST", url, payload)
	if err != nil {
		log.Printf("failed to create a new request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})

		return
	}
	request.Header.Add("Accept", "*/*")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Printf("failed to send the request :%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})

		return
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("failed to read the response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})

		return
	}

	var json_data map[string]any
	err = json.Unmarshal(responseBody, &json_data)
	if err != nil {
		log.Printf("failed to unmarshal the response body: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}

	log.Printf("json-data: %+v", json_data)

	c.JSON(response.StatusCode, json_data)
}
