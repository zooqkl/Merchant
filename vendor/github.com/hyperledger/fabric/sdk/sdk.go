/*
Copyright Yunphant Corp. All Rights Reserved.
*/

package sdk

import (
	"fmt"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/sdk/client/event"
	"github.com/hyperledger/fabric/sdk/client/orderer"
	"github.com/hyperledger/fabric/sdk/client/peer"
	"github.com/hyperledger/fabric/sdk/config"
	"github.com/hyperledger/fabric/sdk/logging"
	"github.com/hyperledger/fabric/sdk/msp"
	sdkutils "github.com/hyperledger/fabric/sdk/utils"
	"github.com/hyperledger/fabric/sdk/utils/pathvar"
	"io/ioutil"
	"math/rand"
	"time"
	"sync"
	"github.com/golang/protobuf/proto"
)

const (
	CHAINCODE_TYPE      = "GOLANG"
	SHIM_OK             = 200
	SHIM_ERRORTHRESHOLD = 400
	SHIM_ERROR          = 500
)

// Fabric sdk 接口定义
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
	
	AllPeer() []string
	AllOrderer() []string

	RegisterChaincodeEvent(ccid, eventName string) <-chan ChaincodeEvent

	GetTpsOutput() (<-chan uint64, error)

	RandPeer() *peer.PeerClient
	RandOrderer() *orderer.OrdererClient

	GetPeerClient(peerName string) (*peer.PeerClient, error)
	GetOrdererClient(ordererName string) (*orderer.OrdererClient, error)

	GetMspManager() *msp.MspManager
  /*
		作用: 查询合约的世界状态
		参数:
		  - peer: 请求的peer节点名称
		  - channel: 链名
		  - chaincode: 合约名
		返回:
		  - result: 查询结果
		  - err: 错误
	*/
    GetStateDBByCC(peer,channel,chaincode string) (result []byte,err error)
    /*
		作用: 查询合约中指定key的历史状态
		参数:
		  - peer: 请求的peer节点名称
		  - channel: 链名
		  - chaincode: 合约名
		  - key: 键名
		返回:
		  - result: 查询结果
		  - err: 错误
	*/
    GetHistoryByKey(peer,channel,chaincode,key string) (result []byte,err error)


	RegisterWsConn(uuid string,conn interface{}) error
	RegisterEvent(ccName,uuid string,dappuid []string) int
	GetEventHandler() interface{}


	GetCcInfo(peer,channel string,args ...string) (res interface{},version string,err error)


}

// Fabric sdk 的实现
type sdkImpl struct {
	peers      map[string]*peer.PeerClient
	orderers   map[string]*orderer.OrdererClient
	config     *config.ConfigFactory
	mspManager *msp.MspManager
}

var logger sdklogging.Logger

func NewSDK(configPath string) (FabricSDK, error) {
	s := &sdkImpl{
	}
	// load sdk config
	cf, err := config.LoadFromFile(configPath)
	if err != nil {
		panic(fmt.Sprintf("Load sdk config fail : %s", err))
		return nil, err
	}
	s.config = cf
	s.initSDKFromConfig()

	_, err = s.getEventHandler()
	if err != nil {
		panic(err.Error())
	}

	sdkutils.PrettyPrintStruct(cf.GetAllConfig())
	return s, nil
}

func (sdk *sdkImpl) initSDKFromConfig() {
	clientCfg := sdk.config.GetClientConfig()
	levelConfig := sdk.config.GetClientConfig().LogLevel
	logger = sdklogging.InitLogger(levelConfig.Sdk,levelConfig.Fabric)
	// process msp
	mspConfig := clientCfg.MSPConfig
	mspConfig.UserPath = pathvar.Subst(mspConfig.UserPath)
	mspConfig.BCCSP.FileKeyStore.KeyStore = pathvar.Subst(mspConfig.BCCSP.FileKeyStore.KeyStore)
	for k,v := range mspConfig.FakeMsp {
		mspConfig.FakeMsp[k] = config.MspBaseConfig{Id:v.Id,Path:pathvar.Subst(v.Path)}
	}
	mm,err := msp.NewMspManager(mspConfig)
	if err != nil {
		logger.Errorf("Init SDK msp manager from config error: %s", err)
		panic(err.Error())
	}
	sdk.mspManager = mm

	sdk.InitOrdererClientsFromConf()
	sdk.InitPeerClientsFromConf()
}

