package helper

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// A quota estimate prices a request before any channel is chosen, so RelayInfo
// carries no ChannelMeta. Store-channel billing reads ChannelId from there and
// must not dereference the nil embedded struct.
func TestHandleGroupRatioToleratesMissingChannelMeta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{}

	require.NotPanics(t, func() {
		groupRatioInfo := HandleGroupRatio(ctx, info)
		require.Positive(t, groupRatioInfo.GroupRatio)
	})
}
