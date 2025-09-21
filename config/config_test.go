package config

import (
	"os"
	"testing"
)

func TestLoadEnvFiles(t *testing.T) {
	testCases := []struct {
		name     string
		envFiles map[string]string
		expected string
	}{
		{
			name: "Load .env.local with highest priority",
			envFiles: map[string]string{
				".env.local": "TEST_VAR=local_value",
				".env.test":  "TEST_VAR=test_value",
				".env":       "TEST_VAR=default_value",
			},
			expected: "local_value",
		},
		{
			name: "Load .env.test when .env.local missing",
			envFiles: map[string]string{
				".env.test": "TEST_VAR=test_value",
				".env":      "TEST_VAR=default_value",
			},
			expected: "test_value",
		},
		{
			name: "Load .env when others missing",
			envFiles: map[string]string{
				".env": "TEST_VAR=default_value",
			},
			expected: "default_value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			originalDir, _ := os.Getwd()
			defer os.Chdir(originalDir)
			os.Chdir(tempDir)

			for filename, content := range tc.envFiles {
				if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create test file %s: %v", filename, err)
				}
			}

			os.Unsetenv("TEST_VAR")
			loadEnvFiles()

			if got := os.Getenv("TEST_VAR"); got != tc.expected {
				t.Errorf("Expected TEST_VAR=%s, got %s", tc.expected, got)
			}

			os.Unsetenv("TEST_VAR")
		})
	}
}

func TestLoadConfig_EnvironmentVariables(t *testing.T) {
	testEnvVars := map[string]string{
		"QB_USERNAME": "test_qb_user",
		"QB_PASSWORD": "test_qb_pass",
		"QB_URL":      "http://test:8080",
		"KZ_USERNAME": "test_kz_user",
		"KZ_PASSWORD": "test_kz_pass",
		"RT_USERNAME": "test_rt_user",
		"RT_PASSWORD": "test_rt_pass",
		"TG_ID":       "123456789",
		"TG_TOKEN":    "test_token",
		"LISTEN_PORT": "9999",
		"USER_AGENT":  "test_agent",
	}

	originalEnv := make(map[string]string)
	for key := range testEnvVars {
		originalEnv[key] = os.Getenv(key)
	}
	defer func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	for key, value := range testEnvVars {
		os.Setenv(key, value)
	}

	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	config := &AppConfig{}
	GlobalConfig = config
	err := loadConfig()

	if err != nil {
		t.Fatalf("loadConfig() failed: %v", err)
	}

	if config.QBUsername != "test_qb_user" {
		t.Errorf("Expected QBUsername=test_qb_user, got %s", config.QBUsername)
	}
	if config.QBPassword != "test_qb_pass" {
		t.Errorf("Expected QBPassword=test_qb_pass, got %s", config.QBPassword)
	}
	if config.QBUrl != "http://test:8080" {
		t.Errorf("Expected QBUrl=http://test:8080, got %s", config.QBUrl)
	}
	if config.KinozalUsername != "test_kz_user" {
		t.Errorf("Expected KinozalUsername=test_kz_user, got %s", config.KinozalUsername)
	}
	if config.KinozalPassword != "test_kz_pass" {
		t.Errorf("Expected KinozalPassword=test_kz_pass, got %s", config.KinozalPassword)
	}
	if config.RtUsername != "test_rt_user" {
		t.Errorf("Expected RtUsername=test_rt_user, got %s", config.RtUsername)
	}
	if config.RtPassword != "test_rt_pass" {
		t.Errorf("Expected RtPassword=test_rt_pass, got %s", config.RtPassword)
	}
	if config.TelegramChatId != "123456789" {
		t.Errorf("Expected TelegramChatId=123456789, got %s", config.TelegramChatId)
	}
	if config.TelegramToken != "test_token" {
		t.Errorf("Expected TelegramToken=test_token, got %s", config.TelegramToken)
	}
	if config.ListenPort != "9999" {
		t.Errorf("Expected ListenPort=9999, got %s", config.ListenPort)
	}
	if config.UserAgent != "test_agent" {
		t.Errorf("Expected UserAgent=test_agent, got %s", config.UserAgent)
	}
}