func (sdk *sdkImpl) CreateChannel(orderer, channelTxPath, channelID string) (status string, err error) {
	oc, err := sdk.GetOrdererClient(orderer)
	if err != nil {
		logger.Errorf("[CreateChannel] Error: %s", err)
		return "", err
	}

	env, err := sdkutils.BuildCreateChannelEnvelope(sdk.mspManager.NewSigner("default"), channelTxPath, channelID)
	if err != nil {
		logger.Errorf("[CreateChannel] Error building envelope for creating channel: %s", err)
		return "", err
	}

	result, err := oc.SendBroadCast(env)
	if err != nil {
		logger.Error(err)
		return "", err
	}

	return result.String(), nil
}
func (sdk *sdkImpl) JoinChannel(peer, orderer, channelID string) (shimStatusCode int32, err error) {
	if peer == "" {
		err := fmt.Errorf("[JoinChannel] Invalid peer : %s", peer)
		logger.Error(err)
		return SHIM_ERROR, err
	}
	if channelID == "" {
		err := fmt.Errorf("[JoinChannel] Invalid channel name : %s", channelID)
		logger.Error(err)
		return SHIM_ERROR, err
	}
	pc, err := sdk.GetPeerClient(peer)
	if err != nil {
		err := fmt.Errorf("[JoinChannel] Get peer client error: %s", err)
		logger.Error(err)
		return SHIM_ERROR, err
	}
	oc, err := sdk.GetOrdererClient(orderer)
	if err != nil {
		err := fmt.Errorf("[JoinChannel] Get orderer client error: %s", err)
		logger.Error(err)
		return SHIM_ERROR, err
	}

	// 1. seek channel's genesis block
	env, err := sdkutils.BuildSeekBlockEnvelope(sdk.mspManager.NewSigner("default"), channelID, 0, 0)
	if err != nil {
		err := fmt.Errorf("[JoinChannel] BuildSeekBlockEnvelope error: %s", err)
		logger.Error(err)
		return SHIM_ERROR, err
	}
	genesisBlock, err := oc.SendDeliver(env)
	if err != nil {
		err := fmt.Errorf("[JoinChannel] SendDeliver error: %s", err)
		logger.Error(err)
		return SHIM_ERROR, err
	}

	genesisBlockbytes, err := sdkutils.MarshalBlockToByte(genesisBlock)
	if err != nil {
		err := fmt.Errorf("[JoinChannel] Marshal block error: %s", err)
		logger.Error(err)
		return SHIM_ERROR, err
	}

	// 2. send the proposal
	identity,err := sdk.mspManager.GetSigningIdentity(peer)
	if err != nil {
		err := fmt.Errorf("[JoinChannel] GetSigningIdentity error: %s", err)
		logger.Error(err)
		return SHIM_ERROR, err
	}
	prop, err := sdkutils.BuildJoinChannelProposal(identity, genesisBlockbytes)
	if err != nil {
		err := fmt.Errorf("[JoinChannel] Error building join channel proposal : %s", err)
		logger.Error(err)
		return SHIM_ERROR, err
	}

	propResponse, err := pc.SendProposal(prop)
	logger.Debugf("[JoinChannel] proposal response: %s", propResponse.String())
	if err != nil {
		err := fmt.Errorf("[JoinChannel] Error sending proposal to the peer [%s] : %s", peer, err)
		logger.Error(err)
		return SHIM_ERROR, err
	}
	if propResponse.Response.Status != SHIM_OK {
		err := fmt.Errorf("[JoinChannel] Error from proposal response payload: %s", propResponse.Response.Message)
		logger.Error(err)
		return propResponse.Response.Status, err
	}
	return propResponse.Response.Status, nil
}

func (sdk *sdkImpl) InstallChaincode(peer string, spec *ChaincodeInstallSpec) (shimStatusCode int32, err error) {
	err = checkCCInstallArgs(*spec)
	if err != nil {
		logger.Error("[InstallChaincode] Check args error: %s", err)
		return SHIM_ERROR, err
	}
	pc, err := sdk.GetPeerClient(peer)
	if err != nil {
		err := fmt.Errorf("[InstallChaincode] error: %s", err)
		logger.Error(err)
		return SHIM_ERROR, err
	}

	codePackageBytes, err := ioutil.ReadFile(spec.CodeGzFile)
	if err != nil {
		err := fmt.Errorf("[InstallChaincode] read chaincode tar.gz file error: %s", err)
		logger.Error(err)
		return SHIM_ERROR, err
	}

	identity,err := sdk.mspManager.GetSigningIdentity(peer)
	if err != nil {
		err := fmt.Errorf("[InstallChaincode] GetSigningIdentity error: %s", err)
		logger.Error(err)
		return SHIM_ERROR, err
	}
	signedProp, err := sdkutils.BuildChaincodeInstallProposal(identity, CHAINCODE_TYPE, spec.CCName, spec.CCVersion, spec.CCPath, codePackageBytes)
	if err != nil {
		err := fmt.Errorf("[InstallChaincode] Error creating signed proposal: %s", err)
		logger.Error(err)
		return SHIM_ERROR, err
	}

	propResponse, err := pc.SendProposal(signedProp)
	if err != nil {
		err := fmt.Errorf("[InstallChaincode] Error endorsing: %s", err)
		logger.Error(err)
		return SHIM_ERROR, err
	}
	logger.Debugf("[InstallChaincode] proposal response: %s", propResponse.String())

	if propResponse.Response.Status != SHIM_OK {
		err := fmt.Errorf("[InstallChaincode] Error from proposal response payload: %s", propResponse.Response.Message)
		logger.Error(err)
		return propResponse.Response.Status, err
	}
	return propResponse.Response.Status, nil
}

