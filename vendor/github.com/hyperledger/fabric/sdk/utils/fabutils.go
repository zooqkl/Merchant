package utils

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/cauthdsl"
	"github.com/hyperledger/fabric/common/configtx"
	"github.com/hyperledger/fabric/common/tools/configtxgen/localconfig"
	cutil "github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/core/scc/cscc"
	"github.com/hyperledger/fabric/core/scc/qscc"
	"github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	pb "github.com/hyperledger/fabric/protos/peer"
	putils "github.com/hyperledger/fabric/protos/utils"
	"github.com/hyperledger/fabric/sdk/logging"
	fabmsp "github.com/hyperledger/fabric/msp"
	"github.com/hyperledger/fabric/common/crypto"
	"io/ioutil"
	"strconv"
	"github.com/hyperledger/fabric/core/scc/lscc"
)

var logger sdklogging.Logger

func init() {
	logger = sdklogging.GetLogger()
}

func BuildChaincodeInstallProposal(identity fabmsp.SigningIdentity, ccType, ccName, ccVersion, ccPath string, ccpackage []byte) (*pb.SignedProposal, error) {
	spec := &pb.ChaincodeSpec{
		Type:        pb.ChaincodeSpec_Type(pb.ChaincodeSpec_Type_value[ccType]),
		ChaincodeId: &pb.ChaincodeID{Path: ccPath, Name: ccName, Version: ccVersion},
		Input:       &pb.ChaincodeInput{},
	}
	cds := &pb.ChaincodeDeploymentSpec{
		ChaincodeSpec: spec,
		CodePackage:   ccpackage,
	}

	creator, err := identity.Serialize()
	if err != nil {
		return nil, fmt.Errorf("[BuildChaincodeInstallProposal] identity Serialize error: %s", err)
	}

	prop, _, err := putils.CreateInstallProposalFromCDS(cds, creator)
	if err != nil {
		return nil, fmt.Errorf("[BuildChaincodeInstallProposal] Error creating proposal: %s", err)
	}

	signedProp, err := putils.GetSignedProposal(prop, identity)
	if err != nil {
		return nil, fmt.Errorf("[BuildChaincodeInstallProposal] Error Get signed proposal: %s", err)
	}
	return signedProp, nil
}

func BuildChaincodeDeployProposal(identity fabmsp.SigningIdentity, ccType, channelName, ccName, ccVersion, ccPath, policy string, args []string, upgrade bool) (*pb.SignedProposal, *pb.Proposal, error) {
	input := &pb.ChaincodeInput{Args: make([][]byte, 0)}
	for _, arg := range args {
		input.Args = append(input.Args, []byte(arg))
	}
	spec := &pb.ChaincodeSpec{
		Type:        pb.ChaincodeSpec_Type(pb.ChaincodeSpec_Type_value[ccType]),
		ChaincodeId: &pb.ChaincodeID{Path: ccPath, Name: ccName, Version: ccVersion},
		Input:       input,
	}
	cds := &pb.ChaincodeDeploymentSpec{ChaincodeSpec: spec}

	creator, err := identity.Serialize()
	if err != nil {
		return nil, nil, fmt.Errorf("[BuildChaincodeDeployProposal] identity Serialize error: %s", err)
	}

	var plc []byte
	if policy != "" {
		p, err := cauthdsl.FromString(policy)
		if err != nil {
			return nil, nil, fmt.Errorf("[BuildChaincodeDeployProposal] Invalid policy %s", policy)
		}
		plc, err = putils.Marshal(p)
		if err != nil {
			return nil, nil, fmt.Errorf("[BuildChaincodeDeployProposal] Marshal policy error: %s", err)
		}
	}
	var collectionCfg []byte
	var prop *pb.Proposal
	if upgrade {
		prop, _, err = putils.CreateUpgradeProposalFromCDS(channelName, cds, creator, plc, []byte("escc"), []byte("vscc"))
	} else {
		prop, _, err = putils.CreateDeployProposalFromCDS(channelName, cds, creator, plc, []byte("escc"), []byte("vscc"), collectionCfg)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("[BuildChaincodeDeployProposal] Error creating proposal: %s", err)
	}

	signedProp, err := putils.GetSignedProposal(prop, identity)
	if err != nil {
		return nil, nil, fmt.Errorf("[BuildChaincodeDeployProposal] Error creating signed proposal: %s", err)
	}
	return signedProp, prop, nil
}

