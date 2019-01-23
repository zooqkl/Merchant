/*
Copyright Yunphant Corp. All Rights Reserved.
*/

package config

import (
	"time"
)

type Config struct {
	Client   ClientConfig
	Orderers map[string]OrdererConfig
	Peers    map[string]PeerConfig
	Timeout  TimeoutConfig
	Event    EventConfig
}

// OrdererConfig defines an orderer configuration
type OrdererConfig struct {
	GrpcConfig GRPCConfig
}

// PeerConfig defines a peer configuration
type PeerConfig struct {
	GrpcConfig   GRPCConfig
	EventAddress string
}

type GRPCConfig struct {
	Address            string
	ServerNameOverride string
	TlsCaCertPath      string
}

// MSPConfig  msp config
type MSPConfig struct {
	Id       string
	UserPath string
	BCCSP    BCCSPConfig
	FakeMsp  map[string]MspBaseConfig
}

type MspBaseConfig struct {
	Id   string
	Path string
}

// SDK Client config
type ClientConfig struct {
	Tls      ClientTLS
	MSPConfig
	LogLevel LogLevelConfig
}

type LogLevelConfig struct {
	Sdk    string
	Fabric string
	Gin string
}

type ClientTLS struct {
	ClientAuthRequired bool

	// TlsPath is the client tls directory,like
	// .
	// ├── tls
	// 	    ├── ca.crt
	// 	    ├── client.crt
	// 	    └── client.key
	TlsPath string
}

type TimeoutConfig struct {
	Event EventTimeout
}

type EventTimeout struct {
	TxResponse           time.Duration
	RegistrationResponse time.Duration
}

type BCCSPConfig struct {
	Provider     string
	Hash         string
	Security     int
	FileKeyStore LocationConfig
}

type LocationConfig struct {
	KeyStore string
}

type EventConfig struct {
	Source  string
	Retrace RetraceConfig
}

type RetraceConfig struct {
	// RetraceInterval          time.Duration
	// PersistenceInterval      time.Duration
	// MissingBlockLife         time.Duration
	SyncInterval             time.Duration
	MaxCountToRetraceAtStart int
}
