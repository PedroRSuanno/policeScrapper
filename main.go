package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const (
	baseURL    = "https://www.keishicho-gto.metro.tokyo.lg.jp/keishicho-u/reserve/offerList_detail?tempSeq=363"
	lineAPIURL = "https://api.line.me/v2/bot/message/push"
)

// Target configurations
var (
	// Real target
	realLocation = "府中試験場"
	realCategory = "29の国･地域以外の方で、住民票のある方"

	// Test target (known to have available slots)
	testLocation = "江東試験場"
	testCategory = "29の国･地域の方"

	// Current target (will be set based on mode)
	targetLocation string
	targetCategory string

	// LINE configuration
	lineChannelToken string
	lineUserID       string

	// Flags
	noNotify bool // Flag to disable notifications
)

type Slot struct {
	Location  string `json:"location"`
	Category  string `json:"category"`
	Date      string `json:"date"`
	Available bool   `json:"available"`
}

func init() {
	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		log.Printf("Error creating logs directory: %v", err)
		return
	}

	// Set up logging with timestamp
	log.SetFlags(log.Ltime | log.LUTC)

	// Create daily log file
	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(logsDir, today+".log")

	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("Error opening log file: %v", err)
		return
	}

	// Create a multi-writer to write to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multiWriter)

	// Log startup message
	log.Printf("=== Starting new session ===")
}

// Helper function to rotate log file if needed
func rotateLogFile() {
	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join("logs", today+".log")

	// Check if we're already writing to today's log file
	if f, ok := log.Writer().(*os.File); ok {
		if f.Name() == logFile {
			return
		}
		f.Close()
	}

	// Open new log file
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("Error rotating log file: %v", err)
		return
	}

	// Create a multi-writer to write to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multiWriter)
	log.Printf("=== Log rotated to new file ===")
}

func setTargetMode(isTestMode bool) {
	if isTestMode {
		targetLocation = testLocation
		targetCategory = testCategory
		log.Printf("Running in TEST mode - Looking for slots at %s for %s", targetLocation, targetCategory)
	} else {
		targetLocation = realLocation
		targetCategory = realCategory
		log.Printf("Running in REAL mode - Looking for slots at %s for %s", targetLocation, targetCategory)
	}
}