// 创建broadcast给orderer的envelope
// TODO 有空的话可以搞成直接generate profile而不是从目录读取
func BuildCreateChannelEnvelope(signer crypto.LocalSigner, channelTxFile string, channelID string) (*common.Envelope, error) {
	if channelTxFile == "" {
		logger.Errorf("Invalid channelTxFile path : %s", channelTxFile)
		return nil, fmt.Errorf("Invalid channelTxFile path : %s", channelTxFile)
	}
	if channelID == "" || channelID == localconfig.TestChainID {
		logger.Errorf("Invalid channelID : %s", channelID)
		return nil, fmt.Errorf("Invalid channelID : %s", channelID)
	}
	chTxBytes, err := ioutil.ReadFile(channelTxFile)
	if err != nil {
		logger.Errorf("Error reading the channelTxFile : %s", err)
		return nil, err
	}
	chTxEnv, err := putils.UnmarshalEnvelope(chTxBytes)
	if err != nil {
		return nil, err
	}

	env, err := checkAndSignChannelConfigTx(signer, channelID, chTxEnv)
	if err != nil {
		logger.Errorf("Error processing the channel config tx : %s", err)
		return nil, err
	}
	return env, nil
}

func BuildSeekBlockEnvelope(signer crypto.LocalSigner,channelID string, startNum, stopNum uint64) (*common.Envelope, error) {
	startPos := &ab.SeekPosition{
		Type: &ab.SeekPosition_Specified{
			Specified: &ab.SeekSpecified{
				Number: startNum,
			},
		},
	}
	stopPos := &ab.SeekPosition{
		Type: &ab.SeekPosition_Specified{
			Specified: &ab.SeekSpecified{
				Number: stopNum,
			},
		},
	}
	seekInfo := &ab.SeekInfo{
		Start:    startPos,
		Stop:     stopPos,
		Behavior: ab.SeekInfo_BLOCK_UNTIL_READY,
	}

	env, err := putils.CreateSignedEnvelopeWithTLSBinding(common.HeaderType_DELIVER_SEEK_INFO, channelID, signer, seekInfo, int32(0), uint64(0), nil)
	if err != nil {
		return nil, fmt.Errorf("[BuildSeekBlockEnvelope] CreateSignedEnvelope error: %s", err)
	}
	return env, nil
}

func BuildChaincodeInvokeProposal(identity fabmsp.SigningIdentity, ccType, channelName, ccName, fcn string, args []string, transientmap map[string][]byte) (*pb.SignedProposal, *pb.Proposal, string, error) {
	input := make([][]byte, 0)
	input = append(input, []byte(fcn))
	for _, arg := range args {
		input = append(input, []byte(arg))
	}
	invocation := &pb.ChaincodeInvocationSpec{
		ChaincodeSpec: &pb.ChaincodeSpec{
			Type:        pb.ChaincodeSpec_Type(pb.ChaincodeSpec_Type_value[ccType]),
			ChaincodeId: &pb.ChaincodeID{Name: ccName},
			Input:       &pb.ChaincodeInput{Args: input},
		},
	}

	creator, err := identity.Serialize()
	if err != nil {
		logger.Errorf("Error serializing the creator : %s", err)
	}
	prop, txid, err := putils.CreateChaincodeProposalWithTransient(common.HeaderType_ENDORSER_TRANSACTION, channelName, invocation, creator, transientmap)
	if err != nil {
		logger.Errorf("Error creating  Chaincode invoking proposal : %s", err)
		return nil, nil, "", err
	}

	signedProp, err := putils.GetSignedProposal(prop, identity)
	if err != nil {
		return nil, nil, "", err
	}
	return signedProp, prop, txid, nil
}

func BuildQueryTxProposal(identity fabmsp.SigningIdentity, channel, txID string) (*pb.SignedProposal, error) {
	return buildQsccProposal(identity, qscc.GetTransactionByID, channel, txID)
}

// 创建Join channel的Proposal
func BuildJoinChannelProposal(identity fabmsp.SigningIdentity, genesisBlock []byte) (*pb.SignedProposal, error) {
	return buildCsccProposal(identity, cscc.JoinChain, genesisBlock)
}