func (sdk *sdkImpl) InstantiateChaincode(peer, orderer string, spec *ChaincodeDeploySpec) (status string, err error) {
	err = checkCCDeployArgs(*spec)
	if err != nil {
		logger.Errorf("[InstantiateChaincode] Check args error: %s", err)
		return "", err
	}
	pc, err := sdk.GetPeerClient(peer)
	if err != nil {
		logger.Error(err)
		return "", fmt.Errorf("[InstantiateChaincode] error: %s", err)
	}

	oc, err := sdk.GetOrdererClient(orderer)
	if err != nil {
		logger.Error(err)
		return "", fmt.Errorf("[InstallChaincode] error: %s", err)
	}

	identity,err := sdk.mspManager.GetSigningIdentity(peer)
	if err != nil {
		logger.Error(err)
		return "", fmt.Errorf("[InstallChaincode] error: %s", err)
	}
	signedProp, prop, err := sdkutils.BuildChaincodeDeployProposal(identity, CHAINCODE_TYPE, spec.Channel, spec.CCName, spec.CCVersion, spec.CCPath, spec.Policy, spec.Args, false)
	if err != nil {
		err := fmt.Errorf("[InstantiateChaincode] Error creating signed proposal: %s", err)
		logger.Error(err)
		return "", err
	}

	proposalResponse, err := pc.SendProposal(signedProp)
	if err != nil {
		err := fmt.Errorf("[InstantiateChaincode] Error endorsing: %s", err)
		logger.Error(err)
		return "", err
	}
	if proposalResponse != nil && proposalResponse.Response.Status >= SHIM_ERRORTHRESHOLD {
		err := fmt.Errorf("[InstantiateChaincode] Error from proposal response: %s", proposalResponse.Response.Message)
		logger.Error(err)
		return "", err
	}
	logger.Debugf("[InstantiateChaincode] proposal response: %s", proposalResponse.String())

	if proposalResponse != nil {
		// assemble a signed transaction (it's an Envelope message)
		env, err := sdkutils.CreateSignedTx(identity, prop, proposalResponse)
		if err != nil {
			err := fmt.Errorf("[InstantiateChaincode] Could not assemble transaction, err %s", err)
			logger.Error(err)
			return "", err
		}
		result, err := oc.SendBroadCast(env)
		if err != nil {
			err := fmt.Errorf("[InstantiateChaincode] Broadcast error: %s", err)
			logger.Error(err)
			return "", err
		}
		return result.String(), nil
	} else {
		err := fmt.Errorf("[InstantiateChaincode] ProposalResponse is nil!")
		logger.Error(err)
		return "", err
	}
}

