package initialize

import (
	"aurora/httpclient"
	"aurora/httpclient/bogdanfinn"
	"aurora/internal/duckgo"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// SpeechRequest is the OpenAI-compatible request for /v1/audio/speech
type SpeechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice,omitempty"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
}

// ICEServerResponse from Duck.ai
type ICEServerResponse struct {
	ICEServers []struct {
		URLs       []string `json:"urls"`
		Username   string   `json:"username,omitempty"`
		Credential string   `json:"credential,omitempty"`
	} `json:"iceServers"`
}

func (h *Handler) audioSpeech(c *gin.Context) {
	var req SpeechRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"code":    err.Error(),
		}})
		return
	}

	if req.Input == "" {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "input is required",
			"type":    "invalid_request_error",
			"param":   "input",
		}})
		return
	}

	if req.Model == "" {
		req.Model = "tts-1"
	}
	if req.Voice == "" {
		req.Voice = "alloy"
	}
	if req.ResponseFormat == "" {
		req.ResponseFormat = "mp3"
	}

	log.Printf("[TTS] Request: input=%d chars, voice=%s, format=%s", len(req.Input), req.Voice, req.ResponseFormat)

	audioData, err := h.generateSpeechWebRTC(req.Input, req.Voice)
	if err != nil {
		log.Printf("[TTS] Error: %v", err)
		c.JSON(500, gin.H{"error": gin.H{
			"message": fmt.Sprintf("TTS failed: %v", err),
			"type":    "internal_server_error",
		}})
		return
	}

	// Convert to requested format using FFmpeg
	if req.ResponseFormat != "ogg" && req.ResponseFormat != "opus" {
		converted, err := convertAudio(audioData, req.ResponseFormat)
		if err != nil {
			log.Printf("[TTS] FFmpeg conversion failed (%v), returning OGG", err)
			// Fallback to OGG
			c.Header("Content-Type", "audio/ogg")
			c.Data(200, "audio/ogg", audioData)
			return
		}
		audioData = converted
	}

	contentType := "audio/mpeg"
	switch req.ResponseFormat {
	case "mp3":
		contentType = "audio/mpeg"
	case "opus":
		contentType = "audio/ogg"
	case "wav":
		contentType = "audio/wav"
	case "pcm":
		contentType = "audio/pcm"
	case "flac":
		contentType = "audio/flac"
	case "aac":
		contentType = "audio/aac"
	case "ogg":
		contentType = "audio/ogg"
	}

	c.Header("Content-Type", contentType)
	c.Data(200, contentType, audioData)
}