func BuildGetChainInfoProposal(identity fabmsp.SigningIdentity, channel string) (*pb.SignedProposal, error) {
	return buildQsccProposal(identity, qscc.GetChainInfo, channel)
}

func BuildGetChannelsProposal(identity fabmsp.SigningIdentity) (*pb.SignedProposal, error) {
	return buildCsccProposal(identity, cscc.GetChannels)
}

func BuildGetBlockByNumberProposal(identity fabmsp.SigningIdentity, channel string, number uint64) (*pb.SignedProposal, error) {
	strNum := strconv.FormatUint(number, 10)
	return buildQsccProposal(identity, qscc.GetBlockByNumber, channel, strNum)
}

func buildCsccProposal(identity fabmsp.SigningIdentity, fcn string, args ...[]byte) (*pb.SignedProposal, error) {
	inputArgs := [][]byte{[]byte(fcn)}
	for _, arg := range args {
		inputArgs = append(inputArgs, arg)
	}
	invocation := &pb.ChaincodeInvocationSpec{ChaincodeSpec: &pb.ChaincodeSpec{
		Type:        pb.ChaincodeSpec_Type(pb.ChaincodeSpec_Type_value["GOLANG"]),
		ChaincodeId: &pb.ChaincodeID{Name: "cscc"},
		Input:       &pb.ChaincodeInput{Args: inputArgs},
	}}

	creator, err := identity.Serialize()
	if err != nil {
		logger.Errorf("Error serializing the creator : %s", err)
		return nil, err
	}

	prop, _, err := putils.CreateChaincodeProposal(common.HeaderType_CONFIG, "", invocation, creator)
	if err != nil {
		logger.Errorf("Error creating chaincode proposal : %s", err)
		return nil, err
	}
	signedProp, err := putils.GetSignedProposal(prop, identity)
	if err != nil {
		logger.Errorf("Error creating signed proposal : %s", err)
		return nil, err
	}
	return signedProp, nil
}

func buildQsccProposal(identity fabmsp.SigningIdentity, fcn, channel string, args ...string) (*pb.SignedProposal, error) {
	inputArgs := [][]byte{[]byte(fcn), []byte(channel)}
	for _, arg := range args {
		inputArgs = append(inputArgs, []byte(arg))
	}
	invocation := &pb.ChaincodeInvocationSpec{
		ChaincodeSpec: &pb.ChaincodeSpec{
			Type:        pb.ChaincodeSpec_Type(pb.ChaincodeSpec_Type_value["GOLANG"]),
			ChaincodeId: &pb.ChaincodeID{Name: "qscc"},
			Input:       &pb.ChaincodeInput{Args: inputArgs},
		}}

	creator, err := identity.Serialize()
	if err != nil {
		logger.Errorf("Error serializing the creator : %s", err)
		return nil, err
	}
	prop, _, err := putils.CreateChaincodeProposal(common.HeaderType_ENDORSER_TRANSACTION, "", invocation, creator)
	if err != nil {
		logger.Errorf("Error creating query tx proposal : %s", err)
		return nil, err
	}
	p, err := putils.GetSignedProposal(prop, identity)
	if err != nil {
		logger.Errorf("Error creating query tx proposal : %s", err)
		return nil, err
	}
	return p, nil
}

func CreateSignedTx(identity fabmsp.SigningIdentity,proposal *pb.Proposal, resps ...*pb.ProposalResponse) (*common.Envelope, error) {
	return putils.CreateSignedTx(proposal, identity, resps...)
}

