package video

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/drama-generator/backend/pkg/ai"
	"github.com/drama-generator/backend/pkg/utils"
)

type OpenAIChatVideoClient struct {
	client *ai.OpenAIClient
	model  string
}

func NewOpenAIChatVideoClient(baseURL, apiKey, model, endpoint string) *OpenAIChatVideoClient {
	return &OpenAIChatVideoClient{
		client: ai.NewOpenAIClient(baseURL, apiKey, model, endpoint),
		model:  model,
	}
}

func (c *OpenAIChatVideoClient) GenerateVideo(imageURL, prompt string, opts ...VideoOption) (*VideoResult, error) {
	options := &VideoOptions{
		Duration:    4,
		AspectRatio: "16:9",
	}
	for _, opt := range opts {
		opt(options)
	}

	userPrompt := prompt
	if imageURL != "" {
		userPrompt += "\n\nReference image: " + imageURL
	}
	if options.Duration > 0 {
		userPrompt += fmt.Sprintf("\n\nDuration: %d seconds", options.Duration)
	}
	if options.AspectRatio != "" {
		userPrompt += "\n\nAspect ratio: " + options.AspectRatio
	}

	systemPrompt := "Generate a video using the provided model. Respond with JSON only. " +
		"Return one of these fields: video_url, url, or data[0].url. No markdown."

	text, err := c.client.GenerateText(
		userPrompt,
		systemPrompt,
		ai.WithTemperature(0.2),
		ai.WithMaxTokens(1200),
	)
	if err != nil {
		return nil, err
	}

	videoURL := extractVideoURL(text)
	if videoURL == "" {
		return nil, fmt.Errorf("no video url found in response: %s", truncateVideoString(text, 300))
	}

	return &VideoResult{
		Status:    "completed",
		VideoURL:  videoURL,
		Completed: true,
	}, nil
}

func (c *OpenAIChatVideoClient) GetTaskStatus(taskID string) (*VideoResult, error) {
	return nil, fmt.Errorf("not supported for chat-based video client")
}

func extractVideoURL(text string) string {
	cleaned := strings.TrimSpace(text)
	jsonText := utils.ExtractJSONFromText(cleaned)

	var parsed struct {
		VideoURL string `json:"video_url"`
		URL      string `json:"url"`
		Data     []struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if jsonText != "" {
		if err := json.Unmarshal([]byte(jsonText), &parsed); err == nil {
			if parsed.VideoURL != "" {
				return parsed.VideoURL
			}
			if parsed.URL != "" {
				return parsed.URL
			}
			if len(parsed.Data) > 0 && parsed.Data[0].URL != "" {
				return parsed.Data[0].URL
			}
		}
	}

	re := regexp.MustCompile(`https?://\S+`)
	match := re.FindString(cleaned)
	if match != "" {
		return strings.TrimRight(match, ".,)")
	}

	return ""
}

func truncateVideoString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
