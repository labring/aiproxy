module github.com/labring/aiproxy/mcp-servers

go 1.26

replace github.com/labring/aiproxy/core => ../core

replace github.com/labring/aiproxy/openapi-mcp => ../openapi-mcp

replace github.com/oasdiff/yaml => github.com/oasdiff/yaml v0.0.0-20250309154309-f31be36b4037

replace github.com/oasdiff/yaml3 => github.com/oasdiff/yaml3 v0.0.0-20250309153720-d2182401db90

require (
	github.com/JohannesKaufmann/html-to-markdown v1.6.0
	github.com/PuerkitoBio/goquery v1.11.0
	github.com/bytedance/sonic v1.15.0
	github.com/go-shiori/go-readability v0.0.0-20251205110129-5db1dc9836f0
	github.com/mark3labs/mcp-go v0.44.0
	github.com/temoto/robotstxt v1.1.2
	golang.org/x/text v0.34.0
)

require (
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/araddon/dateparse v0.0.0-20210429162001-6b43995a97de // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic/loader v0.5.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/go-shiori/dom v0.0.0-20230515143342-73569d674e1c // indirect
	github.com/gogs/chardet v0.0.0-20211120154057-b7413eaefb8f // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/invopop/jsonschema v0.13.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/mailru/easyjson v0.9.1 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/arch v0.24.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
