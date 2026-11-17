package common_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	common "github.com/labring/aiproxy/core/common"
)

func benchBody(size int) string {
	img := strings.Repeat("A", size)
	return `{"model":"gpt-4o","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,` + img + `"}},{"type":"text","text":"hi"}]}],"stream":true}`
}

// lazy Get + Set model + marshal
func convertGet(body []byte) ([]byte, error) {
	node, err := common.GetJSONNodeNoCopy(body)
	if err != nil {
		return nil, err
	}

	if _, err := node.Set("model", ast.NewString("mapped")); err != nil {
		return nil, err
	}

	return node.MarshalJSON()
}

// full sonic.Unmarshal + Set + marshal
func convertUnmarshal(body []byte) ([]byte, error) {
	var node ast.Node
	if err := sonic.Unmarshal(body, &node); err != nil {
		return nil, err
	}

	if _, err := node.Set("model", ast.NewString("mapped")); err != nil {
		return nil, err
	}

	return node.MarshalJSON()
}

// sonic.Valid + lazy Get + Set + marshal
func convertValidGet(body []byte) ([]byte, error) {
	if !sonic.Valid(body) {
		return nil, errors.New("invalid json")
	}

	return convertGet(body)
}

func BenchmarkConvertSmall(b *testing.B) {
	body := []byte(benchBody(100))
	b.Run("get", func(b *testing.B) { benchConvert(b, body, convertGet) })
	b.Run("unmarshal", func(b *testing.B) { benchConvert(b, body, convertUnmarshal) })
	b.Run("valid+get", func(b *testing.B) { benchConvert(b, body, convertValidGet) })
}

func BenchmarkConvertMedium(b *testing.B) {
	body := []byte(benchBody(50_000))
	b.Run("get", func(b *testing.B) { benchConvert(b, body, convertGet) })
	b.Run("unmarshal", func(b *testing.B) { benchConvert(b, body, convertUnmarshal) })
	b.Run("valid+get", func(b *testing.B) { benchConvert(b, body, convertValidGet) })
}

func BenchmarkConvertLarge(b *testing.B) {
	body := []byte(benchBody(5_000_000))
	b.Run("get", func(b *testing.B) { benchConvert(b, body, convertGet) })
	b.Run("unmarshal", func(b *testing.B) { benchConvert(b, body, convertUnmarshal) })
	b.Run("valid+get", func(b *testing.B) { benchConvert(b, body, convertValidGet) })
}

func benchConvert(b *testing.B, body []byte, f func([]byte) ([]byte, error)) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := f(body); err != nil {
			b.Fatal(err)
		}
	}
}
