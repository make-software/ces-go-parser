package ces

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/make-software/casper-go-sdk/casper"
	"github.com/make-software/casper-go-sdk/types/clvalue"
	"github.com/make-software/casper-go-sdk/types/clvalue/cltype"
)

var (
	ErrFailedDeploy                     = errors.New("error: failed deploy, expected successful deploys")
	ErrEventNameNotInSchema             = errors.New("error: event name not found in Schema")
	ErrFailedToParseContractEventSchema = errors.New("error: failed to parse contract event Schema")
	ErrExpectContractStoredValue        = errors.New("error: expect contract stored value")
	ErrExpectCLValueStoredValue         = errors.New("error: expect clValue stored value")
	ErrMissingRequiredNamedKey          = errors.New("error: missing required named key")
	ErrNoEventPrefixInEvent             = errors.New("error: no event_ prefix in event")
	ErrNilDictionaryInTransform         = errors.New("error: nil dictionary in transform")
)

const (
	eventSchemaNamedKey = "__events_schema"
	eventNamedKey       = "__events"
	eventPrefix         = "event_"
)

type (
	EventParser struct {
		casperClient casper.RPCClient
		// key represent Uref from __events named key
		contractsMetadata map[string]ContractMetadata
	}
	EventName = string

	EventMetadata struct {
		Name    string
		Uref    casper.Uref
		Payload *bytes.Buffer
	}

	ContractMetadata struct {
		Schemas             Schemas
		ContractHash        casper.Hash
		ContractPackageHash casper.Hash
		EventsSchemaURef    casper.Uref
		EventsURef          casper.Uref
	}
)

func NewParser(casperClient casper.RPCClient, contractHashes []casper.Hash) (*EventParser, error) {
	eventParser := EventParser{
		casperClient: casperClient,
	}

	contractsMetadata, err := eventParser.loadContractsMetadata(contractHashes)
	if err != nil {
		return nil, err
	}

	return &EventParser{
		casperClient:      casperClient,
		contractsMetadata: contractsMetadata,
	}, nil
}

// ParseExecutionResults accept casper.ExecutionResult analyze its transforms and trying to parse events according to stored contract schema
func (p *EventParser) ParseExecutionResults(executionResult casper.ExecutionResult) ([]ParseResult, error) {
	if executionResult.Success == nil {
		return nil, ErrFailedDeploy
	}

	var results = make([]ParseResult, 0)

	for _, transform := range executionResult.Success.Effect.Transforms {
		if ok := transform.Transform.IsWriteCLValue(); !ok {
			continue
		}

		eventMetadata, err := ParseEventMetadataFromTransform(transform)
		if err != nil {
			continue
		}

		contractMetadata, ok := p.contractsMetadata[eventMetadata.Uref.String()]
		if !ok {
			continue
		}

		parseResult := ParseResult{
			Event: Event{
				Name: eventMetadata.Name,
			},
		}

		eventSchema, ok := contractMetadata.Schemas[parseResult.Event.Name]
		if !ok {
			parseResult.Error = ErrEventNameNotInSchema
			results = append(results, parseResult)
			continue
		}

		eventData, err := ParseEventDataFromSchemaBytes(eventSchema, eventMetadata.Payload)
		if err != nil {
			parseResult.Error = err
			results = append(results, parseResult)
			continue
		}

		parseResult.Event.ContractHash = contractMetadata.ContractHash
		parseResult.Event.ContractPackageHash = contractMetadata.ContractPackageHash
		parseResult.Event.Data = eventData
		results = append(results, parseResult)
	}

	return results, nil
}

func ParseEventMetadataFromTransform(transform casper.TransformKey) (EventMetadata, error) {
	writeCLValue, err := transform.Transform.ParseAsWriteCLValue()
	if err != nil {
		return EventMetadata{}, err
	}

	if transform.Key.Dictionary == nil {
		return EventMetadata{}, ErrNilDictionaryInTransform
	}

	rawBytes, err := writeCLValue.Value()
	if err != nil {
		return EventMetadata{}, err
	}

	if rawBytes.Any == nil {
		return EventMetadata{}, err
	}

	dictionary, err := newDictionary(rawBytes.Any.Bytes())
	if err != nil {
		return EventMetadata{}, err
	}

	payload := bytes.NewBuffer(dictionary.DataToBytes())
	eventNameWithPrefix, err := clvalue.FromBufferByType(payload, cltype.String)
	if err != nil {
		return EventMetadata{}, err
	}

	return EventMetadata{
		Name:    strings.TrimPrefix(eventNameWithPrefix.String(), eventPrefix),
		Uref:    dictionary.Uref,
		Payload: payload,
	}, nil
}