func TestLoadConfig_INIFile(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	iniContent := `[app]
port = 8888
agent = ini_user_agent

[qbittorrent]
username = ini_qb_user
password = ini_qb_pass
url = http://ini:8080

[kinozal]
username = ini_kz_user
password = ini_kz_pass

[rutracker]
username = ini_rt_user
password = ini_rt_pass

[telegram]
id = 987654321
token = ini_token
`

	if err := os.WriteFile("config.ini", []byte(iniContent), 0644); err != nil {
		t.Fatalf("Failed to create config.ini: %v", err)
	}

	envVarsToUnset := []string{
		"QB_USERNAME", "QB_PASSWORD", "QB_URL",
		"KZ_USERNAME", "KZ_PASSWORD",
		"RT_USERNAME", "RT_PASSWORD",
		"TG_ID", "TG_TOKEN",
		"LISTEN_PORT", "USER_AGENT",
	}
	for _, envVar := range envVarsToUnset {
		os.Unsetenv(envVar)
	}

	config := &AppConfig{}
	GlobalConfig = config
	err := loadConfig()

	if err != nil {
		t.Fatalf("loadConfig() failed: %v", err)
	}

	if config.QBUsername != "ini_qb_user" {
		t.Errorf("Expected QBUsername=ini_qb_user, got %s", config.QBUsername)
	}
	if config.QBPassword != "ini_qb_pass" {
		t.Errorf("Expected QBPassword=ini_qb_pass, got %s", config.QBPassword)
	}
	if config.QBUrl != "http://ini:8080" {
		t.Errorf("Expected QBUrl=http://ini:8080, got %s", config.QBUrl)
	}
	if config.KinozalUsername != "ini_kz_user" {
		t.Errorf("Expected KinozalUsername=ini_kz_user, got %s", config.KinozalUsername)
	}
	if config.KinozalPassword != "ini_kz_pass" {
		t.Errorf("Expected KinozalPassword=ini_kz_pass, got %s", config.KinozalPassword)
	}
	if config.RtUsername != "ini_rt_user" {
		t.Errorf("Expected RtUsername=ini_rt_user, got %s", config.RtUsername)
	}
	if config.RtPassword != "ini_rt_pass" {
		t.Errorf("Expected RtPassword=ini_rt_pass, got %s", config.RtPassword)
	}
	if config.TelegramChatId != "987654321" {
		t.Errorf("Expected TelegramChatId=987654321, got %s", config.TelegramChatId)
	}
	if config.TelegramToken != "ini_token" {
		t.Errorf("Expected TelegramToken=ini_token, got %s", config.TelegramToken)
	}
	if config.ListenPort != "8888" {
		t.Errorf("Expected ListenPort=8888, got %s", config.ListenPort)
	}
	if config.UserAgent != "ini_user_agent" {
		t.Errorf("Expected UserAgent=ini_user_agent, got %s", config.UserAgent)
	}
}

func TestLoadConfig_EnvironmentOverridesINI(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	iniContent := `[qbittorrent]
username = ini_qb_user
password = ini_qb_pass
`

	if err := os.WriteFile("config.ini", []byte(iniContent), 0644); err != nil {
		t.Fatalf("Failed to create config.ini: %v", err)
	}

	originalUsername := os.Getenv("QB_USERNAME")
	defer func() {
		if originalUsername == "" {
			os.Unsetenv("QB_USERNAME")
		} else {
			os.Setenv("QB_USERNAME", originalUsername)
		}
	}()

	os.Setenv("QB_USERNAME", "env_qb_user")

	config := &AppConfig{}
	GlobalConfig = config
	err := loadConfig()

	if err != nil {
		t.Fatalf("loadConfig() failed: %v", err)
	}

	if config.QBUsername != "env_qb_user" {
		t.Errorf("Expected environment variable to override INI: got %s", config.QBUsername)
	}
	if config.QBPassword != "ini_qb_pass" {
		t.Errorf("Expected QBPassword from INI: got %s", config.QBPassword)
	}
}

