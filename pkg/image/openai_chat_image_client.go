package image

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/drama-generator/backend/pkg/ai"
	"github.com/drama-generator/backend/pkg/utils"
)

type OpenAIChatImageClient struct {
	client *ai.OpenAIClient
	model  string
}

func NewOpenAIChatImageClient(baseURL, apiKey, model, endpoint string) *OpenAIChatImageClient {
	return &OpenAIChatImageClient{
		client: ai.NewOpenAIClient(baseURL, apiKey, model, endpoint),
		model:  model,
	}
}

func (c *OpenAIChatImageClient) GenerateImage(prompt string, opts ...ImageOption) (*ImageResult, error) {
	options := &ImageOptions{
		Size:    "1024x1024",
		Quality: "standard",
	}
	for _, opt := range opts {
		opt(options)
	}

	userPrompt := prompt
	if options.NegativePrompt != "" {
		userPrompt += "\n\nNegative prompt: " + options.NegativePrompt
	}
	if options.Size != "" {
		userPrompt += "\n\nImage size: " + options.Size
	}

	systemPrompt := "Generate the image using the provided model. Respond with JSON only. " +
		"Return one of these fields: image_url, url, or data[0].url. No markdown."

	text, err := c.client.GenerateText(
		userPrompt,
		systemPrompt,
		ai.WithTemperature(0.2),
		ai.WithMaxTokens(1200),
	)
	if err != nil {
		return nil, err
	}

	imageURL := extractURLFromResponse(text, true)
	if imageURL == "" {
		return nil, fmt.Errorf("no image url found in response: %s", truncateString(text, 300))
	}

	return &ImageResult{
		Status:    "completed",
		ImageURL:  imageURL,
		Completed: true,
	}, nil
}

func (c *OpenAIChatImageClient) GetTaskStatus(taskID string) (*ImageResult, error) {
	return nil, fmt.Errorf("not supported for chat-based image client")
}

func extractURLFromResponse(text string, allowDataURI bool) string {
	cleaned := strings.TrimSpace(text)

	mdRe := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	if match := mdRe.FindStringSubmatch(cleaned); len(match) > 1 {
		url := strings.TrimSpace(match[1])
		url = strings.TrimRight(url, ".,)")
		if allowDataURI || strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
			return url
		}
	}

	jsonText := utils.ExtractJSONFromText(cleaned)

	var parsed struct {
		ImageURL string `json:"image_url"`
		URL      string `json:"url"`
		Data     []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if jsonText != "" {
		if err := json.Unmarshal([]byte(jsonText), &parsed); err == nil {
			if parsed.ImageURL != "" {
				return parsed.ImageURL
			}
			if parsed.URL != "" {
				return parsed.URL
			}
			if len(parsed.Data) > 0 {
				if parsed.Data[0].URL != "" {
					return parsed.Data[0].URL
				}
				if allowDataURI && parsed.Data[0].B64JSON != "" {
					return "data:image/png;base64," + parsed.Data[0].B64JSON
				}
			}
		}
	}

	if allowDataURI && strings.HasPrefix(cleaned, "data:image/") {
		return cleaned
	}

	re := regexp.MustCompile(`https?://\S+`)
	match := re.FindString(cleaned)
	if match != "" {
		return strings.TrimRight(match, ".,)")
	}

	return ""
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
