package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"code-kanban/api/h"
	"code-kanban/service/websession"
	"code-kanban/utils"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gorilla/websocket"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"go.uber.org/zap"
)

const (
	webSessionTag    = "web-session-会话"
	webSessionWSPath = "/api/v1/web-sessions/ws"
)

type webSessionController struct {
	manager  *websession.Manager
	logger   *zap.Logger
	upgrader websocket.Upgrader
}

func registerWebSessionRoutes(app *fiber.App, group *huma.Group, cfg *utils.AppConfig, logger *zap.Logger) {
	manager, err := websession.NewManager(websession.Config{
		DataDir:             utils.GetDataDir(),
		AttachmentSizeLimit: cfg.AttachmentSizeLimit * 1024,
	}, logger)
	if err != nil {
		logger.Error("failed to initialize web session manager", zap.Error(err))
		return
	}

	ctrl := &webSessionController{
		manager: manager,
		logger:  logger.Named("web-session-controller"),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  32 * 1024,
			WriteBufferSize: 32 * 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}

	ctrl.registerHTTP(app, group)
	ctrl.registerWebsocket(app)
}

func (c *webSessionController) registerHTTP(app *fiber.App, group *huma.Group) {
	huma.Get(group, "/projects/{projectId}/web-sessions", func(
		ctx context.Context,
		input *struct {
			ProjectID string `path:"projectId"`
		},
	) (*h.ItemsResponse[websession.SessionSummary], error) {
		items, err := c.manager.ListSessions(ctx, input.ProjectID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list web sessions", err)
		}
		resp := h.NewItemsResponse(items)
		resp.Status = http.StatusOK
		return resp, nil
	}, func(op *huma.Operation) {
		op.OperationID = "web-session-list"
		op.Summary = "获取会话列表"
		op.Tags = []string{webSessionTag}
	})

	huma.Post(group, "/projects/{projectId}/web-sessions", func(
		ctx context.Context,
		input *struct {
			ProjectID string `path:"projectId"`
			Body      struct {
				WorktreeID      string `json:"worktreeId"`
				Agent           string `json:"agent"`
				Model           string `json:"model"`
				ReasoningEffort string `json:"reasoningEffort"`
				WorkflowMode    string `json:"workflowMode"`
				PermissionLevel string `json:"permissionLevel"`
				PermissionMode  string `json:"permissionMode,omitempty"`
				Title           string `json:"title"`
			}
		},
	) (*h.ItemResponse[websession.SessionSummary], error) {
		workflowMode := websession.WorkflowMode(input.Body.WorkflowMode)
		permissionLevel := websession.PermissionLevel(input.Body.PermissionLevel)
		if strings.TrimSpace(input.Body.PermissionMode) != "" {
			switch strings.ToLower(strings.TrimSpace(input.Body.PermissionMode)) {
			case "plan":
				if strings.TrimSpace(input.Body.WorkflowMode) == "" {
					workflowMode = websession.WorkflowModePlan
				}
				if strings.TrimSpace(input.Body.PermissionLevel) == "" {
					permissionLevel = websession.PermissionLevelElevated
				}
			case "yolo":
				if strings.TrimSpace(input.Body.WorkflowMode) == "" {
					workflowMode = websession.WorkflowModeDefault
				}
				if strings.TrimSpace(input.Body.PermissionLevel) == "" {
					permissionLevel = websession.PermissionLevelYolo
				}
			default:
				if strings.TrimSpace(input.Body.WorkflowMode) == "" {
					workflowMode = websession.WorkflowModeDefault
				}
				if strings.TrimSpace(input.Body.PermissionLevel) == "" {
					permissionLevel = websession.PermissionLevelElevated
				}
			}
		}
		item, err := c.manager.CreateSession(ctx, websession.CreateParams{
			ProjectID:       input.ProjectID,
			WorktreeID:      input.Body.WorktreeID,
			Agent:           websession.Agent(input.Body.Agent),
			Model:           input.Body.Model,
			ReasoningEffort: websession.ReasoningEffort(input.Body.ReasoningEffort),
			WorkflowMode:    workflowMode,
			PermissionLevel: permissionLevel,
			Title:           input.Body.Title,
		})
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		resp := h.NewItemResponse(item)
		resp.Status = http.StatusCreated
		return resp, nil
	}, func(op *huma.Operation) {
		op.OperationID = "web-session-create"
		op.Summary = "创建会话"
		op.Tags = []string{webSessionTag}
	})

	huma.Post(group, "/projects/{projectId}/web-sessions/{sessionId}/rename", func(
		ctx context.Context,
		input *struct {
			ProjectID string `path:"projectId"`
			SessionID string `path:"sessionId"`
			Body      struct {
				Title string `json:"title"`
			}
		},
	) (*h.ItemResponse[websession.SessionSummary], error) {
		record, err := c.manager.GetSession(ctx, input.SessionID)
		if err != nil || record.ProjectID != input.ProjectID {
			return nil, huma.Error404NotFound("session not found")
		}
		item, err := c.manager.RenameSession(ctx, input.SessionID, input.Body.Title)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		resp := h.NewItemResponse(item)
		resp.Status = http.StatusOK
		return resp, nil
	}, func(op *huma.Operation) {
		op.OperationID = "web-session-rename"
		op.Summary = "重命名会话"
		op.Tags = []string{webSessionTag}
	})

	huma.Post(group, "/projects/{projectId}/web-sessions/{sessionId}/close", func(
		ctx context.Context,
		input *struct {
			ProjectID string `path:"projectId"`
			SessionID string `path:"sessionId"`
		},
	) (*h.MessageResponse, error) {
		record, err := c.manager.GetSession(ctx, input.SessionID)
		if err != nil || record.ProjectID != input.ProjectID {
			return nil, huma.Error404NotFound("session not found")
		}
		if err := c.manager.AbortSession(input.SessionID); err != nil {
			return nil, huma.Error500InternalServerError("failed to abort session", err)
		}
		resp := h.NewMessageResponse("session aborted")
		resp.Status = http.StatusOK
		return resp, nil
	}, func(op *huma.Operation) {
		op.OperationID = "web-session-close"
		op.Summary = "停止会话运行"
		op.Tags = []string{webSessionTag}
	})

	huma.Delete(group, "/projects/{projectId}/web-sessions/{sessionId}", func(
		ctx context.Context,
		input *struct {
			ProjectID string `path:"projectId"`
			SessionID string `path:"sessionId"`
		},
	) (*h.MessageResponse, error) {
		record, err := c.manager.GetSession(ctx, input.SessionID)
		if err != nil || record.ProjectID != input.ProjectID {
			return nil, huma.Error404NotFound("session not found")
		}
		if err := c.manager.DeleteSession(ctx, input.SessionID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete session", err)
		}
		resp := h.NewMessageResponse("session deleted")
		resp.Status = http.StatusOK
		return resp, nil
	}, func(op *huma.Operation) {
		op.OperationID = "web-session-delete"
		op.Summary = "删除会话"
		op.Tags = []string{webSessionTag}
	})

	huma.Get(group, "/projects/{projectId}/web-sessions/{sessionId}/command-groups/{groupId}", func(
		ctx context.Context,
		input *struct {
			ProjectID string `path:"projectId"`
			SessionID string `path:"sessionId"`
			GroupID   string `path:"groupId"`
		},
	) (*h.ItemResponse[websession.CommandExecutionGroupDetail], error) {
		record, err := c.manager.GetSession(ctx, input.SessionID)
		if err != nil || record.ProjectID != input.ProjectID {
			return nil, huma.Error404NotFound("session not found")
		}

		item, err := c.manager.GetCommandExecutionGroup(ctx, input.SessionID, input.GroupID)
		if err != nil {
			if errors.Is(err, websession.ErrCommandExecutionGroupNotFound) {
				return nil, huma.Error404NotFound("command execution group not found")
			}
			return nil, huma.Error500InternalServerError("failed to load command execution group", err)
		}

		resp := h.NewItemResponse(item)
		resp.Status = http.StatusOK
		return resp, nil
	}, func(op *huma.Operation) {
		op.OperationID = "web-session-command-group-detail"
		op.Summary = "获取连续命令执行详情"
		op.Tags = []string{webSessionTag}
	})

	app.Post("/api/v1/projects/:projectId/web-sessions/attachments", func(ctx *fiber.Ctx) error {
		projectID := strings.TrimSpace(ctx.Params("projectId"))
		if projectID == "" {
			return fiber.NewError(http.StatusBadRequest, "projectId is required")
		}
		fileHeader, err := ctx.FormFile("file")
		if err != nil || fileHeader == nil {
			return fiber.NewError(http.StatusBadRequest, "file is required")
		}
		if !strings.HasPrefix(strings.ToLower(fileHeader.Header.Get("Content-Type")), "image/") {
			return fiber.NewError(http.StatusBadRequest, "only image attachments are supported")
		}
		attachment, err := c.manager.SaveAttachment(fileHeader)
		if err != nil {
			return fiber.NewError(http.StatusBadRequest, err.Error())
		}
		resp := h.NewItemResponse(attachment)
		resp.Status = http.StatusCreated
		return ctx.Status(http.StatusCreated).JSON(resp)
	})

	app.Get("/api/v1/web-sessions/attachments/:attachmentId", func(ctx *fiber.Ctx) error {
		attachmentID := strings.TrimSpace(ctx.Params("attachmentId"))
		if attachmentID == "" {
			return fiber.NewError(http.StatusBadRequest, "attachmentId is required")
		}

		attachment, err := c.manager.GetAttachment(attachmentID)
		if err != nil {
			return fiber.NewError(http.StatusNotFound, "attachment not found")
		}
		if _, err := os.Stat(attachment.Path); err != nil {
			if os.IsNotExist(err) {
				return fiber.NewError(http.StatusNotFound, "attachment not found")
			}
			return fiber.NewError(http.StatusInternalServerError, "failed to read attachment")
		}

		if attachment.Mime != "" {
			ctx.Set(fiber.HeaderContentType, attachment.Mime)
		}
		ctx.Set(fiber.HeaderContentDisposition, "inline")
		return ctx.SendFile(attachment.Path, false)
	})
}

func (c *webSessionController) registerWebsocket(app *fiber.App) {
	handler := fasthttpadaptor.NewFastHTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.serveWebsocket(w, r)
	}))
	app.Get(webSessionWSPath, func(ctx *fiber.Ctx) error {
		handler(ctx.Context())
		return nil
	})
}

func (c *webSessionController) serveWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		c.logger.Debug("failed to upgrade web session ws", zap.Error(err))
		return
	}
	defer conn.Close()

	client := c.manager.RegisterClient(conn)
	defer c.manager.UnregisterClient(client)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) &&
				!errors.Is(err, context.Canceled) {
				c.logger.Debug("web session ws read failed", zap.Error(err))
			}
			return
		}
		if err := c.manager.HandleCommand(ctx, client, payload); err != nil {
			c.logger.Debug("failed to handle web session command", zap.Error(err))
		}
	}
}