func (sdk *sdkImpl) UpgradeChaincode(peer, orderer string, spec *ChaincodeDeploySpec) (status string, err error) {
	err = checkCCDeployArgs(*spec)
	if err != nil {
		logger.Errorf("[UpgradeChaincode] Check args error: %s", err)
		return "", err
	}
	pc, err := sdk.GetPeerClient(peer)
	if err != nil {
		logger.Error(err)
		return "", fmt.Errorf("[UpgradeChaincode] error: %s", err)
	}

	oc, err := sdk.GetOrdererClient(orderer)
	if err != nil {
		logger.Error(err)
		return "", fmt.Errorf("[UpgradeChaincode] error: %s", err)
	}

	identity,err := sdk.mspManager.GetSigningIdentity(peer)
	if err != nil {
		logger.Error(err)
		return "", fmt.Errorf("[UpgradeChaincode] error: %s", err)
	}
	signedProp, prop, err := sdkutils.BuildChaincodeDeployProposal(identity, CHAINCODE_TYPE, spec.Channel, spec.CCName, spec.CCVersion, spec.CCPath, spec.Policy, spec.Args, true)
	if err != nil {
		err := fmt.Errorf("[UpgradeChaincode] Error creating signed proposal: %s", err)
		logger.Error(err)
		return "", err
	}

	proposalResponse, err := pc.SendProposal(signedProp)
	if err != nil {
		err := fmt.Errorf("[UpgradeChaincode] Error endorsing: %s", err)
		logger.Error(err)
		return "", err
	}
	if proposalResponse != nil && proposalResponse.Response.Status >= SHIM_ERRORTHRESHOLD {
		err := fmt.Errorf("[InstantiateChaincode] Error from proposal response: %s", proposalResponse.Response.Message)
		logger.Error(err)
		return "", err
	}
	logger.Debugf("[UpgradeChaincode] proposal response: %s", proposalResponse.String())

	if proposalResponse != nil {
		// assemble a signed transaction (it's an Envelope message)
		env, err := sdkutils.CreateSignedTx(identity, prop, proposalResponse)
		if err != nil {
			err := fmt.Errorf("[UpgradeChaincode] Could not assemble transaction, err %s", err)
			logger.Error(err)
			return "", err
		}
		result, err := oc.SendBroadCast(env)
		if err != nil {
			err := fmt.Errorf("[UpgradeChaincode] broadcast error: %s", err)
			logger.Error(err)
			return "", err
		}
		return result.String(), nil
	} else {
		err := fmt.Errorf("[UpgradeChaincode] proposalResponse is nil!")
		logger.Error(err)
		return "", err
	}
}

func (sdk *sdkImpl) Invoke(orderer string, endorsers []string, spec *ChaincodeInvokeSpec, async bool) (result *ChaincodeInvokeResult, err error) {
	err = checkCCInvokeArgs(*spec)
	if err != nil {
		logger.Errorf("[Invoke] Check args error: %s", err)
		return nil, err
	}
	if orderer == "" {
		logger.Errorf("Invalid orderer : %s", orderer)
		return nil, fmt.Errorf("Invalid orderer : %s", orderer)
	}
	if len(endorsers) == 0 {
		logger.Errorf("Empty endorsers ! Chaincode invoking needs one endorser at least : %s", endorsers)
		return nil, fmt.Errorf("Empty endorsers ! Chaincode invoking needs one endorser at least : %s", endorsers)
	}
	pcs := make([]*peer.PeerClient, 0)
	for _, endorser := range endorsers {
		pc, ok := sdk.peers[endorser]
		if ok {
			pcs = append(pcs, pc)
		} else {
			logger.Errorf("Fail to get peer client by name '%s'", endorser)
			return nil, fmt.Errorf("Fail to get peer client by name '%s'", endorser)
		}
	}

	identity,err := sdk.mspManager.GetDefaultSigningIdentity()
	if err != nil {
		logger.Errorf("[Invoke] GetDefaultSigningIdentity error: %s", err)
		return nil, err
	}
	signedProposal, prop, txid, err := sdkutils.BuildChaincodeInvokeProposal(identity, CHAINCODE_TYPE, spec.Channel, spec.CCName, spec.Fcn, spec.Args, spec.TransientMap)
	if err != nil {
		logger.Errorf("Error building chaincode invoking proposal : %s", err)
		return nil, err
	}

	// ----- 1. send proposal to endorses -----
	var resps []*pb.ProposalResponse
	var errortmp error
	for _, pc := range pcs {
		res, err := pc.SendProposal(signedProposal)
		errortmp = err
		if err != nil {
			logger.Warnf("Endorsing failed at endorser '%s' : %s", pc.Address(), err)
			continue
		}
		if res != nil && res.Response.Status >= SHIM_ERRORTHRESHOLD {
			logger.Warnf("[Invoke] Error from proposal response: %s", res.Response.Message)
			continue
		}
		logger.Debugf("[Invoke] Response returned from [%s] is: %s", pc.Address(), res.String())
		resps = append(resps, res)
	}
	if len(resps) == 0 {
		return nil, fmt.Errorf("[Invoke] %s", errortmp.Error())
	}
	// ----- 2. make envelope -----
	env, err := sdkutils.CreateSignedTx(identity, prop, resps...)
	if err != nil {
		logger.Errorf("Error creating signed transaction envelope : %s", err)
		return nil, err
	}
	oc, ok := sdk.orderers[orderer]
	if !ok {
		logger.Error("Error getting orderer client by name : %s", orderer)
		return nil, fmt.Errorf("Error getting orderer client by name : %s", orderer)
	}

	// ----- 3. send env and wait for event -----
	if !async {
		eh, err := sdk.getEventHandler()
		if err != nil {
			err := fmt.Errorf("[Invoke] Get event handler error: %s", err)
			logger.Error(err)
			return nil, err
		}
		if !eh.IsConnect() {
			err := eh.Reconnect()
			if err != nil {
				logger.Errorf("[Invoke] Event handler reconnect error: %s", err)
				return nil, err
			}
		}
		ch := eh.RegisterTxEvent(txid)
		defer eh.UnregisterTxEvent(txid)

		_, err = oc.SendBroadCast(env)
		if err != nil {
			logger.Errorf("Error sending broadcast to the orderer '%s' : %s", oc.Address(), err)
			return nil, err
		}

		select {
		case txInfo := <-ch:
			if txInfo.Txid != txid {
				return nil, fmt.Errorf("[Invoke] Txid is not same with the one returned from event handler which is: %s,should be: %s", txInfo.Txid, txid)
			}
			logger.Infof("[Inovke] Received tx [%s] event result: %s", txInfo.Txid, txInfo.ValidationCode)
			return &ChaincodeInvokeResult{TxID: txInfo.Txid, Status: txInfo.ValidationCode}, nil
		case <-time.After(sdk.config.GetEventTimeoutConfig().TxResponse):
			logger.Warn("[Invoke] Timeout waiting for event, txid: %s", txid)
			return &ChaincodeInvokeResult{TxID: txid, Status: "TIMEOUT_WAITING_EVENT"}, nil
		}
	} else {
		_, err = oc.SendBroadCast(env)
		if err != nil {
			logger.Errorf("Error sending broadcast to the orderer '%s' : %s", oc.Address(), err)
			return nil, err
		}
	}

	return &ChaincodeInvokeResult{TxID: txid, Status: "UNKNOWN"}, nil
}

