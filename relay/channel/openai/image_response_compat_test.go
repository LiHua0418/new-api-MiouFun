package openai

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const onePixelPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

func configureLocalImageDownload(t *testing.T) {
	t.Helper()

	fetchSetting := system_setting.GetFetchSetting()
	originalFetchSetting := *fetchSetting
	originalFetchSetting.DomainList = append([]string(nil), fetchSetting.DomainList...)
	originalFetchSetting.IpList = append([]string(nil), fetchSetting.IpList...)
	originalFetchSetting.AllowedPorts = append([]string(nil), fetchSetting.AllowedPorts...)
	originalMaxFileDownloadMB := constant.MaxFileDownloadMB
	originalWorkerURL := system_setting.WorkerUrl

	fetchSetting.EnableSSRFProtection = false
	constant.MaxFileDownloadMB = 1
	system_setting.WorkerUrl = ""
	service.InitHttpClient()

	t.Cleanup(func() {
		if client := service.GetHttpClient(); client != nil {
			client.CloseIdleConnections()
		}
		*fetchSetting = originalFetchSetting
		constant.MaxFileDownloadMB = originalMaxFileDownloadMB
		system_setting.WorkerUrl = originalWorkerURL
		service.InitHttpClient()
	})
}

func configureImageResponseTestMode(t *testing.T) {
	t.Helper()

	originalMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(originalMode) })
}

func setImageResponseTestModel(info *relaycommon.RelayInfo, model string) {
	info.OriginModelName = model
	info.ChannelMeta.UpstreamModelName = model
}

func requireUnusableGPTImage2Response(t *testing.T, body string) {
	t.Helper()

	c, recorder, resp, info := newImageTestContext(t, body, "application/json", false)
	setImageResponseTestModel(info, "gpt-image-2")

	usage, apiErr := OpenaiImageHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Equal(t, types.ErrorCodeBadResponseBody, apiErr.GetErrorCode())
	require.True(t, types.IsSkipRetryError(apiErr))
	require.Empty(t, recorder.Body.Bytes())
	require.Empty(t, recorder.Header().Get("Content-Length"))
}

// TestOpenaiImageHandlerInlinesGPTImage2URL verifies that a URL-only upstream
// response becomes the b64_json contract expected by GPT Image clients without
// dropping usage, revised prompts, or provider-specific fields.
func TestOpenaiImageHandlerInlinesGPTImage2URL(t *testing.T) {
	configureImageResponseTestMode(t)
	configureLocalImageDownload(t)

	imageBytes, err := base64.StdEncoding.DecodeString(onePixelPNGBase64)
	require.NoError(t, err)

	var requestCount atomic.Int32
	var requestedExpectedPath atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		requestedExpectedPath.Store(r.URL.Path == "/generated.png")
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	t.Cleanup(server.Close)

	body := `{"created":1710000000,"data":[{"url":"` + server.URL + `/generated.png","revised_prompt":"draw a cat","provider_id":"img-1"}],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7},"upstream_trace":{"region":"hk"}}`
	c, recorder, resp, info := newImageTestContext(t, body, "application/json", false)
	setImageResponseTestModel(info, "gpt-image-2")

	usage, apiErr := OpenaiImageHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 4, usage.CompletionTokens)
	require.Equal(t, 7, usage.TotalTokens)
	require.Equal(t, int32(1), requestCount.Load())
	require.True(t, requestedExpectedPath.Load())
	require.Equal(t, strconv.Itoa(recorder.Body.Len()), recorder.Header().Get("Content-Length"))

	var payload struct {
		Data []struct {
			B64Json       string `json:"b64_json"`
			RevisedPrompt string `json:"revised_prompt"`
			ProviderID    string `json:"provider_id"`
		} `json:"data"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
		UpstreamTrace struct {
			Region string `json:"region"`
		} `json:"upstream_trace"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload.Data, 1)
	require.Equal(t, onePixelPNGBase64, payload.Data[0].B64Json)
	require.Equal(t, "draw a cat", payload.Data[0].RevisedPrompt)
	require.Equal(t, "img-1", payload.Data[0].ProviderID)
	require.Equal(t, 3, payload.Usage.InputTokens)
	require.Equal(t, 4, payload.Usage.OutputTokens)
	require.Equal(t, 7, payload.Usage.TotalTokens)
	require.Equal(t, "hk", payload.UpstreamTrace.Region)

	var rawPayload map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &rawPayload))
	var rawImages []map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(rawPayload["data"], &rawImages))
	require.NotContains(t, rawImages[0], "url")
}

