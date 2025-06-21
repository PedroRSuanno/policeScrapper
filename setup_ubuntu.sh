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
LINE_CHANNEL_TOKEN=%(ENV_LINE_CHANNEL_TOKEN)s
LINE_USER_ID=%(ENV_LINE_USER_ID)s
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
environment=LINE_CHANNEL_TOKEN=%(ENV_LINE_CHANNEL_TOKEN)s,LINE_USER_ID=%(ENV_LINE_USER_ID)s
user=$USER
EOF

echo "Creating user environment file template..."
tee ~/.police-scraper.env.example << EOF
# Copy this file to ~/.police-scraper.env and set your values
export LINE_CHANNEL_TOKEN="your_line_channel_token"
export LINE_USER_ID="your_line_user_id"

# Also update the supervisor environment file
sudo tee /etc/supervisor/conf.d/env/police-scraper.env << EOL
LINE_CHANNEL_TOKEN=\${LINE_CHANNEL_TOKEN}
LINE_USER_ID=\${LINE_USER_ID}
EOL
EOF

echo "Setup complete! Please follow these steps:"
echo "1. Copy ~/.police-scraper.env.example to ~/.police-scraper.env"
echo "2. Edit ~/.police-scraper.env with your LINE credentials"
echo "3. Source the environment file: source ~/.police-scraper.env"
echo "4. Start the service: sudo supervisorctl reread && sudo supervisorctl update"
echo "5. Check status: sudo supervisorctl status police-scraper"
echo "6. View logs: tail -f ~/logs/scraper.out.log" 