func (sdk *sdkImpl) Query(peer string, spec *ChaincodeInvokeSpec) (result []byte, err error) {
	logger.Debug("==============================  Start QueryChaincode  ================================")
	defer logger.Debug("==============================  Complete QueryChaincode  ================================")
	if peer == "" {
		logger.Errorf("Invalid peer : %s", peer)
		return nil, fmt.Errorf("Invalid peer : %s", peer)
	}
	if err := checkCCInvokeArgs(*spec); err != nil {
		logger.Errorf("Invalid ChaincodeInvokeSpec : %s", err)
		return nil, err
	}
	pc, err := sdk.GetPeerClient(peer)
	if err != nil {
		logger.Errorf("Fail to get peer client by name '%s'", peer)
		return nil, fmt.Errorf("Fail to get peer client by name '%s'", peer)
	}

	identity, err := sdk.mspManager.GetSigningIdentity(peer)
	if err != nil {
		logger.Errorf("[Query] GetSigningIdentity error: %s", err)
		return nil, err
	}
	signedProp, _, _, err := sdkutils.BuildChaincodeInvokeProposal(identity, CHAINCODE_TYPE, spec.Channel, spec.CCName, spec.Fcn, spec.Args, spec.TransientMap)
	if err != nil {
		logger.Errorf("Error building ChaincodeInvokeProposal : %s", err)
		return nil, err
	}

	resp, err := pc.SendProposal(signedProp)
	if err != nil {
		logger.Errorf("Error sending proposal to the peer '%s' : %s", pc.Address(), err)
		return nil, fmt.Errorf("[Query] %s", err)
	}
	if resp.Response.Status >= SHIM_ERRORTHRESHOLD {
		err := fmt.Errorf("[Query] Error from proposal response: %s", resp.Response.Message)
		logger.Error(err)
		return nil, err
	}
	return resp.Response.Payload, nil
}

