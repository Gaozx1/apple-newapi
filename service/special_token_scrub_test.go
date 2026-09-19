package service

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mk builds a DSML-style marker literal without ever writing one in source:
// e.g. mk("end_tool_calls") -> "<" + "||" + name + "|" + ">" shaped text.
func mk(name string) string {
	return "<" + "||" + name + "|" + ">"
}

func TestScrubSpecialTokenTextRemovesMarkers(t *testing.T) {
	leak := fmt.Sprintf("正文开始%s%s%s正文结束", mk("end_parameter"), mk("end_tool_call"), mk("end_tool_calls"))
	assert.Equal(t, "正文开始正文结束", ScrubSpecialTokenText(leak))

	// DeepSeek fullwidth-pipe shape
	fullwidth := "a<｜tool▁call▁begin｜>b"
	assert.Equal(t, "ab", ScrubSpecialTokenText(fullwidth))
}

func TestScrubSpecialTokenTextKeepsNormalText(t *testing.T) {
	for _, s := range []string{
		"a < b and c > d",
		"#include <stdio.h>",
		"<|x|>", // too short, not a marker
		"<not-a-marker>",
		"普通文本",
	} {
		assert.Equal(t, s, ScrubSpecialTokenText(s), s)
	}
}

func TestStreamTextScrubberSplitMarker(t *testing.T) {
	var sc StreamTextScrubber
	var out string
	// Split the marker into many fragments, as real BPE streaming does.
	fragments := []string{"前文", "<", "||", "end", "_t", "ool", "_calls", "|", ">", "后文"}
	for _, f := range fragments {
		out += sc.Feed(f)
	}
	out += sc.Flush()
	assert.Equal(t, "前文后文", out)
}

func TestStreamTextScrubberKeepsDanglingAngleBracket(t *testing.T) {
	var sc StreamTextScrubber
	out := sc.Feed("if a < b then") + sc.Flush()
	assert.Equal(t, "if a < b then", out)
}

func TestScrubStreamChunkTextPreservesUnknownFields(t *testing.T) {
	scrubbers := make(map[string]*StreamTextScrubber)
	chunk := fmt.Sprintf(`{"id":"chatcmpl-1","model":"m","provider_specific_fields":{"x":1},"choices":[{"index":0,"delta":{"content":"hello%s"}}],"trace_id":"t1"}`, mk("end_tool_call"))
	out := ScrubStreamChunkText(chunk, scrubbers)
	require.NotEqual(t, chunk, out)
	assert.Contains(t, out, `"content":"hello"`)
	assert.Contains(t, out, "provider_specific_fields")
	assert.Contains(t, out, "trace_id")

	// flush after stream end returns nothing held
	assert.Empty(t, FlushStreamScrubbers(scrubbers, chunk))
}

func TestScrubChatResponseText(t *testing.T) {
	body := []byte(fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":"答案%s","reasoning_content":"思考%s"}}],"cost_cny":0.1}`, mk("end_parameter"), mk("end_tool_calls")))
	scrubbed := ScrubChatResponseText(body)
	require.NotNil(t, scrubbed)
	assert.Contains(t, string(scrubbed), `"content":"答案"`)
	assert.Contains(t, string(scrubbed), `"reasoning_content":"思考"`)
	assert.Contains(t, string(scrubbed), `"cost_cny":0.1`)

	// no marker -> nil (caller forwards original bytes)
	clean := []byte(`{"choices":[{"message":{"content":"ok"}}]}`)
	assert.Nil(t, ScrubChatResponseText(clean))
}

func TestScrubChatHistoryTextOnlyTouchesAssistant(t *testing.T) {
	messages := []map[string]any{
		{"role": "user", "content": "请解释 " + mk("end_tool_call") + " 是什么"},
		{"role": "assistant", "content": "回答" + mk("end_tool_calls")},
	}
	ScrubChatHistoryText(messages)
	assert.Contains(t, messages[0]["content"], mk("end_tool_call"))
	assert.Equal(t, "回答", messages[1]["content"])
}
