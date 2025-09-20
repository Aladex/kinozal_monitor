package config

import (
	"os"
	"testing"
)

// SetupTestConfig configures the application for testing
func SetupTestConfig(t *testing.T) {
	// Set test environment variable to force loading test config
	if err := os.Setenv("APP_ENV", "test"); err != nil {
		t.Fatalf("Failed to set test environment variable: %v", err)
	}

	// Reinitialize config with test environment
	GlobalConfig = &AppConfig{}
	err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load test configuration: %v", err)
	}
}

// CleanupTestConfig cleans up after tests
func CleanupTestConfig() {
	os.Unsetenv("APP_ENV")
}

// GetTestConfig returns a config instance for testing
func GetTestConfig() *AppConfig {
	return &AppConfig{
		QBUsername:      "test_qb_user",
		QBPassword:      "test_qb_password",
		QBUrl:           "http://localhost:8080",
		KinozalUsername: "test_kinozal_user",
		KinozalPassword: "test_kinozal_password",
		RtUsername:      "test_rutracker_user",
		RtPassword:      "test_rutracker_password",
		TelegramChatId:  "123456789",
		TelegramToken:   "1234567890:ABCDEFGHIJKLMNOPQRSTUVWXYZ123456789",
		ListenPort:      "1323",
		UserAgent:       "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/113.0",
	}
}
