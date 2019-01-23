package orderer

import (
	pcommon "github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/orderer"
	putils "github.com/hyperledger/fabric/protos/utils"
	sdkmsp "github.com/hyperledger/fabric/sdk/msp"
	"fmt"
)
func SeekNewest(ordererClient *OrdererClient,channel string, mspManager *sdkmsp.MspManager) (*pcommon.Block,error) {
	seekInfo := &orderer.SeekInfo{
		Start:    &orderer.SeekPosition{Type: &orderer.SeekPosition_Newest{Newest: &orderer.SeekNewest{}}},
		Stop:     &orderer.SeekPosition{Type: &orderer.SeekPosition_Newest{Newest: &orderer.SeekNewest{}}},
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	}

	env,err := putils.CreateSignedEnvelope(pcommon.HeaderType_DELIVER_SEEK_INFO,channel,mspManager.NewSigner("default"),seekInfo,0,0)
	if err != nil {
		return nil,fmt.Errorf("[SeekNewest] CreateSignedEnvelope error: %s",err)
	}
	return ordererClient.SendDeliver(env)
}