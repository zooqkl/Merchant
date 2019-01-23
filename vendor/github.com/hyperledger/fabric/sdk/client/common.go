/*
Copyright Yunphant Corp. All Rights Reserved.
*/

package client

import (
	"fmt"
	"github.com/hyperledger/fabric/core/comm"
	"github.com/hyperledger/fabric/sdk/config"
	"github.com/hyperledger/fabric/sdk/logging"
	"github.com/hyperledger/fabric/sdk/utils/pathvar"
	"github.com/spf13/cast"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var logger sdklogging.Logger

func init() {
	logger = sdklogging.GetLogger()
}

/*
 作用：创建grpc的tls证书对象
*/
func InitTLSForClient(certPath, serverHostOverride string) (credentials.TransportCredentials, error) {
	logger.Debugf("Loading TLS credentials from path '%s' for host %s ", certPath, serverHostOverride)
	var creds credentials.TransportCredentials
	creds, err := credentials.NewClientTLSFromFile(certPath, serverHostOverride)
	if err != nil {
		return nil, fmt.Errorf("Failed to create TLS credentials %s", err)
	}

	return creds, nil
}

//AttemptSecured is a utility function which verifies URL and returns if secured connections needs to established
// for protocol 'grpcs' in URL returns true
// for protocol 'grpc' in URL returns false
// for no protocol mentioned, returns !allowInSecure
func AttemptSecured(url string, allowInSecure bool) bool {
	ok, err := regexp.MatchString(".*(?i)s://", url)
	if ok && err == nil {
		return true
	} else if strings.Contains(url, "://") {
		return false
	} else {
		return !allowInSecure
	}
}

// 从配置中获取 serverNameOverride
func GetServerNameOverride(cfg map[string]interface{}) string {
	serverNameOverride := ""
	if str, ok := cfg["ssl-target-name-override"].(string); ok {
		serverNameOverride = str
	}
	return serverNameOverride
}

// 从配置中获取 keep-alive相关
func GetKeepAliveOptions(cfg map[string]interface{}) keepalive.ClientParameters {
	var kap keepalive.ClientParameters
	if kaTime, ok := cfg["keep-alive-time"].(time.Duration); ok {
		kap.Time = cast.ToDuration(kaTime)
	}
	if kaTimeout, ok := cfg["keep-alive-timeout"].(time.Duration); ok {
		kap.Timeout = cast.ToDuration(kaTimeout)
	}
	if kaPermit, ok := cfg["keep-alive-permit"].(time.Duration); ok {
		kap.PermitWithoutStream = cast.ToBool(kaPermit)
	}
	return kap
}

// 从配置中获取 fail-fast
func GetFailFast(cfg map[string]interface{}) bool {
	var failFast = true
	if ff, ok := cfg["fail-fast"].(bool); ok {
		failFast = cast.ToBool(ff)
	}
	return failFast
}

// 从配置中获取 allow-insecure 配置
func IsInsecureConnectionAllowed(cfg map[string]interface{}) bool {
	allowInsecure, ok := cfg["allow-insecure"].(bool)
	if ok {
		return allowInsecure
	}
	return false
}

func NewGRPCClient(name string, cf *config.ConfigFactory) (*comm.GRPCClient, error) {
	var grpcCfg config.GRPCConfig
	cfg1 := cf.GetPeerConfig(name)
	cfg2 := cf.GetOrdererConfig(name)
	if cfg1.GrpcConfig.Address != "" {
		grpcCfg = cfg1.GrpcConfig
	}
	if cfg2.GrpcConfig.Address != "" {
		grpcCfg = cfg2.GrpcConfig
	}
	if grpcCfg.Address == "" {
		return nil, fmt.Errorf("No peer or orderer named [%s].", name)
	}

	clientCfg := cf.GetClientConfig()

	grpcClientConfig := comm.ClientConfig{}

	secOpts := &comm.SecureOptions{
		UseTLS:            AttemptSecured(grpcCfg.Address, true),
		RequireClientCert: clientCfg.Tls.ClientAuthRequired,
	}
	if secOpts.UseTLS {
		caPEM, err := ioutil.ReadFile(pathvar.Subst(grpcCfg.TlsCaCertPath))
		if err != nil {
			return nil, fmt.Errorf("unable to load ca.crt: %s", err)
		}
		secOpts.ServerRootCAs = [][]byte{caPEM}
	}
	if secOpts.RequireClientCert {
		keyPEM, err := ioutil.ReadFile(filepath.Join(pathvar.Subst(clientCfg.Tls.TlsPath), "client.key"))
		if err != nil {
			return nil, fmt.Errorf("unable to load client.key: %s", err)
		}
		secOpts.Key = keyPEM
		certPEM, err := ioutil.ReadFile(filepath.Join(pathvar.Subst(clientCfg.Tls.TlsPath), "client.crt"))
		if err != nil {
			return nil, fmt.Errorf("unable to load client.crt: %s", err)
		}
		secOpts.Certificate = certPEM
	}
	grpcClientConfig.SecOpts = secOpts
	grpcClientConfig.Timeout = 3 * time.Second
	grpcClientConfig.KaOpts = nil // use default keepAliveOption for now

	return comm.NewGRPCClient(grpcClientConfig)
}

func IsConnStatusOK(conn *grpc.ClientConn) bool {
	needNewConn := false
	if conn == nil {
		needNewConn = true
	} else {
		connState := conn.GetState().String()
		logger.Debugf("Grpc connection status is : %s", connState)
		if connState != "IDLE" && connState != "READY" {
			conn.Close()
			needNewConn = true
		}
	}
	return !needNewConn
}

func GrpcAddressFromUrl(url string) string {
	if strings.HasPrefix(url, "grpc") {
		splits := strings.Split(url, "://")
		if len(splits) > 1 {
			return splits[1]
		}
	}
	return url
}
