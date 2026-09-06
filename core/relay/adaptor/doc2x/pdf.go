package doc2x

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	commonimage "github.com/labring/aiproxy/core/common/image"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
	log "github.com/sirupsen/logrus"
)

func ConvertParsePdfRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	err := common.ParseMultipartFormWithLimit(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	file, _, err := req.FormFile("file")
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	responseFormat := req.FormValue("response_format")
	meta.Set("response_format", responseFormat)

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type": {"multipart/form-data"},
		},
		Body: file,
	}, nil
}

type ParsePdfResponse struct {
	Code string               `json:"code"`
	Data ParsePdfResponseData `json:"data"`
	Msg  string               `json:"msg"`
}

type ParsePdfResponseData struct {
	UID string `json:"uid"`
}

const (
	StatusResponseDataStatusSuccess    = "success"
	StatusResponseDataStatusReady      = "ready"
	StatusResponseDataStatusProcessing = "processing"
	StatusResponseDataStatusFailed     = "failed"
	statusPollMaxAttempts              = 600
)

var statusPollInterval = time.Second

func isSuccessfulResponseCode(code string) bool {
	return code == "success" || code == "ok"
}

func HandleParsePdfResponse(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	var response ParsePdfResponse

	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"decode response failed: "+err.Error(),
			"decode_response_failed",
			http.StatusBadRequest,
		)
	}

	if !isSuccessfulResponseCode(response.Code) {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"parse pdf failed: "+response.Msg,
			"parse_pdf_failed",
			http.StatusBadRequest,
		)
	}

	if response.Data.UID == "" {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			"parse pdf failed: uid is empty",
			"parse_pdf_failed",
			http.StatusBadGateway,
		)
	}

	ctx := c.Request.Context()
	for attempt := range statusPollMaxAttempts {
		status, err := GetStatus(ctx, meta, response.Data.UID)
		if err != nil {
			return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
				"get status failed: "+err.Error(),
				"get_status_failed",
				http.StatusInternalServerError,
			)
		}

		switch status.Status {
		case StatusResponseDataStatusSuccess:
			if status.Result == nil {
				return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
					"parse pdf failed: result is empty",
					"parse_pdf_failed",
					http.StatusBadGateway,
				)
			}

			return handleParsePdfResponse(meta, c, status.Result)
		case StatusResponseDataStatusReady, StatusResponseDataStatusProcessing:
			if attempt == statusPollMaxAttempts-1 {
				break
			}

			select {
			case <-ctx.Done():
				return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
					"get status canceled: "+ctx.Err().Error(),
					"get_status_canceled",
					http.StatusRequestTimeout,
				)
			case <-time.After(statusPollInterval):
			}
		case StatusResponseDataStatusFailed:
			return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
				"parse pdf failed: "+status.Detail,
				"parse_pdf_failed",
				http.StatusBadRequest,
			)
		default:
			return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
				"get status failed: unknown status "+status.Status,
				"get_status_failed",
				http.StatusBadGateway,
			)
		}
	}

	return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
		"get status failed: process timeout",
		"get_status_timeout",
		http.StatusGatewayTimeout,
	)
}

// Start of Selection
var (
	tableRegex      = regexp.MustCompile(`<table>[\s\S]*?</table>`)
	rowRegex        = regexp.MustCompile(`<tr>(.*?)</tr>`)
	cellRegex       = regexp.MustCompile(`<td[^>]*/>|<td[^>]*>(.*?)</td>`)
	whitespaceRegex = regexp.MustCompile(`\n\s*`)
	tdCleanRegex    = regexp.MustCompile(`<td.*?>|</td>`)
	colspanRegex    = regexp.MustCompile(`colspan="(\d+)"`)
	rowspanRegex    = regexp.MustCompile(`rowspan="(\d+)"`)

	htmlImageRegex = regexp.MustCompile(`<img\s+src="([^"]+)"(?:\s*\?[^>]*)?(?:\s*\/>|>)`)
	imageRegex     = regexp.MustCompile(`!\[(.*?)\]\((http[^)]+)\)`)

	inlineMathDelimiterRegex = regexp.MustCompile(`\\[\(\)]`)
	blockMathDelimiterRegex  = regexp.MustCompile(`\\[\[\]]`)
	mathTagRegex             = regexp.MustCompile(`\$(.+?)\s+\\tag\{(.+?)\}\$`)
	textSubscriptRegex       = regexp.MustCompile(`\\text\{([^}]*?)(\b\w+)_(\w+\b)([^}]*?)\}`)
	simpleFootnoteRefRegex   = regexp.MustCompile(`\$\s*\{\}\^\{(\d+)\}\s*\$`)

	mediaCommentRegex    = regexp.MustCompile(`<!-- Media -->`)
	footnoteCommentRegex = regexp.MustCompile(`<!-- Footnote -->`)
	meanlessCommentRegex = regexp.MustCompile(`(?s)<!-- Meanless:.*?-->`)
	figureTextRegex      = regexp.MustCompile(`(?s)<!-- figureText:\s*(.*?)\s*-->`)
	figureLineBreakRegex = regexp.MustCompile(`(?i)<br\s*/?>`)
	footnoteSectionRegex = regexp.MustCompile(
		`(?s)(?:^|\n)\s*---\s*<!-- Footnote -->\s*(.*?)\s*<!-- Footnote -->\s*---\s*(?:\n|$)`,
	)
	footnoteBlockRegex    = regexp.MustCompile(`(?s)<!-- Footnote -->\s*(.*?)\s*<!-- Footnote -->`)
	plainFootnoteNumRegex = regexp.MustCompile(`^(\d+)\s+`)
	mathFootnoteNumRegex  = regexp.MustCompile(`^\\\(\s*\{\}\^\{(\d+)\}\s*\\\)\s*`)
	excessBlankLinesRegex = regexp.MustCompile(`\n[ \t]*\n(?:[ \t]*\n)+`)
)

