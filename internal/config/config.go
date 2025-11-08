package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	ServerURL  string `mapstructure:"server_url"`
	AuthRealm  string `mapstructure:"auth_realm"`
	Realm      string `mapstructure:"realm"`
	ClientID   string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	GrantType  string `mapstructure:"grant_type"`
}

var Global Config

func findDefaultConfigPath() string {
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		p := filepath.Join(dir, "config.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	p := "config.json"
	if _, err := os.Stat(p); err == nil {
		abs, _ := filepath.Abs(p)
		return abs
	}
	return ""
}

func Load(path string) error {
	v := viper.New()
	if path != "" {
		v.SetConfigFile(path)
	} else {
		def := findDefaultConfigPath()
		if def == "" {
			return errors.New("config.json not found")
		}
		v.SetConfigFile(def)
	}
	v.SetConfigType("json")
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	if err := v.Unmarshal(&Global); err != nil {
		return err
	}
	if Global.ServerURL == "" {
		return errors.New("server_url is required")
	}
	if Global.AuthRealm == "" {
		Global.AuthRealm = "master"
	}
	if Global.GrantType == "" {
		Global.GrantType = "client_credentials"
	}
	return nil
}
