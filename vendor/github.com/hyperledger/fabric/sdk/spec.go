/*
Copyright Yunphant Corp. All Rights Reserved.
*/

package sdk

import (
	"fmt"
    "regexp"
)

type ChaincodeInvokeSpec struct {
	Channel      string            `json:"channel"`
	CCName       string            `json:"ccName"`
	Fcn          string            `json:"fcn"`
	Args         []string          `json:"args"`
	TransientMap map[string][]byte `json:"transientMap"`
}

type DappChaincodeInvokeSpec struct {
	Channel      string            `json:"channel" binding:"required"`
	CCName       string            `json:"ccName" binding:"required"`
	Fcn          string            `json:"fcn" binding:"required"`
	Args         []string          `json:"args" binding:"required"`
	Token        string            `json:"token" binding:"required"`
	TransientMap map[string][]byte `json:"transientMap"`
}

type ChainCode struct {
	Name string	      `json:"name"`
	Status string     `json:"status"`
}

type ChaincodeStatue struct {
	UserName string        `json:"username"`
	Cc       []ChainCode   `json:"cc"`
}

type ChaincodeManageUserSpec struct {
	Channel      string             `json:"channel"`
	CCName       string             `json:"ccName"`
	Fcn          string             `json:"fcn"`
	Args         ChaincodeStatue 	`json:"args"`
	TransientMap map[string][]byte  `json:"transientMap"`
}


type ChaincodeInstallSpec struct {
	CCName     string `json:"ccName"`
	CCVersion  string `json:"ccVersion"`
	CCPath     string `json:"ccPath"`
	CodeGzFile string `json:"codeGzFile"`
}

type ChaincodeDeploySpec struct {
	Channel   string   `json:"channel"`
	CCName    string   `json:"ccName"`
	CCVersion string   `json:"ccVersion"`
	CCPath    string   `json:"ccPath"`
	Args      []string `json:"args"`
	Policy    string   `json:"policy"`
}

type ChaincodeInvokeResult struct {
	TxID   string `json:"txid"`
	Status string `json:"status"`
	// Status should be:
	// 	"VALID",
	// 	"NIL_ENVELOPE",
	// 	"BAD_PAYLOAD",
	// 	"BAD_COMMON_HEADER",
	// 	"BAD_CREATOR_SIGNATURE",
	// 	"INVALID_ENDORSER_TRANSACTION",
	// 	"INVALID_CONFIG_TRANSACTION",
	// 	"UNSUPPORTED_TX_PAYLOAD",
	// 	"BAD_PROPOSAL_TXID",
	// 	"DUPLICATE_TXID",
	// 	"ENDORSEMENT_POLICY_FAILURE",
	// 	"MVCC_READ_CONFLICT",
	// 	"PHANTOM_READ_CONFLICT",
	// 	"UNKNOWN_TX_TYPE",
	// 	"TARGET_CHAIN_NOT_FOUND",
	// 	"MARSHAL_TX_ERROR",
	// 	"NIL_TXACTION",
	// 	"EXPIRED_CHAINCODE",
	// 	"CHAINCODE_VERSION_CONFLICT",
	// 	"BAD_HEADER_EXTENSION",
	// 	"BAD_CHANNEL_HEADER",
	// 	"BAD_RESPONSE_PAYLOAD",
	// 	"BAD_RWSET",
	// 	"ILLEGAL_WRITESET",
	// 	"INVALID_OTHER_REASON",

	//  "TIMEOUT_WAITING_EVENT", @@@ Should use txid query for actual result
	//  "UNKNOWN", @@@ Should use txid query for actual result
}

type ChaincodeEvent struct {
	Error       error  `json:"error"`
	ChaincodeID string `json:"chaincodeID"`
	TxID        string `json:"txid"`
	EventName   string `json:"eventName"`
	Payload     string `json:"payload"`
	ChannelID   string `json:"channelID"`
}

func checkCCInvokeArgs(spec ChaincodeInvokeSpec) error {
	if spec.Channel == "" {
		return fmt.Errorf("Channel name should not be empty!")
	}
	if spec.CCName == "" {
		return fmt.Errorf("Chaincode name should not be empty!")
	}
	if spec.Fcn == "" {
		return fmt.Errorf("Fcn should not be empty!")
	}


	return nil
}

func checkCCInstallArgs(spec ChaincodeInstallSpec) error {
	if spec.CCName == "" {
		return fmt.Errorf("Chaincode name should not be empty!")
	}
	if spec.CCVersion == "" {
		return fmt.Errorf("Chaincode version should not be empty!")
	}
	if spec.CCPath == "" {
		return fmt.Errorf("Chaincode path should not be empty!")
	}
	if spec.CodeGzFile == "" {
		return fmt.Errorf("Chaincode gz file path should not be empty!")
	}

    reg := regexp.MustCompile(".*\\.tar\\.gz")
    if !(reg.MatchString(spec.CodeGzFile)){
        return fmt.Errorf("chaincode file must be tar.gz file")
    }
    return nil
}

func checkCCDeployArgs(spec ChaincodeDeploySpec) error {
	if spec.Channel == "" {
		return fmt.Errorf("Channel name should not be empty!")
	}
	if spec.CCName == "" {
		return fmt.Errorf("Chaincode name should not be empty!")
	}
	if spec.CCVersion == "" {
		return fmt.Errorf("Chaincode version should not be empty!")
	}
	//if spec.CCPath == "" {
	//	return fmt.Errorf("Chaincode path should not be empty!")
	//}
    //ccn_match,_:=regexp.MatchString("^[a-zA-Z0-9-_]+$",spec.CCName)
    //if !ccn_match {
    //    return fmt.Errorf("ccname can only be from a-zA-Z0-9-_")
    //}
    //ccv_match,_:=regexp.MatchString("^[a-zA-Z0-9\\.]+$",spec.CCVersion)
    //if !ccv_match {
    //    return fmt.Errorf("ccversion can only consist of alphanumerics or .")
    //}
    return nil
}
