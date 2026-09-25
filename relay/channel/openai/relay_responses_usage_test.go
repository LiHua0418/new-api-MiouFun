package openai

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesLongInputPrechargeAndUsage(t *testing.T) {
	oldCount, oldTimeout := constant.CountToken, constant.StreamingTimeout
	constant.CountToken, constant.StreamingTimeout = true, 30
	t.Cleanup(func() { constant.CountToken, constant.StreamingTimeout = oldCount, oldTimeout })
	service.InitTokenEncoders()
	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() { require.NoError(t, config.GlobalConfig.LoadFromDB(saved)) })
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode":    `{"gpt-4":"tiered_expr"}`,
		"billing_setting.billing_expr":    `{"gpt-4":"tier(\"base\", p * 3 + c * 15)"}`,
		"group_ratio_setting.group_ratio": `{"default":1}`,
	}))
	delta, err := common.Marshal(map[string]string{
		"type": "response.output_text.delta", "delta": strings.Repeat("a", 8192),
	})
	require.NoError(t, err)
	cases := []struct {
		name, ending          string
		input, output, cached int
	}{
		{
			name:   "upstream usage overrides the local estimate",
			ending: "data: " + `{"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":70017,"output_tokens":13,"total_tokens":70030,"input_tokens_details":{"cached_tokens":50000}}}}` + "\n\ndata: [DONE]\n\n",
			input:  70017, output: 13, cached: 50000,
		},
		{
			name:  "EOF before usage preserves local input and output accounting",
			input: 1024, output: 1024,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _, resp, info := newResponsesChatTestContext(t, "data: "+string(delta)+"\n\n"+tc.ending, true)
			info.RelayFormat = types.RelayFormatOpenAIResponses
			info.OriginModelName, info.UpstreamModelName = "gpt-4", "gpt-4"
			info.UserGroup, info.UsingGroup = "default", "default"
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4")
			meta := &types.TokenCountMeta{TokenType: types.TokenTypeTokenizer, CombineText: strings.Repeat("a", 8192), MaxTokens: 100}
			count, err := service.EstimateRequestToken(c, meta, info)
			require.NoError(t, err)
			require.Equal(t, 1024, count)
			info.SetEstimatePromptTokens(count)
			price, err := helper.ModelPriceHelper(c, info, count, meta)
			require.NoError(t, err)
			// (1024 * $3 + 100 * $15) / 1M tokens, at 500000 quota per dollar.
			assert.Equal(t, 2286, price.QuotaToPreConsume)
			usage, apiErr := OaiResponsesStreamHandler(c, info, resp)
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			assert.Equal(t, tc.input, usage.PromptTokens)
			assert.Equal(t, tc.output, usage.CompletionTokens)
			assert.Equal(t, tc.cached, usage.PromptTokensDetails.CachedTokens)
			assert.Equal(t, tc.input+tc.output, usage.TotalTokens)
		})
	}
}
