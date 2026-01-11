package mtask

import (
	"database/sql"
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
	log.Printf("status: %s", status)
	if !validStatus(status) {
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
	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	if req.TaskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "taskid required"})
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body required"})
		return
	}
	if len(body) > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body too long"})
		return
	}

	authorAny, _ := c.Get("kc.username")
	author := authorAny.(string)

	// OPTIONAL AUTHZ (recommended): ensure caller can view task/team
	// task, err := GetTaskByID(c.Request.Context(), req.TaskID)
	// ... check membership/admin ...

	id, err := CreateComment(c.Request.Context(), req.TaskID, author, body)
	if err != nil {
		log.Printf("failed to create comment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":    "ok",
		"commentid": id,
	})
}

func handleCommentDelete(c *gin.Context) {

}

func handleGetTaskByID(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	taskID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	task, err := GetTaskByID(c.Request.Context(), taskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		log.Printf("failed to get task: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, task)
}

func handleCommentList(c *gin.Context) {
	taskIDStr := strings.TrimSpace(c.Query("taskid"))
	if taskIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "taskid required"})
		return
	}
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid taskid"})
		return
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad limit"})
		return
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	order := c.DefaultQuery("order", "created_asc")
	if order != "created_asc" && order != "created_desc" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad order"})
		return
	}

	items, err := ListCommentsByTaskID(c.Request.Context(), taskID, limit, order)
	if err != nil {
		log.Printf("failed to list comments: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":  items,
		"taskid": taskID,
		"limit":  limit,
		"order":  order,
	})
}
