package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"policeScrapper/internal/browser"
	"policeScrapper/pkg/config"
	"policeScrapper/pkg/line"
)

func init() {
	// Create logs directory if it doesn't exist
	logsDir := "logs"
	// Fix G301: Reduce directory permissions to 0750
	if err := os.MkdirAll(logsDir, 0750); err != nil {
		log.Printf("Error creating logs directory: %v", err)
		return
	}

	// Set up logging with timestamp
	log.SetFlags(log.Ltime | log.LUTC)

	// Create daily log file
	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(logsDir, today+".log")

	// Validate log file path
	if !isValidLogPath(logFile) {
		log.Printf("Invalid log file path: %s", logFile)
		return
	}

	// Fix G302, G304: Reduce file permissions to 0600 and validate path
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600) // #nosec G304 - path is validated by isValidLogPath
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

// isValidLogPath validates the log file path
func isValidLogPath(path string) bool {
	// Get absolute path of logs directory
	logsDir, err := filepath.Abs("logs")
	if err != nil {
		return false
	}

	// Get absolute path of the target file
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Check if the path is within logs directory
	return strings.HasPrefix(absPath, logsDir)
}

// Helper function to rotate log file if needed
func rotateLogFile() {
	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join("logs", today+".log")

	// Validate log file path
	if !isValidLogPath(logFile) {
		log.Printf("Invalid log file path: %s", logFile)
		return
	}

	// Check if we're already writing to today's log file
	if f, ok := log.Writer().(*os.File); ok {
		if f.Name() == logFile {
			return
		}
		// Fix G104: Handle close error
		if err := f.Close(); err != nil {
			log.Printf("Error closing log file: %v", err)
		}
	}

	// Fix G302, G304: Reduce file permissions and validate path
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600) // #nosec G304 - path is validated by isValidLogPath
	if err != nil {
		log.Printf("Error rotating log file: %v", err)
		return
	}

	// Create a multi-writer to write to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multiWriter)
	log.Printf("=== Log rotated to new file ===")
}

func main() {
	// Parse command line arguments
	isTestMode := false
	testNotification := false
	noNotify := false

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

	// Validate LINE credentials
	lineToken := os.Getenv("LINE_CHANNEL_TOKEN")
	lineUserID := os.Getenv("LINE_USER_ID")
	if lineToken == "" || lineUserID == "" {
		log.Printf("⚠️ LINE credentials not set properly:")
		if lineToken == "" {
			log.Printf("  - LINE_CHANNEL_TOKEN is missing")
		}
		if lineUserID == "" {
			log.Printf("  - LINE_USER_ID is missing")
		}
		log.Printf("Notifications will be disabled")
		noNotify = true
	} else {
		log.Printf("✓ LINE credentials found (token length: %d, user ID length: %d)",
			len(lineToken), len(lineUserID))
	}

	// Get target based on mode
	target := config.GetTarget(isTestMode)
	if isTestMode {
		log.Printf("Running in TEST mode - Looking for slots at %s for %s", target.Location, target.Category)
	} else {
		log.Printf("Running in REAL mode - Looking for slots at %s for %s", target.Location, target.Category)
	}

	// Create LINE client
	lineClient := line.NewClient(lineToken, lineUserID, noNotify)

	// If only testing notification system
	if testNotification {
		if err := lineClient.TestNotification(target.Location, target.Category); err != nil {
			log.Printf("Notification test failed: %v", err)
		}
		return
	}

	log.Println("Scraper started - press Ctrl+C to stop")

	// Create browser instance
	b := browser.New(target, 12) // Check up to 12 pages (24 weeks)
	defer b.Close()

	// Send initial test notification
	if !noNotify {
		if err := lineClient.TestNotification(target.Location, target.Category); err != nil {
			log.Printf("⚠️ Initial test notification failed: %v", err)
			log.Printf("⚠️ Notifications will be disabled")
			noNotify = true
			lineClient = line.NewClient(lineToken, lineUserID, true)
		} else {
			log.Println("✓ Initial test notification sent successfully")
		}
	}

	// Main loop
	consecutiveErrors := 0
	for {
		slots, err := b.CheckAvailability()
		if err != nil {
			consecutiveErrors++
			log.Printf("Error during check: %v", err)
			// Exponential backoff for consecutive errors
			backoffDuration := time.Duration(consecutiveErrors*consecutiveErrors) * time.Second
			if backoffDuration > 5*time.Minute {
				backoffDuration = 5 * time.Minute // Cap at 5 minutes
			}
			log.Printf("Waiting %d seconds before retry (consecutive errors: %d)", int(backoffDuration.Seconds()), consecutiveErrors)
			time.Sleep(backoffDuration)
			continue
		}
		// Reset error counter on successful check
		consecutiveErrors = 0

		if len(slots) > 0 {
			if err := lineClient.NotifyAvailableSlots(slots); err != nil {
				log.Printf("Error sending notification: %v", err)
			}
		}

		// Wait 15 minutes before next check
		nextCheck := time.Now().Add(15 * time.Minute)
		log.Printf("✓ Check complete. Next check in 15 minutes at %s",
			nextCheck.Format("15:04:05"))
		time.Sleep(15 * time.Minute)

		// Only rotate log file at the start of each day
		if time.Now().Format("2006-01-02") != time.Now().Add(-15*time.Minute).Format("2006-01-02") {
			rotateLogFile()
		}
	}
}
