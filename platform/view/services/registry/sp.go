/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package registry

import (
	"reflect"
	"sync"

	"github.com/pkg/errors"

	"github.com/hyperledger-labs/fabric-smart-client/platform/view/services/flogging"
)

var logger = flogging.MustGetLogger("view-sdk.eregistry")

type serviceProvider struct {
	services   []interface{}
	serviceMap map[reflect.Type]interface{}
	lock       sync.Mutex
}

func New() *serviceProvider {
	return &serviceProvider{
		services:   []interface{}{},
		serviceMap: map[reflect.Type]interface{}{},
	}
}

func (sp *serviceProvider) GetService(v interface{}) (interface{}, error) {
	sp.lock.Lock()
	defer sp.lock.Unlock()

	var typ reflect.Type
	switch t := v.(type) {
	case reflect.Type:
		typ = t
	default:
		typ = reflect.TypeOf(v)
	}

	service, ok := sp.serviceMap[typ]
	if !ok {
		switch typ.Kind() {
		case reflect.Struct:
			// nothing to do here
		default:
			typ = typ.Elem()
		}
		switch typ.Kind() {
		case reflect.Interface:
			for _, s := range sp.services {
				if reflect.TypeOf(s).Implements(typ) {
					sp.serviceMap[typ] = s
					return s, nil
				}
			}
		default:
			for _, s := range sp.services {
				if typ.AssignableTo(reflect.TypeOf(s).Elem()) {
					sp.serviceMap[typ] = s
					return s, nil
				}
			}
		}
		return nil, errors.Errorf("service [%s/%s] not found in [%v]", typ.PkgPath(), typ.Name(), sp.String())
	}
	return service, nil
}

func (sp *serviceProvider) RegisterService(service interface{}) error {
	sp.lock.Lock()
	defer sp.lock.Unlock()

	logger.Debugf("Register Service [%s]", getIdentifier(service))
	sp.services = append(sp.services, service)

	return nil
}

func (sp *serviceProvider) String() string {
	res := "services ["
	for _, service := range sp.services {
		res += getIdentifier(service) + ", "
	}
	return res + "]"
}

func getIdentifier(v interface{}) string {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.PkgPath() + "/" + t.Name()
}
