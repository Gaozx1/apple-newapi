package relay

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func channelPromptInfo(settings dto.ChannelSettings) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelSetting: settings},
	}
}

// The channel prompt can be injected as the first user message instead of a
// system prompt. That mode always applies — a client-provided system prompt is
// left untouched — so operators can reach models that ignore system roles.
func TestApplyChannelPromptAsFirstUserMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, testCase := range []struct {
		name         string
		messages     []dto.Message
		wantMessages func(prompt string) []dto.Message
	}{
		{
			name: "prefixes the first user message content",
			messages: []dto.Message{
				{Role: "system", Content: "client system prompt"},
				{Role: "user", Content: "first question"},
				{Role: "assistant", Content: "first answer"},
				{Role: "user", Content: "second question"},
			},
			wantMessages: func(prompt string) []dto.Message {
				return []dto.Message{
					{Role: "system", Content: "client system prompt"},
					{Role: "user", Content: prompt + "\n\nfirst question"},
					{Role: "assistant", Content: "first answer"},
					{Role: "user", Content: "second question"},
				}
			},
		},
		{
			name: "injects even though the client sent its own system prompt",
			messages: []dto.Message{
				{Role: "system", Content: "client system prompt"},
				{Role: "user", Content: "hi"},
			},
			wantMessages: func(prompt string) []dto.Message {
				return []dto.Message{
					{Role: "system", Content: "client system prompt"},
					{Role: "user", Content: prompt + "\n\nhi"},
				}
			},
		},
		{
			name: "adds a leading user message when the request has none",
			messages: []dto.Message{
				{Role: "assistant", Content: "previous answer"},
			},
			wantMessages: func(prompt string) []dto.Message {
				return []dto.Message{
					{Role: "user", Content: prompt},
					{Role: "assistant", Content: "previous answer"},
				}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			info := channelPromptInfo(dto.ChannelSettings{
				SystemPrompt:     "channel instruction",
				SystemPromptRole: dto.ChannelPromptRoleUser,
			})
			request := &dto.GeneralOpenAIRequest{Messages: testCase.messages}

			applySystemPromptIfNeeded(ctx, info, request)

			assert.Equal(t, testCase.wantMessages("channel instruction"), request.Messages)
			assert.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyChannelPromptInjectedAsUser))
			assert.False(t, common.GetContextKeyBool(ctx, constant.ContextKeySystemPromptOverride))
		})
	}
}

func TestApplyChannelPromptAsFirstUserMessagePrefixesMediaContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := channelPromptInfo(dto.ChannelSettings{
		SystemPrompt:     "channel instruction",
		SystemPromptRole: dto.ChannelPromptRoleUser,
	})
	request := &dto.GeneralOpenAIRequest{Messages: []dto.Message{
		{Role: "user", Content: []any{map[string]any{"type": "text", "text": "look at this"}}},
	}}

	applySystemPromptIfNeeded(ctx, info, request)

	contents := request.Messages[0].ParseContent()
	require.Len(t, contents, 2)
	assert.Equal(t, "channel instruction", contents[0].Text)
	assert.Equal(t, "look at this", contents[1].Text)
}

// The system role mode keeps its existing behavior: a client system prompt is
// only extended when the operator enabled the concatenation switch.
func TestApplyChannelPromptKeepsSystemRoleDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := channelPromptInfo(dto.ChannelSettings{
		SystemPrompt:     "channel instruction",
		SystemPromptRole: dto.ChannelPromptRoleSystem,
	})
	request := &dto.GeneralOpenAIRequest{Messages: []dto.Message{
		{Role: "system", Content: "client system prompt"},
		{Role: "user", Content: "hi"},
	}}

	applySystemPromptIfNeeded(ctx, info, request)

	assert.Equal(t, "client system prompt", request.Messages[0].StringContent())
	assert.False(t, common.GetContextKeyBool(ctx, constant.ContextKeyChannelPromptInjectedAsUser))
}

func TestApplyChannelPromptAsFirstClaudeUserMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	request := &dto.ClaudeRequest{
		System: "client system prompt",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "first question"},
			{Role: "assistant", Content: "first answer"},
			{Role: "user", Content: "second question"},
		},
	}

	applyChannelPromptToFirstClaudeUserMessage(ctx, request, "channel instruction")

	require.Len(t, request.Messages, 3)
	assert.Equal(t, "channel instruction\n\nfirst question", request.Messages[0].GetStringContent())
	assert.Equal(t, "second question", request.Messages[2].GetStringContent())
	assert.Equal(t, "client system prompt", request.GetStringSystem())
	assert.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyChannelPromptInjectedAsUser))
}

func TestApplyChannelPromptAsFirstGeminiUserTurn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	request := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "user", Parts: []dto.GeminiPart{{Text: "first question"}}},
			{Role: "model", Parts: []dto.GeminiPart{{Text: "first answer"}}},
			{Role: "user", Parts: []dto.GeminiPart{{Text: "second question"}}},
		},
	}

	applyChannelPromptToFirstGeminiUserTurn(ctx, request, "channel instruction")

	require.Len(t, request.Contents, 3)
	assert.Equal(t, "channel instruction\n\nfirst question", request.Contents[0].Parts[0].Text)
	assert.Equal(t, "second question", request.Contents[2].Parts[0].Text)
	assert.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyChannelPromptInjectedAsUser))
}
