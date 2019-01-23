/*
Copyright Yunphant Corp. All Rights Reserved.
*/
package event

import (
	"crypto/x509"
	"fmt"
	"io"
	"sync"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/core/comm"
	"github.com/hyperledger/fabric/events/consumer"
	fabmsp "github.com/hyperledger/fabric/msp"
	ehpb "github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/protos/utils"
	sdklogging "github.com/hyperledger/fabric/sdk/logging"
	"google.golang.org/grpc/credentials"
)

var consumerLogger = sdklogging.GetLogger()

//EventsClient holds the stream and adapter for consumer to work with
type EventsClient struct {
	sync.RWMutex

	grpcConfig      EventGrpcConfig

	regTimeout      time.Duration
	stream          ehpb.Events_ChatClient
	adapter         consumer.EventAdapter
	signingIdentity fabmsp.SigningIdentity
}

// RegistrationConfig holds the information to be used when registering for
// events from the eventhub
type RegistrationConfig struct {
	InterestedEvents []*ehpb.Interest
	Timestamp        *timestamp.Timestamp
	TlsCert          *x509.Certificate
}

type EventGrpcConfig struct {
	address            string
	serverNameOverride string
	tlsEnabled         bool
	certFile           string
}

//NewEventsClient Returns a new grpc.ClientConn to the configured local PEER.
func NewEventsClient(grpcConfig EventGrpcConfig, regTimeout time.Duration, adapter consumer.EventAdapter, identity fabmsp.SigningIdentity) (*EventsClient, error) {
	var err error
	if regTimeout < 100*time.Millisecond {
		regTimeout = 100 * time.Millisecond
		err = fmt.Errorf("regTimeout >= 0, setting to 100 msec")
	} else if regTimeout > 60*time.Second {
		regTimeout = 60 * time.Second
		err = fmt.Errorf("regTimeout > 60, setting to 60 sec")
	}

	return &EventsClient{sync.RWMutex{}, grpcConfig, regTimeout, nil, adapter, identity}, err
}

//newEventsClientConnectionWithAddress Returns a new grpc.ClientConn to the configured local PEER.
func newEventsClientConnectionWithAddress(eventAddress, serverNameOverride, certFile string, tlsEnabled bool) (*grpc.ClientConn, error) {
	if tlsEnabled {
		cred, err := credentials.NewClientTLSFromFile(certFile, serverNameOverride)
		if err != nil {
			return nil, fmt.Errorf("[newEventsClientConnectionWithAddress] make event client conn error: %s", err)
		}
		return comm.NewClientConnectionWithAddress(eventAddress, true, true, cred, nil)
	}
	return comm.NewClientConnectionWithAddress(eventAddress, true, false,
		nil, nil)
}

func (ec *EventsClient) send(emsg *ehpb.Event) error {
	ec.Lock()
	defer ec.Unlock()

	//pass the signer's cert to Creator
	signerCert, err := ec.signingIdentity.Serialize()
	if err != nil {
		return fmt.Errorf("fail to serialize the default signing identity, err %s", err)
	}
	emsg.Creator = signerCert

	signedEvt, err := utils.GetSignedEvent(emsg, ec.signingIdentity)
	if err != nil {
		return fmt.Errorf("could not sign outgoing event, err %s", err)
	}

	return ec.stream.Send(signedEvt)
}

// RegisterAsync - registers interest in a event and doesn't wait for a response
func (ec *EventsClient) RegisterAsync(config *RegistrationConfig) error {
	creator, err := ec.signingIdentity.Serialize()
	if err != nil {
		return fmt.Errorf("[Event - RegisterAsync] Getting creator from MSP error: %s", err)
	}
	emsg := &ehpb.Event{Event: &ehpb.Event_Register{Register: &ehpb.Register{Events: config.InterestedEvents}}, Creator: creator, Timestamp: config.Timestamp}

	if config.TlsCert != nil {
		emsg.TlsCertHash = util.ComputeSHA256(config.TlsCert.Raw)
	}
	if err = ec.send(emsg); err != nil {
		consumerLogger.Errorf("error on Register send %s\n", err)
	}
	return err
}

