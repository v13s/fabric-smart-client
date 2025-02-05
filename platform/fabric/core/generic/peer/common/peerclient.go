/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"context"
	"crypto/tls"
	"io/ioutil"
	"time"

	"github.com/hyperledger-labs/fabric-smart-client/platform/view/services/grpc"
	"github.com/hyperledger/fabric-protos-go/discovery"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/core/config"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// PeerClient represents a client for communicating with a peer
type PeerClient struct {
	CommonClient
}

// NewPeerClientFromEnv creates an instance of a PeerClient from the global
// Viper instance
func NewPeerClientFromEnv() (*PeerClient, error) {
	address, override, clientConfig, err := configFromEnv("peer")
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load config for PeerClient")
	}
	return newPeerClientForClientConfig(address, override, clientConfig)
}

// NewPeerClientForAddress creates an instance of a PeerClient using the
// provided peer address and, if TLS is enabled, the TLS root cert file
func NewPeerClientForAddress(address, tlsRootCertFile string) (*PeerClient, error) {
	if address == "" {
		return nil, errors.New("peer address must be set")
	}

	override := viper.GetString("peer.tls.serverhostoverride")
	clientConfig := grpc.ClientConfig{}
	clientConfig.Timeout = viper.GetDuration("peer.client.connTimeout")
	if clientConfig.Timeout == time.Duration(0) {
		clientConfig.Timeout = defaultConnTimeout
	}

	secOpts := grpc.SecureOptions{
		UseTLS:            viper.GetBool("peer.tls.enabled"),
		RequireClientCert: viper.GetBool("peer.tls.clientAuthRequired"),
	}

	if secOpts.RequireClientCert {
		keyPEM, err := ioutil.ReadFile(config.GetPath("peer.tls.clientKey.file"))
		if err != nil {
			return nil, errors.WithMessage(err, "unable to load peer.tls.clientKey.file")
		}
		secOpts.Key = keyPEM
		certPEM, err := ioutil.ReadFile(config.GetPath("peer.tls.clientCert.file"))
		if err != nil {
			return nil, errors.WithMessage(err, "unable to load peer.tls.clientCert.file")
		}
		secOpts.Certificate = certPEM
	}
	clientConfig.SecOpts = secOpts

	if clientConfig.SecOpts.UseTLS {
		if tlsRootCertFile == "" {
			return nil, errors.New("tls root cert file must be set")
		}
		caPEM, res := ioutil.ReadFile(tlsRootCertFile)
		if res != nil {
			return nil, errors.WithMessagef(res, "unable to load TLS root cert file from %s", tlsRootCertFile)
		}
		clientConfig.SecOpts.ServerRootCAs = [][]byte{caPEM}
	}
	return newPeerClientForClientConfig(address, override, clientConfig)
}

func newPeerClientForClientConfig(address, override string, clientConfig grpc.ClientConfig) (*PeerClient, error) {
	gClient, err := grpc.NewGRPCClient(clientConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create PeerClient from config")
	}
	pClient := &PeerClient{
		CommonClient: CommonClient{
			Client:  gClient,
			Address: address,
			Sn:      override}}
	return pClient, nil
}

// TODO: improve by providing grpc connection pool
func (pc *PeerClient) Close() {
	go pc.CommonClient.Client.Close()
}

// Endorser returns a client for the Endorser service
func (pc *PeerClient) Endorser() (pb.EndorserClient, error) {
	conn, err := pc.CommonClient.NewConnection(pc.Address, grpc.ServerNameOverride(pc.Sn))
	if err != nil {
		return nil, errors.WithMessagef(err, "endorser client failed to connect to %s", pc.Address)
	}
	return pb.NewEndorserClient(conn), nil
}

func (pc *PeerClient) Discovery() (discovery.DiscoveryClient, error) {
	conn, err := pc.CommonClient.NewConnection(pc.Address, grpc.ServerNameOverride(pc.Sn))
	if err != nil {
		return nil, errors.WithMessagef(err, "discovery client failed to connect to %s", pc.Address)
	}
	return discovery.NewDiscoveryClient(conn), nil
}

// Deliver returns a client for the Deliver service
func (pc *PeerClient) Deliver() (pb.Deliver_DeliverClient, error) {
	conn, err := pc.CommonClient.NewConnection(pc.Address, grpc.ServerNameOverride(pc.Sn))
	if err != nil {
		return nil, errors.WithMessagef(err, "deliver client failed to connect to %s", pc.Address)
	}
	return pb.NewDeliverClient(conn).Deliver(context.TODO())
}

// PeerDeliver returns a client for the Deliver service for peer-specific use
// cases (i.e. DeliverFiltered)
func (pc *PeerClient) PeerDeliver() (pb.DeliverClient, error) {
	conn, err := pc.CommonClient.NewConnection(pc.Address, grpc.ServerNameOverride(pc.Sn))
	if err != nil {
		return nil, errors.WithMessagef(err, "deliver client failed to connect to %s", pc.Address)
	}
	return pb.NewDeliverClient(conn), nil
}

// Certificate returns the TLS client certificate (if available)
func (pc *PeerClient) Certificate() tls.Certificate {
	return pc.CommonClient.Certificate()
}

// GetEndorserClient returns a new endorser client. If the both the address and
// tlsRootCertFile are not provided, the target values for the client are taken
// from the configuration settings for "peer.address" and
// "peer.tls.rootcert.file"
func GetEndorserClient(address, tlsRootCertFile string) (pb.EndorserClient, error) {
	var peerClient *PeerClient
	var err error
	if address != "" {
		peerClient, err = NewPeerClientForAddress(address, tlsRootCertFile)
	} else {
		peerClient, err = NewPeerClientFromEnv()
	}
	if err != nil {
		return nil, err
	}
	return peerClient.Endorser()
}

// GetCertificate returns the client's TLS certificate
func GetCertificate() (tls.Certificate, error) {
	peerClient, err := NewPeerClientFromEnv()
	if err != nil {
		return tls.Certificate{}, err
	}
	return peerClient.Certificate(), nil
}

// GetDeliverClient returns a new deliver client. If both the address and
// tlsRootCertFile are not provided, the target values for the client are taken
// from the configuration settings for "peer.address" and
// "peer.tls.rootcert.file"
func GetDeliverClient(address, tlsRootCertFile string) (pb.Deliver_DeliverClient, error) {
	var peerClient *PeerClient
	var err error
	if address != "" {
		peerClient, err = NewPeerClientForAddress(address, tlsRootCertFile)
	} else {
		peerClient, err = NewPeerClientFromEnv()
	}
	if err != nil {
		return nil, err
	}
	return peerClient.Deliver()
}

// GetPeerDeliverClient returns a new deliver client. If both the address and
// tlsRootCertFile are not provided, the target values for the client are taken
// from the configuration settings for "peer.address" and
// "peer.tls.rootcert.file"
func GetPeerDeliverClient(address, tlsRootCertFile string) (pb.DeliverClient, error) {
	var peerClient *PeerClient
	var err error
	if address != "" {
		peerClient, err = NewPeerClientForAddress(address, tlsRootCertFile)
	} else {
		peerClient, err = NewPeerClientFromEnv()
	}
	if err != nil {
		return nil, err
	}
	return peerClient.PeerDeliver()
}
