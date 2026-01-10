package mtask

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func handleListTasks(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Query("teamid"), 10, 64)
	if err != nil || teamID <= 0 {
		c.JSON(400, gin.H{"error": "teamid required"})

		return
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil {
		c.JSON(400, gin.H{"error": "bad datas"})

		return
	}
	order := c.DefaultQuery("order", "created_desc")
	status := c.Query("status")
	assignee := c.Query("assignee")

	items, err := ListTasks(c.Request.Context(), ListTasksFilter{
		TeamID:   teamID,
		Assignee: assignee,
		Status:   status,
		Limit:    limit,
		Order:    order,
	})
	if err != nil {
		log.Printf("failed to list: %v", err)
		c.JSON(500, gin.H{"error": "db error"})

		return
	}

	payload := gin.H{
		"items":  items,
		"limit":  normalizeLimit(limit),
		"order":  order,
		"status": status,
	}

	c.JSON(http.StatusOK, payload)
}

func handleTaskCreate(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid input"})
		return
	}

	authorAny, _ := c.Get("kc.username")
	author, _ := authorAny.(string)
	if author == "" {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}

	id, err := CreateTask(c.Request.Context(), author, req)
	if err != nil {
		log.Printf("failed to create task: %v", err)
		c.JSON(500, gin.H{"error": "db error"})

		return
	}

	c.JSON(201, gin.H{"status": "ok", "taskid": id})
}

func handleTaskDelete(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Query("taskid"), 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(400, gin.H{"error": "taskid required"})
		return
	}

	err = DeleteTask(c.Request.Context(), taskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(404, gin.H{"error": "task not found"})
			return
		}
		c.JSON(500, gin.H{"error": "db error"})
		return
	}

	c.JSON(200, gin.H{"status": "ok"})

}

func handleTaskUpdate(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Query("taskid"), 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(400, gin.H{"error": "taskid required"})
		return
	}

	var req UpdateTaskRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid input"})
		return
	}

	err = UpdateTask(c.Request.Context(), taskID, req)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(404, gin.H{"error": "task not found"})
			return
		}
		if strings.Contains(err.Error(), "no fields") {
			c.JSON(400, gin.H{"error": "provide fields to update"})
			return
		}
		c.JSON(500, gin.H{"error": "db error"})
		return
	}

	c.JSON(200, gin.H{"status": "ok"})
}

func handleTaskPatch(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Query("taskid"), 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(400, gin.H{"error": "taskid required"})

		return
	}
	status := c.Query("status")
	if validStatus(status) {
		c.JSON(400, gin.H{"error": "invalid status"})

		return
	}

	ur := UpdateTaskRequest{
		Status: &status,
	}

	err = UpdateTask(c.Request.Context(), taskID, ur)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(404, gin.H{"error": "task not found"})
			return
		}

		c.JSON(500, gin.H{"error": "db error"})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})

}

func handlePersonalTask(c *gin.Context) {

}

func handleCommentCreate(c *gin.Context) {

}

func handleCommentDelete(c *gin.Context) {

}

func handleCommentList(c *gin.Context) {

}