// CleanMarkdown applies the text-only normalization required by Doc2X's
// Markdown output. Image downloads and HTML table conversion remain separate
// operations because they need request context and network access respectively.
func CleanMarkdown(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = footnoteSectionRegex.ReplaceAllStringFunc(content, func(match string) string {
		parts := footnoteSectionRegex.FindStringSubmatch(match)
		return formatFootnote(parts[1])
	})
	content = footnoteBlockRegex.ReplaceAllStringFunc(content, func(match string) string {
		parts := footnoteBlockRegex.FindStringSubmatch(match)
		return formatFootnote(parts[1])
	})
	content = figureTextRegex.ReplaceAllStringFunc(content, func(match string) string {
		parts := figureTextRegex.FindStringSubmatch(match)
		figureText := figureLineBreakRegex.ReplaceAllString(parts[1], "\n")
		return "\n\n**图内文字**\n\n" + strings.TrimSpace(figureText) + "\n\n"
	})
	content = inlineMathDelimiterRegex.ReplaceAllString(content, "$")
	content = blockMathDelimiterRegex.ReplaceAllStringFunc(content, func(string) string {
		return "$$"
	})
	content = mathTagRegex.ReplaceAllStringFunc(content, func(match string) string {
		parts := mathTagRegex.FindStringSubmatch(match)
		return "$$" + parts[1] + " \\qquad \\qquad (" + parts[2] + ")$$"
	})
	content = textSubscriptRegex.ReplaceAllStringFunc(content, func(match string) string {
		parts := textSubscriptRegex.FindStringSubmatch(match)
		return "\\text{" + parts[1] + parts[2] + "\\_" + parts[3] + parts[4] + "}"
	})
	content = simpleFootnoteRefRegex.ReplaceAllString(content, "[^$1]")
	content = mediaCommentRegex.ReplaceAllString(content, "")
	content = footnoteCommentRegex.ReplaceAllString(content, "")
	content = meanlessCommentRegex.ReplaceAllString(content, "")
	content = excessBlankLinesRegex.ReplaceAllString(content, "\n\n")

	return strings.TrimSpace(content)
}

func formatFootnote(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	var number string
	if parts := plainFootnoteNumRegex.FindStringSubmatch(body); len(parts) > 1 {
		number = parts[1]
		body = plainFootnoteNumRegex.ReplaceAllString(body, "")
	} else if parts := mathFootnoteNumRegex.FindStringSubmatch(body); len(parts) > 1 {
		number = parts[1]
		body = mathFootnoteNumRegex.ReplaceAllString(body, "")
	}

	body = strings.TrimSpace(body)
	if number == "" {
		return "\n\n" + body + "\n\n"
	}

	lines := strings.Split(body, "\n")
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			lines[i] = "    " + lines[i]
		}
	}

	return "\n\n[^" + number + "]: " + strings.Join(lines, "\n") + "\n\n"
}