func (sdk *sdkImpl) QueryTx(peer, channel, txid string) (result *ChaincodeInvokeResult, err error) {
	if txid == "" {
		logger.Errorf("Invalid transaction id : %s", txid)
		return nil, fmt.Errorf("Invalid transaction id  : %s", txid)
	}
	if channel == "" {
		logger.Errorf("invalid channel name : %s", channel)
		return nil, fmt.Errorf("Invalid channel name : %s", channel)
	}
	if peer == "" {
		logger.Errorf("Invalid peer : %s", peer)
		return nil, fmt.Errorf("Invalid peer : %s", peer)

	}
	pc, err := sdk.GetPeerClient(peer)
	if err != nil {
		logger.Errorf("Fail to get peer client by name '%s'", peer)
		return nil, fmt.Errorf("Fail to get peer client by name '%s'", peer)
	}

	identity, err := sdk.mspManager.GetSigningIdentity(peer)
	if err != nil {
		logger.Errorf("[QueryTx] GetSigningIdentity error: %s", err)
		return nil, err
	}
	signedProposal, err := sdkutils.BuildQueryTxProposal(identity, channel, txid)
	if err != nil {
		logger.Errorf("Error building the QueryTxProposal : %s", err)
		return nil, err
	}
	res, err := pc.SendProposal(signedProposal)
	if err != nil {
		logger.Errorf("Error sending proposal : %s", err)
		return nil, err
	}
	if res.Response.Status >= SHIM_ERRORTHRESHOLD {
		return nil, fmt.Errorf("[QueryTx] Error from proposal response: %s", res.Response.Message)
	}

	logger.Debugf("[QueryTx] response from peer [%s] is: %s", pc.Address(), res.String())

	status, err := sdkutils.UnmarshalValidateCodeFromProposalResponse(res)
	if err != nil {
		err := fmt.Errorf("[QueryTx] Get validate code error: %s", err)
		logger.Error(err)
		return nil, err
	}

	return &ChaincodeInvokeResult{TxID: txid, Status: status}, nil
}

func (sdk *sdkImpl) GetPeerClient(peerName string) (*peer.PeerClient, error) {
	var err error
	pc, ok := sdk.peers[peerName]
	if !ok {
		pc, err = peer.NewPeerClientFromConf(peerName, sdk.config)
		if err != nil {
			return nil, err
		}
		sdk.peers[peerName] = pc
	}
	return pc, nil
}

func (sdk *sdkImpl) GetOrdererClient(ordererName string) (*orderer.OrdererClient, error) {
	var err error
	oc, ok := sdk.orderers[ordererName]
	if !ok {
		oc, err = orderer.NewOrdererFromConf(ordererName, sdk.config)
		if err != nil {
			return nil, err
		}
		sdk.orderers[ordererName] = oc
	}
	return oc, nil
}

func (sdk *sdkImpl) InitOrdererClientsFromConf() {
	sdk.orderers = make(map[string]*orderer.OrdererClient)
	var err error
	orderers := sdk.config.GetAllOrdererConfig()
	for name, _ := range orderers {
		_, err = sdk.GetOrdererClient(name)
		if err != nil {
			logger.Errorf("Init orderer clients form config error: %s", err)
		}
	}
}

func (sdk *sdkImpl) InitPeerClientsFromConf() {
	sdk.peers = make(map[string]*peer.PeerClient)
	var err error
	peers := sdk.config.GetAllPeerConfig()
	for name, _ := range peers {
		_, err = sdk.GetPeerClient(name)
		if err != nil {
			logger.Errorf("Init peer clients form config error: %s", err)
		}
	}
}

// 随机选择一个orderer
func (sdk *sdkImpl) RandOrderer() *orderer.OrdererClient {
	if len(sdk.orderers) == 0 {
		return nil
	}
	randI := rand.Intn(len(sdk.orderers))
	targetI := 0
	guarantee := &orderer.OrdererClient{}

	for _, o := range sdk.orderers {
		guarantee = o
		if targetI == randI {
			break
		}
		targetI = targetI + 1
	}
	logger.Debugf("[RandOrderer] Get orderer: %s, address: %s", guarantee.Name(), guarantee.Address())
	return guarantee
}

// 随机选择一个orderer
func (sdk *sdkImpl) RandPeer() *peer.PeerClient {
	if len(sdk.peers) == 0 {
		return nil
	}
	randI := rand.Intn(len(sdk.peers))
	targetI := 0
	guarantee := &peer.PeerClient{}

	for _, o := range sdk.peers {
		guarantee = o
		if targetI == randI {
			break
		}
		targetI = targetI + 1
	}
	logger.Debugf("[RandPeer] Get peer: %s, address: %s", guarantee.Name(), guarantee.Address())
	return guarantee
}

func (sdk *sdkImpl) getEventHandler() (*event.EventHandler, error) {
	pc, err := sdk.GetPeerClient(sdk.config.GetEventConfig().Source)
	if err != nil {
		return nil, err
	}

	identity, err := sdk.mspManager.GetDefaultSigningIdentity()
	if err != nil {
		return nil, fmt.Errorf("[getEventHandler] GetDefaultSigningIdentity error: %s", err)
	}
	eh, err := event.GetEventHandler(pc, sdk.config, identity)
	if err != nil {
		return nil, fmt.Errorf("Get event handler error: %s", err)
	}
	return eh, nil
}

