package scraper

// Slot represents an available time slot
type Slot struct {
	Location  string `json:"location"`
	Category  string `json:"category"`
	Date      string `json:"date"`
	Available bool   `json:"available"`
}

// SlotDates extracts dates from slots
func SlotDates(slots []Slot) []string {
	dates := make([]string, len(slots))
	for i, slot := range slots {
		dates[i] = slot.Date
	}
	return dates
}
