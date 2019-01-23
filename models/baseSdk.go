package models

import (
	"github.com/hyperledger/fabric/sdk"
)

var sdkInstance sdk.FabricSDK
var (
	TicketChannelName   string = ""
	TicketChaincodeName string = ""
	TicketQueryFcName   string = "QueryTicketByNumber"
	TicketConsumeFcName string = "ConsumeTicket"
)

type TicketInvokeSpec struct {
	TicketFcName string
	Args         []string
}

func GetSdkInstance() (sdk.FabricSDK) {
	if sdkInstance == nil {
		sdk, err := sdk.NewSDK("../config/config_sdk.yaml")
		if err != nil {
			logger.Fatalf("Error initialize sdk : %s", err)
		}
		sdkInstance = sdk
	}
	return sdkInstance
}

func Invoke(spec TicketInvokeSpec) (result *sdk.ChaincodeInvokeResult, err error) {
	GetSdkInstance()
	return sdkInstance.Invoke(sdkInstance.RandOrderer().Name(), []string{sdkInstance.RandPeer().Name()}, &sdk.ChaincodeInvokeSpec{
		Channel: TicketChannelName,
		CCName:  TicketChaincodeName,
		Fcn:     spec.TicketFcName,
		Args:    spec.Args,
	}, false)
}

func Query(spec TicketInvokeSpec) (result []byte, err error) {
	GetSdkInstance()
	return sdkInstance.Query(sdkInstance.RandPeer().Name(), &sdk.ChaincodeInvokeSpec{
		Channel: TicketChannelName,
		CCName:  TicketChaincodeName,
		Fcn:     spec.TicketFcName,
		Args:    spec.Args,
	})
}
