package typesafeai_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/typesafeai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
)

func TestAdaptorSupportMode(t *testing.T) {
	a := &typesafeai.Adaptor{}

	assert.True(t, a.SupportMode(&meta.Meta{Mode: mode.SystemOne}))
	assert.False(t, a.SupportMode(&meta.Meta{Mode: mode.ChatCompletions}))
	assert.False(t, a.SupportMode(&meta.Meta{Mode: mode.Anthropic}))
}

func TestAdaptorDefaultBaseURL(t *testing.T) {
	a := &typesafeai.Adaptor{}
	assert.Equal(t, "https://api.typesafe.ai/v1", a.DefaultBaseURL())
}

func TestAdaptorGetRequestURL(t *testing.T) {
	a := &typesafeai.Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{BaseURL: "https://api.typesafe.ai/v1"},
		mode.SystemOne,
		"gpt-4o",
		coremodel.ModelConfig{},
	)

	got, err := a.GetRequestURL(m, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, http.MethodPost, got.Method)
	assert.Equal(t, "https://api.typesafe.ai/v1/systemone", got.URL)
}

func TestAdaptorGetRequestURLCustomBaseURL(t *testing.T) {
	a := &typesafeai.Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{BaseURL: "https://proxy.example.com/typesafeai"},
		mode.SystemOne,
		"gpt-4o",
		coremodel.ModelConfig{},
	)

	got, err := a.GetRequestURL(m, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, http.MethodPost, got.Method)
	assert.Equal(t, "https://proxy.example.com/typesafeai/systemone", got.URL)
}

func TestConvertRequestValid(t *testing.T) {
	a := &typesafeai.Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.SystemOne,
		"gpt-4o",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "gpt-4o"

	body := `{"state":"test state","model":"gpt-4o","questions":{"q1":{"type":"choice","instructions":"What is 2+2?","criteria":{"a":"4","b":"5"}}}}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/systemone",
		strings.NewReader(body),
	)
	assert.NoError(t, err)

	result, err := a.ConvertRequest(m, nil, req)
	assert.NoError(t, err)

	convertedBody, err := io.ReadAll(result.Body)
	assert.NoError(t, err)

	var converted typesafeai.SystemOneRequest

	err = sonic.Unmarshal(convertedBody, &converted)
	assert.NoError(t, err)

	// Model should be set to ActualModel
	assert.Equal(t, "gpt-4o", converted.Model)
	// State should be preserved
	assert.Equal(t, "test state", converted.State)
	// Questions should be preserved
	assert.Contains(t, converted.Questions, "q1")
	assert.Equal(t, "choice", converted.Questions["q1"].Type)
	// Instructions should be preserved (regression test for instructions being dropped)
	assert.Equal(t, "What is 2+2?", converted.Questions["q1"].Instructions)
}

// TestConvertRequestPreservesInstructions is a regression test for the bug where
// the "instructions" field was dropped during request conversion because the
// Question struct used "question" as the JSON tag instead of "instructions".
func TestConvertRequestPreservesInstructions(t *testing.T) {
	a := &typesafeai.Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.SystemOne,
		"clef",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "clef"

	// This is the exact request shape reported in the bug:
	// the upstream returned "questions.is_urgent": ""instructions" must be provided"
	body := `{"state":"Help! My payouts have been failing for 3 days.","model":"clef","questions":{"is_urgent":{"type":"noul","instructions":"Does this convey urgency?"}}}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/systemone",
		strings.NewReader(body),
	)
	assert.NoError(t, err)

	result, err := a.ConvertRequest(m, nil, req)
	assert.NoError(t, err)

	convertedBody, err := io.ReadAll(result.Body)
	assert.NoError(t, err)

	// The converted body should still contain the "instructions" field
	assert.Contains(t, string(convertedBody), `"instructions":"Does this convey urgency?"`)

	// Verify by parsing the structure
	var converted typesafeai.SystemOneRequest

	err = sonic.Unmarshal(convertedBody, &converted)
	assert.NoError(t, err)

	assert.Contains(t, converted.Questions, "is_urgent")
	assert.Equal(t, "noul", converted.Questions["is_urgent"].Type)
	assert.Equal(t, "Does this convey urgency?", converted.Questions["is_urgent"].Instructions)
}