// FetchContractSchemasBytes accept contract hash to fetch stored contract schema
func (p *EventParser) FetchContractSchemasBytes(contractHash casper.Hash) ([]byte, error) {
	schemasURefValue, err := p.casperClient.QueryGlobalStateByStateHash(context.Background(), nil, fmt.Sprintf("hash-%s", contractHash.ToHex()), []string{eventSchemaNamedKey})
	if err != nil {
		return nil, err
	}

	value := schemasURefValue.StoredValue.CLValue
	if value == nil {
		return nil, ErrExpectCLValueStoredValue
	}

	bytesData, err := value.Value()
	if err != nil {
		return nil, err
	}

	return bytesData.Any.Bytes(), nil
}

func (p *EventParser) loadContractsMetadata(contractHashes []casper.Hash) (map[string]ContractMetadata, error) {
	stateRootHash, err := p.casperClient.GetStateRootHashLatest(context.Background())
	if err != nil {
		return nil, err
	}

	stateRootString := stateRootHash.StateRootHash.ToHex()
	contractsSchemas := make(map[string]ContractMetadata, len(contractHashes))
	for _, hash := range contractHashes {
		contractResult, err := p.casperClient.QueryGlobalStateByStateHash(context.Background(), &stateRootString, fmt.Sprintf("hash-%s", hash), nil)
		if err != nil {
			return nil, err
		}

		if contractResult.StoredValue.Contract == nil {
			return nil, ErrExpectContractStoredValue
		}

		contractMetadata, err := LoadContractMetadataWithoutSchema(*contractResult.StoredValue.Contract)
		if err != nil {
			return nil, err
		}

		schemas, err := LoadContractEventSchemas(p.casperClient, stateRootString, contractMetadata.EventsSchemaURef)
		if err != nil {
			return nil, ErrFailedToParseContractEventSchema
		}

		contractMetadata.ContractHash = hash
		contractMetadata.Schemas = schemas
		contractsSchemas[contractMetadata.EventsURef.String()] = contractMetadata
	}

	return contractsSchemas, nil
}

func LoadContractMetadataWithoutSchema(contractResult casper.Contract) (ContractMetadata, error) {
	var (
		eventsURefStr       string
		eventsSchemaURefStr string
	)

	for _, namedKey := range contractResult.NamedKeys {
		switch namedKey.Name {
		case eventNamedKey:
			eventsURefStr = namedKey.Key.String()
		case eventSchemaNamedKey:
			eventsSchemaURefStr = namedKey.Key.String()
		}

		if eventsURefStr != "" && eventsSchemaURefStr != "" {
			break
		}
	}

	if eventsURefStr == "" || eventsSchemaURefStr == "" {
		return ContractMetadata{}, ErrMissingRequiredNamedKey
	}

	eventsSchemaURef, err := casper.NewUref(eventsSchemaURefStr)
	if err != nil {
		return ContractMetadata{}, err
	}

	eventsURef, err := casper.NewUref(eventsURefStr)
	if err != nil {
		return ContractMetadata{}, err
	}

	return ContractMetadata{
		ContractPackageHash: contractResult.ContractPackageHash.Hash,
		EventsSchemaURef:    eventsSchemaURef,
		EventsURef:          eventsURef,
	}, nil
}

func LoadContractEventSchemas(casperClient casper.RPCClient, stateRootHash string, eventSchemaUref casper.Uref) (Schemas, error) {
	schemasURefValue, err := casperClient.QueryGlobalStateByStateHash(context.Background(), &stateRootHash, eventSchemaUref.String(), nil)
	if err != nil {
		return nil, err
	}

	if schemasURefValue.StoredValue.CLValue == nil {
		return nil, ErrExpectCLValueStoredValue
	}

	// We cannot parse CLValue based on the CLType from the Argument raw data, as it may contain an Any type
	// which we do not know how to parse. Therefore, we should parse the raw bytes, ignore the clType field,
	// and provide the hardcoded CLType with the cltype.Dynamic type instead of Any
	hexBytes, err := schemasURefValue.StoredValue.CLValue.Bytes()
	if err != nil {
		return nil, err
	}
	return NewSchemasFromBytes(hexBytes)
}
