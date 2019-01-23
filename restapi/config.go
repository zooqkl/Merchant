package main

import (
	"github.com/spf13/viper"
	"fmt"
)

type Config struct {
	ReleaseMode bool
	HttpPort    uint16
	DbName      string
	DbUser      string
	DbPasswd    string
	DbType      string
	DbIp        string
	DbPort      uint16
	DbCharset   string
	SignKey     string
}

func NewConfigFromYaml(yamlFile string) *Config {
	v := viper.New()
	v.SetConfigFile(yamlFile)
	err := v.ReadInConfig()
	if err != nil {
		panic(fmt.Sprintf("Viper Read in config error: %s", err))
	}
	config := &Config{}
	err = v.UnmarshalKey("app", config)
	if err != nil {
		panic(fmt.Sprintf("Viper unmarshal app config error: %s", err))
	}
	return config
}