func checkAvailability(ctx context.Context) error {
	startTime := time.Now()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ Panic: %v", r)
		}
	}()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.WindowSize(1920, 1080),
		chromedp.NoSandbox,
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-site-isolation-trials", true),
		chromedp.Flag("disable-features", "SameSiteByDefaultCookies,CookiesWithoutSameSiteMustBeSecure"),
		chromedp.Headless,
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	taskCtx, cancel := chromedp.NewContext(
		allocCtx,
		chromedp.WithLogf(func(format string, args ...interface{}) {
			// Only log critical browser errors, ignore routine messages and cookie errors
			msg := fmt.Sprintf(format, args...)
			if (strings.Contains(msg, "error") || strings.Contains(msg, "failed")) &&
				!strings.Contains(msg, "cookiePart") &&
				!strings.Contains(msg, "unmarshal event") {
				log.Printf("🌐 %s", msg)
			}
		}),
	)
	defer cancel()

	taskCtx, cancel = context.WithTimeout(taskCtx, 5*time.Minute)
	defer cancel()

	// Add retry logic for initial page load
	maxRetries := 3
	var err error
	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			time.Sleep(time.Duration(retry) * time.Second)
		}

		err = chromedp.Run(taskCtx,
			chromedp.Navigate(baseURL),
			chromedp.WaitVisible(`table.time--table`, chromedp.ByQuery),
		)
		if err == nil {
			break
		}
		if retry < maxRetries-1 {
			log.Printf("⚠️ Retry %d/%d", retry+1, maxRetries)
		}
	}
	if err != nil {
		return fmt.Errorf("❌ Failed to load page: %v", err)
	}

	// Keep track of how many pages we've checked
	pagesChecked := 0
	maxPages := 12 // Limit to checking 24 weeks ahead (12 clicks of "2週後")
	slotsFound := false

	for pagesChecked < maxPages {
		// Wait for the table and SVG elements to load
		if err := chromedp.Run(taskCtx,
			chromedp.WaitVisible(`table.time--table`, chromedp.ByQuery),
			chromedp.WaitVisible(`svg[aria-label="予約可能"], svg[aria-label="空き無"], svg[aria-label="時間外"]`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond),
		); err != nil {
			return fmt.Errorf("❌ Failed to find elements: %v", err)
		}

		// Try to find available slots using JavaScript
		var availableSlots []Slot
		slotScript := fmt.Sprintf(`
			function findAvailableSlots() {
				const slots = [];
				const table = document.querySelector('table.time--table');
				if (!table) return slots;

				// Get the date header row first and parse all dates
				const headerRow = table.querySelector('tr#height_headday');
				if (!headerRow) {
					console.log("Could not find header row");
					return slots;
				}

				// Create a map of column index to date
				const dateMap = new Map();
				Array.from(headerRow.cells).forEach((cell, index) => {
					if (cell.textContent) {
						// Handle multi-line date format (e.g., "07/30\n(Wed)")
						const fullText = cell.textContent.trim();
						const dateMatch = fullText.match(/(\d{2}\/\d{2})/);
						const dayMatch = fullText.match(/\((.*?)\)/);
						
						if (dateMatch) {
							const dateText = dateMatch[1];
							const dayText = dayMatch ? dayMatch[1] : '';
							console.log("Column " + index + ": Date = " + dateText + ", Day = " + dayText);
							dateMap.set(index, dateText);
						}
					}
				});

				// First get all rows with available slots
				const rows = table.querySelectorAll('tr');
				rows.forEach((row, rowIndex) => {
					// Skip header rows
					if (row.id === 'height_head' || row.id === 'height_headday') {
						console.log("Skipping header row " + rowIndex);
						return;
					}

					// Get location and category first
					const locationCell = row.querySelector('th a');
					const location = locationCell ? locationCell.textContent.trim() : '';
					if (location !== %q) {
						console.log("Skipping non-target location: " + location);
						return;
					}

					const categoryCell = row.querySelector('th.main_color');
					const category = categoryCell ? categoryCell.textContent.trim() : '';
					if (category !== %q) {
						console.log("Skipping non-target category: " + category);
						return;
					}

					console.log("Processing row " + rowIndex + " for " + location + " - " + category);

					// Get all cells in this row
					const cells = Array.from(row.cells);
					cells.forEach((cell, cellIndex) => {
						// Skip if this is not a selectable cell
						if (!cell.classList.contains('tdSelect') || !cell.classList.contains('enable')) {
							console.log("Column " + cellIndex + ": Not a selectable cell");
							return;
						}

						// Verify the cell has the correct SVG
						const availableSVG = cell.querySelector('svg[aria-label="予約可能"]');
						if (!availableSVG) {
							console.log("Column " + cellIndex + ": No available SVG");
							return;
						}

						// Get the date from our map
						const dateText = dateMap.get(cellIndex);
						if (!dateText) {
							console.log("Column " + cellIndex + ": No date found in map");
							return;
						}

						console.log("Found potential slot at column " + cellIndex + ":", {
							location,
							category,
							date: dateText,
							cellClasses: cell.className,
							svgLabel: availableSVG.getAttribute('aria-label')
						});

						// Parse the date (format: "MM/DD")
						const dateParts = dateText.match(/(\d{2})\/(\d{2})/);
						if (!dateParts) {
							console.log("Invalid date format for column " + cellIndex + ": " + dateText);
							return;
						}

						// Get current date in Japan timezone
						const now = new Date(new Date().toLocaleString("en-US", {timeZone: "Asia/Tokyo"}));
						const month = parseInt(dateParts[1], 10);
						const day = parseInt(dateParts[2], 10);
						
						// Create date object for the slot (assume current year, adjust if needed)
						let slotDate = new Date(now.getFullYear(), month - 1, day);
						
						// If the slot month is less than current month, it's in next year
						if (month < now.getMonth() + 1) {
							slotDate.setFullYear(now.getFullYear() + 1);
						}

						// Skip if date is in the past
						if (slotDate < now) {
							console.log("Column " + cellIndex + ": Skipping past date: " + dateText);
							return;
						}

						// Skip if it's a closed day (check for 休 or × mark)
						const closedMark = cell.querySelector('svg[aria-label="休"]') || cell.querySelector('svg[aria-label="×"]');
						if (closedMark) {
							console.log("Column " + cellIndex + ": Skipping closed day: " + dateText);
							return;
						}

						// Log the full cell content for debugging
						console.log("Column " + cellIndex + " HTML:", cell.innerHTML);

						slots.push({
							location: location,
							category: category,
							date: dateText,
							available: true
						});
					});
				});

				console.log("Final slots found:", slots);
				return slots;
			}
			findAvailableSlots();
		`, targetLocation, targetCategory)

		if err := chromedp.Run(taskCtx, chromedp.Evaluate(slotScript, &availableSlots)); err != nil {
			log.Printf("❌ Error checking slots: %v", err)
		}

		if len(availableSlots) > 0 {
			duration := time.Since(startTime)
			log.Printf("🎯 Found %d slots: %s (checked %d pages in %.1fs)",
				len(availableSlots),
				strings.Join(slotDates(availableSlots), ", "),
				pagesChecked+1,
				duration.Seconds())
			notifyAvailableSlots(availableSlots)
			slotsFound = true
			break
		}

		// Try to click the "2週後" button if it's enabled
		var nextButtonEnabled bool
		if err := chromedp.Run(taskCtx,
			chromedp.Evaluate(`!document.querySelector('input[value="2週後＞"]').disabled`, &nextButtonEnabled),
		); err != nil {
			return fmt.Errorf("❌ Failed to check button: %v", err)
		}

		if !nextButtonEnabled {
			break
		}

		if err := chromedp.Run(taskCtx,
			chromedp.Click(`input[value="2週後＞"]`),
			chromedp.WaitVisible(`table.time--table`, chromedp.ByQuery),
		); err != nil {
			return fmt.Errorf("❌ Failed to click button: %v", err)
		}

		pagesChecked++
	}

	duration := time.Since(startTime)
	if !slotsFound {
		log.Printf("✓ No slots found (checked %d pages in %.1fs)", pagesChecked+1, duration.Seconds())
	}
	return nil
}