func (h *Handler) generateSpeechWebRTC(text string, voice string) ([]byte, error) {
	proxyUrl := h.proxy.GetProxyIP()
	client := bogdanfinn.NewStdClient()
	if proxyUrl != "" {
		client.SetProxy(proxyUrl)
	}

	// Step 1: Get ICE servers
	iceServers, err := h.getICEServers(client, proxyUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get ICE servers: %w", err)
	}
	log.Printf("[TTS] Got %d ICE servers", len(iceServers))

	// Step 2: Create WebRTC peer connection
	rtcICEServers := make([]webrtc.ICEServer, 0, len(iceServers))
	for _, s := range iceServers {
		rtcICEServers = append(rtcICEServers, webrtc.ICEServer{
			URLs:       s.URLs,
			Username:   s.Username,
			Credential: s.Credential,
		})
	}

	config := webrtc.Configuration{
		ICEServers:         rtcICEServers,
		ICETransportPolicy: webrtc.ICETransportPolicyRelay,
	}

	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer peerConnection.Close()

	// Step 3: Collect received audio as RTP packets
	var audioMu sync.Mutex
	var rtpPackets []*rtp.Packet
	audioStarted := make(chan struct{})
	audioDone := make(chan struct{})

	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("[TTS] Received track: %s", track.Codec().MimeType)
		for {
			rtpPacket, _, err := track.ReadRTP()
			if err != nil {
				return
			}
			audioMu.Lock()
			rtpPackets = append(rtpPackets, rtpPacket)
			audioMu.Unlock()
		}
	})

	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("[TTS] ICE state: %s", state.String())
	})

	// Step 4: Create DataChannel for control messages
	dc, err := peerConnection.CreateDataChannel("duckai-voice-session", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create data channel: %w", err)
	}

	dc.OnOpen(func() {
		log.Printf("[TTS] DataChannel opened, sending text prompt")
		// Send text as a conversation item
		msg := map[string]interface{}{
			"type": "conversation.item.create",
			"item": map[string]interface{}{
				"type": "message",
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "input_text", "text": text},
				},
			},
		}
		data, _ := json.Marshal(msg)
		dc.SendText(string(data))

		// Request response
		respMsg := map[string]interface{}{
			"type": "response.create",
		}
		respData, _ := json.Marshal(respMsg)
		dc.SendText(string(respData))
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if msg.IsString {
			data := string(msg.Data)
			if len(data) > 200 {
				data = data[:200]
			}
			log.Printf("[TTS] DC: %s", data)

			// Detect audio start/stop events
			if bytes.Contains(msg.Data, []byte("output_audio_buffer.started")) {
				log.Printf("[TTS] Audio output started")
				select {
				case <-audioStarted:
				default:
					close(audioStarted)
				}
			}
			if bytes.Contains(msg.Data, []byte("output_audio_buffer.stopped")) {
				log.Printf("[TTS] Audio output stopped")
				select {
				case <-audioDone:
				default:
					close(audioDone)
				}
			}
			if bytes.Contains(msg.Data, []byte("response.done")) {
				log.Printf("[TTS] Response done")
				select {
				case <-audioDone:
				default:
					close(audioDone)
				}
			}
		}
	})

	// Step 5: Add a dummy audio track (we need one for the SDP offer)
	opusTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio", "duck2api-tts",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create track: %w", err)
	}
	if _, err = peerConnection.AddTrack(opusTrack); err != nil {
		return nil, fmt.Errorf("failed to add track: %w", err)
	}

	// Step 6: Create SDP offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create offer: %w", err)
	}

	if err = peerConnection.SetLocalDescription(offer); err != nil {
		return nil, fmt.Errorf("failed to set local description: %w", err)
	}

	// Step 7: Send SDP offer to Duck.ai session endpoint
	sdpAnswer, err := h.sendSDPOffer(client, proxyUrl, offer.SDP)
	if err != nil {
		return nil, fmt.Errorf("failed to send SDP offer: %w", err)
	}
	log.Printf("[TTS] Got SDP answer, length: %d", len(sdpAnswer))

	// Step 8: Set remote description
	err = peerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  sdpAnswer,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set remote description: %w", err)
	}

	// Step 9: Wait for AI to start sending audio, then wait for it to finish
	log.Printf("[TTS] Waiting for AI response...")

	// Wait for audio to start (max 15s)
	select {
	case <-audioStarted:
		log.Printf("[TTS] Audio started, collecting...")
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("TTS timeout - AI did not start responding")
	}

	// Wait for audio to finish (max 30s)
	select {
	case <-audioDone:
		log.Printf("[TTS] Audio finished")
	case <-time.After(30 * time.Second):
		log.Printf("[TTS] Audio timeout, using what we have")
	}

	// Small delay to collect trailing packets
	time.Sleep(500 * time.Millisecond)
	// Step 10: Wrap Opus frames in OGG container
	audioMu.Lock()
	oggData := wrapOpusInOGG(rtpPackets)
	audioMu.Unlock()

	log.Printf("[TTS] Generated %d bytes of OGG/Opus audio", len(oggData))
	return oggData, nil
}

