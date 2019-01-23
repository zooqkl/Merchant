# Fabric-sdk yunphant version

## 说明

`SDK`为应用层与`fabric`通信的依赖包，目前支持的功能包括

- 创建链
- 加入链
- 安装合约
- 升级合约
- 部署合约
- 调用合约
- 查询合约
- 查询交易上链结果

## 快速上手

> `SDK`依赖`fabric`的 vendor 中的第三方库，为保证版本一致，需要将`sdk`放在`fabric`目录下。**<font color="red">【重要】此fabric版本要保证与搭建的fabirc网络所用版本一致以确保兼容性！</font>**

使用`sdk`开发一个简单的应用可以参考：

```go
package main

import (
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric/sdk"
)

func main() {
	sdkInstance, err := sdk.NewSDK("./config.yaml")
	if err != nil {
		panic(fmt.Sprintf("Error initialize sdk : %s", err))
	}

	// Use fabric e2e_cli example to make network for testing.
	// Chaincode using "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02".
	orderer := "orderer0.example.com"
	endorsers := []string{"peer0.org1.example.com"}
	spec := &sdk.ChaincodeInvokeSpec{
		Channel: "mychannel",
		CCName:  "mycc",
		Fcn:     "invoke",
		Args:    []string{"a", "b", "1"},
	}

	result, _ := sdkInstance.Invoke(orderer, endorsers, spec, false)
	resultByte, _ := json.Marshal(result)

	// Output will be: {"txid":"3162e6a6c2aeaf59cc31beaa13f0ee4f414a7d739417944e78e5f23473702822","status":"VALID"}
	fmt.Println(string(resultByte))
}
```

## 接口说明

```go
type FabricSDK interface {
	/*
		作用: 创建链
		参数:
		  - orderer: 请求的orderer名称，配置文件config.yaml中的orderer名称(config.orderers中的一个key)
		  - channelTxPath: channel.tx的路径
		  - channelID: 链名
		返回:
		  - status: 命令执行状态
		  - err: 错误
	*/
	CreateChannel(orderer, channelTxPath, channelID string) (status string, err error)
	/*
		作用: 加入链
		参数:
		  - peer: 加入链的peer名称
		  - orderer: 请求的orderer名称
		  - channelID: 链名
		返回:
		  - shimStatusCode: 命令执行状态
		  - err: 错误
	*/
	JoinChannel(peer, orderer, channelID string) (shimStatusCode int32, err error)

	/*
		作用: 安装合约
		参数:
		  - peer: 请求的peer名称
		  - spec: 安装合约需要的参数
		返回:
		  - shimStatusCode: 命令执行状态
		  - err: 错误
	*/
	InstallChaincode(peer string, spec *ChaincodeInstallSpec) (shimStatusCode int32, err error)
	/*
		作用: 部署合约
		参数:
		  - peer: 请求的peer名称
		  - orderer: 请求的orderer名称
		  - spec: 部署合约需要的参数
		返回:
		  - status: 命令执行状态
		  - err: 错误
	*/
	InstantiateChaincode(peer, orderer string, spec *ChaincodeDeploySpec) (status string, err error)
	/*
		作用: 升级合约
		参数:
		  - peer: 请求的peer名称
		  - orderer: 请求的orderer名称
		  - spec: 升级合约需要的参数
		返回:
		  - status: 命令执行状态
		  - err: 错误
	*/
	UpgradeChaincode(peer, orderer string, spec *ChaincodeDeploySpec) (status string, err error)
	/*
		作用: 调用合约
		参数:
		  - endorsers: 需要背书的peer节点名称列表
		  - orderer: 请求的orderer名称
		  - spec: 调用合约需要的参数
		  - async: 是否异步调用(async设为true不会等待区块结果的返回，需要手动查询交易结果)
		返回:
		  - result: 命令执行状态
		  - err: 错误
	*/
	Invoke(orderer string, endorsers []string, spec *ChaincodeInvokeSpec, async bool) (result *ChaincodeInvokeResult, err error)
	/*
		作用: 查询合约
		参数:
		  - peer: 请求的peer节点名称
		  - spec: 查询合约需要的参数
		返回:
		  - result: 查询结果
		  - err: 错误
	*/
	Query(peer string, spec *ChaincodeInvokeSpec) (result []byte, err error)
	/*
		作用: 根据txid查询交易上链结果
		参数:
		  - peer: 请求的peer节点名称
		  - channel: 链名
		  - txid: invoke返回的交易id
		返回:
		  - result: 查询结果
		  - err: 错误
	*/
	QueryTx(peer, channel, txid string) (result *ChaincodeInvokeResult, err error)

	RandPeer() *peer.PeerClient
	RandOrderer() *orderer.OrdererClient
}

```

## Restful接口

`SDK`中实现了restful接口的服务，使用方法：

```sh
cd $GOPATH/src/github.com/hyperledger/fabric/sdk/restapi
go build
./restapi
```