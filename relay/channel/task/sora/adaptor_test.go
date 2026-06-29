package sora

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

func TestBuildRequestBodyPreservesImageURLDataURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	imageURL := "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD+abc/def=="
	body := `{"model":"grok-imagine-video-1.5-preview","prompt":"make it move","image_url":"` + imageURL + `","seconds":"4"}`

	req := httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	storage, err := common.CreateBodyStorage([]byte(body))
	if err != nil {
		t.Fatalf("CreateBodyStorage() error = %v", err)
	}
	defer storage.Close()
	c.Set(common.KeyBodyStorage, storage)

	adaptor := &TaskAdaptor{}
	reader, err := adaptor.BuildRequestBody(c, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "upstream-grok-video",
		},
	})
	if err != nil {
		t.Fatalf("BuildRequestBody() error = %v", err)
	}

	newBody, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(newBody, &got); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", newBody, err)
	}
	if got["model"] != "upstream-grok-video" {
		t.Fatalf("model = %q, want %q", got["model"], "upstream-grok-video")
	}
	if got["image_url"] != imageURL {
		t.Fatalf("image_url changed:\n got: %q\nwant: %q", got["image_url"], imageURL)
	}
}

func TestBuildRequestBodyCleansGrokInlineJPEGImageURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	assertGrokInlineImageIsCleanedToJPEG(t, testJPEGDataURL(t))
}

func TestBuildRequestBodyCleansGrokMislabeledPNGImageURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	assertGrokInlineImageIsCleanedToJPEG(t, strings.Replace(testJPEGDataURL(t), "data:image/jpeg;base64,", "data:image/png;base64,", 1))
}

func TestBuildRequestBodyCleansGrokInlinePNGImageURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	assertGrokInlineImageIsCleanedToJPEG(t, testPNGDataURL(t))
}

func assertGrokInlineImageIsCleanedToJPEG(t *testing.T, imageURL string) {
	t.Helper()

	oldServerAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "https://api.mioufun.top"
	t.Cleanup(func() {
		system_setting.ServerAddress = oldServerAddress
	})

	body := `{"model":"grok-imagine-video-1.5-preview","prompt":"make it move","image_url":"` + imageURL + `","duration":10}`

	req := httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	storage, err := common.CreateBodyStorage([]byte(body))
	if err != nil {
		t.Fatalf("CreateBodyStorage() error = %v", err)
	}
	defer storage.Close()
	c.Set(common.KeyBodyStorage, storage)

	adaptor := &TaskAdaptor{}
	reader, err := adaptor.BuildRequestBody(c, &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_test",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "grok-imagine-video-1.5-preview",
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
	if got["model"] != "grok-imagine-video-1.5-preview" {
		t.Fatalf("model = %q, want %q", got["model"], "grok-imagine-video-1.5-preview")
	}
	imageURLValue, ok := got["image_url"].(string)
	if !ok {
		t.Fatalf("image_url = %T, want string", got["image_url"])
	}
	if !strings.HasPrefix(imageURLValue, "https://api.mioufun.top/v1/video-inputs/task_test-") || !strings.HasSuffix(imageURLValue, ".jpg") {
		t.Fatalf("image_url = %q, want public video input URL", imageURLValue)
	}
	if imageURLValue == imageURL {
		t.Fatal("image_url was not normalized")
	}

	parsedURL, err := url.Parse(imageURLValue)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	raw, err := os.ReadFile(filepath.Join("/data/video-inputs", filepath.Base(parsedURL.Path)))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if _, err := jpeg.Decode(bytes.NewReader(raw)); err != nil {
		t.Fatalf("jpeg.Decode() error = %v", err)
	}
}

func testJPEGDataURL(t *testing.T) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, A: 255})

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func testPNGDataURL(t *testing.T) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}
