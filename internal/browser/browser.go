package browser

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"errors"

	"policeScrapper/pkg/config"
	"policeScrapper/pkg/scraper"

	"github.com/chromedp/chromedp"
)

// Browser handles the Chrome automation
type Browser struct {
	allocCtx    context.Context
	cancelAlloc context.CancelFunc
	target      config.Target
	maxPages    int
}

// New creates a new browser instance
func New(target config.Target, maxPages int) *Browser {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.WindowSize(1920, 1080),
		chromedp.NoSandbox,
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-site-isolation-trials", true),
		chromedp.Flag("disable-features", "SameSiteByDefaultCookies,CookiesWithoutSameSiteMustBeSecure"),
		chromedp.Headless,
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)

	return &Browser{
		allocCtx:    allocCtx,
		cancelAlloc: cancelAlloc,
		target:      target,
		maxPages:    maxPages,
	}
}

// Close closes the browser allocator
func (b *Browser) Close() {
	b.cancelAlloc()
}

// CheckAvailability checks for available slots
func (b *Browser) CheckAvailability() ([]scraper.Slot, error) {
	startTime := time.Now()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ Panic: %v", r)
		}
	}()

	// Create a new context for this check
	ctx, cancel := chromedp.NewContext(
		b.allocCtx,
		chromedp.WithLogf(func(format string, args ...interface{}) {
			msg := fmt.Sprintf(format, args...)
			if (strings.Contains(msg, "error") || strings.Contains(msg, "failed")) &&
				!strings.Contains(msg, "cookiePart") &&
				!strings.Contains(msg, "unmarshal event") {
				log.Printf("🌐 %s", msg)
			}
		}),
	)
	defer cancel()

	// Add timeout for this check
	ctx, cancel = context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	// Add retry logic for initial page load with exponential backoff
	maxRetries := 3
	var err error
	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			backoffDuration := time.Duration(retry*retry) * time.Second
			log.Printf("⚠️ Retry %d/%d (waiting %d seconds)", retry+1, maxRetries, retry*retry)
			time.Sleep(backoffDuration)
		}

		err = chromedp.Run(ctx,
			chromedp.Navigate(config.BaseURL),
			chromedp.WaitVisible(`table.time--table`, chromedp.ByQuery),
		)
		if err == nil {
			break
		}
	}
	if err != nil {
		
		if errors.Is(err, context.DeadlineExceeded) {
		        log.Println("Request timed out!")
		    }

		return nil, fmt.Errorf("❌ Failed to load page after %d retries: %v", maxRetries, err)
	}

	// Keep track of how many pages we've checked
	pagesChecked := 0

	for pagesChecked < b.maxPages {
		// Wait for the table and SVG elements to load
		if err := chromedp.Run(ctx,
			chromedp.WaitVisible(`table.time--table`, chromedp.ByQuery),
			chromedp.WaitVisible(`svg[aria-label="予約可能"], svg[aria-label="空き無"], svg[aria-label="時間外"]`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond),
		); err != nil {
			return nil, fmt.Errorf("❌ Failed to find elements: %v", err)
		}

		// Try to find available slots using JavaScript
		var availableSlots []scraper.Slot
		slotScript := b.createSlotScript()

		if err := chromedp.Run(ctx, chromedp.Evaluate(slotScript, &availableSlots)); err != nil {
			log.Printf("❌ Error checking slots: %v", err)
		}

		if len(availableSlots) > 0 {
			duration := time.Since(startTime)
			log.Printf("🎯 Found %d slots: %s (checked %d pages in %.1fs)",
				len(availableSlots),
				strings.Join(scraper.SlotDates(availableSlots), ", "),
				pagesChecked+1,
				duration.Seconds())
			return availableSlots, nil // Return immediately when slots are found
		}

		// Try to click the "2週後" button if it's enabled
		var nextButtonEnabled bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`!document.querySelector('input[value="2週後＞"]').disabled`, &nextButtonEnabled),
		); err != nil {
			return nil, fmt.Errorf("❌ Failed to check button: %v", err)
		}

		if !nextButtonEnabled {
			break
		}

		if err := chromedp.Run(ctx,
			chromedp.Click(`input[value="2週後＞"]`),
			chromedp.WaitVisible(`table.time--table`, chromedp.ByQuery),
		); err != nil {
			return nil, fmt.Errorf("❌ Failed to click button: %v", err)
		}

		pagesChecked++
	}

	duration := time.Since(startTime)
	log.Printf("✓ No slots found (checked %d pages in %.1fs)", pagesChecked+1, duration.Seconds())
	return nil, nil
}

// createSlotScript creates the JavaScript to find available slots
func (b *Browser) createSlotScript() string {
	return fmt.Sprintf(`
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

					slots.push({
						Location: location,
						Category: category,
						Date: dateText
					});
				});
			});

			return slots;
		}
		findAvailableSlots();
	`, b.target.Location, b.target.Category)
}