func HTMLTable2Md(content string) string {
	return tableRegex.ReplaceAllStringFunc(content, func(htmlTable string) string {
		cleanHTML := whitespaceRegex.ReplaceAllString(htmlTable, "")

		rows := rowRegex.FindAllString(cleanHTML, -1)
		if len(rows) == 0 {
			return htmlTable
		}

		var tableData [][]string

		maxColumns := 0
		for rowIndex, row := range rows {
			for len(tableData) <= rowIndex {
				tableData = append(tableData, []string{})
			}

			colIndex := 0

			cells := cellRegex.FindAllString(row, -1)
			if len(cells) > maxColumns {
				maxColumns = len(cells)
			}

			for _, cell := range cells {
				colspan := 1
				if matches := colspanRegex.FindStringSubmatch(cell); len(matches) > 1 {
					colspan, _ = strconv.Atoi(matches[1])
				}

				rowspan := 1
				if matches := rowspanRegex.FindStringSubmatch(cell); len(matches) > 1 {
					rowspan, _ = strconv.Atoi(matches[1])
				}

				content := strings.TrimSpace(tdCleanRegex.ReplaceAllString(cell, ""))
				for i := range rowspan {
					for j := range colspan {
						for len(tableData) <= rowIndex+i {
							tableData = append(tableData, []string{})
						}

						for len(tableData[rowIndex+i]) <= colIndex+j {
							tableData[rowIndex+i] = append(tableData[rowIndex+i], "")
						}

						if i == 0 && j == 0 {
							tableData[rowIndex+i][colIndex+j] = content
						} else {
							tableData[rowIndex+i][colIndex+j] = "^^"
						}
					}
				}

				colIndex += colspan
			}
		}

		for i := range tableData {
			for len(tableData[i]) < maxColumns {
				tableData[i] = append(tableData[i], " ")
			}
		}

		var chunks []string

		headerCells := make([]string, maxColumns)
		for i := range maxColumns {
			if i < len(tableData[0]) {
				headerCells[i] = tableData[0][i]
			} else {
				headerCells[i] = " "
			}
		}

		chunks = append(chunks, fmt.Sprintf("| %s |", strings.Join(headerCells, " | ")))

		separatorCells := make([]string, maxColumns)
		for i := range maxColumns {
			separatorCells[i] = "---"
		}

		chunks = append(chunks, fmt.Sprintf("| %s |", strings.Join(separatorCells, " | ")))
		for _, row := range tableData[1:] {
			chunks = append(chunks, fmt.Sprintf("| %s |", strings.Join(row, " | ")))
		}

		return strings.Join(chunks, "\n")
	})
}

func HTMLImage2Md(content string) string {
	return htmlImageRegex.ReplaceAllString(content, "![img]($1)")
}

func InlineMdImage(ctx context.Context, m *meta.Meta, text string) string {
	text = HTMLImage2Md(text)

	matches := imageRegex.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text
	}

	var (
		resultText strings.Builder
		wg         sync.WaitGroup
		mutex      sync.Mutex
		slots      = make(chan struct{}, 8)
	)

	type imageInfo struct {
		startPos    int
		endPos      int
		altText     string
		url         string
		replacement string
	}

	imageInfos := make([]imageInfo, len(matches))

	for i, match := range matches {
		altTextStart, altTextEnd := match[2], match[3]
		urlStart, urlEnd := match[4], match[5]

		imageInfos[i] = imageInfo{
			startPos: match[0],
			endPos:   match[1],
			altText:  text[altTextStart:altTextEnd],
			url:      text[urlStart:urlEnd],
		}
	}

	for i := range imageInfos {
		wg.Add(1)

		go func(index int) {
			defer wg.Done()

			info := &imageInfos[index]
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				mutex.Lock()
				info.replacement = text[info.startPos:info.endPos]
				mutex.Unlock()
				return
			}

			replacement, err := imageURL2MdBase64(ctx, m, info.url, info.altText)
			if err != nil {
				log.Printf("failed to process image %s: %v", info.url, err)
				// when the image is not found, keep the original link
				mutex.Lock()

				info.replacement = text[info.startPos:info.endPos]

				mutex.Unlock()

				return
			}

			mutex.Lock()

			info.replacement = replacement

			mutex.Unlock()
		}(i)
	}

	wg.Wait()

	lastPos := 0
	for _, info := range imageInfos {
		resultText.WriteString(text[lastPos:info.startPos])
		resultText.WriteString(info.replacement)
		lastPos = info.endPos
	}

	resultText.WriteString(text[lastPos:])

	return resultText.String()
}

