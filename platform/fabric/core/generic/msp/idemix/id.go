/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package idemix

import (
	"bytes"
	"fmt"
	"time"

	"github.com/hyperledger-labs/fabric-smart-client/platform/view/driver"

	"github.com/golang/protobuf/proto"
	m "github.com/hyperledger/fabric-protos-go/msp"
	"github.com/hyperledger/fabric/bccsp"
	"github.com/hyperledger/fabric/msp"
	"github.com/pkg/errors"

	"github.com/hyperledger-labs/fabric-smart-client/platform/fabric/core/generic/csp"
)

type identity struct {
	NymPublicKey bccsp.Key
	support      *support
	id           *msp.IdentityIdentifier
	Role         *m.MSPRole
	OU           *m.OrganizationUnit
	// associationProof contains cryptographic proof that this identity
	// belongs to the MSP id.provider, i.e., it proves that the pseudonym
	// is constructed from a secret key on which the CA issued a credential.
	associationProof []byte
}

func newIdentity(provider *support, NymPublicKey bccsp.Key, role *m.MSPRole, ou *m.OrganizationUnit, proof []byte) *identity {
	id := &identity{}
	id.NymPublicKey = NymPublicKey
	id.support = provider
	id.Role = role
	id.OU = ou
	id.associationProof = proof

	raw, err := NymPublicKey.Bytes()
	if err != nil {
		panic(fmt.Sprintf("unexpected condition, failed marshalling nym public key [%s]", err))
	}
	id.id = &msp.IdentityIdentifier{
		Mspid: provider.name,
		Id:    bytes.NewBuffer(raw).String(),
	}

	return id
}

func (id *identity) Anonymous() bool {
	return true
}

func (id *identity) ExpiresAt() time.Time {
	// Idemix MSP currently does not use expiration dates or revocation,
	// so we return the zero time to indicate this.
	return time.Time{}
}

func (id *identity) GetIdentifier() *msp.IdentityIdentifier {
	return id.id
}

func (id *identity) GetMSPIdentifier() string {
	return id.support.name
}

func (id *identity) GetOrganizationalUnits() []*msp.OUIdentifier {
	// we use the (serialized) public key of this MSP as the CertifiersIdentifier
	certifiersIdentifier, err := id.support.issuerPublicKey.Bytes()
	if err != nil {
		logger.Errorf("Failed to marshal ipk in GetOrganizationalUnits: %s", err)
		return nil
	}

	return []*msp.OUIdentifier{{CertifiersIdentifier: certifiersIdentifier, OrganizationalUnitIdentifier: id.OU.OrganizationalUnitIdentifier}}
}

func (id *identity) Validate() error {
	// logger.Debugf("Validating identity %+v", id)
	if id.GetMSPIdentifier() != id.support.name {
		return errors.Errorf("the supplied identity does not belong to this msp")
	}
	return id.verifyProof()
}

func (id *identity) Verify(msg []byte, sig []byte) error {
	_, err := id.support.csp.Verify(
		id.NymPublicKey,
		sig,
		msg,
		&csp.IdemixNymSignerOpts{
			IssuerPK: id.support.issuerPublicKey,
		},
	)
	return err
}

func (id *identity) SatisfiesPrincipal(principal *m.MSPPrincipal) error {
	panic("not implemented yet")
}

func (id *identity) Serialize() ([]byte, error) {
	serialized := &m.SerializedIdemixIdentity{}

	raw, err := id.NymPublicKey.Bytes()
	if err != nil {
		return nil, errors.Wrapf(err, "could not serialize nym of identity %s", id.id)
	}
	// This is an assumption on how the underlying idemix implementation work.
	// TODO: change this in future version
	serialized.NymX = raw[:len(raw)/2]
	serialized.NymY = raw[len(raw)/2:]
	ouBytes, err := proto.Marshal(id.OU)
	if err != nil {
		return nil, errors.Wrapf(err, "could not marshal OU of identity %s", id.id)
	}

	roleBytes, err := proto.Marshal(id.Role)
	if err != nil {
		return nil, errors.Wrapf(err, "could not marshal role of identity %s", id.id)
	}

	serialized.Ou = ouBytes
	serialized.Role = roleBytes
	serialized.Proof = id.associationProof

	idemixIDBytes, err := proto.Marshal(serialized)
	if err != nil {
		return nil, err
	}

	sID := &m.SerializedIdentity{Mspid: id.GetMSPIdentifier(), IdBytes: idemixIDBytes}
	idBytes, err := proto.Marshal(sID)
	if err != nil {
		return nil, errors.Wrapf(err, "could not marshal a SerializedIdentity structure for identity %s", id.id)
	}

	return idBytes, nil
}

func (id *identity) verifyProof() error {
	// Verify signature
	valid, err := id.support.csp.Verify(
		id.support.issuerPublicKey,
		id.associationProof,
		nil,
		&csp.IdemixSignerOpts{
			RevocationPublicKey: id.support.revocationPK,
			Attributes: []csp.IdemixAttribute{
				{Type: csp.IdemixBytesAttribute, Value: []byte(id.OU.OrganizationalUnitIdentifier)},
				{Type: csp.IdemixIntAttribute, Value: getIdemixRoleFromMSPRole(id.Role)},
				{Type: csp.IdemixHiddenAttribute},
				{Type: csp.IdemixHiddenAttribute},
			},
			RhIndex: rhIndex,
			Epoch:   id.support.epoch,
		},
	)
	if err == nil && !valid {
		panic("unexpected condition, an error should be returned for an invalid signature")
	}

	return err
}

type signingIdentity struct {
	*identity
	Cred         []byte
	UserKey      bccsp.Key
	NymKey       bccsp.Key
	enrollmentId string
}

func (id *signingIdentity) Sign(msg []byte) ([]byte, error) {
	// logger.Debugf("Idemix identity %s is signing", id.GetIdentifier())

	sig, err := id.support.csp.Sign(
		id.UserKey,
		msg,
		&csp.IdemixNymSignerOpts{
			Nym:      id.NymKey,
			IssuerPK: id.support.issuerPublicKey,
		},
	)
	if err != nil {
		return nil, err
	}
	return sig, nil
}

func (id *signingIdentity) GetPublicVersion() driver.Identity {
	return id.identity
}
