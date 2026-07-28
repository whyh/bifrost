package gemini

import (
	"testing"

	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	schemas "github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayloadOrdering_GeminiGenerationRequest(t *testing.T) {
	req := &GeminiGenerationRequest{
		Model: "gemini-2.0-flash",
		Contents: []Content{
			{
				Parts: []*Part{{Text: "hello"}},
				Role:  "user",
			},
		},
		GenerationConfig: GenerationConfig{
			Temperature: schemas.Ptr(float64(0.7)),
		},
		Tools: []Tool{
			{
				FunctionDeclarations: []*FunctionDeclaration{
					{
						Name:        "get_weather",
						Description: "Get weather",
						Parameters: &Schema{
							Type: "OBJECT",
							Properties: map[string]*Schema{
								"location": {Type: "STRING"},
							},
							Required: []string{"location"},
						},
					},
				},
			},
		},
	}

	result, err := providerUtils.MarshalSorted(req)
	require.NoError(t, err)

	golden := `{"model":"gemini-2.0-flash","contents":[{"parts":[{"text":"hello"}],"role":"user"}],"generationConfig":{"temperature":0.7},"tools":[{"functionDeclarations":[{"description":"Get weather","name":"get_weather","parameters":{"properties":{"location":{"type":"STRING"}},"required":["location"],"type":"OBJECT"}}]}]}`

	assert.Equal(t, golden, string(result), "payload field ordering changed — if intentional, update the golden string")

	// Determinism: 100 iterations must produce identical bytes
	for i := 0; i < 100; i++ {
		iter, err := providerUtils.MarshalSorted(req)
		require.NoError(t, err)
		assert.Equal(t, string(result), string(iter), "non-deterministic marshal output on iteration %d", i)
	}
}

func TestNormalizeRawGenerateContentRequestForCompatibility(t *testing.T) {
	t.Run("keeps valid audio and removes unsupported generation config fields", func(t *testing.T) {
		raw := []byte(`{"contents":[{"parts":[{"text":"Transcribe"},{"inlineData":{"mimeType":"audio/wav","data":"UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAIA+AAACABAAZGF0YQAAAAA="}}]}],"generationConfig":{"responseLogprobs":true,"logprobs":3,"presencePenalty":0.5,"frequencyPenalty":0.5,"temperature":0.2}}`)

		got := NormalizeRawGenerateContentRequestForCompatibility(raw)

		assert.Equal(t, `{"contents":[{"parts":[{"text":"Transcribe"},{"inlineData":{"mimeType":"audio/wav","data":"UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAIA+AAACABAAZGF0YQAAAAA="}}]}],"generationConfig":{"temperature":0.2}}`, string(got))
	})

	t.Run("accepts url safe base64 audio", func(t *testing.T) {
		raw := []byte(`{"contents":[{"parts":[{"inlineData":{"mimeType":"audio/wav","data":"AA-_"}}]}]}`)

		got := NormalizeRawGenerateContentRequestForCompatibility(raw)

		assert.Equal(t, string(raw), string(got))
	})

	t.Run("removes invalid audio-only content entries", func(t *testing.T) {
		raw := []byte(`{"contents":[{"parts":[{"inlineData":{"mimeType":"audio/wav","data":"***"}}]},{"parts":[{"text":"keep"}]}]}`)

		got := NormalizeRawGenerateContentRequestForCompatibility(raw)

		assert.Equal(t, `{"contents":[{"parts":[{"text":"keep"}]}]}`, string(got))
	})
}