func (sdk *sdkImpl) RegisterChaincodeEvent(ccid, eventName string) <-chan ChaincodeEvent {
	resultCh := make(chan ChaincodeEvent)
	go func() {
		defer close(resultCh)
		eh, err := sdk.getEventHandler()
		if err != nil {
			resultCh <- ChaincodeEvent{
				Error:       err,
				ChaincodeID: ccid,
			}
			return
		}
		if !eh.IsConnect() {
			err := eh.Reconnect()
			if err != nil {
				resultCh <- ChaincodeEvent{
					Error:       err,
					ChaincodeID: ccid,
				}
				return
			}
		}

		ch,_ := eh.RegitsterChaincodeEvent(ccid,"test",[]string{"111"})

		for {
			ccEventInfo := <-ch
			resultCh <- ChaincodeEvent{
				Error:       nil,
				ChaincodeID: ccEventInfo.ChaincodeID,
				ChannelID:   ccEventInfo.ChannelID,
				TxID:        ccEventInfo.TxID,
				EventName:   ccEventInfo.EventName,
				Payload:     string(ccEventInfo.Payload),
			}
		}
	}()
	return resultCh
}

func (sdk *sdkImpl) GetTpsOutput() (<-chan uint64, error) {
	eh, err := sdk.getEventHandler()
	if err != nil {
		return nil, err
	}
	return eh.GetTpsOutput()
}

func (sdk *sdkImpl) AllPeer() []string {
	peers := []string{}
	if len(sdk.peers) == 0 {
		return peers
	}

	for _, o := range sdk.peers {
		tmpname := o.Name()
		peers = append(peers, tmpname)
	}
	return peers
}

func (sdk *sdkImpl) AllOrderer() []string {
	orderers := []string{}
	if len(sdk.orderers) == 0 {
		return orderers
	}

	for _, o := range sdk.orderers {
		tmpname := o.Name()
		orderers = append(orderers, tmpname)
	}
	return orderers
}

func (sdk *sdkImpl) GetMspManager() *msp.MspManager {
	if sdk.mspManager == nil {
		panic("Msp manager has not been inited!")
	}
	return sdk.mspManager
}

//根据channel名称及chaincode名称获取statedb中数据
//返回数据为：KV列表
/*
type KV struct {
	Key string
	Value string
}
*/
func (sdk *sdkImpl) GetStateDBByCC(peer,channel,chaincode string) (result []byte,err error) {
    if chaincode == "" {
        logger.Errorf("Invalid chaincode name: %s",chaincode)
        return nil,fmt.Errorf("Invalid chaincode name: %s",chaincode)
    }
    if channel == "" {
		logger.Errorf("invalid channel name : %s", channel)
		return nil, fmt.Errorf("Invalid channel name : %s", channel)
	}
	if peer == "" {
		logger.Errorf("Invalid peer : %s", peer)
		return nil, fmt.Errorf("Invalid peer : %s", peer)
	}

    pc, err := sdk.GetPeerClient(peer)
	if err != nil {
		logger.Errorf("Fail to get peer client by name '%s'", peer)
		return nil, fmt.Errorf("Fail to get peer client by name '%s'", peer)
	}

	identity, err := sdk.mspManager.GetSigningIdentity(peer)
	if err != nil {
		logger.Errorf("GetSigningIdentity error: %s", err)
		return nil, fmt.Errorf("GetSigningIdentity error: %s", err)
	}
	signedProposal, err := sdkutils.BuildGetStateDBByCCProposal(identity,channel,chaincode)
	if err != nil {
		logger.Errorf("Error building the GetStateDBByCCProposal : %s", err)
		return nil, err
	}
	res, err := pc.SendProposal(signedProposal)
	if err != nil {
		logger.Errorf("Error sending proposal : %s", err)
		return nil, err
	}
	if res.Response.Status >= SHIM_ERRORTHRESHOLD {
		return nil, fmt.Errorf("[GetStateDBByCC] Error from proposal response: %s", res.Response.Message)
	}

	logger.Debugf("[GetStateDBByCC] response from peer [%s] is: %s", pc.Address(), res.String())
	return res.Response.Payload,nil
}

