package duckgo

import (
	duckgotypes "aurora/typings/duckgo"
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// ImageResult holds the extracted image data from the SSE stream
type ImageResult struct {
	Text   string
	Images []duckgotypes.ImagePart
}

// ReadImageResponse reads the SSE response and extracts both text and image parts
func ReadImageResponse(response *http.Response) ImageResult {
	reader := bufio.NewReader(response.Body)
	var textBuilder strings.Builder
	var images []duckgotypes.ImagePart

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return ImageResult{}
		}
		if len(line) < 6 {
			continue
		}
		line = line[6:]
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[DONE]") || strings.HasPrefix(line, "[PING]") || strings.HasPrefix(line, "[CHAT_TITLE") {
			continue
		}

		var apiResp duckgotypes.ApiResponse
		err = json.Unmarshal([]byte(line), &apiResp)
		if err != nil || apiResp.Action != "success" {
			continue
		}

		if apiResp.Message != "" {
			textBuilder.WriteString(apiResp.Message)
		}

		// Extract image from parts (legacy format)
		for _, part := range apiResp.Parts {
			if part.Type == "generated-image" || part.Type == "image" {
				images = append(images, part)
			}
		}

		// Extract image from data field (new format: ui-component with GenerateImage)
		if apiResp.ToolName == "GenerateImage" && apiResp.Data != nil {
			if imgData := apiResp.GetImageData(); imgData != nil && imgData.B64Image != "" {
				images = append(images, duckgotypes.ImagePart{
					Type:   "generated-image",
					Result: imgData.B64Image,
					Format: imgData.Format,
					Width:  imgData.Width,
					Height: imgData.Height,
				})
			}
		}
	}

	return ImageResult{
		Text:   textBuilder.String(),
		Images: images,
	}
}