// TestWrapGeminiCountTokensBody covers the countTokens envelope. The endpoint rejects
// systemInstruction/tools/toolConfig/generationConfig at the top level and silently
// ignores a top-level contents/model once generateContentRequest is set, so the body
// must carry the envelope and nothing beside it.
func TestWrapGeminiCountTokensBody(t *testing.T) {
	t.Run("wraps a flat body and keeps every counted field", func(t *testing.T) {
		raw := []byte(`{"model":"gemini-3.6-flash","contents":[{"role":"user","parts":[{"text":"hi"}]}],"systemInstruction":{"parts":[{"text":"be terse"}]},"tools":[{"functionDeclarations":[{"name":"probe"}]}],"toolConfig":{"functionCallingConfig":{"mode":"AUTO"}},"generationConfig":{"temperature":0.2}}`)

		got := wrapGeminiCountTokensBody(raw, "gemini-3.6-flash")

		assert.JSONEq(t, `{"generateContentRequest":{"model":"models/gemini-3.6-flash","contents":[{"role":"user","parts":[{"text":"hi"}]}],"systemInstruction":{"parts":[{"text":"be terse"}]},"tools":[{"functionDeclarations":[{"name":"probe"}]}],"toolConfig":{"functionCallingConfig":{"mode":"AUTO"}},"generationConfig":{"temperature":0.2}}}`, string(got))
	})

	t.Run("leaves nothing at the top level besides the envelope", func(t *testing.T) {
		got := wrapGeminiCountTokensBody([]byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`), "gemini-3.6-flash")

		require.True(t, providerUtils.JSONFieldExists(got, "generateContentRequest"))
		assert.False(t, providerUtils.JSONFieldExists(got, "contents"))
		assert.False(t, providerUtils.JSONFieldExists(got, "model"))
	})

	t.Run("does not double wrap an already enveloped body", func(t *testing.T) {
		raw := []byte(`{"generateContentRequest":{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"systemInstruction":{"parts":[{"text":"be terse"}]}}}`)

		got := wrapGeminiCountTokensBody(raw, "gemini-3.6-flash")

		assert.JSONEq(t, `{"generateContentRequest":{"model":"models/gemini-3.6-flash","contents":[{"role":"user","parts":[{"text":"hi"}]}],"systemInstruction":{"parts":[{"text":"be terse"}]}}}`, string(got))
		assert.False(t, providerUtils.JSONFieldExists(got, "generateContentRequest.generateContentRequest"))
	})

	t.Run("qualifies the model without doubling the prefix", func(t *testing.T) {
		got := wrapGeminiCountTokensBody([]byte(`{"contents":[]}`), "models/gemini-3.6-flash")

		assert.Equal(t, "models/gemini-3.6-flash", providerUtils.GetJSONField(got, "generateContentRequest.model").String())
	})

	t.Run("strips fields GenerateContentRequest does not define", func(t *testing.T) {
		raw := []byte(`{"contents":[],"labels":{"a":"b"},"fallbacks":["gemini/other"]}`)

		got := wrapGeminiCountTokensBody(raw, "gemini-3.6-flash")

		assert.False(t, providerUtils.JSONFieldExists(got, "generateContentRequest.labels"))
		assert.False(t, providerUtils.JSONFieldExists(got, "generateContentRequest.fallbacks"))
	})

	t.Run("handles empty body", func(t *testing.T) {
		assert.Empty(t, wrapGeminiCountTokensBody(nil, "gemini-3.6-flash"))
	})
}

// TestGeminiCountTokensRequestToGenerationRequest covers the genai ingress: clients may
// post bare contents or the documented generateContentRequest envelope, and only the
// latter can carry a system instruction.
func TestGeminiCountTokensRequestToGenerationRequest(t *testing.T) {
	t.Run("unwraps the envelope and keeps the system instruction", func(t *testing.T) {
		req := &GeminiCountTokensRequest{
			Model: "gemini-3.6-flash",
			GenerateContentRequest: &GeminiGenerationRequest{
				Contents:          []Content{{Role: "user", Parts: []*Part{{Text: "hi"}}}},
				SystemInstruction: &Content{Parts: []*Part{{Text: "be terse"}}},
			},
		}

		got := req.ToGeminiGenerationRequest()

		require.NotNil(t, got.SystemInstruction)
		assert.Equal(t, "be terse", got.SystemInstruction.Parts[0].Text)
		assert.Equal(t, "gemini-3.6-flash", got.Model)
		assert.True(t, got.IsCountTokens)
	})

	t.Run("falls back to bare contents", func(t *testing.T) {
		req := &GeminiCountTokensRequest{
			Model:    "gemini-3.6-flash",
			Contents: []Content{{Role: "user", Parts: []*Part{{Text: "hi"}}}},
		}

		got := req.ToGeminiGenerationRequest()

		require.Len(t, got.Contents, 1)
		assert.Equal(t, "hi", got.Contents[0].Parts[0].Text)
	})

	t.Run("path model wins over the envelope model", func(t *testing.T) {
		req := &GeminiCountTokensRequest{
			Model:                  "gemini-3.6-flash",
			GenerateContentRequest: &GeminiGenerationRequest{Model: "models/stale"},
		}

		assert.Equal(t, "gemini-3.6-flash", req.ToGeminiGenerationRequest().Model)
	})
}
