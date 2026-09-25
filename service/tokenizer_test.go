package service

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tiktoken-go/tokenizer"
)

func TestLongInputStillProvidesPromptTokenEstimate(t *testing.T) {
	oldCountToken, oldEncoder, oldMap := constant.CountToken, defaultTokenEncoder, tokenEncoderMap
	constant.CountToken = true
	InitTokenEncoders()
	tokenEncoderMap = make(map[string]tokenizer.Codec)
	t.Cleanup(func() {
		constant.CountToken, defaultTokenEncoder, tokenEncoderMap = oldCountToken, oldEncoder, oldMap
	})
	for _, model := range []string{"gpt-4", "gpt-5.6-sol", "gpt-6-astra"} {
		t.Run(model, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			common.SetContextKey(c, constant.ContextKeyOriginalModel, model)
			meta := &types.TokenCountMeta{TokenType: types.TokenTypeTokenizer, CombineText: strings.Repeat("a", 8192)}
			info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAIResponses}
			count, err := EstimateRequestToken(c, meta, info)
			require.NoError(t, err)
			assert.Equal(t, 1024, count)
			assert.Equal(t, 1024, common.GetContextKeyInt(c, constant.ContextKeyPromptTokens))
		})
	}
}
