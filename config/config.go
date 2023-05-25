package config

import (
	"fmt"
	"gopkg.in/ini.v1"
	"os"
)

type AppConfig struct {
	QBUsername      string
	QBPassword      string
	QBUrl           string
	KinozalUsername string
	KinozalPassword string
}

// GlobalConfig is a global variable for storing user data
var GlobalConfig *AppConfig

func loadConfig(user *AppConfig) error {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		fmt.Printf("Fail to read file: %v", err)
		user.QBUsername = os.Getenv("QB_USERNAME")
		user.QBPassword = os.Getenv("QB_PASSWORD")
		user.QBUrl = os.Getenv("QB_URL")
		user.KinozalUsername = os.Getenv("KZ_USERNAME")
		user.KinozalPassword = os.Getenv("KZ_PASSWORD")

	} else {
		user.QBUsername = cfg.Section("qbittorrent").Key("username").String()
		user.QBPassword = cfg.Section("qbittorrent").Key("password").String()
		user.QBUrl = cfg.Section("qbittorrent").Key("url").String()
		user.KinozalUsername = cfg.Section("kinozal").Key("username").String()
		user.KinozalPassword = cfg.Section("kinozal").Key("password").String()
	}
	return nil
}

func init() {
	GlobalConfig = &AppConfig{}
	err := loadConfig(GlobalConfig)
	if err != nil {
		panic(err)
	}
}
