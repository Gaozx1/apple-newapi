package openai

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestConvertImageEditRequestMultipart verifies that ConvertImageRequest
// re-serializes multipart image edit requests with all fields (including
// stream) and the file intact, both when the form was already parsed and when
// it must be re-parsed from the reusable body.
func TestConvertImageEditRequestMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newMultipartContext := func(t *testing.T, prompt string) *gin.Context {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("model", "gpt-image-1"))
		require.NoError(t, writer.WriteField("prompt", prompt))
		require.NoError(t, writer.WriteField("stream", "true"))
		require.NoError(t, writer.WriteField("partial_images", "3"))
		part, err := writer.CreateFormFile("image", "input.png")
		require.NoError(t, err)
		_, err = part.Write([]byte("fake image"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		return c
	}

	convertAndReplay := func(t *testing.T, c *gin.Context, prompt string) {
		info := &relaycommon.RelayInfo{
			RelayMode: relayconstant.RelayModeImagesEdits,
		}
		request := dto.ImageRequest{
			Model:  "gpt-image-1",
			Prompt: prompt,
			Stream: common.GetPointer(true),
		}

		converted, err := (&Adaptor{}).ConvertImageRequest(c, info, request)
		require.NoError(t, err)
		convertedBody, ok := converted.(*bytes.Buffer)
		require.True(t, ok)

		replayedRequest := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(convertedBody.Bytes()))
		replayedRequest.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
		require.NoError(t, replayedRequest.ParseMultipartForm(32<<20))

		require.Equal(t, "gpt-image-1", replayedRequest.PostForm.Get("model"))
		require.Equal(t, prompt, replayedRequest.PostForm.Get("prompt"))
		require.Equal(t, "true", replayedRequest.PostForm.Get("stream"))
		require.Equal(t, "3", replayedRequest.PostForm.Get("partial_images"))
		require.Len(t, replayedRequest.MultipartForm.File["image"], 1)

		file, err := replayedRequest.MultipartForm.File["image"][0].Open()
		require.NoError(t, err)
		defer file.Close()
		fileBytes, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, []byte("fake image"), fileBytes)
	}

	t.Run("with pre-parsed form", func(t *testing.T) {
		prompt := "edit this image"
		c := newMultipartContext(t, prompt)
		require.NoError(t, c.Request.ParseMultipartForm(32<<20))

		convertAndReplay(t, c, prompt)
	})

	t.Run("re-parses reusable body when form is missing", func(t *testing.T) {
		prompt := "edit without pre-parsed form"
		c := newMultipartContext(t, prompt)

		storage, err := common.GetBodyStorage(c)
		require.NoError(t, err)
		c.Request.Body = io.NopCloser(storage)
		c.Request.MultipartForm = nil
		c.Request.PostForm = nil

		convertAndReplay(t, c, prompt)
	})
}

// TestConvertImageEditRequestJSONUpstream covers the channel setting for
// upstreams that reject multipart image edits: the same form is resubmitted as
// JSON with the reference image inlined as a data URI.
func TestConvertImageEditRequestJSONUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newContext := func(t *testing.T, imageNames ...string) *gin.Context {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("model", "grok-imagine-image"))
		require.NoError(t, writer.WriteField("prompt", "make it red"))
		require.NoError(t, writer.WriteField("size", "1024x1024"))
		for _, name := range imageNames {
			part, err := writer.CreateFormFile("image", name)
			require.NoError(t, err)
			_, err = part.Write([]byte("fake-" + name))
			require.NoError(t, err)
		}
		require.NoError(t, writer.Close())

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		return c
	}

	jsonInfo := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{ImageEditJSON: true},
		},
	}

	t.Run("inlines one reference image as a data URI", func(t *testing.T) {
		c := newContext(t, "ref.png")
		request := dto.ImageRequest{
			Model:  "grok-imagine-image",
			Prompt: "make it red",
			Size:   "1024x1024",
			N:      common.GetPointer(uint(2)),
		}

		converted, err := (&Adaptor{}).ConvertImageRequest(c, jsonInfo, request)
		require.NoError(t, err)

		payload, ok := converted.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "grok-imagine-image", payload["model"])
		require.Equal(t, "make it red", payload["prompt"])
		require.Equal(t, "1024x1024", payload["size"])
		// The bounded image count survives the conversion.
		require.Equal(t, uint(2), payload["n"])
		require.Equal(t, map[string]string{
			"url": "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("fake-ref.png")),
		}, payload["image"])
		// The client request keeps its multipart form: a retry must convert it
		// again instead of falling through to the bare request struct.
		require.False(t, isJSONRequest(c))
		require.NotNil(t, c.Request.MultipartForm)

		convertedAgain, err := (&Adaptor{}).ConvertImageRequest(c, jsonInfo, request)
		require.NoError(t, err)
		require.Equal(t, payload, convertedAgain)
	})

	t.Run("uses an images array for multiple references", func(t *testing.T) {
		c := newContext(t, "a.png", "b.png")

		converted, err := (&Adaptor{}).ConvertImageRequest(c, jsonInfo, dto.ImageRequest{
			Model:  "grok-imagine-image",
			Prompt: "blend them",
		})
		require.NoError(t, err)

		payload, ok := converted.(map[string]any)
		require.True(t, ok)
		require.NotContains(t, payload, "image")
		images, ok := payload["images"].([]map[string]string)
		require.True(t, ok)
		require.Len(t, images, 2)
		require.Contains(t, images[0]["url"], "data:image/png;base64,")
		require.False(t, images[0]["url"] == images[1]["url"])
	})

	t.Run("keeps multipart when the setting is off", func(t *testing.T) {
		c := newContext(t, "ref.png")
		info := &relaycommon.RelayInfo{
			RelayMode:   relayconstant.RelayModeImagesEdits,
			ChannelMeta: &relaycommon.ChannelMeta{},
		}

		converted, err := (&Adaptor{}).ConvertImageRequest(c, info, dto.ImageRequest{
			Model:  "gpt-image-1",
			Prompt: "make it red",
		})
		require.NoError(t, err)
		_, ok := converted.(*bytes.Buffer)
		require.True(t, ok)
		require.False(t, isJSONRequest(c))
	})

	t.Run("rejects an edit without a reference image", func(t *testing.T) {
		c := newContext(t)

		_, err := (&Adaptor{}).ConvertImageRequest(c, jsonInfo, dto.ImageRequest{
			Model:  "grok-imagine-image",
			Prompt: "make it red",
		})
		require.ErrorContains(t, err, "image is required")
	})

	t.Run("labels the outbound request as JSON", func(t *testing.T) {
		c := newContext(t, "ref.png")

		jsonHeaders := http.Header{}
		require.NoError(t, (&Adaptor{}).SetupRequestHeader(c, &jsonHeaders, jsonInfo))
		require.Equal(t, "application/json", jsonHeaders.Get("Content-Type"))

		formInfo := &relaycommon.RelayInfo{
			RelayMode:   relayconstant.RelayModeImagesEdits,
			ChannelMeta: &relaycommon.ChannelMeta{},
		}
		formHeaders := http.Header{}
		require.NoError(t, (&Adaptor{}).SetupRequestHeader(c, &formHeaders, formInfo))
		require.Contains(t, formHeaders.Get("Content-Type"), "multipart/form-data")
	})
}
