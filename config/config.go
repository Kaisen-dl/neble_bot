package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Token                 string
	GuildID               string
	RoleChannelID         string
	NotificationChannelID string
	StatsChannelID        string
	RoleDuration          time.Duration
	RenewalDuration       time.Duration
	RoleMessageID         string
}

func Load() *Config {
	return &Config{
		Token:                 getEnv("BOT_TOKEN", ""),
		GuildID:               getEnv("GUILD_ID", ""),
		RoleChannelID:         getEnv("ROLE_CHANNEL_ID", ""),
		NotificationChannelID: getEnv("NOTIFICATION_CHANNEL_ID", ""),
		StatsChannelID:        getEnv("STATS_CHANNEL_ID", ""),
		RoleDuration:          getDurationEnv("ROLE_DURATION_HOURS", 65) * time.Minute,
		RenewalDuration:       getDurationEnv("RENEWAL_DURATION_HOURS", 10) * time.Minute,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue int) time.Duration {
	if value := os.Getenv(key); value != "" {
		if hours, err := strconv.Atoi(value); err == nil {
			return time.Duration(hours)
		}
	}
	return time.Duration(defaultValue)
}
