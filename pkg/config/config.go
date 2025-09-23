package config

// Target configurations
const (
	// Real target
	RealLocation = "府中試験場"
	RealCategory = "29の国･地域以外の方で、住民票のある方"

	// Test target (known to have available slots)
	TestLocation = "江東試験場"
	TestCategory = "29の国･地域の方"

	// Base URL for the reservation system
	BaseURL = "https://www.keishicho-gto.metro.tokyo.lg.jp/keishicho-u/reserve/offerList_detail?tempSeq=461"
)

// Config holds the application configuration
type Config struct {
	LineChannelToken string
	LineUserID       string
	IsTestMode       bool
	NoNotify         bool
	MaxPages         int // Maximum number of pages to check (24 weeks)
}

// Target represents a location and category to check
type Target struct {
	Location string
	Category string
}

// GetTarget returns the appropriate target based on test mode
func GetTarget(isTestMode bool) Target {
	if isTestMode {
		return Target{
			Location: TestLocation,
			Category: TestCategory,
		}
	}
	return Target{
		Location: RealLocation,
		Category: RealCategory,
	}
}
