package ces

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/make-software/casper-go-sdk/casper"
	"github.com/make-software/casper-go-sdk/types/key"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/make-software/ces-go-parser/utils/mocks"
)

func TestEventParser(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockedClient := mocks.NewMockClient(mockCtrl)

	contractHashToParse, err := casper.NewHash("ea0c001d969da098fefec42b141db88c74c5682e49333ded78035540a0b4f0bc")
	assert.NoError(t, err)

	contractPackageHash, err := casper.NewContractPackageHash("7a5fce1d9ad45c9d71a5e59638602213295a51a6cf92518f8b262cd3e23d6d7e")
	assert.NoError(t, err)

	eventParser := EventParser{
		casperClient: mockedClient,
	}

	t.Run("Test several events parsing", func(t *testing.T) {
		var schemaHex = `08000000100000004164646564546f57686974656c6973740100000007000000616464726573730b0e00000042616c6c6f7443616e63656c65640500000005000000766f7465720b09000000766f74696e675f6964040b000000766f74696e675f74797065030600000063686f69636503050000007374616b65080a00000042616c6c6f74436173740500000005000000766f7465720b09000000766f74696e675f6964040b000000766f74696e675f74797065030600000063686f69636503050000007374616b65080c0000004f776e65724368616e67656401000000090000006e65775f6f776e65720b1400000052656d6f76656446726f6d57686974656c6973740100000007000000616464726573730b1300000053696d706c65566f74696e67437265617465640c0000000d000000646f63756d656e745f686173680a0700000063726561746f720b050000007374616b650d0809000000766f74696e675f69640416000000636f6e6669675f696e666f726d616c5f71756f72756d041b000000636f6e6669675f696e666f726d616c5f766f74696e675f74696d650514000000636f6e6669675f666f726d616c5f71756f72756d0419000000636f6e6669675f666f726d616c5f766f74696e675f74696d650516000000636f6e6669675f746f74616c5f6f6e626f61726465640822000000636f6e6669675f646f75626c655f74696d655f6265747765656e5f766f74696e6773001d000000636f6e6669675f766f74696e675f636c6561726e6573735f64656c7461082e000000636f6e6669675f74696d655f6265747765656e5f696e666f726d616c5f616e645f666f726d616c5f766f74696e67050e000000566f74696e6743616e63656c65640300000009000000766f74696e675f6964040b000000766f74696e675f747970650308000000756e7374616b6573110b080b000000566f74696e67456e6465640d00000009000000766f74696e675f6964040b000000766f74696e675f74797065030d000000766f74696e675f726573756c74030e0000007374616b655f696e5f6661766f72080d0000007374616b655f616761696e73740816000000756e626f756e645f7374616b655f696e5f6661766f720815000000756e626f756e645f7374616b655f616761696e7374080e000000766f7465735f696e5f6661766f72040d000000766f7465735f616761696e73740408000000756e7374616b657311130b0408060000007374616b657311130b0408050000006275726e7311130b0408050000006d696e747311130b0408`

		hash, _ := casper.NewHash("002596e815c7235dccf76358695de0088b4636ecb2473c12bb5ff0fbbb7ae94a")
		mockedClient.EXPECT().GetStateRootHashLatest(context.Background()).Return(casper.ChainGetStateRootHashResult{StateRootHash: hash}, nil)
		eventUref, err := key.NewKey("uref-d2263e86f497f42e405d5d1390aa3c1a8bfc35f3699fdc3be806a5cfe139dac9-007")
		assert.NoError(t, err)
		eventSchemaUref, err := key.NewKey("uref-12263e86f497f42e405d5d1390aa3c1a8bfc35f3699fdc3be806a5cfe139dac9-007")
		assert.NoError(t, err)
		mockedClient.EXPECT().GetStateItem(context.Background(), "002596e815c7235dccf76358695de0088b4636ecb2473c12bb5ff0fbbb7ae94a", fmt.Sprintf("hash-%s", contractHashToParse.ToHex()), nil).Return(casper.StateGetItemResult{
			StoredValue: casper.StoredValue{
				Contract: &casper.Contract{
					ContractPackageHash: contractPackageHash,
					NamedKeys: casper.NamedKeys{
						casper.NamedKey{
							Name: eventNamedKey,
							Key:  eventUref,
						}, casper.NamedKey{
							Name: eventSchemaNamedKey,
							Key:  eventSchemaUref,
						}},
				},
			},
		}, nil)

		var arg casper.Argument
		err = json.Unmarshal([]byte(fmt.Sprintf(`{"cl_type": "Any", "bytes": "%s"}`, schemaHex)), &arg)
		require.NoError(t, err)

		mockedClient.EXPECT().GetStateItem(context.Background(), "002596e815c7235dccf76358695de0088b4636ecb2473c12bb5ff0fbbb7ae94a", "uref-12263e86f497f42e405d5d1390aa3c1a8bfc35f3699fdc3be806a5cfe139dac9-007", nil).Return(
			casper.StateGetItemResult{
				StoredValue: casper.StoredValue{
					CLValue: &arg,
				},
			}, nil)

		contractsMetadata, err := eventParser.loadContractsMetadata([]casper.Hash{contractHashToParse})
		require.NoError(t, err)

		eventParser.contractsMetadata = contractsMetadata

		var res casper.InfoGetDeployResult

		data, err := os.ReadFile("./utils/fixtures/deploys/voting_created.json")
		assert.NoError(t, err)

		err = json.Unmarshal(data, &res)
		assert.NoError(t, err)

		parseResults, err := eventParser.ParseExecutionResults(res.ExecutionResults[0].Result)
		assert.NoError(t, err)
		require.True(t, len(parseResults) == 2)

		assert.Equal(t, parseResults[0].Event.Name, "BallotCast")
		assert.Equal(t, parseResults[0].Event.ContractHash.String(), contractHashToParse.String())
		assert.Equal(t, parseResults[0].Event.ContractPackageHash.String(), contractPackageHash.String())
		assert.True(t, len(parseResults[0].Event.Data) > 0)

		assert.Equal(t, parseResults[1].Event.Name, "SimpleVotingCreated")
		assert.Equal(t, parseResults[1].Event.ContractHash.String(), contractHashToParse.String())
		assert.Equal(t, parseResults[1].Event.ContractPackageHash.String(), contractPackageHash.String())
		assert.True(t, len(parseResults[1].Event.Data) > 0)
	})
}

