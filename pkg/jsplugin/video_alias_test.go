package jsplugin

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A vendor-published video path must reach the same plugin as the host path it
// aliases, and an unrelated path must stay unclaimed.
func TestOpenAIVideoCreateBindsVendorAliasPath(t *testing.T) {
	registry := NewRegistry()
	plugin := mustCompileRoutingPlugin(t, "video-alias", 17, `["video-a"]`, `protocols: ["openai_video"],`, `
export function listArtifacts() { return []; }
export function buildContentRequest() { return {url: "https://example.com/v.mp4"}; }
export const protocols = {openai_video: {
  decodeRequest: function(ctx) { return {kind: "submit", model: ctx.model, requestBody: ctx.body.value}; },
  render: function(ctx, task) { return {id: task.task_id, status: "completed"}; }
}};`)
	require.NoError(t, registry.ReplaceOverrides([]*LoadedPlugin{plugin}))

	for _, path := range []string{"/v1/videos", "/v1/videos/generations"} {
		binding, ok := registry.Generation().LookupEndpoint(http.MethodPost, path, "video-a")
		require.True(t, ok, path)
		assert.Equal(t, "openai_video", binding.Protocol)
		assert.Equal(t, "create", binding.Operation.Name)
		assert.Same(t, plugin, binding.Plugin)
	}

	_, ok := registry.Generation().LookupEndpoint(http.MethodPost, "/v1/images/generations", "video-a")
	assert.False(t, ok, "the alias must not bind an image endpoint")

	_, ok = registry.Generation().LookupEndpoint(http.MethodGet, "/v1/videos/generations", "video-a")
	assert.False(t, ok, "the alias is a POST binding only")
}

// The alias is one registry entry, so both paths resolve to the identical
// operation and therefore the same body kinds, model field, and required hooks.
func TestOpenAIVideoAliasSharesOneOperationDefinition(t *testing.T) {
	protocol, create, found := LookupHostProtocolOperation(http.MethodPost, "/v1/videos")
	require.True(t, found)
	aliasProtocol, aliasCreate, found := LookupHostProtocolOperation(http.MethodPost, "/v1/videos/generations")
	require.True(t, found)

	assert.Equal(t, "openai_video", protocol)
	assert.Equal(t, protocol, aliasProtocol)
	assert.Equal(t, create, aliasCreate, "both paths must resolve to the same registry entry, not two entries that can drift")
	assert.Equal(t, []string{"/v1/videos", "/v1/videos/generations"}, create.Paths())
}

// Paths must list the primary binding first and stay independent of the table.
func TestHostProtocolOperationPathsAreCopied(t *testing.T) {
	_, create, found := LookupHostProtocolOperation(http.MethodPost, "/v1/videos")
	require.True(t, found)
	assert.Equal(t, []string{"/v1/videos", "/v1/videos/generations"}, create.Paths())
	assert.True(t, create.MatchesPath("/v1/videos"))
	assert.True(t, create.MatchesPath("/v1/videos/generations"))
	assert.False(t, create.MatchesPath("/v1/videos/generations/extra"))

	paths := create.Paths()
	paths[0] = "/mutated"
	_, again, found := LookupHostProtocolOperation(http.MethodPost, "/v1/videos")
	require.True(t, found)
	assert.Equal(t, []string{"/v1/videos", "/v1/videos/generations"}, again.Paths(), "the host table must not be mutable through a returned operation")
}

// An operation with no alias keeps a single binding.
func TestHostProtocolOperationWithoutAliasHasOnePath(t *testing.T) {
	operation := HostProtocolOperation{Name: "x", Path: "/one"}
	assert.Equal(t, []string{"/one"}, operation.Paths())
	assert.True(t, operation.MatchesPath("/one"))
	assert.False(t, operation.MatchesPath("/two"))

	// GetOps on a path the alias does not cover stays unclaimed.
	_, _, found := LookupHostProtocolOperation(http.MethodGet, "/v1/videos/generations")
	assert.False(t, found, "the alias binds POST only")
}