func TestConvertRequestMissingState(t *testing.T) {
	a := &typesafeai.Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.SystemOne,
		"gpt-4o",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "gpt-4o"

	body := `{"model":"gpt-4o","questions":{"q1":{"type":"choice"}}}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/systemone",
		strings.NewReader(body),
	)
	assert.NoError(t, err)

	_, err = a.ConvertRequest(m, nil, req)
	assert.Error(t, err)

	var sysErr adaptor.Error

	ok := errors.As(err, &sysErr)
	assert.True(t, ok)

	if ok {
		assert.Equal(t, http.StatusBadRequest, sysErr.StatusCode())
	}
}

func TestConvertRequestMissingQuestions(t *testing.T) {
	a := &typesafeai.Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.SystemOne,
		"gpt-4o",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "gpt-4o"

	body := `{"state":"test","model":"gpt-4o"}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/systemone",
		strings.NewReader(body),
	)
	assert.NoError(t, err)

	_, err = a.ConvertRequest(m, nil, req)
	assert.Error(t, err)

	var sysErr adaptor.Error

	ok := errors.As(err, &sysErr)
	assert.True(t, ok)

	if ok {
		assert.Equal(t, http.StatusBadRequest, sysErr.StatusCode())
	}
}

func TestConvertRequestInvalidJSON(t *testing.T) {
	a := &typesafeai.Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.SystemOne,
		"gpt-4o",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "gpt-4o"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/systemone",
		strings.NewReader(`{invalid json}`),
	)
	assert.NoError(t, err)

	_, err = a.ConvertRequest(m, nil, req)
	assert.Error(t, err)

	var sysErr adaptor.Error

	ok := errors.As(err, &sysErr)
	assert.True(t, ok)

	if ok {
		assert.Equal(t, http.StatusBadRequest, sysErr.StatusCode())
	}
}

func TestDoResponseUsageExtraction(t *testing.T) {
	// Parse a valid response body to verify usage extraction
	body := `{
		"answers": {
			"q1": {
				"type": "choice",
				"choice": "a",
				"confidence": 0.95,
				"probabilities": {"a": 0.9, "b": 0.1}
			}
		},
		"usage": {
			"input_tokens": 10,
			"output_tokens": 20,
			"total_tokens": 30
		}
	}`

	var resp typesafeai.SystemOneResponse

	err := sonic.Unmarshal([]byte(body), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 10, resp.Usage.InputTokens)
	assert.Equal(t, 20, resp.Usage.OutputTokens)
	assert.Equal(t, 30, resp.Usage.TotalTokens)
	assert.Contains(t, resp.Answers, "q1")
	assert.Equal(t, "a", resp.Answers["q1"].Choice)
	assert.Equal(t, 0.95, resp.Answers["q1"].Confidence)
}