// Helper function to extract dates from slots
func slotDates(slots []Slot) []string {
	dates := make([]string, len(slots))
	for i, slot := range slots {
		dates[i] = slot.Date
	}
	return dates
}

type LineMessage struct {
	To       string        `json:"to"`
	Messages []LineContent `json:"messages"`
}

type LineContent struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	AltText  string      `json:"altText,omitempty"`
	Contents interface{} `json:"contents,omitempty"`
}

type FlexContainer struct {
	Type   string     `json:"type"`
	Body   FlexBody   `json:"body"`
	Styles FlexStyles `json:"styles,omitempty"`
}

type FlexBody struct {
	Type     string        `json:"type"`
	Layout   string        `json:"layout"`
	Contents []interface{} `json:"contents"`
}

type FlexStyles struct {
	Body FlexStyle `json:"body"`
}

type FlexStyle struct {
	BackgroundColor string `json:"backgroundColor"`
}

type FlexText struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Weight string `json:"weight,omitempty"`
	Size   string `json:"size,omitempty"`
	Color  string `json:"color,omitempty"`
	Wrap   bool   `json:"wrap,omitempty"`
}

type FlexSeparator struct {
	Type  string `json:"type"`
	Color string `json:"color"`
}

func createFlexMessage(slots []Slot) LineContent {
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
					"uri":   baseURL,
				},
				"color": "#1DB446",
			},
		},
		"margin": "md",
	}

	// Add button to boxes
	boxes = append(boxes, button)

	// Create the full flex message
	flexMessage := LineContent{
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

	return flexMessage
}

func sendLineMessage(message string) error {
	if lineChannelToken == "" || lineUserID == "" {
		return fmt.Errorf("LINE configuration is incomplete")
	}

	payload := LineMessage{
		To: lineUserID,
		Messages: []LineContent{
			{
				Type: "text",
				Text: message,
			},
		},
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
	req.Header.Set("Authorization", "Bearer "+lineChannelToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("message failed with status: %d", resp.StatusCode)
	}

	return nil
}

func notifyAvailableSlots(slots []Slot) {
	if len(slots) == 0 {
		return
	}

	// Skip LINE notification if noNotify is true
	if noNotify {
		log.Println("📱 Notification skipped (--no-notify)")
		return
	}

	// Create and send the Flex Message
	flexMessage := createFlexMessage(slots)
	payload := LineMessage{
		To:       lineUserID,
		Messages: []LineContent{flexMessage},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("❌ Failed to marshal message: %v", err)
		return
	}

	req, err := http.NewRequest("POST", lineAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("❌ Failed to create request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+lineChannelToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Failed to send message: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ Message failed with status: %d", resp.StatusCode)
		return
	}

	log.Printf("📱 Notification sent")
}

// Test the notification system with sample data
func testNotificationSystem() error {
	log.Println("🧪 Testing notification system with sample data...")
	testSlots := []Slot{
		{
			Location:  targetLocation,
			Category:  targetCategory,
			Date:      "08/01 (Fri)",
			Available: true,
		},
		{
			Location:  targetLocation,
			Category:  targetCategory,
			Date:      "08/02 (Sat)",
			Available: true,
		},
	}

	notifyAvailableSlots(testSlots)
	return nil
}

func main() {
	// Load configuration
	config := loadConfig()
	lineChannelToken = config.LineChannelToken
	lineUserID = config.LineUserID

	if lineChannelToken == "" || lineUserID == "" {
		log.Fatal("LINE configuration is incomplete. Please check config.json")
	}

	// Parse command line arguments
	isTestMode := false
	testNotification := false
	noNotify = false

	for _, arg := range os.Args[1:] {
		switch arg {
		case "test":
			isTestMode = true
		case "notify-test":
			testNotification = true
		case "--no-notify":
			noNotify = true
			log.Println("Notifications disabled (--no-notify flag is set)")
		}
	}

	// Set target based on mode
	setTargetMode(isTestMode)

	// If only testing notification system
	if testNotification {
		if err := testNotificationSystem(); err != nil {
			log.Printf("Notification test failed: %v", err)
		}
		return
	}

	log.Println("Scraper started - press Ctrl+C to stop")

	// Create a context that can be cancelled
	ctx := context.Background()

	// Run the first check immediately
	rotateLogFile() // Ensure we're using today's log file
	if err := checkAvailability(ctx); err != nil {
		log.Printf("Error during check: %v", err)
	}
}
