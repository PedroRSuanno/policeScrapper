# Police License Exchange Slot Scraper

This Go application monitors the Tokyo Metropolitan Police Department's driver's license reservation system for available slots at 府中試験場 for non-29 country applicants. When an available slot is found, it sends an email notification.

## Prerequisites

- Go 1.21 or later
- Chrome/Chromium browser (required for web scraping)
- Gmail account (for sending notifications)

## Setup

1. Clone the repository:

```bash
git clone <your-repo-url>
cd policeScrapper
```

2. Install dependencies:

```bash
go mod download
```

3. Set up environment variables:

```bash
export EMAIL_FROM="your-gmail@gmail.com"
export EMAIL_TO="your-notification@email.com"
export EMAIL_PASSWORD="your-app-specific-password"
```

Note: For Gmail, you'll need to use an App Password instead of your regular password. You can generate one at: https://myaccount.google.com/apppasswords

## Running the Scraper

```bash
go run main.go
```

The scraper will:

- Check for available slots every 10 minutes
- Look through the next 90 days of availability
- Send an email notification when slots are found
- Run continuously until stopped

## How it Works

1. The scraper navigates to the reservation page
2. Selects the 府中試験場 facility for non-29 country applicants
3. Checks each week for the ⭕️ mark indicating available slots
4. Clicks through multiple weeks using the 2 週間後 (2 weeks later) button
5. Sends an email notification when slots are found

## Troubleshooting

If you encounter any issues:

1. Make sure all environment variables are set correctly
2. Ensure Chrome/Chromium is installed
3. Check your Gmail settings allow less secure app access or use App Passwords
4. Verify your internet connection is stable
