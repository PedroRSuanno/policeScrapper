package line

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"policeScrapper/pkg/scraper"
)

const lineAPIURL = "https://api.line.me/v2/bot/message/push"

// Client handles LINE notifications
type Client struct {
	channelToken string
	userID       string
	noNotify     bool
}

// NewClient creates a new LINE client
func NewClient(channelToken, userID string, noNotify bool) *Client {
	return &Client{
		channelToken: channelToken,
		userID:       userID,
		noNotify:     noNotify,
	}
}

// Message represents a LINE message
type Message struct {
	To       string        `json:"to"`
	Messages []LineContent `json:"messages"`
}

// LineContent represents the content of a LINE message
type LineContent struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	AltText  string      `json:"altText,omitempty"`
	Contents interface{} `json:"contents,omitempty"`
}

// NotifyAvailableSlots sends a notification about available slots
func (c *Client) NotifyAvailableSlots(slots []scraper.Slot) error {
	if len(slots) == 0 {
		return nil
	}

	if c.noNotify {
		log.Println("📱 Notification skipped (--no-notify)")
		return nil
	}

	flexMessage := c.createFlexMessage(slots)
	payload := Message{
		To:       c.userID,
		Messages: []LineContent{flexMessage},
	}

	return c.sendMessage(payload)
}

func (c *Client) sendMessage(payload Message) error {
	if c.channelToken == "" || c.userID == "" {
		return fmt.Errorf("LINE configuration is incomplete")
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	req, err := http.NewRequest("POST", lineAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.channelToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("message failed with status: %d", resp.StatusCode)
	}

	log.Printf("📱 Notification sent")
	return nil
}

func (c *Client) createFlexMessage(slots []scraper.Slot) LineContent {
	// Create boxes for each slot
	boxes := make([]interface{}, len(slots))
	for i, slot := range slots {
		boxes[i] = map[string]interface{}{
			"type":   "box",
			"layout": "vertical",
			"contents": []interface{}{
				map[string]interface{}{
					"type":   "box",
					"layout": "vertical",
					"contents": []interface{}{
						map[string]interface{}{
							"type":   "text",
							"text":   "📍 " + slot.Location,
							"size":   "md",
							"weight": "bold",
							"color":  "#1DB446",
						},
						map[string]interface{}{
							"type":   "text",
							"text":   "👥 " + slot.Category,
							"size":   "sm",
							"color":  "#666666",
							"margin": "sm",
						},
						map[string]interface{}{
							"type":   "text",
							"text":   "📅 " + slot.Date,
							"size":   "sm",
							"color":  "#666666",
							"margin": "sm",
						},
					},
					"spacing": "sm",
				},
				map[string]interface{}{
					"type":   "separator",
					"margin": "md",
				},
			},
		}
	}

	// Add a button at the bottom
	button := map[string]interface{}{
		"type":   "box",
		"layout": "vertical",
		"contents": []interface{}{
			map[string]interface{}{
				"type":  "button",
				"style": "primary",
				"action": map[string]interface{}{
					"type":  "uri",
					"label": "予約する",
					"uri":   "https://www.keishicho-gto.metro.tokyo.lg.jp/keishicho-u/reserve/offerList_detail?tempSeq=363",
				},
				"color": "#1DB446",
			},
		},
		"margin": "md",
	}

	boxes = append(boxes, button)

	return LineContent{
		Type:    "flex",
		AltText: fmt.Sprintf("空き枠が見つかりました！(%d件)", len(slots)),
		Contents: map[string]interface{}{
			"type": "bubble",
			"header": map[string]interface{}{
				"type":   "box",
				"layout": "vertical",
				"contents": []interface{}{
					map[string]interface{}{
						"type":   "text",
						"text":   "🎉 空き枠発見！",
						"size":   "xl",
						"weight": "bold",
						"color":  "#1DB446",
					},
				},
			},
			"body": map[string]interface{}{
				"type":     "box",
				"layout":   "vertical",
				"contents": boxes,
				"spacing":  "md",
			},
		},
	}
}
