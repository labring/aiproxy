package model

import (
	"github.com/labring/aiproxy/core/relay/adaptor"
)

type VideoGenerationJobRequest struct {
	Prompt    string `json:"prompt"`
	Model     string `json:"model"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	NVariants int    `json:"n_variants"`
	NSeconds  int    `json:"n_seconds"`
}

type VideoGenerationJobStatus = string

const (
	VideoGenerationJobStatusQueued     VideoGenerationJobStatus = "queued"
	VideoGenerationJobStatusProcessing VideoGenerationJobStatus = "processing"
	VideoGenerationJobStatusRunning    VideoGenerationJobStatus = "running"
	VideoGenerationJobStatusSucceeded  VideoGenerationJobStatus = "succeeded"
)

type VideoGenerationJob struct {
	Object       string                   `json:"object"`
	ID           string                   `json:"id"`
	Status       VideoGenerationJobStatus `json:"status"`
	CreatedAt    int64                    `json:"created_at"`
	FinishedAt   *int64                   `json:"finished_at"`
	ExpiresAt    *int64                   `json:"expires_at"`
	Generations  []VideoGenerations       `json:"generations"`
	Prompt       string                   `json:"prompt"`
	Model        string                   `json:"model"`
	NVariants    int                      `json:"n_variants"`
	NSeconds     int                      `json:"n_seconds"`
	Width        int                      `json:"width"`
	Height       int                      `json:"height"`
	FinishReason *string                  `json:"finish_reason"`
}

type VideoGenerations struct {
	Object    string `json:"object"`
	ID        string `json:"id"`
	JobID     string `json:"job_id"`
	CreatedAt int64  `json:"created_at"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Prompt    string `json:"prompt"`
	NSeconds  int    `json:"n_seconds"`
}

type OpenAIVideoError struct {
	Detail string `json:"detail"`
}

func NewOpenAIVideoError(statusCode int, err OpenAIVideoError) adaptor.Error {
	return adaptor.NewError(statusCode, err)
}

func WrapperOpenAIVideoError(err error, statusCode int) adaptor.Error {
	return WrapperOpenAIVideoErrorWithMessage(err.Error(), statusCode)
}

func WrapperOpenAIVideoErrorWithMessage(message string, statusCode int) adaptor.Error {
	return NewOpenAIVideoError(statusCode, OpenAIVideoError{
		Detail: message,
	})
}
