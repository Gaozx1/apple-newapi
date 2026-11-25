package controller

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	lmfpd "github.com/Ikaleio/lm-detector/sdk/go"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

const (
	fingerprintOption         = "ModelFingerprintTargets"
	fingerprintMaxTargets     = 8
	fingerprintMinProbability = 0.90
	fingerprintGatewayHost    = "87.83.206.165"
	fingerprintBaseURL        = "http://" + fingerprintGatewayHost
	fingerprintSiteURL        = "https://api.mcapple.top"
)

const fingerprintSnapshotOption = "ModelFingerprintLastRun"

const fingerprintScheduleOption = "ModelFingerprintSchedule"

type fingerprintSchedule struct {
	Enabled bool `json:"enabled"`
	Minutes int  `json:"minutes"`
}

func configuredFingerprintSchedule() fingerprintSchedule {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[fingerprintScheduleOption]
	common.OptionMapRWMutex.RUnlock()
	var schedule fingerprintSchedule
	if raw == "" || common.UnmarshalJsonStr(raw, &schedule) != nil || schedule.Minutes < 1 || schedule.Minutes > 1440 {
		return fingerprintSchedule{Minutes: 10}
	}
	return schedule
}

type fingerprintSnapshot struct {
	ReferenceBuiltAt string              `json:"reference_built_at"`
	CheckedAt        time.Time           `json:"checked_at"`
	Results          []fingerprintResult `json:"results"`
}

var fingerprintRun sync.Mutex

var fingerprintGatewayClient = &http.Client{
	Transport: &http.Transport{
		// Use the IP virtual host over loopback so the probe token stays on this server.
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if address != fingerprintGatewayHost+":80" {
				return nil, errors.New("Unexpected model fingerprint gateway address")
			}
			dialer := net.Dialer{Timeout: 10 * time.Second}
			return dialer.DialContext(ctx, network, "127.0.0.1:80")
		},
		IdleConnTimeout: 90 * time.Second,
	},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

type fingerprintTarget struct {
	Group     string `json:"group"`
	Model     string `json:"model"`
	Reference string `json:"reference"`
}

type fingerprintResult struct {
	fingerprintTarget
	ChannelID        int             `json:"channel_id,omitempty"`
	ChannelName      string          `json:"channel_name,omitempty"`
	ReportedModel    string          `json:"reported_model,omitempty"`
	CheckedAt        *time.Time      `json:"checked_at,omitempty"`
	ReferenceBuiltAt string          `json:"reference_built_at,omitempty"`
	Verdict          string          `json:"verdict"`
	FailedSamples    int             `json:"failed_samples"`
	Analysis         *lmfpd.Analysis `json:"analysis,omitempty"`
}

func fingerprintEndpoint(channelType int, modelName string) constant.EndpointType {
	if channelType == constant.ChannelTypeTaskPlugin || common.IsImageGenerationModel(modelName) {
		return ""
	}
	for _, kind := range common.GetEndpointTypesByChannelType(channelType, modelName) {
		switch kind {
		case constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse, constant.EndpointTypeAnthropic, constant.EndpointTypeGemini:
			return kind
		}
	}
	return ""
}

func fingerprintCatalog() (map[string][]string, error) {
	abilities, err := model.GetAllEnableAbilityWithChannels()
	if err != nil {
		return nil, err
	}
	catalog := make(map[string][]string)
	seen := make(map[[2]string]bool)
	for _, ability := range abilities {
		pair := [2]string{ability.Group, ability.Model}
		if ability.ChannelType == 0 || seen[pair] || fingerprintEndpoint(ability.ChannelType, ability.Model) == "" {
			continue
		}
		seen[pair] = true
		catalog[ability.Group] = append(catalog[ability.Group], ability.Model)
	}
	for _, models := range catalog {
		slices.Sort(models)
	}
	return catalog, nil
}

func fingerprintTargets() ([]fingerprintTarget, error) {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[fingerprintOption]
	common.OptionMapRWMutex.RUnlock()
	targets := make([]fingerprintTarget, 0)
	if raw == "" {
		return targets, nil
	}
	if err := common.UnmarshalJsonStr(raw, &targets); err != nil {
		return nil, err
	}
	return targets, nil
}

