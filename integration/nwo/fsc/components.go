/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package fsc

type BuilderClient interface {
	Build(path string) string
}

type Builder struct {
	client BuilderClient
}

func (c *Builder) Cryptogen() string {
	return c.Build("github.com/hyperledger-labs/fabric-smart-client/cmd/cryptogen")
}

func (c *Builder) Build(path string) string {
	return c.client.Build(path)
}