func TestDoResponseWritesRawBody(t *testing.T) {
	a := &typesafeai.Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.SystemOne,
		"gpt-4o",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "gpt-4o"

	responseBody := `{"answers":{"q1":{"type":"choice","choice":"a"}},"usage":{"input_tokens":5,"output_tokens":10,"total_tokens":15}}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	result, err := a.DoResponse(m, nil, c, resp)
	assert.NoError(t, err)

	// Verify usage was extracted
	assert.Equal(t, int64(5), int64(result.Usage.InputTokens))
	assert.Equal(t, int64(10), int64(result.Usage.OutputTokens))
	assert.Equal(t, int64(15), int64(result.Usage.TotalTokens))

	// Verify raw response was written
	w.Flush()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, responseBody, w.Body.String())
}

func TestDoResponseErrorStatus(t *testing.T) {
	a := &typesafeai.Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.SystemOne,
		"gpt-4o",
		coremodel.ModelConfig{},
	)

	responseBody := `{"error":{"type":"invalid_request","message":"Invalid model"}}`

	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	result, err := a.DoResponse(m, nil, c, resp)
	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, err.StatusCode())
	assert.Empty(t, result.Usage.InputTokens)
}

func TestErrorHanlder(t *testing.T) {
	t.Run("nil response", func(t *testing.T) {
		err := typesafeai.ErrorHanlder(nil)
		assert.NotNil(t, err)
		assert.Equal(t, http.StatusInternalServerError, err.StatusCode())
	})

	t.Run("non-200 with error body", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body: io.NopCloser(
				strings.NewReader(`{"error":{"type":"bad_request","message":"test error"}}`),
			),
			Header: make(http.Header),
		}

		err := typesafeai.ErrorHanlder(resp)
		assert.NotNil(t, err)
		assert.Equal(t, http.StatusBadRequest, err.StatusCode())
	})
}

func TestIsSuccessfulResponseStatus(t *testing.T) {
	assert.True(t, typesafeai.IsSuccessfulResponseStatus(http.StatusOK))
	assert.False(t, typesafeai.IsSuccessfulResponseStatus(http.StatusBadRequest))
	assert.False(t, typesafeai.IsSuccessfulResponseStatus(http.StatusInternalServerError))
	assert.False(t, typesafeai.IsSuccessfulResponseStatus(http.StatusNotFound))
}

func TestSystemOneRequestTypes(t *testing.T) {
	t.Run("choice question", func(t *testing.T) {
		req := typesafeai.SystemOneRequest{
			State: "context",
			Model: "gpt-4o",
			Questions: map[string]typesafeai.Question{
				"q1": {
					Type:         "choice",
					Instructions: "Which team should handle this?",
					Criteria: map[string]string{
						"billing":   "Payments, invoicing, refunds",
						"technical": "Bugs, outages, integrations",
					},
				},
			},
		}
		assert.Equal(t, "context", req.State)
		assert.Equal(t, "gpt-4o", req.Model)
		assert.Equal(t, "choice", req.Questions["q1"].Type)
		assert.Equal(t, "Which team should handle this?", req.Questions["q1"].Instructions)
	})

	t.Run("score question", func(t *testing.T) {
		question := typesafeai.Question{
			Type:         "score",
			Instructions: "How frustrated is the customer?",
			Criteria:     []string{"Calm", "Frustrated", "Very angry"},
		}
		assert.Equal(t, "score", question.Type)
		assert.Equal(t, "How frustrated is the customer?", question.Instructions)
		criteria, ok := question.Criteria.([]string)
		assert.True(t, ok)
		assert.Len(t, criteria, 3)
	})

	t.Run("noul question", func(t *testing.T) {
		question := typesafeai.Question{
			Type:         "noul",
			Instructions: "Does this convey urgency?",
			Criteria: map[string]string{
				"true":  "Explicitly time-sensitive",
				"false": "No urgency expressed",
			},
		}
		assert.Equal(t, "noul", question.Type)
		assert.Equal(t, "Does this convey urgency?", question.Instructions)
		criteria, ok := question.Criteria.(map[string]string)
		assert.True(t, ok)
		assert.Len(t, criteria, 2)
	})
}

func TestAnswerTypes(t *testing.T) {
	t.Run("choice answer", func(t *testing.T) {
		answer := typesafeai.Answer{
			Type:       "choice",
			Choice:     "a",
			Confidence: 0.95,
			Probabilities: map[string]float64{
				"a": 0.9,
				"b": 0.1,
			},
		}
		assert.Equal(t, "choice", answer.Type)
		assert.Equal(t, "a", answer.Choice)
		assert.Equal(t, 0.95, answer.Confidence)
		assert.Equal(t, 0.9, answer.Probabilities["a"])
	})

	t.Run("score answer", func(t *testing.T) {
		answer := typesafeai.Answer{
			Type:       "score",
			Score:      1.05,
			Confidence: 0.92,
			Legend: map[string]string{
				"0": "Calm",
				"1": "Frustrated",
				"2": "Very angry",
			},
			Probabilities: map[string]float64{
				"0": 0.0,
				"1": 0.95,
				"2": 0.05,
			},
		}
		assert.Equal(t, "score", answer.Type)
		assert.Equal(t, 1.05, answer.Score)
		assert.Equal(t, "Calm", answer.Legend["0"])
		assert.Equal(t, 0.95, answer.Probabilities["1"])
	})

	t.Run("noul answer", func(t *testing.T) {
		answer := typesafeai.Answer{
			Type:       "noul",
			Noul:       0.95,
			Confidence: 0.99,
		}
		assert.Equal(t, "noul", answer.Type)
		assert.Equal(t, 0.95, answer.Noul)
	})
}
