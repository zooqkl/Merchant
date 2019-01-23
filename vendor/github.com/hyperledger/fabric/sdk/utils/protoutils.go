package utils

import (
	"github.com/golang/protobuf/proto"
	pcommon "github.com/hyperledger/fabric/protos/common"
	pb "github.com/hyperledger/fabric/protos/peer"
	putils "github.com/hyperledger/fabric/protos/utils"
	"fmt"
)

func UnmarshalValidateCodeFromProposalResponse(response *pb.ProposalResponse) (string, error) {
	processedTx := &pb.ProcessedTransaction{}
	err := proto.Unmarshal(response.Response.Payload, processedTx)
	if err != nil {
		return "", err
	}
	return pb.TxValidationCode(processedTx.ValidationCode).String(), nil
}

func UnmarshalChainInfoFromProposalResponse(response *pb.ProposalResponse) (*pcommon.BlockchainInfo, error) {
	chainInfo := &pcommon.BlockchainInfo{}
	err := proto.Unmarshal(response.Response.Payload, chainInfo)
	if err != nil {
		return nil, err
	}
	return chainInfo, nil
}

func UnmarshalChannelsFromProposalResponse(response *pb.ProposalResponse) ([]*pb.ChannelInfo, error) {
	channelsInfo := &pb.ChannelQueryResponse{}
	err := proto.Unmarshal(response.Response.Payload, channelsInfo)
	if err != nil {
		return nil, err
	}
	return channelsInfo.Channels, nil
}

func UnmarshalBlockFromProposalResponse(response *pb.ProposalResponse) (*pcommon.Block, error) {
	block := &pcommon.Block{}
	err := proto.Unmarshal(response.Response.Payload, block)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func MarshalBlockToByte(block *pcommon.Block) ([]byte, error) {
	return putils.Marshal(block)
}



func GetTxChannelHeaderFromDataByte(data []byte) (*pcommon.ChannelHeader, error) {
	env, err := putils.GetEnvelopeFromBlock(data)
	if err != nil {
		return nil, fmt.Errorf("[GetTxChannelHeaderFromDataByte] Get envelope from block error: %s", err)
	}

	pl := &pcommon.Payload{}
	proto.Unmarshal(env.Payload,pl)

	payload, err := putils.GetPayload(env)
	if err != nil {
		return nil, fmt.Errorf("[GetTxChannelHeaderFromDataByte] Get payload from env error: %s", err)
	}

	chH := &pcommon.ChannelHeader{}
	err = proto.Unmarshal(payload.GetHeader().GetChannelHeader(), chH)

	if err != nil {
		return nil, fmt.Errorf("[GetTxChannelHeaderFromDataByte] Unmarshal channel header error: %s", err)
	}
	return chH, nil
}


func GetTxChannelSignatureFromDataByte(data []byte) ([]byte, error) {
	env, err := putils.GetEnvelopeFromBlock(data)
	if err != nil {
		return nil, fmt.Errorf("[GetTxChannelHeaderFromDataByte] Get envelope from block error: %s", err)
	}

	return env.Signature, nil
}