#!/bin/bash

# Exit on any error
set -e

echo "Building scraper..."
go build -o ~/bin/scraper cmd/scraper/main.go

echo "Restarting supervisor..."
sudo systemctl restart supervisor

echo "Waiting for service to start..."
sleep 2

echo "Service status:"
sudo supervisorctl status police-scraper

echo "Tailing logs (Ctrl+C to exit)..."
tail -f ~/logs/scraper.out.log 