func TestLoadConfig_DefaultValues(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	envVarsToUnset := []string{
		"LISTEN_PORT", "USER_AGENT",
	}
	originalValues := make(map[string]string)
	for _, envVar := range envVarsToUnset {
		originalValues[envVar] = os.Getenv(envVar)
		os.Unsetenv(envVar)
	}
	defer func() {
		for envVar, value := range originalValues {
			if value == "" {
				os.Unsetenv(envVar)
			} else {
				os.Setenv(envVar, value)
			}
		}
	}()

	config := &AppConfig{}
	GlobalConfig = config
	err := loadConfig()

	if err != nil {
		t.Fatalf("loadConfig() failed: %v", err)
	}

	expectedPort := "1323"
	expectedAgent := "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/113.0"

	if config.ListenPort != expectedPort {
		t.Errorf("Expected default ListenPort=%s, got %s", expectedPort, config.ListenPort)
	}
	if config.UserAgent != expectedAgent {
		t.Errorf("Expected default UserAgent=%s, got %s", expectedAgent, config.UserAgent)
	}
}

func TestLoadConfig_MissingINIFile(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	originalUsername := os.Getenv("QB_USERNAME")
	defer func() {
		if originalUsername == "" {
			os.Unsetenv("QB_USERNAME")
		} else {
			os.Setenv("QB_USERNAME", originalUsername)
		}
	}()

	os.Setenv("QB_USERNAME", "env_only_user")

	config := &AppConfig{}
	GlobalConfig = config
	err := loadConfig()

	if err != nil {
		t.Fatalf("loadConfig() should not fail when INI file is missing: %v", err)
	}

	if config.QBUsername != "env_only_user" {
		t.Errorf("Expected QBUsername from env when INI missing: got %s", config.QBUsername)
	}
}

func TestAppConfigStruct(t *testing.T) {
	config := &AppConfig{
		QBUsername:      "test_qb",
		QBPassword:      "test_pass",
		QBUrl:           "http://test:8080",
		KinozalUsername: "test_kinozal",
		KinozalPassword: "test_kinozal_pass",
		RtUsername:      "test_rt",
		RtPassword:      "test_rt_pass",
		TelegramChatId:  "123456789",
		TelegramToken:   "test_token",
		ListenPort:      "8080",
		UserAgent:       "test_agent",
	}

	if config.QBUsername != "test_qb" {
		t.Errorf("AppConfig field assignment failed for QBUsername")
	}
	if config.QBPassword != "test_pass" {
		t.Errorf("AppConfig field assignment failed for QBPassword")
	}
	if config.QBUrl != "http://test:8080" {
		t.Errorf("AppConfig field assignment failed for QBUrl")
	}
	if config.KinozalUsername != "test_kinozal" {
		t.Errorf("AppConfig field assignment failed for KinozalUsername")
	}
	if config.KinozalPassword != "test_kinozal_pass" {
		t.Errorf("AppConfig field assignment failed for KinozalPassword")
	}
	if config.RtUsername != "test_rt" {
		t.Errorf("AppConfig field assignment failed for RtUsername")
	}
	if config.RtPassword != "test_rt_pass" {
		t.Errorf("AppConfig field assignment failed for RtPassword")
	}
	if config.TelegramChatId != "123456789" {
		t.Errorf("AppConfig field assignment failed for TelegramChatId")
	}
	if config.TelegramToken != "test_token" {
		t.Errorf("AppConfig field assignment failed for TelegramToken")
	}
	if config.ListenPort != "8080" {
		t.Errorf("AppConfig field assignment failed for ListenPort")
	}
	if config.UserAgent != "test_agent" {
		t.Errorf("AppConfig field assignment failed for UserAgent")
	}
}