// A snapshot may outlive a configuration edit or a change in group access.
func lastFingerprintRun(targets []fingerprintTarget, allowed map[string]string, admin bool) (*fingerprintSnapshot, error) {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[fingerprintSnapshotOption]
	common.OptionMapRWMutex.RUnlock()
	if raw == "" {
		return nil, nil
	}
	var run fingerprintSnapshot
	if err := common.UnmarshalJsonStr(raw, &run); err != nil {
		return nil, err
	}
	visible := make([]fingerprintResult, 0, len(run.Results))
	for _, result := range run.Results {
		if !slices.Contains(targets, result.fingerprintTarget) {
			continue
		}
		if !admin {
			if _, ok := allowed[result.Group]; !ok {
				continue
			}
			result.ChannelID, result.ChannelName, result.ReportedModel = 0, "", ""
		}
		if result.Analysis != nil {
			result.Verdict = fingerprintVerdict(result.fingerprintTarget, result.Analysis)
		}
		visible = append(visible, result)
	}
	if len(visible) == 0 {
		return nil, nil
	}
	run.Results = visible
	return &run, nil
}

func GetModelFingerprintResults(c *gin.Context) {
	user, err := model.GetUserCache(c.GetInt("id"))
	if err != nil {
		common.ApiErrorT(c, "Could not load fingerprint results")
		return
	}
	groups := model.FilterModelSquareUserGroups(service.GetUserUsableGroups(user.Group), c.GetInt("id"))
	targets, err := fingerprintTargets()
	if err != nil {
		common.ApiErrorT(c, "Could not load fingerprint results")
		return
	}
	run, err := lastFingerprintRun(targets, groups, false)
	if err != nil {
		common.ApiErrorT(c, "Could not load fingerprint results")
		return
	}
	common.ApiSuccess(c, run)
}

func GetModelFingerprintConfig(c *gin.Context) {
	catalog, err := fingerprintCatalog()
	if err != nil {
		common.ApiErrorT(c, "Could not load fingerprint configuration")
		return
	}
	targets, err := fingerprintTargets()
	if err != nil {
		common.ApiErrorT(c, "Could not load fingerprint configuration")
		return
	}
	last, err := lastFingerprintRun(targets, nil, true)
	if err != nil {
		common.ApiErrorT(c, "Could not load fingerprint results")
		return
	}
	common.ApiSuccess(c, gin.H{"schedule": configuredFingerprintSchedule(), "last_run": last, "catalog": catalog, "targets": targets, "max_targets": fingerprintMaxTargets, "base_url": fingerprintBaseURL})
}

func UpdateModelFingerprintConfig(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	var form struct {
		Targets  *[]fingerprintTarget `json:"targets"`
		Schedule *fingerprintSchedule `json:"schedule"`
	}
	if err := common.DecodeJson(c.Request.Body, &form); err != nil || form.Targets == nil || len(*form.Targets) > fingerprintMaxTargets {
		common.ApiErrorT(c, "Invalid fingerprint targets")
		return
	}
	catalog, err := fingerprintCatalog()
	if err != nil {
		common.ApiErrorT(c, "Could not load fingerprint configuration")
		return
	}
	targets := *form.Targets
	seen := make(map[[2]string]bool)
	for i := range targets {
		target := &targets[i]
		target.Reference = strings.TrimSpace(target.Reference)
		pair := [2]string{target.Group, target.Model}
		if !slices.Contains(catalog[target.Group], target.Model) || seen[pair] || len(target.Reference) > 255 {
			common.ApiErrorT(c, "Invalid fingerprint targets")
			return
		}
		seen[pair] = true
	}
	schedule := configuredFingerprintSchedule()
	if form.Schedule != nil {
		schedule = *form.Schedule
	}
	if schedule.Minutes < 1 || schedule.Minutes > 1440 || (schedule.Enabled && len(targets) == 0) {
		common.ApiErrorT(c, "Invalid fingerprint schedule")
		return
	}
	encoded, err := common.Marshal(targets)
	if err == nil {
		var timer []byte
		timer, err = common.Marshal(schedule)
		if err == nil {
			err = model.UpdateOptionsBulk(map[string]string{fingerprintOption: string(encoded), fingerprintScheduleOption: string(timer)})
		}
	}

	if err != nil {
		common.ApiErrorT(c, "Could not save fingerprint configuration")
		return
	}
	common.ApiSuccess(c, targets)
}

