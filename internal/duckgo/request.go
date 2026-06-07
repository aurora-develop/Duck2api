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
	UA        = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
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
	if Token.Token == "" || Token.ExpireAt.Before(time.Now()) {
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
		Token.ExpireAt = time.Now().Add(time.Minute * 3)
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
	response, err := postConversationOnce(client, request, token)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusTeapot && response.StatusCode != http.StatusTooManyRequests {
		return response, nil
	}

	response.Body.Close()
	resetXVQD()
	nextToken, err := InitXVQD(client, proxyUrl)
	if err != nil {
		return nil, err
	}
	return postConversationOnce(client, request, nextToken)
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
	header.Set("sec-ch-ua", `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`)
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
	header.Set("x-ddg-journey-id", randomHex(16))
	header.Set("x-fe-signals", createFESignals())
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

func createFESignals() string {
	start := time.Now().UnixMilli()
	payload := map[string]interface{}{
		"start": start,
		"events": []map[string]interface{}{
			{
				"name":  "startNewChat_free",
				"delta": 56,
			},
		},
		"end": 246,
	}
	body, _ := json.Marshal(payload)
	return base64.StdEncoding.EncodeToString(body)
}

func randomHex(byteLength int) string {
	buffer := make([]byte, byteLength)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

func resetXVQD() {
	if Token == nil {
		return
	}
	Token.M.Lock()
	defer Token.M.Unlock()
	Token.Token = ""
	Token.ExpireAt = time.Time{}
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
