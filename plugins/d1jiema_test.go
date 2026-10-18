package plugins_test

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The d1jiema vendor API answers inside the submission request, so every task
// it creates is terminal before it is persisted. These cases pin that contract
// and the vendor failure detection, because a misread body would either charge
// for a refused call or hide a refusal behind a successful task.
func TestD1jiemaSubmissionCompletesImmediately(t *testing.T) {
	source, err := builtinplugins.Source("d1jiema")
	require.NoError(t, err)
	registry := jsplugin.NewRegistry()
	plugin, err := registry.RegisterFactory(source, jsplugin.Options{Key: "d1jiema"})
	require.NoError(t, err)

	t.Run("claims every model on the responses endpoint and on native routes", func(t *testing.T) {
		for _, model := range plugin.Meta.Models {
			binding, found := registry.Generation().LookupEndpoint("POST", "/v1/responses", model)
			require.True(t, found, model)
			assert.Same(t, plugin, binding.Plugin)
		}
		for _, route := range []struct {
			path  string
			model string
		}{
			{"/d1jiema/getPhone", "getPhone"},
			{"/d1jiema/getMsg", "getMsg"},
			{"/d1jiema/send", "send"},
		} {
			binding, found := registry.Generation().LookupDeclaredRoute("POST", route.path)
			require.True(t, found, route.path)
			assert.Equal(t, "d1jiema", binding.Plugin.Meta.Key)
			assert.Equal(t, jsplugin.RouteTypeSubmit, binding.Route.Type)
		}
	})

	parse := func(t *testing.T, ctx map[string]any, body any) map[string]any {
		t.Helper()
		value, callErr := plugin.Engine.Call(t.Context(), "parseSubmitResponse", ctx, map[string]any{
			"statusCode": 200,
			"headers":    map[string]any{"Content-Type": []any{"text/html"}},
			"body":       body,
		})
		require.NoError(t, callErr)
		encoded, marshalErr := common.Marshal(value)
		require.NoError(t, marshalErr)
		var decoded map[string]any
		require.NoError(t, common.Unmarshal(encoded, &decoded))
		return decoded
	}

	t.Run("a taken number settles as an immediate success", func(t *testing.T) {
		result := parse(t, map[string]any{"action": "getPhone", "model": "getPhone", "requestBody": map[string]any{"keyWord": "毛竹"}}, "13012345678")
		assert.True(t, strings.HasPrefix(result["taskId"].(string), "d1j_"))
		assert.Equal(t, map[string]any{"operation": "getPhone", "raw": "13012345678", "phone": "13012345678"}, result["taskData"])
		immediate := result["immediate"].(map[string]any)
		assert.Equal(t, "SUCCESS", immediate["status"])
		assert.Equal(t, "100%", immediate["progress"])
	})

	t.Run("a vendor refusal is a terminal failure with its reason", func(t *testing.T) {
		result := parse(t, map[string]any{"action": "getMsg", "model": "getMsg", "requestBody": map[string]any{"phone": "13012345678"}}, "ERROR:token错误，请重新通过API登录")
		immediate := result["immediate"].(map[string]any)
		assert.Equal(t, "FAILURE", immediate["status"])
		assert.Equal(t, "ERROR:token错误，请重新通过API登录", immediate["reason"])
		assert.Equal(t, "ERROR:token错误，请重新通过API登录", result["taskData"].(map[string]any)["error"])
	})

	t.Run("a message that has not arrived fails so nothing is billed", func(t *testing.T) {
		result := parse(t, map[string]any{"action": "getMsg", "model": "getMsg", "requestBody": map[string]any{"phone": "13012345678"}}, "[尚未收到] 请稍后再试")
		data := result["taskData"].(map[string]any)
		assert.Equal(t, true, data["pending"])
		assert.Equal(t, "13012345678", data["phone"])
		immediate := result["immediate"].(map[string]any)
		assert.Equal(t, "FAILURE", immediate["status"])
		assert.Contains(t, immediate["reason"], "本次不计费")
		assert.Contains(t, immediate["reason"], "[尚未收到] 请稍后再试")
	})

	t.Run("an older gateway reporting pending is treated as not delivered", func(t *testing.T) {
		upstream := map[string]any{"action": "getMsg", "model": "getMsg", "upstream": map[string]any{"kind": "new_api"}}
		result := parse(t, upstream, map[string]any{
			"code": "ok", "operation": "getMsg", "task_id": "task_x", "raw": "[尚未收到] 请稍后再试", "pending": true,
		})
		assert.Equal(t, "FAILURE", result["immediate"].(map[string]any)["status"])
	})

	t.Run("another New API gateway answers with this plugin's envelope", func(t *testing.T) {
		upstream := map[string]any{"action": "getMsg", "model": "getMsg", "upstream": map[string]any{"kind": "new_api"}}
		result := parse(t, upstream, map[string]any{
			"code": "ok", "operation": "getMsg", "task_id": "task_x", "raw": "【毛竹】验证码9876", "phone": "13012345678", "pending": false,
		})
		assert.Equal(t, "【毛竹】验证码9876", result["taskData"].(map[string]any)["raw"])
		assert.Equal(t, "SUCCESS", result["immediate"].(map[string]any)["status"])

		failed := parse(t, upstream, map[string]any{
			"code": "error", "operation": "getPhone", "task_id": "task_y", "reason": "ERROR:没有可用的号码",
		})
		assert.Equal(t, "FAILURE", failed["immediate"].(map[string]any)["status"])
		assert.Equal(t, "ERROR:没有可用的号码", failed["immediate"].(map[string]any)["reason"])
	})

	t.Run("the native presenter exposes the number and the message", func(t *testing.T) {
		value, callErr := plugin.Engine.CallMember(t.Context(), "native", "renderResult", map[string]any{}, map[string]any{
			"task_id": "task_1",
			"status":  "SUCCESS",
			"data":    map[string]any{"operation": "getMsg", "raw": "【毛竹】验证码9876", "message": "【毛竹】验证码9876", "phone": "13012345678", "pending": false},
		})
		require.NoError(t, callErr)
		encoded, marshalErr := common.Marshal(value)
		require.NoError(t, marshalErr)
		var decoded map[string]any
		require.NoError(t, common.Unmarshal(encoded, &decoded))
		assert.Equal(t, map[string]any{
			"code": "ok", "operation": "getMsg", "task_id": "task_1",
			"phone": "13012345678", "message": "【毛竹】验证码9876", "pending": false, "raw": "【毛竹】验证码9876",
		}, decoded)
	})
}
