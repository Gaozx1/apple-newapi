package service

import (
	"bytes"
	"context"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestResetStatusCode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		statusCode       int
		statusCodeConfig string
		expectedCode     int
	}{
		{
			name:             "map string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"503"}`,
			expectedCode:     503,
		},
		{
			name:             "map int value",
			statusCode:       429,
			statusCodeConfig: `{"429":503}`,
			expectedCode:     503,
		},
		{
			name:             "skip invalid string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"bad-code"}`,
			expectedCode:     429,
		},
		{
			name:             "skip status code 200",
			statusCode:       200,
			statusCodeConfig: `{"200":503}`,
			expectedCode:     200,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			newAPIError := &types.NewAPIError{
				StatusCode: tc.statusCode,
			}
			ResetStatusCode(newAPIError, tc.statusCodeConfig)
			require.Equal(t, tc.expectedCode, newAPIError.StatusCode)
		})
	}
}

func TestRelayErrorHandlerTruncatesInvalidJSONBodyInLog(t *testing.T) {
	withDebugEnabled(t, false)

	body := strings.Repeat("b", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, "bad response status code 500", newAPIError.Error())
	require.Contains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), fmt.Sprintf("original_length=%d", len(body)))
	require.NotContains(t, logBuffer.String(), strings.Repeat("b", common.LocalLogContentLimit+1))
}

func TestRelayErrorHandlerKeepsStructuredErrorMessage(t *testing.T) {
	message := strings.Repeat("c", common.LocalLogContentLimit+256)
	body := `{"message":"` + message + `"}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerKeepsOpenAIErrorMessage(t *testing.T) {
	message := strings.Repeat("d", common.LocalLogContentLimit+256)
	body := `{"error":{"message":"` + message + `","type":"server_error","code":"server_error"}}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerKeepsInvalidJSONBodyInDebugLog(t *testing.T) {
	withDebugEnabled(t, true)

	body := strings.Repeat("e", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.NotContains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), body)
}

func withDebugEnabled(t *testing.T, enabled bool) {
	t.Helper()

	oldDebug := common.DebugEnabled
	common.DebugEnabled = enabled
	t.Cleanup(func() {
		common.DebugEnabled = oldDebug
	})
}

// ErrorWarrantsChannelDisable is the rule the manual batch key test uses to
// decide whether a probe result proves a key dead. It must disable keys on
// credential/account failures and must NOT disable them on transient failures,
// otherwise a flaky upstream would drain a healthy key pool.
func TestErrorWarrantsChannelDisableIgnoresTransientFailures(t *testing.T) {
	testCases := []struct {
		name string
		err  *types.NewAPIError
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "401 invalid credential disables the key",
			err:  types.NewOpenAIError(fmt.Errorf("Invalid token"), types.ErrorCodeBadResponse, http.StatusUnauthorized),
			want: true,
		},
		{
			name: "server error does not disable the key",
			err:  types.NewOpenAIError(fmt.Errorf("upstream exploded"), types.ErrorCodeBadResponse, http.StatusInternalServerError),
			want: false,
		},
		{
			name: "bad gateway does not disable the key",
			err:  types.NewOpenAIError(fmt.Errorf("bad gateway"), types.ErrorCodeBadResponse, http.StatusBadGateway),
			want: false,
		},
		{
			name: "rate limit does not disable the key",
			err:  types.NewOpenAIError(fmt.Errorf("too many requests"), types.ErrorCodeBadResponse, http.StatusTooManyRequests),
			want: false,
		},
		{
			name: "quota exhausted keyword disables the key",
			err:  types.NewOpenAIError(fmt.Errorf("You exceeded your current quota, please check your plan"), types.ErrorCodeBadResponse, http.StatusOK),
			want: true,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, ErrorWarrantsChannelDisable(test.err))
		})
	}
}

// The manual batch key test is an explicit administrator action, so it must not
// be silently neutered by the global auto-disable switch. Live traffic keeps
// honoring that switch through ShouldDisableChannel.
func TestManualKeyTestDisableIgnoresGlobalSwitch(t *testing.T) {
	original := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = false
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = original })

	authErr := types.NewOpenAIError(fmt.Errorf("Invalid token"), types.ErrorCodeBadResponse, http.StatusUnauthorized)

	require.True(t, ErrorWarrantsChannelDisable(authErr),
		"manual batch test must still disable a dead key while the global switch is off")
	require.False(t, ShouldDisableChannel(authErr),
		"live traffic must keep honoring the global auto-disable switch")
}
