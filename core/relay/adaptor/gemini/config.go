package gemini

type Config struct {
	Safety                      string `json:"safety"`
	DisableAutoImageURLToBase64 bool   `json:"disable_auto_image_url_to_base64"`
}
