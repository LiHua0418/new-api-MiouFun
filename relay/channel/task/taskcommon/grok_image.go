package taskcommon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// NormalizeGrokInlineImageURL converts inline image data URLs in Grok video JSON
// requests to clean JPEG URLs that upstreams can fetch reliably.
func NormalizeGrokInlineImageURL(c *gin.Context, body []byte, info *relaycommon.RelayInfo) ([]byte, bool) {
	if info == nil || info.ChannelMeta == nil {
		return body, false
	}
	modelName := info.UpstreamModelName
	if modelName == "" {
		modelName = info.OriginModelName
	}
	if !strings.HasPrefix(modelName, "grok-imagine-video") {
		return body, false
	}

	for _, path := range []string{"image_url", "image_url.url"} {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type != gjson.String {
			continue
		}
		imageURL, ok := imageDataURLToPublicJPEGURL(c, value.String(), info)
		if !ok {
			return body, false
		}
		newBody, err := sjson.SetBytes(body, path, imageURL)
		if err != nil {
			return body, false
		}
		return newBody, true
	}

	return body, false
}

func imageDataURLToPublicJPEGURL(c *gin.Context, dataURL string, info *relaycommon.RelayInfo) (string, bool) {
	cleanJPEG, ok := imageDataURLToCleanJPEGBytes(dataURL)
	if !ok {
		return "", false
	}
	publicURL, ok := writePublicVideoInputImage(c, cleanJPEG, info)
	if ok {
		return publicURL, true
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(cleanJPEG), true
}

func imageDataURLToCleanJPEGBytes(dataURL string) ([]byte, bool) {
	comma := strings.IndexByte(dataURL, ',')
	if comma < 0 {
		return nil, false
	}
	header := strings.ToLower(strings.TrimSpace(dataURL[:comma]))
	if !strings.HasPrefix(header, "data:image/jpeg") &&
		!strings.HasPrefix(header, "data:image/jpg") &&
		!strings.HasPrefix(header, "data:image/png") {
		return nil, false
	}
	if !strings.Contains(header, ";base64") {
		return nil, false
	}

	payload := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\r', '\t':
			return -1
		default:
			return r
		}
	}, dataURL[comma+1:])

	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(payload)
		if err != nil {
			return nil, false
		}
	}
	cleanJPEG, ok := normalizeImageBytesToJPEG(raw)
	if !ok {
		return nil, false
	}
	return cleanJPEG, true
}

func writePublicVideoInputImage(c *gin.Context, imageBytes []byte, info *relaycommon.RelayInfo) (string, bool) {
	serverAddress := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if serverAddress == "" {
		return "", false
	}

	publicTaskID := "task"
	if info != nil && info.PublicTaskID != "" {
		publicTaskID = info.PublicTaskID
	}
	sum := sha256.Sum256(imageBytes)
	fileName := fmt.Sprintf("%s-%x.jpg", safeVideoInputName(publicTaskID), sum[:8])

	dir := "/data/video-inputs"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", false
	}
	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, imageBytes, 0644); err != nil {
		return "", false
	}
	return serverAddress + "/v1/video-inputs/" + fileName, true
}

func safeVideoInputName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "task"
	}
	return b.String()
}

func normalizeImageBytesToJPEG(raw []byte) ([]byte, bool) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err == nil {
		var jpegBuf bytes.Buffer
		if err := jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 85}); err == nil {
			return jpegBuf.Bytes(), true
		}
	}
	return normalizeImageBytesWithImageMagick(raw)
}

func normalizeImageBytesWithImageMagick(raw []byte) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "convert", "-", "-auto-orient", "-strip", "-colorspace", "sRGB", "-quality", "85", "jpeg:-")
	cmd.Stdin = bytes.NewReader(raw)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, false
	}
	if out.Len() == 0 {
		return nil, false
	}
	if _, err := jpeg.Decode(bytes.NewReader(out.Bytes())); err != nil {
		return nil, false
	}
	return out.Bytes(), true
}
