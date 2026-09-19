package service

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// specialTokenTextPattern matches special-token text leaks: DeepSeek DSML
// markers, Kimi tool-call markers, and similar <|...|> control sequences that
// should only ever exist as special tokens. When a model spells one out as
// plain BPE text, no layer parses it and it would leak into user-visible
// content. Inner names are limited to marker alphabets (letters, digits,
// underscore, ▁, hyphen), so prose containing angle brackets never matches.
var specialTokenTextPattern = regexp.MustCompile(`<[|｜│]{1,2}[A-Za-z0-9_▁\-]{3,}[|｜│]{0,2}>`)

// partialSpecialTokenPrefix matches a string that could still grow into a
// special-token marker: "<", "<|", "<|end_tool_ca", "<||end_tool_calls|"
// (closing pipe arrived, ">" not yet), etc.
var partialSpecialTokenPrefix = regexp.MustCompile(`^<[|｜│]{0,2}[A-Za-z0-9_▁\-]*[|｜│]{0,2}$`)

// ScrubSpecialTokenText removes special-token-shaped text leaks from s.
func ScrubSpecialTokenText(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	return specialTokenTextPattern.ReplaceAllString(s, "")
}

// StreamTextScrubber scrubs streamed text deltas. Markers can be split across
// many deltas (observed: 7-9 chunks per marker), so a trailing fragment that
// could still grow into a marker is held back until the next Feed/Flush.
type StreamTextScrubber struct {
	pending strings.Builder
}

// Feed consumes one delta and returns the text that is safe to emit now.
func (s *StreamTextScrubber) Feed(delta string) string {
	if delta == "" {
		return ""
	}
	s.pending.WriteString(delta)
	text := specialTokenTextPattern.ReplaceAllString(s.pending.String(), "")
	// Hold back a trailing fragment that might still become a marker.
	if idx := strings.LastIndex(text, "<"); idx >= 0 {
		if tail := text[idx:]; partialSpecialTokenPrefix.MatchString(tail) {
			s.pending.Reset()
			s.pending.WriteString(tail)
			return text[:idx]
		}
	}
	s.pending.Reset()
	return text
}

// Flush returns whatever is still held back. A dangling partial marker (stream
// ended before ">") is passed through verbatim — it is not a complete marker.
func (s *StreamTextScrubber) Flush() string {
	text := specialTokenTextPattern.ReplaceAllString(s.pending.String(), "")
	s.pending.Reset()
	return text
}

// streamScrubFields are the OpenAI delta/message text fields that can carry
// marker leaks: visible content and both reasoning field spellings.
var streamScrubFields = []string{"content", "reasoning_content", "reasoning"}

// ScrubStreamChunkText scrubs the delta text fields of an OpenAI SSE chunk.
// The chunk is decoded into a map so unknown top-level/provider-specific
// fields survive verbatim; unchanged chunks return the original string.
// scrubbers holds per-(choice, field) state and must be per-request.
func ScrubStreamChunkText(data string, scrubbers map[string]*StreamTextScrubber) string {
	if !strings.Contains(data, "<") {
		return data
	}
	var chunk map[string]any
	if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
		return data
	}
	choices, ok := chunk["choices"].([]any)
	if !ok || len(choices) == 0 {
		return data
	}
	changed := false
	for _, choiceAny := range choices {
		choice, ok := choiceAny.(map[string]any)
		if !ok {
			continue
		}
		idx := fmt.Sprintf("%v", choice["index"])
		delta, ok := choice["delta"].(map[string]any)
		if !ok {
			continue
		}
		for _, field := range streamScrubFields {
			text, ok := delta[field].(string)
			if !ok || text == "" {
				continue
			}
			key := idx + "\x00" + field
			scrubber := scrubbers[key]
			if scrubber == nil {
				scrubber = &StreamTextScrubber{}
				scrubbers[key] = scrubber
			}
			if out := scrubber.Feed(text); out != text {
				delta[field] = out
				changed = true
			}
		}
	}
	if !changed {
		return data
	}
	scrubbed, err := common.Marshal(chunk)
	if err != nil {
		return data
	}
	return string(scrubbed)
}

// FlushStreamScrubbers renders text still held by stream scrubbers as one
// final OpenAI delta chunk (id/model/created copied from baseChunk). Returns
// "" when nothing was held.
func FlushStreamScrubbers(scrubbers map[string]*StreamTextScrubber, baseChunk string) string {
	if len(scrubbers) == 0 {
		return ""
	}
	type heldText struct {
		choiceIdx int
		field     string
		text      string
	}
	var held []heldText
	for key, scrubber := range scrubbers {
		text := scrubber.Flush()
		if text == "" {
			continue
		}
		idxStr, field, _ := strings.Cut(key, "\x00")
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			continue
		}
		held = append(held, heldText{choiceIdx: idx, field: field, text: text})
	}
	if len(held) == 0 {
		return ""
	}
	var base map[string]any
	_ = common.UnmarshalJsonStr(baseChunk, &base)
	chunk := map[string]any{
		"id":      base["id"],
		"object":  "chat.completion.chunk",
		"created": base["created"],
		"model":   base["model"],
	}
	choices := make([]any, 0, len(held))
	for _, h := range held {
		choices = append(choices, map[string]any{
			"index": h.choiceIdx,
			"delta": map[string]any{h.field: h.text},
		})
	}
	chunk["choices"] = choices
	out, err := common.Marshal(chunk)
	if err != nil {
		return ""
	}
	return string(out)
}

// ScrubChatResponseText scrubs message text fields of a non-stream OpenAI
// chat response body, preserving unknown fields. Returns nil when nothing
// changed (callers then forward the original bytes).
func ScrubChatResponseText(body []byte) []byte {
	if !bytes.Contains(body, []byte("<")) {
		return nil
	}
	var resp map[string]any
	if err := common.Unmarshal(body, &resp); err != nil {
		return nil
	}
	choices, ok := resp["choices"].([]any)
	if !ok {
		return nil
	}
	changed := false
	for _, choiceAny := range choices {
		choice, ok := choiceAny.(map[string]any)
		if !ok {
			continue
		}
		message, ok := choice["message"].(map[string]any)
		if !ok {
			continue
		}
		if scrubMessageTextFields(message) {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	scrubbed, err := common.Marshal(resp)
	if err != nil {
		return nil
	}
	return scrubbed
}

func scrubMessageTextFields(message map[string]any) bool {
	changed := false
	for _, field := range streamScrubFields {
		switch value := message[field].(type) {
		case string:
			if value == "" {
				continue
			}
			if out := ScrubSpecialTokenText(value); out != value {
				message[field] = out
				changed = true
			}
		case []any:
			// content as text-part array
			for _, partAny := range value {
				part, ok := partAny.(map[string]any)
				if !ok || part["type"] != "text" {
					continue
				}
				text, ok := part["text"].(string)
				if !ok || text == "" {
					continue
				}
				if out := ScrubSpecialTokenText(text); out != text {
					part["text"] = out
					changed = true
				}
			}
		}
	}
	return changed
}

// ScrubChatHistoryText strips leaked special-token text from assistant
// messages in an outbound request. Leaked markers replayed back upstream
// drive the model into marker-mimicry mode (and some upstreams fail with 502
// when tools are also present), so the contamination must not round-trip.
// The message slice is edited in place; content may be a string or a
// text-part array.
func ScrubChatHistoryText(messages []map[string]any) {
	for _, message := range messages {
		if message["role"] != "assistant" {
			continue
		}
		scrubMessageTextFields(message)
	}
}