//根据channel名称及chaincode名称获取某个key的所有历史数据
//返回数据为：KeyModification列表
/*
type KeyModification struct {
	TxId      string
	Value     []byte                    
	Timestamp *google_protobuf.Timestamp
	IsDelete  bool                       
}
*/
func (sdk *sdkImpl) GetHistoryByKey(peer,channel,chaincode,key string) (result []byte,err error) {
    if key == "" {
        logger.Errorf("Invalid key name: %s",key)
        return nil,fmt.Errorf("Invalid key name: %s",key)
    }
    if chaincode == "" {
        logger.Errorf("Invalid chaincode name: %s",chaincode)
        return nil,fmt.Errorf("Invalid chaincode name: %s",chaincode)
    }
    if channel == "" {
		logger.Errorf("invalid channel name : %s", channel)
		return nil, fmt.Errorf("Invalid channel name : %s", channel)
	}
	if peer == "" {
		logger.Errorf("Invalid peer : %s", peer)
		return nil, fmt.Errorf("Invalid peer : %s", peer)
	}

    pc, err := sdk.GetPeerClient(peer)
	if err != nil {
		logger.Errorf("Fail to get peer client by name '%s'", peer)
		return nil, fmt.Errorf("Fail to get peer client by name '%s'", peer)
	}

	identity, err := sdk.mspManager.GetSigningIdentity(peer)
	if err != nil {
		logger.Errorf("GetSigningIdentity error: %s", err)
		return nil, fmt.Errorf("GetSigningIdentity error: %s", err)
	}
	signedProposal, err := sdkutils.BuildGetHistoryByKeyProposal(identity,channel,chaincode,key)
	if err != nil {
		logger.Errorf("Error building the GetHistoryByKeyProposal : %s", err)
		return nil, err
	}
	res, err := pc.SendProposal(signedProposal)
	if err != nil {
		logger.Errorf("Error sending proposal : %s", err)
		return nil, err
	}
	if res.Response.Status >= SHIM_ERRORTHRESHOLD {
		return nil, fmt.Errorf("[GetHistoryByKey] Error from proposal response: %s", res.Response.Message)
	}

	logger.Debugf("[GetHistoryByKey] response from peer [%s] is: %s", pc.Address(), res.String())
	return res.Response.Payload,nil
}




//mike websocket
func (sdk *sdkImpl)RegisterWsConn(uuid string,conn interface{}) error {
	eh, err := sdk.getEventHandler()
	if err!=nil{
		return err
	}
	eh.SetWsConn(uuid,conn)
	return nil
}


func (sdk *sdkImpl)RegisterEvent(ccName,uuid_dappuid string,eventnames []string) int {
	//mike 在此处启动event模块
	eh, err := sdk.getEventHandler()
	if err != nil {
		panic(err.Error())
	}
	var que sync.WaitGroup
	que.Add(1)
	var errch chan int
	var errcode int
	var ch = make(<-chan event.ChaincodeEventInfo)
	go func() {
		//mike 注册感兴趣的event名称
		ch,errch = eh.RegitsterChaincodeEvent(ccName,uuid_dappuid,eventnames)
		errcode = <-errch
		que.Done()
		for {
			select {
			case data := <-ch:
				logger.Infof("data is :%v",data)
			}
		}
	}()
	que.Wait()
	return errcode
}



func (sdk *sdkImpl)GetEventHandler() interface{} {
	eh, err := sdk.getEventHandler()
	if err!=nil{
		return err
	}
	return eh
}

func (sdk *sdkImpl)GetCcInfo(peer,channel string,args ...string) (interface{},string,error) {
	pc, err := sdk.GetPeerClient(peer)
	if err != nil{
		logger.Error(err)
		return nil,"",err
	}
	identity,err := sdk.mspManager.GetDefaultSigningIdentity()
	if err != nil{
		logger.Error(err)
		return nil,"",err
	}
	prop,err := sdkutils.BuildGetCcInfoProposal(identity,channel,args...)
	if err != nil{
		logger.Error(err)
		return nil,"",err
	}
	propResponse, err := pc.SendProposal(prop)

	if err != nil {
		err := fmt.Errorf("[GetCcInfo] Error sending proposal to the peer [%s] : %s", peer, err)
		logger.Error(err)
		return SHIM_ERROR,"", err
	}


	chaincodeInfo := &pb.ChaincodeInfo{}
	err = proto.Unmarshal(propResponse.Response.GetPayload(), chaincodeInfo)
	if err != nil {
		logger.Infof("Failed unmarshaling message:", err)
		return SHIM_ERROR,"",err
	}


	if propResponse.Response.Status != SHIM_OK {
		err := fmt.Errorf("[GetCcInfo] Error from proposal response payload: %s", propResponse.Response.Message)
		logger.Error(err)
		return propResponse.Response.Status,"", err
	}
	return propResponse.Response.Status,chaincodeInfo.GetVersion(), nil

}
