package codex

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLAlphaSearch(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCodex,
			ChannelBaseUrl: "https://chatgpt.com",
		},
		RelayMode: relayconstant.RelayModeAlphaSearch,
	}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://chatgpt.com/backend-api/codex/alpha/search", url)
}

// The Codex backend rejects these fields, so the adaptor clears them rather
// than forwarding what the client sent.
func TestConvertOpenAIResponsesRequestDropsPenalties(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeCodex},
		RelayMode:   relayconstant.RelayModeResponses,
	}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model:            "gpt-5-codex",
		Input:            json.RawMessage(`"hello"`),
		MaxOutputTokens:  lo.ToPtr(uint(128)),
		Temperature:      lo.ToPtr(1.0),
		FrequencyPenalty: json.RawMessage(`1.5`),
		PresencePenalty:  json.RawMessage(`1.5`),
	})
	require.NoError(t, err)

	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	assert.Nil(t, request.MaxOutputTokens)
	assert.Nil(t, request.Temperature)
	assert.Nil(t, request.FrequencyPenalty)
	assert.Nil(t, request.PresencePenalty)
}

// system_prompt_role = "user" carries the channel prompt in the first user input
// item and leaves `instructions` to the client.
func TestConvertOpenAIResponsesRequestInjectsChannelPromptAsUserInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, testCase := range []struct {
		name          string
		input         string
		wantItemCount int
		wantTexts     []string
	}{
		{
			name:          "a string input keeps one item and gains the channel prompt part",
			input:         `"hello"`,
			wantItemCount: 1,
			wantTexts:     []string{"channel instruction", "hello"},
		},
		{
			name:          "an item list keeps its items and prefixes the first user item",
			input:         `[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}]`,
			wantItemCount: 1,
			wantTexts:     []string{"channel instruction", "hello"},
		},
		{
			name:          "an item list without a user item gains a leading user item",
			input:         `[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"earlier"}]}]`,
			wantItemCount: 2,
			wantTexts:     []string{"channel instruction"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			adaptor := &Adaptor{}
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType: constant.ChannelTypeCodex,
					ChannelSetting: dto.ChannelSettings{
						SystemPrompt:     "channel instruction",
						SystemPromptRole: dto.ChannelPromptRoleUser,
					},
				},
				RelayMode: relayconstant.RelayModeResponses,
			}
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

			converted, err := adaptor.ConvertOpenAIResponsesRequest(ctx, info, dto.OpenAIResponsesRequest{
				Model: "gpt-5-codex",
				Input: json.RawMessage(testCase.input),
			})
			require.NoError(t, err)

			request, ok := converted.(dto.OpenAIResponsesRequest)
			require.True(t, ok)
			var items []map[string]any
			require.NoError(t, common.Unmarshal(request.Input, &items))
			require.Len(t, items, testCase.wantItemCount)

			texts := make([]string, 0, len(testCase.wantTexts))
			for _, part := range items[0]["content"].([]any) {
				if text, ok := part.(map[string]any)["text"].(string); ok {
					texts = append(texts, text)
				}
			}
			assert.Equal(t, testCase.wantTexts, texts)
			assert.JSONEq(t, `""`, string(request.Instructions), "instructions must stay with the client")
			assert.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyChannelPromptInjectedAsUser))
		})
	}
}