// register - registers interest in a event
func (ec *EventsClient) register(config *RegistrationConfig) error {
	var err error
	if err = ec.RegisterAsync(config); err != nil {
		return err
	}

	regChan := make(chan struct{})
	go func() {
		defer close(regChan)
		in, inerr := ec.stream.Recv()
		if inerr != nil {
			err = inerr
			return
		}
		switch in.Event.(type) {
		case *ehpb.Event_Register:
		case nil:
			err = fmt.Errorf("invalid nil object for register")
		default:
			err = fmt.Errorf("invalid registration object")
		}
	}()
	select {
	case <-regChan:
	case <-time.After(ec.regTimeout):
		err = fmt.Errorf("timeout waiting for registration")
	}
	return err
}

// UnregisterAsync - Unregisters interest in a event and doesn't wait for a response
func (ec *EventsClient) UnregisterAsync(ies []*ehpb.Interest) error {
	creator, err := ec.signingIdentity.Serialize()
	if err != nil {
		return fmt.Errorf("[Event - UnregisterAsync] Getting creator from MSP error: %s", err)
	}
	emsg := &ehpb.Event{Event: &ehpb.Event_Unregister{Unregister: &ehpb.Unregister{Events: ies}}, Creator: creator}

	if err = ec.send(emsg); err != nil {
		err = fmt.Errorf("error on unregister send %s\n", err)
	}

	return err
}

// Recv receives next event - use when client has not called Start
func (ec *EventsClient) Recv() (*ehpb.Event, error) {
	in, err := ec.stream.Recv()
	if err == io.EOF {
		// read done.
		if ec.adapter != nil {
			ec.adapter.Disconnected(nil)
		}
		return nil, err
	}
	if err != nil {
		if ec.adapter != nil {
			ec.adapter.Disconnected(err)
		}
		return nil, err
	}
	return in, nil
}
func (ec *EventsClient) processEvents() error {
	defer ec.stream.CloseSend()
	for {
		in, err := ec.stream.Recv()
		if err == io.EOF {
			// read done.
			if ec.adapter != nil {
				ec.adapter.Disconnected(nil)
			}
			return nil
		}
		if err != nil {
			if ec.adapter != nil {
				ec.adapter.Disconnected(err)
			}
			return err
		}
		if ec.adapter != nil {
			cont, err := ec.adapter.Recv(in)
			if !cont {
				return err
			}
		}
	}
}

//Start establishes connection with Event hub and registers interested events with it
func (ec *EventsClient) Start() error {
	conn, err := newEventsClientConnectionWithAddress(ec.grpcConfig.address, ec.grpcConfig.serverNameOverride, ec.grpcConfig.certFile, ec.grpcConfig.tlsEnabled)
	if err != nil {
		return fmt.Errorf("could not create client conn to %s:%s", ec.grpcConfig.address, err)
	}

	ies, err := ec.adapter.GetInterestedEvents()
	if err != nil {
		return fmt.Errorf("error getting interested events:%s", err)
	}

	if len(ies) == 0 {
		return fmt.Errorf("must supply interested events")
	}

	serverClient := ehpb.NewEventsClient(conn)
	ec.stream, err = serverClient.Chat(context.Background())
	if err != nil {
		return fmt.Errorf("could not create client conn to %s:%s", ec.grpcConfig.address, err)
	}

	regConfig := &RegistrationConfig{InterestedEvents: ies, Timestamp: util.CreateUtcTimestamp()}
	if err = ec.register(regConfig); err != nil {
		return err
	}

	go ec.processEvents()

	return nil
}

//Stop terminates connection with event hub
func (ec *EventsClient) Stop() error {
	if ec.stream == nil {
		// in case the steam/chat server has not been established earlier, we assume that it's closed, successfully
		return nil
	}
	return ec.stream.CloseSend()
}
