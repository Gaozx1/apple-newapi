package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	trainingTextLimit     = 60 << 10 // Fits the MySQL TEXT byte limit.
	trainingRequestLimit  = 1 << 20
	trainingResponseLimit = 2 << 20
)

// PlaygroundTrainingChat only applies to the website's playground route.
func PlaygroundTrainingChat() gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := GetSessionAuthIdentity(c)
		if !ok {
			return
		}
		agreement, err := model.GetUserAgreementStatus(c.Request.Context(), identity.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": "Please try again later."}})
			return
		}
		if agreement.Required {
			c.AbortWithStatusJSON(http.StatusPreconditionRequired, gin.H{"error": gin.H{
				"message": "Read and accept the current User Agreement to continue.",
				"code":    "user_agreement_required",
			}})
			return
		}
		consent, err := model.GetTrainingConsent(c.Request.Context(), identity.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": "Please try again later."}})
			return
		}
		if consent.Decision() == model.TrainingUndecided {
			c.AbortWithStatusJSON(http.StatusPreconditionRequired, gin.H{"error": gin.H{
				"message": "Choose whether to allow training data collection.",
				"code":    "training_consent_required",
			}})
			return
		}
		if consent.Decision() != model.TrainingAccepted {
			return
		}
		prompt := playgroundTrainingPrompt(c)
		if prompt == "" {
			return
		}
		writer := &trainingChatWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()
		if c.Request.Context().Err() != nil || writer.Status() != http.StatusOK {
			return
		}
		reply := writer.reply()
		if reply == "" {
			return
		}
		// Bound this optional write without propagating database failures to chat.
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := model.SaveTrainingChat(ctx, identity.UserID, consent, prompt, reply); err != nil {
			common.SysError(common.LogText("failed to save an opted-in training chat"))
		}
	}
}

func playgroundTrainingPrompt(c *gin.Context) string {
	storage, err := common.GetBodyStorage(c)
	if err != nil || storage.Size() > trainingRequestLimit {
		return ""
	}
	data, err := storage.Bytes()
	if err != nil {
		return ""
	}
	var request struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if common.Unmarshal(data, &request) != nil || len(request.Messages) == 0 {
		return ""
	}
	message := request.Messages[len(request.Messages)-1]
	if message.Role != "user" {
		return ""
	}
	var text string
	if common.Unmarshal(message.Content, &text) != nil {
		var parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if common.Unmarshal(message.Content, &parts) != nil {
			return ""
		}
		var lines []string
		for _, part := range parts {
			if part.Type == "text" {
				lines = append(lines, part.Text)
			}
		}
		text = strings.Join(lines, "\n")
	}
	if len(text) > trainingTextLimit || strings.TrimSpace(text) == "" {
		return ""
	}
	return text
}

type trainingChatWriter struct {
	gin.ResponseWriter
	mu      sync.Mutex
	body    bytes.Buffer
	invalid bool
}

func (w *trainingChatWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *trainingChatWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.ResponseWriter.Write(data)
	if err != nil || n != len(data) || n > trainingResponseLimit-w.body.Len() {
		w.invalid = true
		w.body.Reset()
	}
	if !w.invalid {
		w.body.Write(data)
	}
	return n, err
}

func (w *trainingChatWriter) WriteString(data string) (int, error) {
	return w.Write([]byte(data))
}

func (w *trainingChatWriter) reply() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.invalid {
		return ""
	}
	type response struct {
		Error   json.RawMessage `json:"error"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	contentType := w.Header().Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") {
		var data response
		if common.Unmarshal(w.body.Bytes(), &data) != nil || len(data.Error) != 0 || len(data.Choices) == 0 {
			return ""
		}
		text := data.Choices[0].Message.Content
		if len(text) <= trainingTextLimit && strings.TrimSpace(text) != "" {
			return text
		}
		return ""
	}
	if !strings.HasPrefix(contentType, "text/event-stream") {
		return ""
	}
	var text strings.Builder
	for line := range strings.SplitSeq(w.body.String(), "\n") {
		dataLine, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		dataLine = strings.TrimSpace(dataLine)
		if dataLine == "[DONE]" {
			if strings.TrimSpace(text.String()) == "" {
				return ""
			}
			return text.String()
		}
		var event response
		if common.UnmarshalJsonStr(dataLine, &event) != nil || len(event.Error) != 0 {
			return ""
		}
		for _, choice := range event.Choices {
			if choice.Index == 0 {
				if len(choice.Delta.Content) > trainingTextLimit-text.Len() {
					return ""
				}
				text.WriteString(choice.Delta.Content)
			}
		}
	}
	return ""
}
