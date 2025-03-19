package xunfei

import (
	"strings"
)

// https://console.xfyun.cn/services/cbm
// https://www.xfyun.cn/doc/spark/HTTP%E8%B0%83%E7%94%A8%E6%96%87%E6%A1%A3.html

func getXunfeiDomain(modelName string) string {
	_, s, _ := strings.Cut(modelName, "-")
	switch strings.ToLower(s) {
	case "lite":
		return "lite"
	case "pro":
		return "generalv3"
	case "pro-128k":
		return "pro-128k"
	case "max":
		return "generalv3.5"
	case "max-32k":
		return "max-32k"
	case "4.0-ultra":
		return "4.0Ultra"
	default:
		return modelName
	}
}
