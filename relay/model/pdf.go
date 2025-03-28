package model

type ParsePdfResponse struct {
	Pages    int64  `json:"pages"`
	Markdown string `json:"markdown"`
}

type ParsePdfListResponse struct {
	Markdowns []string `json:"markdowns"`
}
