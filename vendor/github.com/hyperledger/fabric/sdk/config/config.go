package config

import (
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"strings"
)

const (
	envRootPrefix = "FABRIC_SDK"
)

var (
	configFactory *ConfigFactory
)

// Load config from path
func LoadFromFile(name string) (*ConfigFactory, error) {
	if name == "" {
		return nil, errors.New(fmt.Sprintf("invalid config filename : %s", name))
	}

	configFactory := &ConfigFactory{cfg: &Config{
		Orderers: make(map[string]OrdererConfig),
		Peers:    make(map[string]PeerConfig),
	}}

	v := newViper(envRootPrefix)
	v.SetConfigFile(name)
	err := reloadConfigFromFile(v, configFactory)
	if err != nil {
		panic(err.Error())
	}
	// reload config when config file changed
	// v.WatchRemoteConfig()
	//
	// v.OnConfigChange(func(e fsnotify.Event) {
	// 	reloadConfigFromFile(v, configFactory)
	// 	onFileChange()
	// })

	return configFactory, nil
}

func GetConfigFactory() (*ConfigFactory, error) {
	if configFactory == nil {
		return nil, fmt.Errorf("[GetConfigFactory] Config factory has not been initialized yet!")
	}
	return configFactory, nil
}

func newViper(cmdRootPrefix string) *viper.Viper {
	myViper := viper.New()
	myViper.SetEnvPrefix(cmdRootPrefix)
	myViper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	myViper.SetEnvKeyReplacer(replacer)
	return myViper
}

func reloadConfigFromFile(v *viper.Viper, cf *ConfigFactory) error {
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("error loading config file '%s' : %s", v.ConfigFileUsed(), err)
	}

	cf.cfg.Timeout.Event.TxResponse = v.GetDuration("timeout.event.txResponse")
	cf.cfg.Timeout.Event.RegistrationResponse = v.GetDuration("timeout.event.registrationResponse")
	v.UnmarshalKey("client", &cf.cfg.Client)
	v.UnmarshalKey("peers", &cf.cfg.Peers)
	v.UnmarshalKey("orderers", &cf.cfg.Orderers)

	cf.cfg.Event.Source = v.GetString("event.source")
	cf.cfg.Event.Retrace = RetraceConfig{
		MaxCountToRetraceAtStart: v.GetInt("event.retrace.maxCountToRetraceAtStart"),
		SyncInterval:             v.GetDuration("event.retrace.syncInterval"),
		// RetraceInterval:          v.GetDuration("event.retrace.retraceInterval"),
		// PersistenceInterval:      v.GetDuration("event.retrace.persistenceInterval"),
		// MissingBlockLife:         v.GetDuration("event.retrace.missingBlockLife"),
	}
	// if err := v.Unmarshal(cfg); err != nil {
	// 	return nil, errors.New(fmt.Sprintf("error parsing config to struct : %s", err))
	// }

	err := cf.CheckConfig()
	if err != nil {
		return fmt.Errorf("Check config error: %s", err)
	}

	return nil
}
