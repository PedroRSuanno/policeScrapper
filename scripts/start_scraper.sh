#!/bin/bash

# Change to the scraper directory
cd "$(dirname "$0")/.."

# Run test check first
echo "Running test check..."
./cmd/scraper/scraper test

# If test was successful (exit code 0), start normal operation
if [ $? -eq 0 ]; then
    echo "Test check successful, starting normal operation..."
    ./cmd/scraper/scraper
else
    echo "Test check failed, not starting normal operation"
    exit 1
fi 