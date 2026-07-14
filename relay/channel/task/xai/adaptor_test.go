package xai

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

func TestBuildRequestURL(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "https://sub.carboninsight.top"}
	got, err := adaptor.BuildRequestURL(&relaycommon.RelayInfo{})
	if err != nil {
		t.Fatalf("BuildRequestURL() error = %v", err)
	}
	want := "https://sub.carboninsight.top/v1/videos/generations"
	if got != want {
		t.Fatalf("BuildRequestURL() = %q, want %q", got, want)
	}
}

func TestBuildRequestBodyJSONReplacesModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := `{"model":"grok-imagine-video","prompt":"make it move","seconds":"4"}`
	c := newJSONContext(t, "/v1/video/generations", body)

	adaptor := &TaskAdaptor{}
	reader, err := adaptor.BuildRequestBody(c, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "grok-imagine-video-1.5",
		},
	})
	if err != nil {
		t.Fatalf("BuildRequestBody() error = %v", err)
	}
	newBody, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(newBody, &got); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", newBody, err)
	}
	if got["model"] != "grok-imagine-video-1.5" {
		t.Fatalf("model = %q, want %q", got["model"], "grok-imagine-video-1.5")
	}
	if got["prompt"] != "make it move" {
		t.Fatalf("prompt = %q, want %q", got["prompt"], "make it move")
	}
	if c.Request.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", c.Request.Header.Get("Content-Type"))
	}
}

func TestDoResponseExtractsTopLevelID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{"id":"upstream-1","status":"queued"}`)),
	}

	upstreamID, raw, taskErr := (&TaskAdaptor{}).DoResponse(c, resp, &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
	})
	if taskErr != nil {
		t.Fatalf("DoResponse() taskErr = %v", taskErr)
	}
	if upstreamID != "upstream-1" {
		t.Fatalf("upstreamID = %q, want upstream-1", upstreamID)
	}
	if string(raw) != `{"id":"upstream-1","status":"queued"}` {
		t.Fatalf("raw = %s", raw)
	}
	if !strings.Contains(w.Body.String(), `"id":"task_public"`) {
		t.Fatalf("response body = %s, want public id", w.Body.String())
	}
}

func TestDoResponseExtractsSub2APIRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{"request_id":"video-request-123","usage":{"prompt_tokens":3,"completion_tokens":4}}`)),
	}

	upstreamID, raw, taskErr := (&TaskAdaptor{}).DoResponse(c, resp, &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
	})
	if taskErr != nil {
		t.Fatalf("DoResponse() taskErr = %v", taskErr)
	}
	if upstreamID != "video-request-123" {
		t.Fatalf("upstreamID = %q, want video-request-123", upstreamID)
	}
	if !strings.Contains(string(raw), `"request_id":"video-request-123"`) {
		t.Fatalf("raw = %s, want original upstream request_id", raw)
	}
	if !strings.Contains(w.Body.String(), `"request_id":"task_public"`) {
		t.Fatalf("response body = %s, want public request_id", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"id":"task_public"`) {
		t.Fatalf("response body = %s, want public id", w.Body.String())
	}
}

func TestDoResponseExtractsNestedTaskID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{"data":{"task_id":"upstream-2"}}`)),
	}

	upstreamID, _, taskErr := (&TaskAdaptor{}).DoResponse(c, resp, &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
	})
	if taskErr != nil {
		t.Fatalf("DoResponse() taskErr = %v", taskErr)
	}
	if upstreamID != "upstream-2" {
		t.Fatalf("upstreamID = %q, want upstream-2", upstreamID)
	}
	if !strings.Contains(w.Body.String(), `"task_id":"task_public"`) {
		t.Fatalf("response body = %s, want public nested task_id", w.Body.String())
	}
}

func TestParseTaskResultStatuses(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		status string
	}{
		{name: "queued", body: `{"status":"queued"}`, status: model.TaskStatusQueued},
		{name: "pending", body: `{"status":"pending"}`, status: model.TaskStatusQueued},
		{name: "processing", body: `{"status":"processing"}`, status: model.TaskStatusInProgress},
		{name: "in_progress", body: `{"status":"in_progress"}`, status: model.TaskStatusInProgress},
		{name: "running", body: `{"status":"running"}`, status: model.TaskStatusInProgress},
		{name: "completed", body: `{"status":"completed","url":"https://example.com/video.mp4"}`, status: model.TaskStatusSuccess},
		{name: "succeeded", body: `{"data":{"status":"succeeded","videos":[{"url":"https://example.com/video.mp4"}]}}`, status: model.TaskStatusSuccess},
		{name: "sub2api done", body: `{"status":"done","video":{"url":"https://vidgen.x.ai/video.mp4","duration":10},"progress":100}`, status: model.TaskStatusSuccess},
		{name: "failed", body: `{"status":"failed","error":{"message":"bad request"}}`, status: model.TaskStatusFailure},
		{name: "canceled", body: `{"status":"canceled","message":"stopped"}`, status: model.TaskStatusFailure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := (&TaskAdaptor{}).ParseTaskResult([]byte(tt.body))
			if err != nil {
				t.Fatalf("ParseTaskResult() error = %v", err)
			}
			if got.Status != tt.status {
				t.Fatalf("Status = %q, want %q", got.Status, tt.status)
			}
			if tt.status == model.TaskStatusSuccess && got.Url == "" {
				t.Fatalf("Url is empty for success body %s", tt.body)
			}
			if tt.status == model.TaskStatusFailure && got.Reason == "" {
				t.Fatalf("Reason is empty for failure body %s", tt.body)
			}
		})
	}
}

func TestConvertToOpenAIVideoReadsSub2APIVideoDuration(t *testing.T) {
	task := &model.Task{
		TaskID: "task_public",
		Status: model.TaskStatusSuccess,
		Data:   json.RawMessage(`{"status":"done","video":{"url":"https://vidgen.x.ai/video.mp4","duration":10}}`),
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://vidgen.x.ai/video.mp4",
		},
	}

	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	if err != nil {
		t.Fatalf("ConvertToOpenAIVideo() error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", body, err)
	}
	if got["seconds"] != "10" {
		t.Fatalf("seconds = %#v, want %q", got["seconds"], "10")
	}
	for _, key := range []string{"url", "video_url"} {
		if got[key] != "https://vidgen.x.ai/video.mp4" {
			t.Fatalf("%s = %#v, want result URL", key, got[key])
		}
	}
	video, ok := got["video"].(map[string]any)
	if !ok || video["url"] != "https://vidgen.x.ai/video.mp4" {
		t.Fatalf("video = %#v, want nested result URL", got["video"])
	}
	metadata, ok := got["metadata"].(map[string]any)
	if !ok || metadata["url"] != "https://vidgen.x.ai/video.mp4" {
		t.Fatalf("metadata = %#v, want result URL", got["metadata"])
	}
}

func newJSONContext(t *testing.T, path string, body string) *gin.Context {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	storage, err := common.CreateBodyStorage([]byte(body))
	if err != nil {
		t.Fatalf("CreateBodyStorage() error = %v", err)
	}
	t.Cleanup(func() {
		_ = storage.Close()
	})
	c.Set(common.KeyBodyStorage, storage)
	return c
}
