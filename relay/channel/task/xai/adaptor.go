package xai

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var ModelList = []string{
	"grok-imagine-video",
	"grok-imagine-video-1.5",
	"grok-imagine-video-1.5-fast",
	"grok-imagine-video-1.5-preview",
}

const ChannelName = "xai-video"

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	return relaycommon.ValidateMultipartDirect(c, info)
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/v1/videos/generations", a.baseURL), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	if contentType := c.Request.Header.Get("Content-Type"); contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_request_body_failed")
	}
	cachedBody, err := storage.Bytes()
	if err != nil {
		return nil, errors.Wrap(err, "read_body_bytes_failed")
	}
	contentType := c.GetHeader("Content-Type")

	if strings.HasPrefix(contentType, "application/json") {
		body := cachedBody
		if newBody, err := sjson.SetBytes(cachedBody, "model", info.UpstreamModelName); err == nil {
			body = newBody
		}
		if normalizedBody, ok := taskcommon.NormalizeGrokInlineImageURL(c, body, info); ok {
			body = normalizedBody
		}
		return bytes.NewReader(body), nil
	}

	if strings.Contains(contentType, "multipart/form-data") {
		formData, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return bytes.NewReader(cachedBody), nil
		}
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		if err := writer.WriteField("model", info.UpstreamModelName); err != nil {
			return nil, errors.Wrap(err, "write model field failed")
		}
		for key, values := range formData.Value {
			if key == "model" {
				continue
			}
			for _, v := range values {
				if err := writer.WriteField(key, v); err != nil {
					return nil, errors.Wrapf(err, "write form field %s failed", key)
				}
			}
		}
		for fieldName, fileHeaders := range formData.File {
			for _, fh := range fileHeaders {
				f, err := fh.Open()
				if err != nil {
					continue
				}
				ct := fh.Header.Get("Content-Type")
				if ct == "" || ct == "application/octet-stream" {
					buf512 := make([]byte, 512)
					n, _ := io.ReadFull(f, buf512)
					ct = http.DetectContentType(buf512[:n])
					_ = f.Close()
					f, err = fh.Open()
					if err != nil {
						continue
					}
				}
				h := make(textproto.MIMEHeader)
				h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fh.Filename))
				h.Set("Content-Type", ct)
				part, err := writer.CreatePart(h)
				if err != nil {
					_ = f.Close()
					continue
				}
				_, _ = io.Copy(part, f)
				_ = f.Close()
			}
		}
		if err := writer.Close(); err != nil {
			return nil, errors.Wrap(err, "close multipart writer failed")
		}
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		return &buf, nil
	}

	return common.ReaderOnly(storage), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	upstreamID := extractFirstString(responseBody, "id", "task_id", "data.id", "data.task_id")
	if upstreamID == "" {
		return "", responseBody, service.TaskErrorWrapper(fmt.Errorf("upstream task id is empty"), "invalid_response", http.StatusInternalServerError)
	}

	publicBody := responseBody
	for _, path := range []string{"id", "task_id", "data.id", "data.task_id"} {
		if gjson.GetBytes(publicBody, path).Exists() {
			if newBody, err := sjson.SetBytes(publicBody, path, info.PublicTaskID); err == nil {
				publicBody = newBody
			}
		}
	}
	if !gjson.GetBytes(publicBody, "id").Exists() {
		if newBody, err := sjson.SetBytes(publicBody, "id", info.PublicTaskID); err == nil {
			publicBody = newBody
		}
	}
	if !gjson.GetBytes(publicBody, "task_id").Exists() {
		if newBody, err := sjson.SetBytes(publicBody, "task_id", info.PublicTaskID); err == nil {
			publicBody = newBody
		}
	}

	c.Data(http.StatusOK, "application/json; charset=utf-8", publicBody)
	return upstreamID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	uri := fmt.Sprintf("%s/v1/videos/%s", strings.TrimRight(baseUrl, "/"), taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	status := strings.ToLower(strings.TrimSpace(extractFirstString(respBody, "status", "data.status", "task_status", "data.task_status")))
	taskResult := relaycommon.TaskInfo{
		Code:   0,
		TaskID: extractFirstString(respBody, "id", "task_id", "data.id", "data.task_id"),
		Url:    extractFirstString(respBody, "url", "video_url", "output", "data.url", "data.video_url", "data.output", "videos.0.url", "data.videos.0.url"),
	}

	if progress := gjson.GetBytes(respBody, "progress"); progress.Exists() {
		if progress.Type == gjson.Number {
			taskResult.Progress = fmt.Sprintf("%d%%", int(progress.Num))
		} else if progress.Type == gjson.String {
			taskResult.Progress = progress.String()
		}
	}

	switch status {
	case "queued", "pending":
		taskResult.Status = model.TaskStatusQueued
	case "processing", "in_progress", "running":
		taskResult.Status = model.TaskStatusInProgress
	case "completed", "succeeded", "success":
		taskResult.Status = model.TaskStatusSuccess
		if taskResult.Progress == "" {
			taskResult.Progress = taskcommon.ProgressComplete
		}
	case "failed", "cancelled", "canceled", "error":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Reason = extractFirstString(respBody, "error.message", "data.error.message", "message", "data.message", "error", "data.error")
		if taskResult.Reason == "" {
			taskResult.Reason = "task failed"
		}
		if taskResult.Progress == "" {
			taskResult.Progress = taskcommon.ProgressComplete
		}
	default:
		return &taskResult, nil
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = task.TaskID
	openAIVideo.TaskID = task.TaskID
	openAIVideo.Status = task.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(task.Progress)
	openAIVideo.CreatedAt = task.CreatedAt
	openAIVideo.CompletedAt = task.UpdatedAt
	openAIVideo.Model = task.Properties.OriginModelName
	if url := task.GetResultURL(); url != "" {
		openAIVideo.SetMetadata("url", url)
	}
	if seconds := extractFirstString(task.Data, "seconds", "data.seconds", "duration", "data.duration"); seconds != "" {
		openAIVideo.Seconds = seconds
	}
	if size := extractFirstString(task.Data, "size", "data.size"); size != "" {
		openAIVideo.Size = size
	}
	if task.Status == model.TaskStatusFailure {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: task.FailReason,
		}
	}
	return common.Marshal(openAIVideo)
}

func extractFirstString(body []byte, paths ...string) string {
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() {
			continue
		}
		switch value.Type {
		case gjson.String:
			if s := strings.TrimSpace(value.String()); s != "" {
				return s
			}
		case gjson.Number:
			return value.String()
		case gjson.JSON:
			if s := strings.TrimSpace(value.String()); s != "" && s != "null" && s != "{}" && s != "[]" {
				return s
			}
		}
	}
	return ""
}