func (h *Handler) getICEServers(client httpclient.AuroraHttpClient, proxyUrl string) ([]struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}, error) {
	token, err := duckgo.InitXVQD(client, proxyUrl)
	if err != nil {
		return nil, err
	}

	header := make(httpclient.AuroraHeaders)
	header.Set("accept", "*/*")
	header.Set("origin", "https://duck.ai")
	header.Set("referer", "https://duck.ai/")
	header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36")
	header.Set("x-vqd-hash-1", token)
	header.Set("x-ddg-journey-id", duckgo.RandomHex(16))
	header.Set("x-fe-signals", duckgo.CreateFESignals())
	if feVersion, err := duckgo.InitFEVersion(client, ""); err == nil && feVersion != "" {
		header.Set("x-fe-version", feVersion)
	}

	resp, err := client.Request(httpclient.GET, "https://duck.ai/duckchat/v1/ice-servers", header, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ICE servers returned %d: %s", resp.StatusCode, string(body))
	}

	var result ICEServerResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result.ICEServers, nil
}

func (h *Handler) sendSDPOffer(client httpclient.AuroraHttpClient, proxyUrl string, sdp string) (string, error) {
	// Retry loop for VQD challenge
	maxRetries := 3
	for i := 0; i <= maxRetries; i++ {
		token, err := duckgo.InitXVQD(client, proxyUrl)
		if err != nil {
			return "", err
		}

		header := make(httpclient.AuroraHeaders)
		header.Set("Content-Type", "application/sdp")
		header.Set("accept", "*/*")
		header.Set("origin", "https://duck.ai")
		header.Set("referer", "https://duck.ai/")
		header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36")
		header.Set("x-vqd-hash-1", token)
		header.Set("x-ddg-journey-id", duckgo.RandomHex(16))
		header.Set("x-fe-signals", duckgo.CreateFESignals())
		if feVersion, err := duckgo.InitFEVersion(client, ""); err == nil && feVersion != "" {
			header.Set("x-fe-version", feVersion)
		}

		resp, err := client.Request(httpclient.POST, "https://duck.ai/duckchat/v1/session", header, nil, bytes.NewReader([]byte(sdp)))
		if err != nil {
			return "", err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Handle 418 (challenge) - reset VQD and retry
		if resp.StatusCode == 418 || resp.StatusCode == 429 {
			log.Printf("[TTS] Session got %d, retrying (attempt %d/%d)", resp.StatusCode, i+1, maxRetries)
			duckgo.ResetXVQD()
			continue
		}

		if resp.StatusCode != 200 {
			return "", fmt.Errorf("session returned %d: %s", resp.StatusCode, string(body))
		}

		return string(body), nil
	}

	return "", fmt.Errorf("session failed after %d retries", maxRetries)
}

func wrapOpusInOGG(packets []*rtp.Packet) []byte {
	ogg := newOGGWriter(0x12345678)
	for _, pkt := range packets {
		if len(pkt.Payload) > 0 {
			ogg.writeOpusFrame(pkt.Payload)
		}
	}
	return ogg.bytes()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// convertAudio converts OGG/Opus to the target format using FFmpeg
func convertAudio(oggData []byte, format string) ([]byte, error) {
	args := []string{"-i", "pipe:0"} // input from stdin

	switch format {
	case "mp3":
		args = append(args, "-c:a", "libmp3lame", "-b:a", "128k", "-f", "mp3")
	case "wav":
		args = append(args, "-c:a", "pcm_s16le", "-ar", "24000", "-f", "wav")
	case "flac":
		args = append(args, "-c:a", "flac", "-f", "flac")
	case "aac":
		args = append(args, "-c:a", "aac", "-b:a", "128k", "-f", "adts")
	case "pcm":
		args = append(args, "-c:a", "pcm_s16le", "-ar", "24000", "-f", "s16le")
	default:
		args = append(args, "-c:a", "libmp3lame", "-b:a", "128k", "-f", "mp3")
	}

	args = append(args, "pipe:1") // output to stdout

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdin = bytes.NewReader(oggData)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg error: %v, stderr: %s", err, stderr.String())
	}

	if stdout.Len() == 0 {
		return nil, fmt.Errorf("ffmpeg produced no output, stderr: %s", stderr.String())
	}

	log.Printf("[TTS] Converted OGG → %s: %d bytes", format, stdout.Len())
	return stdout.Bytes(), nil
}
