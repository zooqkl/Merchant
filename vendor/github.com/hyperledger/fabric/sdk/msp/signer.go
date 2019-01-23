package msp

import (
	"fmt"
	"github.com/hyperledger/fabric/common/crypto"
	cb "github.com/hyperledger/fabric/protos/common"
	fabmsp "github.com/hyperledger/fabric/msp"
)

type mspSigner struct {
	fabmsp.MSP
}

// NewSignatureHeader creates a SignatureHeader with the correct signing identity and a valid nonce
func (s *mspSigner) NewSignatureHeader() (*cb.SignatureHeader, error) {
	signer, err := s.MSP.GetDefaultSigningIdentity()
	if err != nil {
		return nil, fmt.Errorf("Failed getting MSP-based signer [%s]", err)
	}

	creatorIdentityRaw, err := signer.Serialize()
	if err != nil {
		return nil, fmt.Errorf("Failed serializing creator public identity [%s]", err)
	}

	nonce, err := crypto.GetRandomNonce()
	if err != nil {
		return nil, fmt.Errorf("Failed creating nonce [%s]", err)
	}

	sh := &cb.SignatureHeader{}
	sh.Creator = creatorIdentityRaw
	sh.Nonce = nonce

	return sh, nil
}

// Sign a message which should embed a signature header created by NewSignatureHeader
func (s *mspSigner) Sign(message []byte) ([]byte, error) {
	signer, err := s.MSP.GetDefaultSigningIdentity()
	if err != nil {
		return nil, fmt.Errorf("Failed getting MSP-based signer [%s]", err)
	}

	signature, err := signer.Sign(message)
	if err != nil {
		return nil, fmt.Errorf("Failed generating signature [%s]", err)
	}

	return signature, nil
}