func RunModelFingerprint(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	var form struct {
		Group *string `json:"group"`
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil || (len(bytes.TrimSpace(body)) > 0 && common.Unmarshal(body, &form) != nil) {
		common.ApiErrorT(c, "Invalid fingerprint group")
		return
	}
	group := ""
	if form.Group != nil {
		group = *form.Group
		if strings.TrimSpace(group) == "" {
			common.ApiErrorT(c, "Invalid fingerprint group")
			return
		}
	}
	run, err := runFingerprintCheck(c.Request.Context(), c.GetInt("id"), group)
	if err != nil {
		common.ApiErrorT(c, err.Error())
		return
	}
	common.ApiSuccess(c, run)
}

func runFingerprintCheck(parent context.Context, userID int, group string) (*fingerprintSnapshot, error) {
	if !fingerprintRun.TryLock() {
		return nil, errors.New("A fingerprint run is already in progress")
	}
	defer fingerprintRun.Unlock()
	targets, err := fingerprintTargets()
	if err != nil || len(targets) > fingerprintMaxTargets {
		return nil, errors.New("Could not load fingerprint configuration")
	}
	if len(targets) == 0 {
		return nil, errors.New("Save at least one fingerprint target")
	}
	configuredTargets := targets
	var previous *fingerprintSnapshot
	if group != "" {
		targets = make([]fingerprintTarget, 0)
		for _, target := range configuredTargets {
			if target.Group == group {
				targets = append(targets, target)
			}
		}
		if len(targets) == 0 {
			return nil, errors.New("No saved fingerprint targets in this group")
		}
		previous, err = lastFingerprintRun(configuredTargets, nil, true)
		if err != nil {
			return nil, errors.New("Could not load fingerprint results")
		}
	}
	ctx, cancel := context.WithTimeout(parent, 25*time.Second+time.Duration((len(targets)+1)/2)*120*time.Second)
	defer cancel()
	loadCtx, cancelLoad := context.WithTimeout(ctx, 18*time.Second)
	// Only reference data is fetched from the official data host. Probes go through configured channels.
	detector, err := lmfpd.Load(loadCtx, lmfpd.LoadOptions{Sources: []string{"https://lm.ikale.io/data/"}})
	cancelLoad()
	if err != nil {
		return nil, errors.New("Could not load the fingerprint reference bank")
	}
	results := make([]fingerprintResult, len(targets))
	for i := 0; i < len(targets); i += 2 {
		var workers sync.WaitGroup
		for j := i; j < min(i+2, len(targets)); j++ {
			workers.Go(func() {
				results[j] = detectFingerprint(ctx, userID, targets[j], detector)
			})
		}
		workers.Wait()
	}
	if parent.Err() != nil {
		return nil, parent.Err()
	}
	run := fingerprintSnapshot{ReferenceBuiltAt: detector.BuiltAt(), CheckedAt: time.Now().UTC(), Results: results}
	for i := range run.Results {
		run.Results[i].CheckedAt = &run.CheckedAt
		run.Results[i].ReferenceBuiltAt = run.ReferenceBuiltAt
	}
	if previous != nil {
		run = mergeFingerprintSnapshot(configuredTargets, previous, run)
	}
	raw, err := common.Marshal(run)
	if err == nil {
		err = model.UpdateOptionsBulk(map[string]string{fingerprintSnapshotOption: string(raw)})
	}
	if err != nil {
		return nil, errors.New("Could not save fingerprint results")
	}
	return &run, nil
}

// A group run replaces only the selected targets and preserves the age of other results.
func mergeFingerprintSnapshot(targets []fingerprintTarget, previous *fingerprintSnapshot, current fingerprintSnapshot) fingerprintSnapshot {
	results := make(map[fingerprintTarget]fingerprintResult, len(targets))
	for _, result := range previous.Results {
		if result.CheckedAt == nil {
			result.CheckedAt = &previous.CheckedAt
		}
		if result.ReferenceBuiltAt == "" {
			result.ReferenceBuiltAt = previous.ReferenceBuiltAt
		}
		results[result.fingerprintTarget] = result
	}
	for _, result := range current.Results {
		results[result.fingerprintTarget] = result
	}
	current.Results = make([]fingerprintResult, 0, len(targets))
	for _, target := range targets {
		if result, ok := results[target]; ok {
			current.Results = append(current.Results, result)
		}
	}
	return current
}

func detectFingerprint(ctx context.Context, userID int, target fingerprintTarget, detector *lmfpd.Detector) (result fingerprintResult) {
	result = fingerprintResult{fingerprintTarget: target, Verdict: "unavailable"}
	if ctx.Err() != nil || strings.TrimRight(system_setting.ServerAddress, "/") != fingerprintSiteURL {
		return result
	}
	channel, err := model.GetChannel(target.Group, target.Model, 0, nil)
	if err != nil || channel == nil || channel.Status != common.ChannelStatusEnabled {
		return result
	}
	result.ChannelID, result.ChannelName = channel.Id, channel.Name
	endpoint := fingerprintEndpoint(channel.Type, target.Model)
	if endpoint == "" {
		return result
	}
	challenges, err := lmfpd.GenerateChallenges(3)
	if err != nil || strings.Contains(target.Model, ",") {
		return result
	}
	user, err := model.GetUserCache(userID)
	if err != nil || !service.IsUserSelectableGroup(user.Group, target.Group) {
		return result
	}
	key, err := common.GenerateKey()
	if err != nil {
		return result
	}
	now := time.Now()
	token := model.Token{
		UserId: userID, Key: key, Name: "Model Fingerprinting", Group: target.Group,
		CreatedTime: now.Unix(), ExpiredTime: now.Add(125 * time.Second).Unix(),
		RemainQuota: common.QuotaFromFloat(10 * common.QuotaPerUnit), ModelLimitsEnabled: true, ModelLimits: target.Model,
	}
	if err := token.Insert(); err != nil {
		return result
	}
	defer func() {
		if token.Delete() != nil {
			common.SysError("Could not remove an expiring model fingerprint token")
		}
	}()
	// Pin every sample to the same channel using the normal gateway token mechanism.
	authorization := "Bearer sk-" + key + "-" + strconv.Itoa(channel.Id)
	outputs := make([]lmfpd.Output, len(challenges))
	reported := make([]string, len(challenges))
	var samples sync.WaitGroup
	for i, challenge := range challenges {
		samples.Go(func() {
			defer func() {
				if recover() != nil {
					outputs[i].Text = ""
				}
			}()
			outputs[i].ExpectedCount = challenge.ExpectedCount
			sampleCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
			defer cancel()
			outputs[i].Text, reported[i] = collectFingerprintSample(sampleCtx, channel, target, endpoint, challenge.Prompt, authorization)
		})
	}
	samples.Wait()
	reportedModels := make(map[string]bool)
	for i, output := range outputs {
		if strings.TrimSpace(output.Text) == "" {
			result.FailedSamples++
		}
		if reported[i] != "" {
			reportedModels[reported[i]] = true
		}
	}
	if len(reportedModels) == 1 {
		for name := range reportedModels {
			result.ReportedModel = name
		}
	}
	if result.FailedSamples == len(outputs) {
		return result
	}
	analysis, err := detector.Analyze(outputs, false)
	if err != nil {
		return result
	}
	result.Analysis = analysis
	result.Verdict = fingerprintVerdict(target, analysis)
	return result
}

func fingerprintVerdict(target fingerprintTarget, analysis *lmfpd.Analysis) string {
	if analysis.Decision != "scored" || analysis.Probability == nil || !(*analysis.Probability > fingerprintMinProbability) {
		return "inconclusive"
	}
	expected := target.Reference
	if expected == "" {
		expected = target.Model
	}
	for _, candidate := range analysis.Results {
		if strings.EqualFold(candidate.Model, expected) || strings.EqualFold(candidate.DisplayName, expected) {
			if candidate.Model == analysis.Prediction {
				return "match"
			}
			return "mismatch"
		}
	}
	return "reference_missing"
}

func collectFingerprintSample(ctx context.Context, channel *model.Channel, target fingerprintTarget, endpoint constant.EndpointType, prompt, authorization string) (string, string) {
	request := buildTestRequest(target.Model, string(endpoint), channel, false)
	limit := uint(8192)
	switch req := request.(type) {
	case *dto.GeneralOpenAIRequest:
		req.Messages[0].Content = prompt
		req.MaxTokens = &limit
	case *dto.OpenAIResponsesRequest:
		req.Input, _ = common.Marshal(prompt)
		req.MaxOutputTokens = &limit
	case *dto.ClaudeRequest:
		req.Messages[0].Content = prompt
		req.MaxTokens = &limit
	case *dto.GeminiChatRequest:
		req.Contents[0].Parts[0].Text = prompt
		req.GenerationConfig.MaxOutputTokens = &limit
	default:
		return "", ""
	}
	path, ok := common.GetDefaultEndpointInfo(endpoint)
	if !ok {
		return "", ""
	}
	body, err := common.Marshal(request)
	if err != nil {
		return "", ""
	}
	uri := fingerprintBaseURL + strings.ReplaceAll(path.Path, "{model}", url.PathEscape(target.Model))
	for attempt := range 2 {
		if ctx.Err() != nil {
			break
		}
		if attempt != 0 {
			timer := time.NewTimer(300 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ""
			case <-timer.C:
			}
		}
		text, reported, retry := requestFingerprintSample(ctx, uri, authorization, body)
		if !retry {
			return text, reported
		}
	}
	return "", ""
}

func requestFingerprintSample(ctx context.Context, uri, authorization string, body []byte) (text, reported string, retry bool) {
	attemptCtx, cancel := context.WithTimeout(ctx, 58*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, uri, bytes.NewReader(body))
	if err != nil {
		return "", "", false
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Type", "application/json")
	resp, err := fingerprintGatewayClient.Do(req)
	if err != nil {
		return "", "", ctx.Err() == nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
	}
	const maxReplyBytes = 512 << 10
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxReplyBytes+1))
	if err != nil || len(raw) > maxReplyBytes {
		return "", "", true
	}
	text, reported, err = fingerprintReply(raw)
	if err != nil || strings.TrimSpace(text) == "" {
		return "", "", true
	}
	return text, reported, false
}

