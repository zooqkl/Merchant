package orderer

import (
	"context"
	"fmt"
	"github.com/hyperledger/fabric/core/comm"
	"github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	sdkclient "github.com/hyperledger/fabric/sdk/client"
	"github.com/hyperledger/fabric/sdk/config"
	"github.com/hyperledger/fabric/sdk/logging"
	"google.golang.org/grpc"
	"time"
)

var logger sdklogging.Logger

type OrdererClient struct {
	name       string
	grpcClient *comm.GRPCClient
	cfg        config.OrdererConfig
	// one OrdererClient only maintain one grpcBroadcastConn/grpcDiliverConn,get a new OrdererClient if async request needed
	grpcBroadcastConn *grpc.ClientConn
	grpcDiliverConn   *grpc.ClientConn
}

func (oc *OrdererClient) Address() string {
	return oc.cfg.GrpcConfig.Address
}

func (oc *OrdererClient) Name() string {
	return oc.name
}

func NewOrdererFromConf(name string, cf *config.ConfigFactory) (*OrdererClient, error) {
	logger = sdklogging.GetLogger()
	cfg := cf.GetOrdererConfig(name)

	oc := &OrdererClient{name: name}
	grpcClient, err := sdkclient.NewGRPCClient(name, cf)
	if err != nil {
		return nil, fmt.Errorf("New grpc client error: %s", err)
	}
	oc.grpcClient = grpcClient
	oc.cfg = cfg
	return oc, nil
}

// Broadcast returns a broadcast client for the AtomicBroadcast service
func (oc *OrdererClient) Broadcast() (ab.AtomicBroadcast_BroadcastClient, error) {
	var err error
	if !sdkclient.IsConnStatusOK(oc.grpcBroadcastConn) {
		oc.grpcBroadcastConn, err = oc.newGrpcConn()
		if err != nil {
			return nil, err
		}
	}
	// TODO: check to see if we should actually handle error before returning
	return ab.NewAtomicBroadcastClient(oc.grpcBroadcastConn).Broadcast(context.TODO())
}

// Deliver returns a deliver client for the AtomicBroadcast service
func (oc *OrdererClient) Deliver() (ab.AtomicBroadcast_DeliverClient, error) {
	var err error
	if !sdkclient.IsConnStatusOK(oc.grpcDiliverConn) {
		oc.grpcDiliverConn, err = oc.newGrpcConn()
		if err != nil {
			return nil, err
		}
	}
	// TODO: check to see if we should actually handle error before returning
	return ab.NewAtomicBroadcastClient(oc.grpcDiliverConn).Deliver(context.TODO())

}

func (oc *OrdererClient) CloseBroadcastConnection() error {
	err := oc.grpcBroadcastConn.Close()
	oc.grpcBroadcastConn = nil
	if err != nil {
		return fmt.Errorf("Close grpc connection of orderer [%s] error: %s", oc.cfg.GrpcConfig.Address, err)
	}
	return nil
}

func (oc *OrdererClient) CloseDiliverConnection() error {
	err := oc.grpcDiliverConn.Close()
	oc.grpcDiliverConn = nil
	if err != nil {
		return fmt.Errorf("Close grpc connection of orderer [%s] error: %s", oc.cfg.GrpcConfig.Address, err)
	}
	return nil
}

func (oc *OrdererClient) newGrpcConn() (*grpc.ClientConn, error) {
	conn, err := oc.grpcClient.NewConnection(sdkclient.GrpcAddressFromUrl(oc.cfg.GrpcConfig.Address), oc.cfg.GrpcConfig.ServerNameOverride)
	if err != nil {
		return nil, fmt.Errorf("New grpc connection error: %s", err)
	}
	return conn, nil
}

// SendBroadCast
func (oc *OrdererClient) SendBroadCast(envelope *common.Envelope) (*common.Status, error) {
	bc, err := oc.Broadcast()
	if err != nil {
		return nil, fmt.Errorf("NewAtomicBroadcastClient failed: [%s]", err)
	}
	errCh := make(chan error, 1)
	statusCh := make(chan common.Status)
	go func() {
		defer func() {
			close(statusCh)
			close(errCh)
		}()
		res, err := bc.Recv()
		if err != nil {
			errCh <- fmt.Errorf("Error receiving the message from orderer: %s", err)
			return
		}
		if res.Status != common.Status_SUCCESS {
			errCh <- fmt.Errorf("Un-successful grpc res recevied from orderer : %s", res.Status)
			return
		}
		statusCh <- res.Status
	}()
	logger.Debugf("Sending 'Broadcast' grpc request to orderer : %s", oc.cfg.GrpcConfig.Address)
	if err = bc.Send(envelope); err != nil {
		return nil, fmt.Errorf("Error sending broadcast msg to orderer: %s", err)
	}
	if err = bc.CloseSend(); err != nil {
		logger.Warnf("Error closing the sending client : %s", err)
	}
	select {
	case s := <-statusCh:
		return &s, nil
	case err = <-errCh:
		return nil, err
	case <-time.After(60 * time.Second):
		return nil, fmt.Errorf("[OrdererClient.SendBroadCast] Timeout")
	}
}

// SendDeliver
func (oc *OrdererClient) SendDeliver(envelope *common.Envelope) (*common.Block, error) {
	dc, err := oc.Deliver()
	if err != nil {
		return nil, fmt.Errorf("grpc method 'Deliver' failed : [%s]", err)
	}
	responseCh := make(chan *common.Block)
	errCh := make(chan error)
	go func() {
		defer func() {
			close(responseCh)
			close(errCh)
		}()
		// seek block
		for {
			response, err := dc.Recv()
			if err != nil {
				errCh <- fmt.Errorf("Error receiving the block from orderer '%s' : %s", oc.cfg.GrpcConfig.Address, err)
				return
			}
			switch t := response.Type.(type) {
			case *ab.DeliverResponse_Status:
				logger.Debugf("Received deliver response status from ordering service: %s", t.Status)
				if t.Status != common.Status_SUCCESS {
					errCh <- fmt.Errorf("error status from ordering service : %v", t.Status)
				}
				return
			case *ab.DeliverResponse_Block:
				logger.Debug("Received block from ordering service")
				responseCh <- response.GetBlock()
			default:
				errCh <- fmt.Errorf("unknown reponse type from ordering service %T", t)
				return
			}
		}
	}()
	logger.Debugf("Requesting blocks from orderer '%s'", oc.cfg.GrpcConfig.Address)
	if err = dc.Send(envelope); err != nil {
		dc.CloseSend()
		return nil, fmt.Errorf("Error sending block seeking request to orderer '%s': %s", oc.cfg.GrpcConfig.Address, err)
	}
	if err = dc.CloseSend(); err != nil {
		logger.Warnf("unable to close deliver client [%s]", err)
	}
	select {
	case s := <-responseCh:
		return s, nil
	case err = <-errCh:
		return nil, err
	case <-time.After(60 * time.Second):
		return nil, fmt.Errorf("[OrdererClient.SendDeliver] Timeout")
	}
}
