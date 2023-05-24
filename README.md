# Go CES Parser

`go-ces-parser` parses contract-level events that follow
the [Casper Event Standard](https://github.com/make-software/casper-event-standard).

The library is built on top of the 'casper-go-sdk' and operates on types defined by the SDK.

## Install

``
go get github.com/make-software/go-ces-parser
``

## Usage

Here is an example of parsing CES events using `go-ces-parser` from a real Testnet deploy loaded
with `casper-go-sdk`:

```
package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/make-software/casper-go-sdk/casper"

	ces "go-ces-parser"
)

func main() {
	testnetNodeAddress := "<put testnet node address here>"
	rpcClient := casper.NewRPCClient(casper.NewRPCHandler(testnetNodeAddress, http.DefaultClient))

	ctx := context.Background()
	deployResult, err := rpcClient.GetDeploy(ctx, "19ee17d9e3b4c1527b433598e647b69aa9a153864eb12433489f99224bfc9442")
	if err != nil {
		panic(err)
	}

	contractHash, err := casper.NewHash("e7062b42c9a22002fa3cd216debd605b7056ad180efb3c99555676f1a1e801e5")
	if err != nil {
		panic(err)
	}

	parser, err := ces.NewParser(rpcClient, []casper.Hash{contractHash})
	if err != nil {
		panic(err)
	}

	parseResults, err := parser.ParseExecutionResults(deployResult.ExecutionResults[0].Result)
	if err != nil {
		panic(err)
	}
	for _, result := range parseResults {
		if result.Error != nil {
			panic(err)
		}
		fmt.Println(result.Event)
	}
}
```

## API

Go CES Parser provides several public types and functions:

- [`Parser`](#Parser)
  - [`NewParser`](#NewParser)
  - [`Parser.ParseExecutionResults`](#ParseExecutionResults)
  - [`Parser.FetchContractSchemasBytes`](#FetchContractSchemasBytes)
- [`NewSchemasFromBytes`](#NewSchemasFromBytes)
- [`EventData`](#EventData)
- [`Event`](#Event)
  - [`ParseEventNameAndData`](#ParseEventNameAndData)
- [`ParseResult`](#ParseResult)
- [`Schemas`](#Schemas)
- [`SchemaData`](#SchemaData)

### `Parser`

Parser that accepts a list of observed contracts and provides possibility to parse CES events out of deploy execution
results

#### `NewParser`

`NewParser` constructor that accepts `casper-go-sdk` client:

| Argument          | Type               | Description                                |
|-------------------|--------------------|--------------------------------------------|
| `casperRPCClient` | `casper.RPCClient` | Instance of the `casper-go-sdk` RPC client |
| `contracts`       | `[]casper.Hash`    | List of the observed contract hashes       |

**Example**

```
rpcClient := casper.NewRPCClient(casper.NewRPCHandler("http://localhost:11101/rpc", http.DefaultClient))
contractHash, err := casper.NewHash("e7062b42c9a22002fa3cd216debd605b7056ad180efb3c99555676f1a1e801e5")

parser, err := ces.NewParser(rpc, []casper.Hash{contractHash})
```

#### `ParseExecutionResults`

`ParseExecutionResults` method that accepts deploy execution results and returns `[]ces.ParseResult`:

| Argument           | Type                      | Description                                                                      |                                           
|--------------------|---------------------------|----------------------------------------------------------------------------------|
| `executionResults` | `casper.ExecutionResults` | Deploy execution results provided as the corresponding type from `casper-go-sdk` |

#### `FetchContractSchemasBytes`

`FetchContractSchemasBytes` method that accepts contract hash and return bytes representation of stored schema:

| Argument       | Type          | Description                             |                                           
|----------------|---------------|-----------------------------------------|
| `contractHash` | `casper.Hash` | Contract hash schema want to be fetched |

### `NewSchemasFromBytes`

`NewSchemasFromBytes` constructor that accepts raw CES schema bytes stored under the contract `__events_schema` URef and
returns `ces.Schemas`:

| Argument     | Type     | Description                |         
|--------------|----------|----------------------------|
| `rawSchemas` | `[]byte` | Raw contract schemas bytes |

### `ParseEventNameAndData`

Function that accepts raw event bytes and contract event schemas and returns `ParseResult`:

| Argument  | Type          | Description                  |            
|-----------|---------------|------------------------------|
| `event`   | `string`      | Raw event bytes in hex       |
| `schemas` | `ces.Schemas` | The list of contract schemas |

**Example**

```
schemas, err := ces.NewSchemasFromBytes(rawSchemas)
rawEvent  := BytesFromString("some real example here")

eventData, err := ces.ParseEvent(rawEvent, schemas)
```

### `EventData`

Value-object that represents an event data:

| Property | Type                        | Description |
|----------|-----------------------------|-------------|
| `Name`   | `string`                    | Event name  |
| `Data`   | `map[string]casper.CLValue` | Event Data  |

### `Event`

Value-object that represents an event:

| Property              | Type                          | Description               |
|-----------------------|-------------------------------|---------------------------|
| `EventData`           | [`ces.EventData`](#EventData) | EventData                 |
| `ContractHash`        | `casper.Hash`                 | Event ContractHash        |
| `ContractPackageHash` | `casper.Hash`                 | Event ContractPackageHash |

### `ParseResult`

Value-object that represents a parse result. Contains error representing weather parsing was successful or not.

| Property | Type                  | Description        |
|----------|-----------------------|--------------------|
| `Error`  | `error`               | Parse result error |
| `Event`  | [`ces.Event`](#Event) | ces Event          |

### `SchemaData`

SchemaData is - value-object that represents an schema item.

| Property    | Type            | Description       |
|-------------|-----------------|-------------------|
| `ParamName` | `string`        | Name of the param |
| `ParamType` | `casper.CLType` | casper CLType     |

### `Schemas`

Schemas represent a map of event name and list of SchemaData.

## Tests

To run unit tests for the library, make sure you are in the root of the library:

``
go test ./...
``
