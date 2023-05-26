package config

import (
	"gopkg.in/ini.v1"
	"log"
	"os"
	"strings"
)

type AppConfig struct {
	QBUsername      string
	QBPassword      string
	QBUrl           string
	KinozalUsername string
	KinozalPassword string
	TelegramChatId  string
	TelegramToken   string
	ListenPort      string
}

// GlobalConfig is a global variable for storing user data
var GlobalConfig *AppConfig

func loadConfig() error {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		log.Printf("Fail to read file: %v", err)
	} else {
		log.Println("Config loaded")
	}

	configFieldMap := map[string]map[string]*string{
		"app": {
			"APP_PORT": &GlobalConfig.ListenPort,
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
		"telegram": {
			"TG_ID":    &GlobalConfig.TelegramChatId,
			"TG_TOKEN": &GlobalConfig.TelegramToken,
		},
	}

	defaultValues := map[string]string{
		"LISTEN_PORT": "1323",
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