func TestGetTestConfig(t *testing.T) {
	testConfig := GetTestConfig()

	if testConfig == nil {
		t.Fatal("GetTestConfig() returned nil")
	}

	expectedValues := map[string]string{
		"QBUsername":      "test_qb_user",
		"QBPassword":      "test_qb_password",
		"QBUrl":           "http://localhost:8080",
		"KinozalUsername": "test_kinozal_user",
		"KinozalPassword": "test_kinozal_password",
		"RtUsername":      "test_rutracker_user",
		"RtPassword":      "test_rutracker_password",
		"TelegramChatId":  "123456789",
		"TelegramToken":   "1234567890:ABCDEFGHIJKLMNOPQRSTUVWXYZ123456789",
		"ListenPort":      "1323",
		"UserAgent":       "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/113.0",
	}

	actualValues := map[string]string{
		"QBUsername":      testConfig.QBUsername,
		"QBPassword":      testConfig.QBPassword,
		"QBUrl":           testConfig.QBUrl,
		"KinozalUsername": testConfig.KinozalUsername,
		"KinozalPassword": testConfig.KinozalPassword,
		"RtUsername":      testConfig.RtUsername,
		"RtPassword":      testConfig.RtPassword,
		"TelegramChatId":  testConfig.TelegramChatId,
		"TelegramToken":   testConfig.TelegramToken,
		"ListenPort":      testConfig.ListenPort,
		"UserAgent":       testConfig.UserAgent,
	}

	for field, expected := range expectedValues {
		if actual := actualValues[field]; actual != expected {
			t.Errorf("GetTestConfig() field %s: expected %s, got %s", field, expected, actual)
		}
	}
}

func TestSetupTestConfig(t *testing.T) {
	originalEnv := os.Getenv("APP_ENV")
	defer func() {
		if originalEnv == "" {
			os.Unsetenv("APP_ENV")
		} else {
			os.Setenv("APP_ENV", originalEnv)
		}
	}()

	originalConfig := GlobalConfig

	SetupTestConfig(t)

	if os.Getenv("APP_ENV") != "test" {
		t.Error("SetupTestConfig should set APP_ENV to 'test'")
	}

	if GlobalConfig == nil {
		t.Error("SetupTestConfig should initialize GlobalConfig")
	}

	if GlobalConfig == originalConfig {
		t.Error("SetupTestConfig should create a new GlobalConfig instance")
	}

	CleanupTestConfig()

	if os.Getenv("APP_ENV") != "" {
		t.Error("CleanupTestConfig should unset APP_ENV")
	}
}

func TestLoadConfig_EdgeCases(t *testing.T) {
	t.Run("Empty INI file", func(t *testing.T) {
		tempDir := t.TempDir()
		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(tempDir)

		if err := os.WriteFile("config.ini", []byte(""), 0644); err != nil {
			t.Fatalf("Failed to create empty config.ini: %v", err)
		}

		config := &AppConfig{}
		GlobalConfig = config
		err := loadConfig()

		if err != nil {
			t.Fatalf("loadConfig() should not fail with empty INI file: %v", err)
		}
	})

	t.Run("Malformed INI file", func(t *testing.T) {
		tempDir := t.TempDir()
		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(tempDir)

		malformedContent := `[qbittorrent
username = test
invalid line without equals
[section without closing bracket
`

		if err := os.WriteFile("config.ini", []byte(malformedContent), 0644); err != nil {
			t.Fatalf("Failed to create malformed config.ini: %v", err)
		}

		config := &AppConfig{}
		GlobalConfig = config
		err := loadConfig()

		if err != nil {
			t.Fatalf("loadConfig() should handle malformed INI gracefully: %v", err)
		}
	})
}
