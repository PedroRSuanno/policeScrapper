#!/bin/bash

# LINE Messaging API configuration
# To get these values:
# 1. LINE_CHANNEL_TOKEN: Channel access token from LINE Developers Console (✓ Done)
# 2. LINE_USER_ID: Add @151sehhv as a friend and send it a message to get your user ID
export LINE_CHANNEL_TOKEN="REMOVED_LINE_TOKEN="

# We still need your LINE User ID - add the bot (@151sehhv) and send it a message
export LINE_USER_ID="U00dc4ae5fa6425a1c75241ee037fad97"

# Available modes:
# ./run_scraper.sh              - Normal mode (府中試験場)
# ./run_scraper.sh test         - Test mode (江東試験場)
# ./run_scraper.sh notify-test  - Test notifications only
# ./run_scraper.sh --no-notify  - Run without sending LINE notifications
#
# Flags can be combined:
# ./run_scraper.sh test --no-notify  - Test mode without notifications
# ./run_scraper.sh notify-test --no-notify - Test notification system without actually sending

# Change to the script directory
cd /Users/melkor/Documents/Projects/policeScrapper

# Run the scraper
/usr/local/go/bin/go run main.go config.go "$@" 