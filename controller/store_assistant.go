package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// The assistant runs in the browser (it reuses the user's own token through
// /pg/chat/completions) and calls these endpoints for anything privileged. Every
// action is scoped to the caller, so the model can never touch another user.

type assistantPurchaseView struct {
	OrderId     int     `json:"order_id"`
	ProductName string  `json:"product_name"`
	Price       int     `json:"price"`
	PriceAmount float64 `json:"price_amount"`
	Status      int     `json:"status"`
	CreatedAt   int64   `json:"created_at"`
}

// GetStoreAssistantContext returns everything the assistant may look at: wallet,
// subscription, purchases and after-sales history.
func GetStoreAssistantContext(c *gin.Context) {
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	orders, _, err := model.ListStoreOrders(userId, 1, 10)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	purchases := make([]assistantPurchaseView, 0, len(orders))
	for _, order := range orders {
		purchases = append(purchases, assistantPurchaseView{
			OrderId:     order.Id,
			ProductName: order.ProductName,
			Price:       order.Price,
			PriceAmount: storeAmountFromQuota(order.Price),
			Status:      order.Status,
			CreatedAt:   order.CreatedAt,
		})
	}
	tickets, err := model.ListBuyerStoreTickets(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	groups, err := model.ListStoreGroupClaims(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	claimed := make([]gin.H, 0, len(groups))
	for _, claim := range groups {
		claimed = append(claimed, gin.H{"group_id": claim.GroupId})
	}
	common.ApiSuccess(c, gin.H{
		"account": gin.H{
			"username":        user.Username,
			"display_name":    user.DisplayName,
			"group":           user.Group,
			"quota":           user.Quota,
			"quota_amount":    storeAmountFromQuota(user.Quota),
			"used_quota":      user.UsedQuota,
			"request_count":   user.RequestCount,
			"aff_quota":       user.AffQuota,
			"recharge_amount": user.RechargeAmount,
		},
		"purchases":      purchases,
		"tickets":        toStoreTicketViews(tickets),
		"claimed_groups": claimed,
	})
}

// ---------------------------------------------------------------------------
// AI 客服：模型与密钥由管理员在 options 表配置，服务端调用，不消耗用户额度。
// ---------------------------------------------------------------------------

const (
	assistantOptionBaseURL = "AssistantBaseURL"
	assistantOptionKey     = "AssistantKey"
	assistantOptionModel   = "AssistantModel"
)

func assistantSetting(key string) string {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	if common.OptionMap == nil {
		return ""
	}
	return strings.TrimSpace(common.OptionMap[key])
}

type assistantMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type assistantChatRequest struct {
	Messages []assistantMessage `json:"messages"`
}

type assistantModelReply struct {
	Reply  string         `json:"reply"`
	Action string         `json:"action"`
	Args   map[string]any `json:"args"`
}

const assistantSystemPrompt = "你是 new-api 网站的中文 AI 客服，只能依据给出的 JSON 上下文回答，不要编造数据。\n" +
	"需要代办时只输出一个 JSON 对象（不要多余文字）：\n" +
	"- 建售后单 {\"reply\":\"给用户的话\",\"action\":\"create_after_sales_ticket\",\"args\":{\"order_id\":7,\"reason\":\"问题描述\"}}\n" +
	"- 升级投诉 {\"reply\":\"...\",\"action\":\"complain_ticket\",\"args\":{\"ticket_id\":1,\"reason\":\"...\"}}\n" +
	"- 取消领取 {\"reply\":\"...\",\"action\":\"cancel_claim\",\"args\":{\"group_id\":2}}\n" +
	"其它情况用自然语言简短回答，不要输出 JSON。用户上下文："

// assistantEndpoint resolves the chat-completions URL of an OpenAI-compatible
// upstream. The configured base may or may not carry the /v1 segment.
func assistantEndpoint(baseURL string) string {
	endpoint := strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(endpoint, "/v1") {
		endpoint += "/v1"
	}
	return endpoint + "/chat/completions"
}

// AssistantChat answers one turn using the administrator-configured model.
func AssistantChat(c *gin.Context) {
	baseURL := assistantSetting("AssistantBaseURL")
	key := assistantSetting("AssistantKey")
	modelName := assistantSetting("AssistantModel")
	// AssistantChannelId reuses an existing channel's address and key, so the
	// administrator only has to pick a channel id instead of pasting a key.
	if channelId := assistantSetting("AssistantChannelId"); channelId != "" {
		if parsed, err := strconv.Atoi(channelId); err == nil && parsed > 0 {
			channel, err := model.GetChannelById(parsed, true)
			if err != nil {
				common.ApiErrorMsg(c, "AI 客服渠道不存在")
				return
			}
			if channel.BaseURL != nil && strings.TrimSpace(*channel.BaseURL) != "" {
				baseURL = strings.TrimSpace(*channel.BaseURL)
			}
			key = channel.Key
		}
	}
	if baseURL == "" || key == "" || modelName == "" {
		common.ApiErrorMsg(c, "管理员尚未配置 AI 客服")
		return
	}
	request := assistantChatRequest{}
	if err := c.ShouldBindJSON(&request); err != nil || len(request.Messages) == 0 {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	contextPayload, err := buildAssistantContext(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	messages := make([]assistantMessage, 0, len(request.Messages)+1)
	messages = append(messages, assistantMessage{Role: "system", Content: assistantSystemPrompt + contextJSON(contextPayload)})
	history := request.Messages
	if len(history) > 12 {
		history = history[len(history)-12:]
	}
	for _, message := range history {
		if message.Role != "user" && message.Role != "assistant" {
			continue
		}
		messages = append(messages, message)
	}
	body, err := common.Marshal(gin.H{
		"model":       modelName,
		"messages":    messages,
		"stream":      false,
		"temperature": 0.3,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	endpoint := assistantEndpoint(baseURL)
	httpRequest, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+key)
	client := &http.Client{Timeout: 60 * time.Second}
	response, err := client.Do(httpRequest)
	if err != nil {
		common.ApiErrorMsg(c, "AI 客服暂时不可用: "+err.Error())
		return
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		common.ApiErrorMsg(c, fmt.Sprintf("AI 客服返回异常(%d): %s", response.StatusCode, truncateForMessage(string(raw))))
		return
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := common.Unmarshal(raw, &completion); err != nil || len(completion.Choices) == 0 {
		common.ApiErrorMsg(c, "AI 客服返回内容无法解析")
		return
	}
	content := strings.TrimSpace(completion.Choices[0].Message.Content)
	reply := content
	actionResult := ""
	if strings.HasPrefix(content, "{") {
		var parsed assistantModelReply
		if err := common.UnmarshalJsonStr(content, &parsed); err == nil && parsed.Action != "" {
			reply = strings.TrimSpace(parsed.Reply)
			actionResult = executeAssistantAction(c.GetInt("id"), parsed.Action, parsed.Args)
		}
	}
	if reply == "" && actionResult == "" {
		reply = "抱歉，我没能理解你的问题，可以再描述一次吗？"
	}
	common.ApiSuccess(c, gin.H{"reply": reply, "action_result": actionResult})
}

func executeAssistantAction(userId int, action string, args map[string]any) string {
	argInt := func(key string) int {
		switch value := args[key].(type) {
		case float64:
			return int(value)
		case string:
			if parsed, err := strconv.Atoi(value); err == nil {
				return parsed
			}
		}
		return 0
	}
	argString := func(key string) string {
		if value, ok := args[key].(string); ok {
			return strings.TrimSpace(value)
		}
		return ""
	}
	switch action {
	case "create_after_sales_ticket":
		reason := argString("reason")
		if reason == "" {
			return "建单失败：缺少问题描述"
		}
		ticket, err := model.CreateStoreTicket(userId, argInt("order_id"), argInt("group_id"), reason)
		if err != nil {
			return "建单失败：" + err.Error()
		}
		return fmt.Sprintf("已创建售后工单 #%d，商家会收到通知", ticket.Id)
	case "complain_ticket":
		reason := argString("reason")
		if reason == "" {
			return "投诉失败：缺少投诉理由"
		}
		if err := model.ComplainStoreTicket(argInt("ticket_id"), userId, reason); err != nil {
			return "投诉失败：" + err.Error()
		}
		return "已提交投诉，管理员会介入处理"
	case "cancel_claim":
		groupId := argInt("group_id")
		if groupId == 0 {
			return "取消失败：缺少分组"
		}
		if err := model.UnclaimStoreGroup(userId, groupId); err != nil {
			return "取消失败：" + err.Error()
		}
		return "已取消领取该分组"
	default:
		return "不支持的操作"
	}
}

func contextJSON(payload gin.H) string {
	raw, err := common.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func truncateForMessage(text string) string {
	text = strings.TrimSpace(text)
	if len([]rune(text)) > 200 {
		return string([]rune(text)[:200])
	}
	return text
}

// RunStoreAssistantAction keeps the explicit tool endpoint for the UI.
func RunStoreAssistantAction(c *gin.Context) {
	request := struct {
		Action string         `json:"action"`
		Args   map[string]any `json:"args"`
	}{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "参数格式错误")
		return
	}
	result := executeAssistantAction(c.GetInt("id"), request.Action, request.Args)
	common.ApiSuccess(c, gin.H{"message": result})
}

// buildAssistantContext 收集当前用户可被 AI 看到的数据。
func buildAssistantContext(userId int) (gin.H, error) {
	user, err := model.GetUserById(userId, false)
	if err != nil {
		return nil, err
	}
	orders, _, err := model.ListStoreOrders(userId, 1, 10)
	if err != nil {
		return nil, err
	}
	purchases := make([]gin.H, 0, len(orders))
	for _, order := range orders {
		purchases = append(purchases, gin.H{
			"order_id":     order.Id,
			"product_name": order.ProductName,
			"price":        storeAmountFromQuota(order.Price),
			"status":       order.Status,
			"created_at":   order.CreatedAt,
		})
	}
	tickets, err := model.ListBuyerStoreTickets(userId)
	if err != nil {
		return nil, err
	}
	ticketItems := make([]gin.H, 0, len(tickets))
	for _, ticket := range tickets {
		ticketItems = append(ticketItems, gin.H{
			"ticket_id":    ticket.Id,
			"order_id":     ticket.OrderId,
			"product_name": ticket.ProductName,
			"status":       ticket.Status,
			"reason":       ticket.Reason,
		})
	}
	return gin.H{
		"account": gin.H{
			"username":      user.Username,
			"display_name":  user.DisplayName,
			"group":         user.Group,
			"quota":         storeAmountFromQuota(user.Quota),
			"used_quota":    storeAmountFromQuota(user.UsedQuota),
			"request_count": user.RequestCount,
		},
		"purchases": purchases,
		"tickets":   ticketItems,
	}, nil
}
