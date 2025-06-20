package main

import (
	"os"
)

type Config struct {
	LineChannelToken string
	LineUserID       string
}

func loadConfig() Config {
	config := Config{
		LineChannelToken: os.Getenv("LINE_CHANNEL_TOKEN"),
		LineUserID:       os.Getenv("LINE_USER_ID"),
	}

	return config
}