// TestOpenaiImageHandlerPreservesResponsesThatNeedNoCompatibilityConversion
// guards both an already-inline GPT Image response and URL responses from all
// other image models against unnecessary downloads or rewrites.
func TestOpenaiImageHandlerPreservesResponsesThatNeedNoCompatibilityConversion(t *testing.T) {
	configureImageResponseTestMode(t)

	tests := []struct {
		name  string
		model string
		body  string
	}{
		{
			name:  "gpt-image-2 already has base64",
			model: "gpt-image-2",
			body:  `{"created":1710000000,"data":[{"url":"http://127.0.0.1:1/must-not-fetch","b64_json":"YWxyZWFkeQ==","revised_prompt":"kept"}],"vendor":"kept"}`,
		},
		{
			name:  "other model keeps URL response",
			model: "gpt-image-1",
			body:  `{"created":1710000000,"data":[{"url":"http://127.0.0.1:1/must-not-fetch","revised_prompt":"kept"}],"vendor":"kept"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, recorder, resp, info := newImageTestContext(t, test.body, "application/json", false)
			setImageResponseTestModel(info, test.model)

			usage, apiErr := OpenaiImageHandler(c, info, resp)
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			require.Equal(t, test.body, recorder.Body.String())
			require.Equal(t, strconv.Itoa(len(test.body)), recorder.Header().Get("Content-Length"))
		})
	}
}

// TestOpenaiImageHandlerRejectsUnusableGPTImage2Responses ensures unsuccessful
// URL retrieval and nominally successful responses without image content stop
// before any response is written or usage can be settled.
func TestOpenaiImageHandlerRejectsUnusableGPTImage2Responses(t *testing.T) {
	configureImageResponseTestMode(t)

	t.Run("download failure", func(t *testing.T) {
		configureLocalImageDownload(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "asset unavailable", http.StatusBadGateway)
		}))
		t.Cleanup(server.Close)

		requireUnusableGPTImage2Response(t, `{"data":[{"url":"`+server.URL+`/missing.png"}],"usage":{"total_tokens":7}}`)
	})

	t.Run("invalid image body", func(t *testing.T) {
		configureLocalImageDownload(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("not an image"))
		}))
		t.Cleanup(server.Close)

		requireUnusableGPTImage2Response(t, `{"data":[{"url":"`+server.URL+`/invalid.png"}],"usage":{"total_tokens":7}}`)
	})

	t.Run("missing data", func(t *testing.T) {
		requireUnusableGPTImage2Response(t, `{"usage":{"total_tokens":7}}`)
	})

	t.Run("empty data", func(t *testing.T) {
		requireUnusableGPTImage2Response(t, `{"data":[],"usage":{"total_tokens":7}}`)
	})

	t.Run("empty image item", func(t *testing.T) {
		requireUnusableGPTImage2Response(t, `{"data":[{}],"usage":{"total_tokens":7}}`)
	})
}

// TestOpenaiImageStreamHandlerInlinesGPTImage2JSONFallback verifies the same
// compatibility conversion before a non-SSE upstream response is wrapped as
// image completion events.
func TestOpenaiImageStreamHandlerInlinesGPTImage2JSONFallback(t *testing.T) {
	configureImageResponseTestMode(t)
	configureLocalImageDownload(t)

	imageBytes, err := base64.StdEncoding.DecodeString(onePixelPNGBase64)
	require.NoError(t, err)

	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	t.Cleanup(server.Close)

	body := `{"created":1710000000,"data":[{"url":"` + server.URL + `/stream.png","revised_prompt":"draw a cat"}],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`
	c, recorder, resp, info := newImageTestContext(t, body, "application/json", true)
	setImageResponseTestModel(info, "gpt-image-2")

	usage, apiErr := OpenaiImageStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 4, usage.CompletionTokens)
	require.Equal(t, 7, usage.TotalTokens)
	require.Equal(t, int32(1), requestCount.Load())
	require.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	require.Contains(t, recorder.Body.String(), `event: image_generation.completed`)
	require.Contains(t, recorder.Body.String(), `"b64_json":"`+onePixelPNGBase64+`"`)
	require.Contains(t, recorder.Body.String(), `"revised_prompt":"draw a cat"`)
	require.NotContains(t, recorder.Body.String(), `"url"`)
	require.Contains(t, recorder.Body.String(), `data: [DONE]`)
}
