package model

type TextToSpeechRequest struct {
	Model          string  `json:"model"           binding:"required"`
	Input          string  `json:"input"           binding:"required"`
	Voice          string  `json:"voice"           binding:"required"`
	ResponseFormat string  `json:"response_format"`
	Speed          float64 `json:"speed"`
}
