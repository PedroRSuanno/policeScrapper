#!/bin/bash

# Exit on any error
set -e

# Run test check first
echo "Running test check..."
~/bin/scraper test

# If test was successful, start normal scraper
echo "Starting normal scraper..."
exec ~/bin/scraper 