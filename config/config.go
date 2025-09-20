package config

import (
	"github.com/joho/godotenv"
	"gopkg.in/ini.v1"
	logger "kinozaltv_monitor/logging"
	"os"
	"strings"
)

type AppConfig struct {
	QBUsername      string
	QBPassword      string
	QBUrl           string
	KinozalUsername string
	KinozalPassword string
	RtUsername      string
	RtPassword      string
	TelegramChatId  string
	TelegramToken   string
	ListenPort      string
	UserAgent       string
}

// GlobalConfig is a global variable for storing user data
var GlobalConfig *AppConfig

var log = logger.New("config")

func loadEnvFiles() {
	// Try to load .env files in order of priority
	envFiles := []string{
		".env.local", // Local override (highest priority)
		".env.test",  // Test environment
		".env",       // Default environment
	}

	for _, envFile := range envFiles {
		if _, err := os.Stat(envFile); err == nil {
			err := godotenv.Load(envFile)
			if err != nil {
				log.Error("env_load_warning", "Could not load "+envFile, map[string]string{"error": err.Error()})
			} else {
				log.Info("env_loaded", "Loaded environment from "+envFile, nil)
				break // Load only the first available file
			}
		}
	}
}

func loadConfig() error {
	// Load environment variables from .env files first
	loadEnvFiles()

	cfg, err := ini.Load("config.ini")
	if err != nil {
		log.Error("config_ini_warning", "Could not read config.ini (using environment variables)", map[string]string{"error": err.Error()})
	} else {
		log.Info("config_ini_loaded", "Config.ini loaded successfully", nil)
	}

	configFieldMap := map[string]map[string]*string{
		"app": {
			"LISTEN_PORT": &GlobalConfig.ListenPort,
			"USER_AGENT":  &GlobalConfig.UserAgent,
		},
		"qbittorrent": {
			"QB_USERNAME": &GlobalConfig.QBUsername,
			"QB_PASSWORD": &GlobalConfig.QBPassword,
			"QB_URL":      &GlobalConfig.QBUrl,
		},
		"kinozal": {
			"KZ_USERNAME": &GlobalConfig.KinozalUsername,
			"KZ_PASSWORD": &GlobalConfig.KinozalPassword,
		},
		"rutracker": {
			"RT_USERNAME": &GlobalConfig.RtUsername,
			"RT_PASSWORD": &GlobalConfig.RtPassword,
		},
		"telegram": {
			"TG_ID":    &GlobalConfig.TelegramChatId,
			"TG_TOKEN": &GlobalConfig.TelegramToken,
		},
	}

	defaultValues := map[string]string{
		"LISTEN_PORT": "1323",
		"USER_AGENT":  "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/113.0",
	}

	for section, fields := range configFieldMap {
		for key, field := range fields {
			if err == nil {
				iniKey := strings.ToLower(strings.Split(key, "_")[1])
				iniValue := cfg.Section(section).Key(iniKey).String()
				envValue := os.Getenv(key)

				if envValue != "" {
					*field = envValue
				} else if iniValue != "" {
					*field = iniValue
				} else {
					*field = defaultValues[key]
				}
			} else {
				// If config.ini not found, use environment variables
				envValue := os.Getenv(key)
				if envValue != "" {
					*field = envValue
				} else {
					*field = defaultValues[key]
				}
			}
		}
	}
	return nil
}

func init() {
	GlobalConfig = &AppConfig{}
	err := loadConfig()
	if err != nil {
		panic(err)
	}
}
