#!/bin/bash

PLIST_NAME="com.policeScraper"
PLIST_PATH="$HOME/Library/LaunchAgents/${PLIST_NAME}.plist"

case "$1" in
    install)
        echo "Installing launch agent..."
        mkdir -p "$HOME/Library/LaunchAgents"
        cp "${PLIST_NAME}.plist" "$PLIST_PATH"
        launchctl bootstrap gui/$UID "$PLIST_PATH"
        echo "Launch agent installed and started!"
        ;;
    uninstall)
        echo "Uninstalling launch agent..."
        launchctl bootout gui/$UID "$PLIST_PATH"
        rm "$PLIST_PATH"
        echo "Launch agent uninstalled!"
        ;;
    status)
        echo "Checking launch agent status..."
        launchctl print gui/$UID/${PLIST_NAME}
        ;;
    *)
        echo "Usage: $0 {install|uninstall|status}"
        exit 1
        ;;
esac 