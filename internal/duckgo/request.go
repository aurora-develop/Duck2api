package duckgo

import (
	"aurora/httpclient"
	duckgotypes "aurora/typings/duckgo"
	officialtypes "aurora/typings/official"
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	Token     *XqdgToken
	FEVersion *XqdgToken
	UA        = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
)

type XqdgToken struct {
	Token    string     `json:"token"`
	M        sync.Mutex `json:"-"`
	ExpireAt time.Time  `json:"expire"`
}

func InitXVQD(client httpclient.AuroraHttpClient, proxyUrl string) (string, error) {
	if Token == nil {
		Token = &XqdgToken{
			Token: "",
			M:     sync.Mutex{},
		}
	}
	Token.M.Lock()
	defer Token.M.Unlock()
	if Token.Token == "" {
		status, err := postStatus(client, proxyUrl)
		if err != nil {
			return "", err
		}
		defer status.Body.Close()
		vqdHash := status.Header.Get("x-vqd-hash-1")
		if vqdHash == "" {
			return "", errors.New("no x-vqd-hash-1 token")
		}
		token, err := GenerateVQDHash(vqdHash)
		if err != nil {
			return "", err
		}
		Token.Token = token
	}

	return Token.Token, nil
}

func postStatus(client httpclient.AuroraHttpClient, proxyUrl string) (*http.Response, error) {
	if proxyUrl != "" {
		client.SetProxy(proxyUrl)
	}
	header := createHeader()
	header.Set("accept", "*/*")
	header.Set("x-vqd-accept", "1")
	response, err := client.Request(httpclient.GET, "https://duck.ai/duckchat/v1/status", header, nil, nil)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func POSTconversation(client httpclient.AuroraHttpClient, request duckgotypes.ApiRequest, token string, proxyUrl string) (*http.Response, error) {
	if proxyUrl != "" {
		client.SetProxy(proxyUrl)
	}

	maxRetries := 3
	var response *http.Response
	var err error

	for i := 0; i <= maxRetries; i++ {
		response, err = postConversationOnce(client, request, token)
		if err != nil {
			return nil, err
		}

		if response.StatusCode != http.StatusTeapot && response.StatusCode != http.StatusTooManyRequests {
			return response, nil
		}

		response.Body.Close()
		ResetXVQD()
		token, err = InitXVQD(client, proxyUrl)
		if err != nil {
			return nil, err
		}
	}

	return response, nil
}

func Handle_request_error(c *gin.Context, response *http.Response) bool {
	if response.StatusCode != 200 {
		// Try read response body as JSON
		var error_response map[string]interface{}
		err := json.NewDecoder(response.Body).Decode(&error_response)
		if err != nil {
			// Read response body
			body, _ := io.ReadAll(response.Body)
			c.JSON(response.StatusCode, gin.H{"error": gin.H{
				"message": "Unknown error",
				"type":    "internal_server_error",
				"param":   nil,
				"code":    "500",
				"details": string(body),
			}})
			return true
		}
		c.JSON(response.StatusCode, gin.H{"error": gin.H{
			"message": error_response["detail"],
			"type":    response.Status,
			"param":   nil,
			"code":    "error",
		}})
		return true
	}
	return false
}

func createHeader() httpclient.AuroraHeaders {
	header := make(httpclient.AuroraHeaders)
	header.Set("accept-language", "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7")
	header.Set("content-type", "application/json")
	header.Set("origin", "https://duck.ai")
	header.Set("referer", "https://duck.ai/")
	header.Set("sec-ch-ua", `"Google Chrome";v="149", "Chromium";v="149", "Not)A;Brand";v="24"`)
	header.Set("sec-ch-ua-mobile", "?0")
	header.Set("sec-ch-ua-platform", `"Windows"`)
	header.Set("sec-fetch-dest", "empty")
	header.Set("sec-fetch-mode", "cors")
	header.Set("sec-fetch-site", "same-origin")
	header.Set("user-agent", UA)
	return header
}

func postConversationOnce(client httpclient.AuroraHttpClient, request duckgotypes.ApiRequest, token string) (*http.Response, error) {
	bodyJSON, err := json.Marshal(request)
	if err != nil {
		return &http.Response{}, err
	}
	header := createHeader()
	header.Set("accept", "text/event-stream")
	header.Set("priority", "u=1, i")
	header.Set("x-ddg-journey-id", RandomHex(16))
	header.Set("x-fe-signals", CreateFESignals())
	if feVersion, err := InitFEVersion(client, ""); err == nil && feVersion != "" {
		header.Set("x-fe-version", feVersion)
	}
	header.Set("x-vqd-hash-1", token)
	return client.Request(httpclient.POST, "https://duck.ai/duckchat/v1/chat", header, nil, bytes.NewBuffer(bodyJSON))
}

func InitFEVersion(client httpclient.AuroraHttpClient, proxyUrl string) (string, error) {
	if FEVersion == nil {
		FEVersion = &XqdgToken{
			Token: "",
			M:     sync.Mutex{},
		}
	}
	FEVersion.M.Lock()
	defer FEVersion.M.Unlock()
	if FEVersion.Token != "" && FEVersion.ExpireAt.After(time.Now()) {
		return FEVersion.Token, nil
	}

	if proxyUrl != "" {
		client.SetProxy(proxyUrl)
	}
	header := createHeader()
	header.Set("accept", "text/html")
	response, err := client.Request(httpclient.GET, "https://duck.ai/", header, nil, nil)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	versionTagMatch := regexp.MustCompile(`data-version-tag="([^"]+)"`).FindSubmatch(body)
	versionShaMatch := regexp.MustCompile(`data-version-sha="([^"]+)"`).FindSubmatch(body)
	if len(versionTagMatch) < 2 || len(versionShaMatch) < 2 {
		return "", errors.New("duck.ai version metadata not found")
	}

	FEVersion.Token = fmt.Sprintf("%s-%s", versionTagMatch[1], versionShaMatch[1])
	FEVersion.ExpireAt = time.Now().Add(30 * time.Minute)
	return FEVersion.Token, nil
}

func CreateFESignals() string {
	now := time.Now().UnixMilli()
	// Reproduce the event log the duck.ai frontend records between page load
	// request): onboarding_impression -> action -> onboarding_finish -> startNewChat_free.
	// 模拟真实用户行为: 页面加载 -> 用户思考输入 -> 完成输入 -> 点击发送
	// 时间间隔调整为更接近真实用户的行为模式
	impression := 50 + randInt63n(100)              // 50-150ms (页面加载完成)
	action := impression + 5000 + randInt63n(25000) // 5-30秒 (用户阅读并思考)
	finish := action + 1000 + randInt63n(9000)      // 1-10秒 (输入问题)
	startChat := finish + 10 + randInt63n(90)       // 10-100ms (点击发送按钮)
	end := startChat + randInt63n(10)
	payload := map[string]interface{}{
		"start": now - end,
		"events": []map[string]interface{}{
			{"name": "onboarding_impression", "delta": impression},
			{"name": "action", "delta": action, "trusted": true},
			{"name": "onboarding_finish", "delta": finish},
			{"name": "startNewChat_free", "delta": startChat},
		},
		"end": end,
	}
	body, _ := json.Marshal(payload)
	return base64.StdEncoding.EncodeToString(body)
}

// randInt63n returns a uniform random non-negative int64 in [0, n).
func randInt63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return 0
	}
	var v int64
	for _, b := range buf {
		v = v<<8 | int64(b)
	}
	if v < 0 {
		v = -v
	}
	return v % n
}

