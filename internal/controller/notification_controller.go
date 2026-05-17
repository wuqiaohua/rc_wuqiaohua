package controller

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/wujunqi/rc_wujunqi/internal/model"
	"github.com/wujunqi/rc_wujunqi/internal/service"
)

type NotificationController struct {
	service        *service.NotificationService
	authMiddleware gin.HandlerFunc
}

func NewNotificationController(service *service.NotificationService, authMiddleware ...gin.HandlerFunc) *NotificationController {
	controller := &NotificationController{service: service}
	if len(authMiddleware) > 0 {
		controller.authMiddleware = authMiddleware[0]
	}
	return controller
}

func (c *NotificationController) RegisterRoutes(router *gin.Engine) {
	router.GET("/healthz", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	notifications := router.Group("")
	if c.authMiddleware != nil {
		notifications.Use(c.authMiddleware)
	}
	notifications.POST("/notifications", c.create)
	notifications.GET("/notifications/:id", c.get)
}

type createNotificationRequest struct {
	Vendor         string          `json:"vendor"`
	Payload        json.RawMessage `json:"payload"`
	IdempotencyKey string          `json:"idempotency_key"`
	MaxRetries     int             `json:"max_retries"`
}

func (c *NotificationController) create(ctx *gin.Context) {
	var req createNotificationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	task, created, err := c.service.Create(ctx.Request.Context(), service.CreateNotificationInput{
		AppID:          AuthAppID(ctx),
		Vendor:         req.Vendor,
		Payload:        req.Payload,
		IdempotencyKey: req.IdempotencyKey,
		MaxRetries:     req.MaxRetries,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	status := http.StatusAccepted
	if !created {
		status = http.StatusOK
	}
	ctx.JSON(status, gin.H{
		"task_id": task.ID,
		"status":  task.Status,
		"created": created,
	})
}

func (c *NotificationController) get(ctx *gin.Context) {
	task, err := c.service.Get(ctx.Request.Context(), AuthAppID(ctx), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, taskResponse(task))
}

func writeServiceError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidNotification):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrIdempotencyConflict):
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrForbidden):
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

func taskResponse(task *model.NotificationTask) gin.H {
	resp := gin.H{
		"task_id":          task.ID,
		"app_id":           task.AppID,
		"vendor":           task.Vendor,
		"target_url":       task.TargetURL,
		"method":           task.Method,
		"headers":          task.Headers(),
		"payload":          json.RawMessage(task.PayloadJSON),
		"idempotency_key":  nullableString(task.IdempotencyKey),
		"status":           task.Status,
		"retry_count":      task.RetryCount,
		"max_retries":      task.MaxRetries,
		"next_retry_at":    nullableTime(task.NextRetryAt),
		"last_status_code": nullableInt(task.LastStatusCode),
		"last_error":       nullableString(task.LastError),
		"mq_message_id":    nullableString(task.MQMessageID),
		"created_at":       task.CreatedAt,
		"updated_at":       task.UpdatedAt,
	}
	return resp
}

func nullableString(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func nullableInt(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func nullableTime(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time
}
