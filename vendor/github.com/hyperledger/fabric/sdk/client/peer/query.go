/*
Copyright Yunphant Corp. All Rights Reserved.
*/
package peer

import (
	"fmt"
	pcommon "github.com/hyperledger/fabric/protos/common"
	sdkutils "github.com/hyperledger/fabric/sdk/utils"
	fabmsp "github.com/hyperledger/fabric/msp"
)

func GetChainInfo(peerClient *PeerClient, channel string, identity fabmsp.SigningIdentity) (*pcommon.BlockchainInfo, error) {
	signedProp, err := sdkutils.BuildGetChainInfoProposal(identity, channel)
	if err != nil {
		return nil, fmt.Errorf("[GetChainInfo] Build getChaininfo proposal error: %s", err)
	}
	resp, err := peerClient.SendProposal(signedProp)
	if err != nil {
		return nil, fmt.Errorf("[GetChainInfo] Send proposal error: %s", err)
	}
	chainInfo, err := sdkutils.UnmarshalChainInfoFromProposalResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("[GetChainInfo] Unmarshal chain info from proposal response error: %s", err)
	}
	return chainInfo, nil
}

func GetChannels(peerClient *PeerClient, identity fabmsp.SigningIdentity) ([]string, error) {
	signedProp, err := sdkutils.BuildGetChannelsProposal(identity)
	if err != nil {
		return nil, fmt.Errorf("[GetChannels] Build getChannels proposal error: %s", err)
	}
	resp, err := peerClient.SendProposal(signedProp)
	if err != nil {
		return nil, fmt.Errorf("[GetChannels] Send proposal error: %s", err)
	}
	channels, err := sdkutils.UnmarshalChannelsFromProposalResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("[GetChannels] Unmarshal channels from proposal response error: %s", err)
	}
	channelIds := make([]string, 0)
	for _, channel := range channels {
		channelIds = append(channelIds, channel.ChannelId)
	}
	return channelIds, nil
}

func GetBlockByNumber(peerClient *PeerClient, channel string, number uint64, identity fabmsp.SigningIdentity) (*pcommon.Block, error) {
	signedProp, err := sdkutils.BuildGetBlockByNumberProposal(identity, channel, number)
	if err != nil {
		return nil, fmt.Errorf("[GetBlockByNumber] Build getBlockByNumber proposal error: %s", err)
	}
	resp, err := peerClient.SendProposal(signedProp)
	if err != nil {
		return nil, fmt.Errorf("[GetBlockByNumber] Send proposal error: %s", err)
	}
	block, err := sdkutils.UnmarshalBlockFromProposalResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("[GetBlockByNumber] Unmarshal block from proposal response error: %s", err)
	}
	return block, nil
}

func GetBlockByTxID(peerClient *PeerClient, channel string, txid string, identity fabmsp.SigningIdentity) (*pcommon.Block,error) {
	signedProp, err := sdkutils.BuildGetBlockByTxIDProposal(identity, channel, txid)
	if err != nil {
		return nil, fmt.Errorf("[GetBlockByTxID] Build getBlockByTxID proposal error: %s", err)
	}
	resp, err := peerClient.SendProposal(signedProp)
	if err != nil {
		return nil, fmt.Errorf("[GetBlockByTxID] Send proposal error: %s", err)
	}
	block, err := sdkutils.UnmarshalBlockFromProposalResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("[GetBlockByTxID] Unmarshal block from proposal response error: %s", err)
	}
	return block, nil
}
