/*
Copyright Yunphant Corp. All Rights Reserved.
*/

package peer

import (
	"context"
	"fmt"
	"github.com/hyperledger/fabric/core/comm"
	pb "github.com/hyperledger/fabric/protos/peer"
	sdkclient "github.com/hyperledger/fabric/sdk/client"
	"github.com/hyperledger/fabric/sdk/config"
	"google.golang.org/grpc"
)

type PeerClient struct {
	name       string
	grpcClient *comm.GRPCClient
	cfg        config.PeerConfig
	// one PeerClient only maintain one grpcConn,get a new PeerClient if async request needed
	grpcConn *grpc.ClientConn
}

func NewPeerClientFromConf(name string, cf *config.ConfigFactory) (*PeerClient, error) {
	cfg := cf.GetPeerConfig(name)

	pc := &PeerClient{name: name}
	grpcClient, err := sdkclient.NewGRPCClient(name, cf)
	if err != nil {
		return nil, fmt.Errorf("New grpc client error: %s", err)
	}
	pc.grpcClient = grpcClient
	pc.cfg = cfg
	return pc, nil
}

func (pc *PeerClient) Address() string {
	return pc.cfg.GrpcConfig.Address
}

func (pc *PeerClient) Name() string {
	return pc.name
}

func (pc *PeerClient) EventAddress() string {
	return pc.cfg.EventAddress
}

func (pc *PeerClient) Endorser() (pb.EndorserClient, error) {
	var err error
	if !sdkclient.IsConnStatusOK(pc.grpcConn) {
		pc.grpcConn, err = pc.newGrpcConn()
		if err != nil {
			return nil, err
		}
	}
	return pb.NewEndorserClient(pc.grpcConn), nil
}

func (pc *PeerClient) SendProposal(proposal *pb.SignedProposal) (*pb.ProposalResponse, error) {
	endorser, err := pc.Endorser()
	if err != nil {
		return nil, fmt.Errorf("Get endorder error: %s", err)
	}
	response, err := endorser.ProcessProposal(context.Background(), proposal)
	if err != nil {
		return nil, fmt.Errorf("Process proposal failed : %s", err)
	}
	return response, nil
}

func (pc *PeerClient) newGrpcConn() (*grpc.ClientConn, error) {
	conn, err := pc.grpcClient.NewConnection(sdkclient.GrpcAddressFromUrl(pc.cfg.GrpcConfig.Address), pc.cfg.GrpcConfig.ServerNameOverride)
	if err != nil {
		return nil, fmt.Errorf("New grpc connection error: %s", err)
	}
	return conn, nil
}

func (pc *PeerClient) CloseConnection() error {
	err := pc.grpcConn.Close()
	pc.grpcConn = nil
	if err != nil {
		return fmt.Errorf("Close grpc connection of peer [%s] error: %s", pc.cfg.GrpcConfig.Address, err)
	}
	return nil
}