func RandomHex(byteLength int) string {
	buffer := make([]byte, byteLength)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

func ResetXVQD() {
	if Token == nil {
		return
	}
	Token.M.Lock()
	defer Token.M.Unlock()
	Token.Token = ""
}

func ReadResponseError(response *http.Response) error {
	var errorResponse map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&errorResponse); err == nil {
		if detail, ok := errorResponse["detail"]; ok {
			return fmt.Errorf("%s: %v", response.Status, detail)
		}
		return fmt.Errorf("%s: %v", response.Status, errorResponse)
	}

	body, _ := io.ReadAll(response.Body)
	if len(body) == 0 {
		return fmt.Errorf("%s", response.Status)
	}
	return fmt.Errorf("%s: %s", response.Status, string(body))
}

func ReadResponseText(response *http.Response) string {
	reader := bufio.NewReader(response.Body)
	var previousText strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return ""
		}
		if len(line) < 6 {
			continue
		}
		line = line[6:]
		if strings.HasPrefix(line, "[DONE]") {
			continue
		}

		var originalResponse duckgotypes.ApiResponse
		err = json.Unmarshal([]byte(line), &originalResponse)
		if err != nil || originalResponse.Action != "success" {
			continue
		}
		previousText.WriteString(originalResponse.Message)
	}
	return previousText.String()
}

func Handler(c *gin.Context, response *http.Response, oldRequest duckgotypes.ApiRequest, stream bool) string {
	reader := bufio.NewReader(response.Body)
	if stream {
		// Response content type is text/event-stream
		c.Header("Content-Type", "text/event-stream")
	} else {
		// Response content type is application/json
		c.Header("Content-Type", "application/json")
	}

	var previousText strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return ""
		}
		if len(line) < 6 {
			continue
		}
		line = line[6:]
		if !strings.HasPrefix(line, "[DONE]") {
			var originalResponse duckgotypes.ApiResponse
			err = json.Unmarshal([]byte(line), &originalResponse)
			if err != nil {
				continue
			}
			if originalResponse.Action != "success" {
				c.JSON(500, gin.H{"error": "Error"})
				return ""
			}
			responseString := ""
			if originalResponse.Message != "" {
				previousText.WriteString(originalResponse.Message)
				translatedResponse := officialtypes.NewChatCompletionChunkWithModel(originalResponse.Message, originalResponse.Model)
				responseString = "data: " + translatedResponse.String() + "\n\n"
			}

			if responseString == "" {
				continue
			}

			if stream {
				_, err = c.Writer.WriteString(responseString)
				if err != nil {
					return ""
				}
				c.Writer.Flush()
			}
		} else {
			if stream {
				final_line := officialtypes.StopChunkWithModel("stop", oldRequest.Model)
				c.Writer.WriteString("data: " + final_line.String() + "\n\n")
			}
		}
	}
	return previousText.String()
}