func checkAndSignChannelConfigTx(signer crypto.LocalSigner, channelID string, env *common.Envelope) (*common.Envelope, error) {
	payload, err := putils.ExtractPayload(env)
	if err != nil {
		return nil, fmt.Errorf("[checkAndSignChannelConfigTx] Bad payload: %s", err)
	}
	if payload.Header == nil || payload.Header.ChannelHeader == nil {
		return nil, fmt.Errorf("[checkAndSignChannelConfigTx] Bad header!")
	}

	ch, err := putils.UnmarshalChannelHeader(payload.Header.ChannelHeader)
	if err != nil {
		return nil, fmt.Errorf("[checkAndSignChannelConfigTx] Could not unmarshal channel header: %s", err)
	}

	if ch.Type != int32(common.HeaderType_CONFIG_UPDATE) {
		return nil, fmt.Errorf("[checkAndSignChannelConfigTx] Bad type!")
	}

	if ch.ChannelId == "" {
		return nil, fmt.Errorf("[checkAndSignChannelConfigTx] Empty channel id!")
	}

	if channelID == "" {
		channelID = ch.ChannelId
	}

	if ch.ChannelId != channelID {
		return nil, fmt.Errorf("[checkAndSignChannelConfigTx] Mismatched channel ID %s != %s", ch.ChannelId, channelID)
	}

	configUpdateEnv, err := configtx.UnmarshalConfigUpdateEnvelope(payload.Data)
	if err != nil {
		return nil, fmt.Errorf("[checkAndSignChannelConfigTx] Bad config update env: %s", err)
	}

	sigHeader, err := signer.NewSignatureHeader()
	if err != nil {
		return nil, fmt.Errorf("[checkAndSignChannelConfigTx] Get signature header error: %s", err)
	}

	marSigHeader, err := proto.Marshal(sigHeader)
	if err != nil {
		return nil, fmt.Errorf("[checkAndSignChannelConfigTx] Marshal signature header error: %s", err)
	}
	configSig := &common.ConfigSignature{
		SignatureHeader: marSigHeader,
	}

	configSig.Signature, err = signer.Sign(cutil.ConcatenateBytes(configSig.SignatureHeader, configUpdateEnv.ConfigUpdate))
	if err != nil {
		return nil, fmt.Errorf("[checkAndSignChannelConfigTx] Sign message error: %s", err)
	}

	configUpdateEnv.Signatures = append(configUpdateEnv.Signatures, configSig)
	return putils.CreateSignedEnvelopeWithTLSBinding(common.HeaderType_CONFIG_UPDATE, channelID, signer, configUpdateEnv, 0, 0, nil)
}

func BuildGetHistoryByKeyProposal(identity fabmsp.SigningIdentity, channel, chaincode, key string) (*pb.SignedProposal, error) {
	return buildQsccProposal(identity, qscc.GetHistoryByKey, channel, chaincode, key)
}

func BuildGetStateDBByCCProposal(identity fabmsp.SigningIdentity, channel, chaincode string) (*pb.SignedProposal, error) {
	return buildQsccProposal(identity, qscc.GetStateDBByCC, channel, chaincode)
}

func BuildGetBlockByTxIDProposal(identity fabmsp.SigningIdentity, channel, txid string) (*pb.SignedProposal, error) {
	return buildQsccProposal(identity, qscc.GetBlockByTxID, channel, txid)
}






func BuildGetCcInfoProposal(identity fabmsp.SigningIdentity,channel string, args ...string) (*pb.SignedProposal, error) {
	return buildLsccProposal(identity, lscc.GETCCDATA, channel, args...)
}


func buildLsccProposal(identity fabmsp.SigningIdentity, fcn, channel string, args ...string) (*pb.SignedProposal, error) {
	inputArgs := [][]byte{[]byte(fcn), []byte(channel)}
	for _, arg := range args {
		inputArgs = append(inputArgs, []byte(arg))
	}
	invocation := &pb.ChaincodeInvocationSpec{
		ChaincodeSpec: &pb.ChaincodeSpec{
			Type:        pb.ChaincodeSpec_Type(pb.ChaincodeSpec_Type_value["GOLANG"]),
			ChaincodeId: &pb.ChaincodeID{Name: "lscc"},
			Input:       &pb.ChaincodeInput{Args: inputArgs},
		}}

	creator, err := identity.Serialize()
	if err != nil {
		logger.Errorf("Error serializing the creator : %s", err)
		return nil, err
	}
	prop, _, err := putils.CreateChaincodeProposal(common.HeaderType_ENDORSER_TRANSACTION, channel, invocation, creator)
	if err != nil {
		logger.Errorf("Error creating chaincode proposal : %s", err)
		return nil, err
	}
	p, err := putils.GetSignedProposal(prop, identity)
	if err != nil {
		logger.Errorf("Error get signed proposal : %s", err)
		return nil, err
	}
	return p, nil
}
