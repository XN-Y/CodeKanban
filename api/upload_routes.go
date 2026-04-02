package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"code-kanban/api/h"
	"code-kanban/utils"
)

const uploadTag = "upload-上传"

type uploadController struct {
	cfg    *utils.AppConfig
	logger *zap.Logger
}

func registerUploadRoutes(app *fiber.App, group *huma.Group, cfg *utils.AppConfig, logger *zap.Logger) {
	ctrl := &uploadController{
		cfg:    cfg,
		logger: logger.Named("upload-controller"),
	}

	app.Post("/api/v1/upload/clipboard-image-stream", ctrl.handleClipboardImageStream)

	// NOTE: Web 终端图片输入端点
	// 浏览器端会在粘贴图片或拖放图片时调用此接口，将图片保存到本机临时目录，
	// 再把返回的文件路径插入到终端中，供 Codex/Claude Code/shell 继续引用。
	huma.Post(group, "/upload/clipboard-image", func(
		ctx context.Context,
		input *uploadClipboardImageInput,
	) (*h.ItemResponse[uploadImageResponse], error) {
		return ctrl.handleClipboardImage(ctx, input)
	}, func(op *huma.Operation) {
		op.OperationID = "upload-clipboard-image"
		op.Summary = "上传剪贴板图片"
		op.Tags = []string{uploadTag}
	})
}

// handleClipboardImage 处理浏览器侧传入的剪贴板/拖放图片上传请求。
func (c *uploadController) handleClipboardImage(
	ctx context.Context,
	input *uploadClipboardImageInput,
) (*h.ItemResponse[uploadImageResponse], error) {
	encodedData := strings.TrimSpace(input.Body.Data)
	if encodedData == "" {
		return nil, huma.Error400BadRequest("image data is required")
	}

	data, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return nil, huma.Error400BadRequest("invalid base64 data")
	}
	if len(data) == 0 {
		return nil, huma.Error400BadRequest("image data is required")
	}

	item, err := c.saveClipboardImage(bytes.NewReader(data), input.Body.FileName, input.Body.Source)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to save image", err)
	}

	resp := h.NewItemResponse(item)
	resp.Status = http.StatusCreated
	return resp, nil
}

func (c *uploadController) handleClipboardImageStream(ctx *fiber.Ctx) error {
	fileHeader, err := ctx.FormFile("file")
	if err != nil || fileHeader == nil {
		return fiber.NewError(http.StatusBadRequest, "file is required")
	}
	if fileHeader.Size == 0 {
		return fiber.NewError(http.StatusBadRequest, "image data is required")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return fiber.NewError(http.StatusBadRequest, "failed to open uploaded file")
	}
	defer file.Close()

	fileName := strings.TrimSpace(ctx.FormValue("fileName"))
	if fileName == "" {
		fileName = fileHeader.Filename
	}

	item, err := c.saveClipboardImage(file, fileName, ctx.FormValue("source"))
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "failed to save image")
	}

	resp := h.NewItemResponse(item)
	resp.Status = http.StatusCreated
	return ctx.Status(http.StatusCreated).JSON(resp)
}

func (c *uploadController) saveClipboardImage(
	reader io.Reader,
	requestedFileName string,
	source string,
) (uploadImageResponse, error) {
	tempDir := filepath.Join(os.TempDir(), "code-kanban-clipboard")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return uploadImageResponse{}, err
	}

	timestamp := time.Now().Format("20060102-150405")
	fileName := sanitizeUploadFileName(requestedFileName)
	if fileName == "" {
		fileName = "pasted-image.png"
	}
	fileName = fmt.Sprintf("clipboard-%s-%s", timestamp, fileName)
	filePath := filepath.Join(tempDir, fileName)

	output, err := os.Create(filePath)
	if err != nil {
		return uploadImageResponse{}, err
	}
	defer output.Close()

	written, err := io.Copy(output, reader)
	if err != nil {
		_ = os.Remove(filePath)
		return uploadImageResponse{}, err
	}
	if written == 0 {
		_ = os.Remove(filePath)
		return uploadImageResponse{}, fmt.Errorf("image data is required")
	}

	item := uploadImageResponse{
		Path:     filePath,
		FileName: fileName,
		Size:     int(written),
	}

	c.logger.Info("clipboard image saved",
		zap.String("path", filePath),
		zap.String("source", normalizeUploadSource(source)),
		zap.Int("size", item.Size))

	return item, nil
}

type uploadClipboardImageInput struct {
	Body struct {
		FileName string `json:"fileName" doc:"文件名"`
		Data     string `json:"data" doc:"图片数据（base64 编码）"`
		Source   string `json:"source,omitempty" doc:"来源（paste 或 drop）"`
	} `json:"body"`
}

type uploadImageResponse struct {
	Path     string `json:"path" doc:"文件路径"`
	FileName string `json:"fileName" doc:"文件名"`
	Size     int    `json:"size" doc:"文件大小（字节）"`
}

func sanitizeUploadFileName(name string) string {
	baseName := filepath.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	switch baseName {
	case "", ".", string(filepath.Separator):
		return ""
	default:
		return baseName
	}
}

func normalizeUploadSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "paste":
		return "paste"
	case "drop":
		return "drop"
	default:
		return "unknown"
	}
}