func imageURL2MdBase64(ctx context.Context, m *meta.Meta, url, altText string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	var (
		resp        *http.Response
		downloadErr error
	)

	retries := 0
	maxRetries := 3

	client, err := utils.LoadHTTPClientWithTLSConfigE(
		m.RequestTimeout,
		m.Channel.ProxyURL,
		m.Channel.SkipTLSVerify,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create http client: %w", err)
	}

	for retries <= maxRetries {
		resp, downloadErr = client.Do(req)
		if downloadErr != nil {
			return "", fmt.Errorf("failed to download image: %w", downloadErr)
		}

		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()

			if retries == maxRetries {
				return "", fmt.Errorf(
					"failed to download image, status code: %d after %d retries",
					resp.StatusCode,
					retries,
				)
			}

			retries++

			time.Sleep(1 * time.Second)

			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return "", fmt.Errorf("failed to download image, status code: %d", resp.StatusCode)
		}

		break
	}

	defer resp.Body.Close()

	data, err := common.GetResponseBodyLimit(resp, commonimage.MaxImageSize)
	if err != nil {
		return "", fmt.Errorf("failed to read image data: %w", err)
	}

	mime := commonimage.TrimImageContentType(resp.Header.Get("Content-Type"))
	if !commonimage.IsImageURL(mime) {
		detectedMime := http.DetectContentType(data)
		if !commonimage.IsImageURL(detectedMime) {
			return "", fmt.Errorf("downloaded content is not an image: %s", detectedMime)
		}

		mime = detectedMime
	}

	if mime == "" {
		mime = inferMimeType(url)
	}

	base64Data := base64.StdEncoding.EncodeToString(data)

	return fmt.Sprintf("![%s](data:%s;base64,%s)", altText, mime, base64Data), nil
}

func inferMimeType(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return "image/jpeg"
	}

	lowerURL := strings.ToLower(p.Path)
	switch {
	case strings.HasSuffix(lowerURL, ".png"):
		return "image/png"
	case strings.HasSuffix(lowerURL, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lowerURL, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lowerURL, ".svg"):
		return "image/svg+xml"
	default:
		return "image/jpeg"
	}
}

func handleConvertPdfToMd(ctx context.Context, m *meta.Meta, str string) string {
	result := CleanMarkdown(str)
	result = InlineMdImage(ctx, m, result)
	result = HTMLTable2Md(result)

	return result
}

func joinMarkdownPages(pages []string) string {
	return strings.Join(pages, "\n\n")
}

func handleParsePdfResponse(
	meta *meta.Meta,
	c *gin.Context,
	response *StatusResponseDataResult,
) (adaptor.DoResponseResult, adaptor.Error) {
	mds := make([]string, 0, len(response.Pages))

	totalLength := 0
	for _, page := range response.Pages {
		mds = append(mds, page.MD)
		totalLength += len(page.MD)
	}

	pages := int64(len(response.Pages))

	switch meta.GetString("response_format") {
	case "list":
		for i, md := range mds {
			result := handleConvertPdfToMd(c.Request.Context(), meta, md)
			mds[i] = result
		}

		c.JSON(http.StatusOK, relaymodel.ParsePdfListResponse{
			Markdowns: mds,
		})
	default:
		builder := strings.Builder{}
		builder.Grow(totalLength + max(0, len(mds)-1)*2)
		builder.WriteString(joinMarkdownPages(mds))

		result := handleConvertPdfToMd(c.Request.Context(), meta, builder.String())
		c.JSON(http.StatusOK, relaymodel.ParsePdfResponse{
			Pages:    pages,
			Markdown: result,
		})
	}

	return adaptor.DoResponseResult{Usage: model.Usage{
		InputTokens: model.ZeroNullInt64(pages),
		TotalTokens: model.ZeroNullInt64(pages),
	}}, nil
}

type StatusResponse struct {
	Code string              `json:"code"`
	Msg  string              `json:"msg"`
	Data *StatusResponseData `json:"data"`
}

type StatusResponseData struct {
	Progress int                       `json:"progress"`
	Status   string                    `json:"status"`
	Detail   string                    `json:"detail"`
	Result   *StatusResponseDataResult `json:"result"`
}

type StatusResponseDataResult struct {
	Version string                         `json:"version"`
	Pages   []StatusResponseDataResultPage `json:"pages"`
}

type StatusResponseDataResultPage struct {
	URL        string `json:"url"`
	PageIdx    int    `json:"page_idx"`
	PageWidth  int    `json:"page_width"`
	PageHeight int    `json:"page_height"`
	MD         string `json:"md"`
}

func GetStatus(ctx context.Context, meta *meta.Meta, uid string) (*StatusResponseData, error) {
	url := fmt.Sprintf("%s/api/v2/parse/status?uid=%s", meta.Channel.BaseURL, uid)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+meta.Channel.Key)

	resp, err := utils.DoRequestWithMeta(req, meta)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response StatusResponse

	err = sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	if !isSuccessfulResponseCode(response.Code) {
		return nil, errors.New("get status failed: " + response.Msg)
	}

	if response.Data == nil {
		return nil, errors.New("get status failed: response data is empty")
	}

	return response.Data, nil
}
