/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package rest_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hyperledger-labs/fabric-smart-client/platform/view/services/rest"
	"github.com/hyperledger-labs/fabric-smart-client/platform/view/services/rest/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type Fruit struct {
	Name     string
	Quantity int
}

type FruitBasket struct {
	Fruits []string
}

func TestHttpHandler(t *testing.T) {
	l, err := zap.NewDevelopment()
	require.NoError(t, err)

	h := rest.NewHttpHandler(l.Sugar())

	rh := &mocks.FakeRequestHandler{}
	rh.HandleRequestStub = func(ctx *rest.ReqContext) (interface{}, int) {
		query := ctx.Query.(*Fruit)

		var res FruitBasket
		for i := 0; i < query.Quantity; i++ {
			res.Fruits = append(res.Fruits, query.Name)
		}

		require.Equal(t, ctx.Vars["Fruit"], "pineapple")

		return res, 200
	}

	rh.ParsePayloadStub = func(payload []byte) (interface{}, error) {
		var f Fruit
		err := json.Unmarshal(payload, &f)
		require.NoError(t, err)
		return &f, nil
	}

	h.RegisterURI("/test/{Fruit}", "PUT", rh)

	resp := httptest.NewRecorder()
	pineappleRequest := bytes.NewBufferString(`{"Name": "pineapple", "Quantity": 3}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/test/pineapple", pineappleRequest)
	h.ServeHTTP(resp, req)

	expectedPineappleResponse := FruitBasket{Fruits: []string{"pineapple", "pineapple", "pineapple"}}
	var actualResponse FruitBasket
	json.Unmarshal(resp.Body.Bytes(), &actualResponse)
	require.Equal(t, expectedPineappleResponse, actualResponse)
}