func fingerprintReply(body []byte) (string, string, error) {
	type textPart struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		Thought bool   `json:"thought"`
	}
	var value struct {
		Model        string `json:"model"`
		ModelVersion string `json:"modelVersion"`
		Choices      []struct {
			Message dto.Message `json:"message"`
		} `json:"choices"`
		Content []textPart `json:"content"`
		Output  []struct {
			Type    string     `json:"type"`
			Content []textPart `json:"content"`
		} `json:"output"`
		Candidates []struct {
			Content struct {
				Parts []textPart `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := common.Unmarshal(body, &value); err != nil {
		return "", "", err
	}
	var text strings.Builder
	if len(value.Choices) > 0 {
		text.WriteString(value.Choices[0].Message.StringContent())
	}
	for _, item := range value.Content {
		if item.Type == "text" {
			text.WriteString(item.Text)
		}
	}
	for _, item := range value.Output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == "output_text" {
				text.WriteString(part.Text)
			}
		}
	}
	if len(value.Candidates) > 0 {
		for _, part := range value.Candidates[0].Content.Parts {
			if !part.Thought {
				text.WriteString(part.Text)
			}
		}
	}
	if text.Len() == 0 {
		return "", "", errors.New("no visible model reply")
	}
	if value.Model == "" {
		value.Model = value.ModelVersion
	}
	return text.String(), value.Model, nil
}
