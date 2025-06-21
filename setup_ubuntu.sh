#!/bin/bash

# Exit on any error
set -e

echo "Installing dependencies..."
# Add Google Chrome repository
wget -q -O - https://dl-ssl.google.com/linux/linux_signing_key.pub | sudo apt-key add -
sudo sh -c 'echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google.list'

# Update package list and install dependencies
sudo apt-get update
sudo apt-get install -y \
    google-chrome-stable \
    golang \
    supervisor

# Create directories
mkdir -p ~/logs
mkdir -p ~/bin

# Build the scraper
echo "Building scraper..."
go build -o ~/bin/scraper cmd/scraper/main.go

# Create supervisor environment file
echo "Creating supervisor environment file..."
sudo mkdir -p /etc/supervisor/conf.d/env
sudo tee /etc/supervisor/conf.d/env/police-scraper.env << EOF
LINE_CHANNEL_TOKEN=${LINE_CHANNEL_TOKEN}
LINE_USER_ID=${LINE_USER_ID}
EOF

# Create supervisor config
echo "Setting up supervisor service..."
sudo tee /etc/supervisor/conf.d/police-scraper.conf << EOF
[program:police-scraper]
command=/home/$USER/bin/scraper
directory=/home/$USER
autostart=true
autorestart=true
stderr_logfile=/home/$USER/logs/scraper.err.log
stdout_logfile=/home/$USER/logs/scraper.out.log
environment=LINE_CHANNEL_TOKEN="%(ENV_LINE_CHANNEL_TOKEN)s",LINE_USER_ID="%(ENV_LINE_USER_ID)s"
user=$USER
startsecs=10
stopwaitsecs=10
EOF

# Ensure proper permissions
sudo chown $USER:$USER ~/logs ~/bin
sudo chmod 750 ~/logs ~/bin

echo "Creating user environment file template..."
tee ~/.police-scraper.env.example << EOF
# Copy this file to ~/.police-scraper.env and set your values
export LINE_CHANNEL_TOKEN="your_line_channel_token"
export LINE_USER_ID="your_line_user_id"

# After editing this file with your credentials:
# 1. Source it: source ~/.police-scraper.env
# 2. Run setup again: ./setup_ubuntu.sh
# 3. Restart supervisor: sudo supervisorctl reread && sudo supervisorctl update
EOF

echo "Setup complete! Please follow these steps:"
echo "1. Copy ~/.police-scraper.env.example to ~/.police-scraper.env"
echo "2. Edit ~/.police-scraper.env with your LINE credentials"
echo "3. Source the environment file: source ~/.police-scraper.env"
echo "4. Run this setup script again: ./setup_ubuntu.sh"
echo "5. Restart supervisor completely: sudo systemctl restart supervisor"
echo "6. Check status: sudo supervisorctl status police-scraper"
echo "7. View logs: tail -f ~/logs/scraper.out.log" 