func TestParseEventAndData(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	contractHashToParse, err := casper.NewHash("ea0c001d969da098fefec42b141db88c74c5682e49333ded78035540a0b4f0bc")
	assert.NoError(t, err)

	mockedClient := mocks.NewMockClient(mockCtrl)

	eventParser := EventParser{
		casperClient: mockedClient,
	}

	var schemaHex = `08000000100000004164646564546f57686974656c6973740100000007000000616464726573730b0e00000042616c6c6f7443616e63656c65640500000005000000766f7465720b09000000766f74696e675f6964040b000000766f74696e675f74797065030600000063686f69636503050000007374616b65080a00000042616c6c6f74436173740500000005000000766f7465720b09000000766f74696e675f6964040b000000766f74696e675f74797065030600000063686f69636503050000007374616b65080c0000004f776e65724368616e67656401000000090000006e65775f6f776e65720b1400000052656d6f76656446726f6d57686974656c6973740100000007000000616464726573730b1300000053696d706c65566f74696e67437265617465640c0000000d000000646f63756d656e745f686173680a0700000063726561746f720b050000007374616b650d0809000000766f74696e675f69640416000000636f6e6669675f696e666f726d616c5f71756f72756d041b000000636f6e6669675f696e666f726d616c5f766f74696e675f74696d650514000000636f6e6669675f666f726d616c5f71756f72756d0419000000636f6e6669675f666f726d616c5f766f74696e675f74696d650516000000636f6e6669675f746f74616c5f6f6e626f61726465640822000000636f6e6669675f646f75626c655f74696d655f6265747765656e5f766f74696e6773001d000000636f6e6669675f766f74696e675f636c6561726e6573735f64656c7461082e000000636f6e6669675f74696d655f6265747765656e5f696e666f726d616c5f616e645f666f726d616c5f766f74696e67050e000000566f74696e6743616e63656c65640300000009000000766f74696e675f6964040b000000766f74696e675f747970650308000000756e7374616b6573110b080b000000566f74696e67456e6465640d00000009000000766f74696e675f6964040b000000766f74696e675f74797065030d000000766f74696e675f726573756c74030e0000007374616b655f696e5f6661766f72080d0000007374616b655f616761696e73740816000000756e626f756e645f7374616b655f696e5f6661766f720815000000756e626f756e645f7374616b655f616761696e7374080e000000766f7465735f696e5f6661766f72040d000000766f7465735f616761696e73740408000000756e7374616b657311130b0408060000007374616b657311130b0408050000006275726e7311130b0408050000006d696e747311130b0408`

	var arg casper.Argument
	err = json.Unmarshal([]byte(fmt.Sprintf(`{"cl_type": "Any", "bytes": "%s"}`, schemaHex)), &arg)
	require.NoError(t, err)

	hash, _ := casper.NewHash("002596e815c7235dccf76358695de0088b4636ecb2473c12bb5ff0fbbb7ae94a")
	mockedClient.EXPECT().GetStateRootHashLatest(context.Background()).Return(casper.ChainGetStateRootHashResult{StateRootHash: hash}, nil)
	mockedClient.EXPECT().GetStateItem(context.Background(), "002596e815c7235dccf76358695de0088b4636ecb2473c12bb5ff0fbbb7ae94a", fmt.Sprintf("hash-%s", contractHashToParse.ToHex()), []string{eventSchemaNamedKey}).Return(
		casper.StateGetItemResult{
			StoredValue: casper.StoredValue{
				CLValue: &arg,
			},
		}, nil)

	contractSchemaBytes, err := eventParser.FetchContractSchemasBytes(contractHashToParse)
	assert.NoError(t, err)

	schema, err := NewSchemasFromBytes(contractSchemaBytes)
	assert.NoError(t, err)

	eventHex := "420000003e000000100000006576656e745f42616c6c6f74436173740056befc13a6fd62e18f361700a5e08f966901c34df8041b36ec97d54d605c23de00000000000102e8030e0320000000d2263e86f497f42e405d5d1390aa3c1a8bfc35f3699fdc3be806a5cfe139dac90100000032"

	eventName, eventData, err := ParseEventNameAndData(eventHex, schema)
	assert.NoError(t, err)
	assert.Equal(t, eventName, "BallotCast")
	assert.True(t, len(eventData) > 0)
}
