/*
Copyright Yunphant Corp. All Rights Reserved.
*/

package config

import "fmt"

type ConfigFactory struct {
	cfg *Config
}

func (cf *ConfigFactory) CheckConfig() error {
	// TODO: check every config to ensure GetXXXConfig method is safe without error return

	// check event config
	if cf.cfg.Event.Source == "" {
		return fmt.Errorf(`[Config] Check config error: "event.source" should not be empty!`)
	}

	return nil
}

/*
 作用：根据peer名称获得peer的配置信息
*/
func (cf *ConfigFactory) GetPeerConfig(peerName string) PeerConfig {
	peerCfg, ok := cf.cfg.Peers[peerName]
	if ok {
		return peerCfg
	}
	return PeerConfig{}
}

func (cf *ConfigFactory) GetAllPeerConfig() map[string]PeerConfig {
	return cf.cfg.Peers
}

/*
 作用：根据peer名称获得peer的配置信息
*/
func (cf *ConfigFactory) GetOrdererConfig(ordererName string) OrdererConfig {
	orderCfg, ok := cf.cfg.Orderers[ordererName]
	if ok {
		return orderCfg
	}
	return OrdererConfig{}
}

func (cf *ConfigFactory) GetAllOrdererConfig() map[string]OrdererConfig {
	return cf.cfg.Orderers
}

func (cf *ConfigFactory) GetClientConfig() ClientConfig {
	return cf.cfg.Client
}

func (cf *ConfigFactory) GetEventTimeoutConfig() EventTimeout {
	return cf.cfg.Timeout.Event
}

func (cf *ConfigFactory) GetAllConfig() Config {
	return *cf.cfg
}

func (cf *ConfigFactory) GetEventConfig() EventConfig {
	return cf.cfg.Event
}
