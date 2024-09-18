package ces

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/make-software/casper-go-sdk/v2/casper"
	"github.com/make-software/casper-go-sdk/v2/rpc"
	"github.com/make-software/casper-go-sdk/v2/types/clvalue"
	"github.com/make-software/casper-go-sdk/v2/types/clvalue/cltype"
	"github.com/make-software/casper-go-sdk/v2/types/key"
	"golang.org/x/sync/errgroup"
)

var (
	ErrFailedDeploy                      = errors.New("error: failed deploy, expected successful deploys")
	ErrEventNameNotInSchema              = errors.New("error: event name not found in Schema")
	ErrFailedToParseContractEventSchema  = errors.New("error: failed to parse contract event Schema")
	ErrExpectContractStoredValue         = errors.New("error: expect contract stored value")
	ErrExpectCLValueStoredValue          = errors.New("error: expect clValue stored value")
	ErrMissingRequiredNamedKey           = errors.New("error: missing required named key")
	ErrNoEventPrefixInEvent              = errors.New("error: no event_ prefix in event")
	ErrNilDictionaryInTransform          = errors.New("error: nil dictionary in transform")
	ErrNotSmartContractAddressableEntity = errors.New("not SmartContract addressable entity type")
)

type NetworkVersion uint

const (
	NetworkVersionV1 NetworkVersion = iota
	NetworkVersionV2
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
		// version of the network used by parser
		networkVersion NetworkVersion
	}
	EventName = string

	EventMetadata struct {
		Name    string
		Uref    casper.Uref
		Payload *bytes.Buffer
		EventID uint
	}

	ContractMetadata struct {
		Schemas             Schemas
		ContractHash        casper.Hash
		ContractPackageHash casper.Hash
		EventsSchemaURef    casper.Uref
		EventsURef          casper.Uref
	}
)

func NewParserWithVersion(casperClient casper.RPCClient, contractHashes []casper.Hash, version NetworkVersion) (*EventParser, error) {
	eventParser := EventParser{
		casperClient: casperClient,
	}

	contractsMetadata, err := eventParser.loadContractsMetadata(contractHashes, version)
	if err != nil {
		return nil, err
	}

	return &EventParser{
		casperClient:      casperClient,
		contractsMetadata: contractsMetadata,
	}, nil
}

func NewParser(casperClient casper.RPCClient, contractHashes []casper.Hash) (*EventParser, error) {
	return NewParserWithVersion(casperClient, contractHashes, NetworkVersionV2)
}

// ParseExecutionResults accept casper.ExecutionResult analyze its transforms and trying to parse events according to stored contract schema
func (p *EventParser) ParseExecutionResults(executionResult casper.ExecutionResult) ([]ParseResult, error) {
	if executionResult.ErrorMessage != nil {
		return nil, ErrFailedDeploy
	}

	var results = make([]ParseResult, 0)

	for transformIDx, transform := range executionResult.Effects {
		if ok := transform.Kind.IsWriteCLValue(); !ok {
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
				Name:        eventMetadata.Name,
				TransformID: uint(transformIDx),
				EventID:     eventMetadata.EventID,
			},
		}

		eventSchema, ok := contractMetadata.Schemas[parseResult.Event.Name]
		if !ok {
			parseResult.Error = ErrEventNameNotInSchema
			results = append(results, parseResult)
			continue
		}

		rawData := eventMetadata.Payload.Bytes()
		eventData, err := ParseEventDataFromSchemaBytes(eventSchema, eventMetadata.Payload)
		if err != nil {
			parseResult.Error = err
			results = append(results, parseResult)
			continue
		}

		parseResult.Event.ContractHash = contractMetadata.ContractHash
		parseResult.Event.ContractPackageHash = contractMetadata.ContractPackageHash
		parseResult.Event.RawData = hex.EncodeToString(rawData)
		parseResult.Event.Data = eventData
		results = append(results, parseResult)
	}

	return results, nil
}

func ParseEventMetadataFromTransform(transform casper.Transform) (EventMetadata, error) {
	writeCLValue, err := transform.Kind.ParseAsWriteCLValue()
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

	eventID, err := strconv.Atoi(dictionary.Key)
	if err != nil {
		return EventMetadata{}, err
	}

	return EventMetadata{
		Name:    strings.TrimPrefix(eventNameWithPrefix.String(), eventPrefix),
		Uref:    dictionary.Uref,
		Payload: payload,
		EventID: uint(eventID),
	}, nil
}

// FetchContractSchemasBytes accept contract hash to fetch stored contract schema
func (p *EventParser) FetchContractSchemasBytes(contractHash casper.Hash) ([]byte, error) {
	loadContractSchemasFromEntity := func(contractHash casper.Hash) (rpc.QueryGlobalStateResult, error) {
		entity, err := p.casperClient.GetLatestEntity(context.Background(), rpc.EntityIdentifier{
			EntityAddr: &key.EntityAddr{
				SmartContract: &contractHash,
			},
		})
		if err != nil {
			return rpc.QueryGlobalStateResult{}, err
		}

		addressableEntity := entity.Entity.AddressableEntity
		if addressableEntity != nil && addressableEntity.Entity.EntityKind.SmartContract != nil {

			var eventSchemaUref string
			for _, namedKey := range addressableEntity.NamedKeys {
				if namedKey.Name == eventSchemaNamedKey {
					eventSchemaUref = namedKey.Key.String()
					break
				}
			}

			schemasURefValue, err := p.casperClient.QueryGlobalStateByStateHash(context.Background(), nil, eventSchemaUref, nil)
			if err != nil {
				return rpc.QueryGlobalStateResult{}, err
			}

			return schemasURefValue, nil
		}
		return rpc.QueryGlobalStateResult{}, ErrNotSmartContractAddressableEntity
	}

	schemasURefValue, err := loadContractSchemasFromEntity(contractHash)
	if err != nil {
		log.Println("Error pn fetching schemas bytes from addressable entity: ", err)

		schemasURefValue, err = p.casperClient.QueryGlobalStateByStateHash(context.Background(), nil, fmt.Sprintf("hash-%s", contractHash.ToHex()), []string{eventSchemaNamedKey})
		if err != nil {
			return nil, err
		}
	}

	value := schemasURefValue.StoredValue.CLValue
	if value == nil {
		return nil, ErrExpectCLValueStoredValue
	}

	return value.Bytes()
}

func (p *EventParser) loadContractsMetadata(contractHashes []casper.Hash, version NetworkVersion) (map[string]ContractMetadata, error) {
	stateRootHash, err := p.casperClient.GetStateRootHashLatest(context.Background())
	if err != nil {
		return nil, err
	}

	stateRootString := stateRootHash.StateRootHash.ToHex()
	contractsSchemas := make(map[string]ContractMetadata, len(contractHashes))
	metadatas := make(chan *ContractMetadata, len(contractHashes))

	errGroup, ctx := errgroup.WithContext(context.Background())

	loadMetadata := func(hash casper.Hash) {
		errGroup.Go(func() error {
			var contractMetadata *ContractMetadata
			// try to load contract metadata as AddressableEntity in case of network version V2
			if version == NetworkVersionV2 {
				contractMetadata, err = p.loadContractMetadatAsAddressableEntity(ctx, hash)
				if err != nil {
					log.Println("Error on trying to load contract metadata from addressable entity: ", err)
				}
			}

			if contractMetadata == nil {
				log.Println("Trying to load contract metadata from global state...")
				// in case of error try to load metadata as stored contract
				contractMetadata, err = p.loadContractMetadatAsStoredContract(ctx, hash, stateRootString)
				if err != nil {
					return err
				}
			}

			schemas, err := LoadContractEventSchemas(p.casperClient, stateRootString, contractMetadata.EventsSchemaURef)
			if err != nil {
				return ErrFailedToParseContractEventSchema
			}

			contractMetadata.ContractHash = hash
			contractMetadata.Schemas = schemas
			metadatas <- contractMetadata
			return nil
		})
	}

	for _, hash := range contractHashes {
		loadMetadata(hash)
	}

	if err = errGroup.Wait(); err != nil {
		return nil, err
	}

	close(metadatas)
	for contractMetadata := range metadatas {
		contractsSchemas[contractMetadata.EventsURef.String()] = *contractMetadata
	}
	return contractsSchemas, nil
}

func LoadContractMetadataWithoutSchema(contractPackage casper.Hash, namedKeys casper.NamedKeys) (ContractMetadata, error) {
	var (
		eventsURefStr       string
		eventsSchemaURefStr string
	)

	for _, namedKey := range namedKeys {
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
		ContractPackageHash: contractPackage,
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

func (p *EventParser) loadContractMetadatAsAddressableEntity(ctx context.Context, hash casper.Hash) (*ContractMetadata, error) {
	entity, err := p.casperClient.GetLatestEntity(ctx, rpc.EntityIdentifier{
		EntityAddr: &key.EntityAddr{
			SmartContract: &hash,
		},
	})
	if err != nil {
		return nil, err
	}

	addressableEntity := entity.Entity.AddressableEntity
	if addressableEntity != nil && addressableEntity.Entity.EntityKind.SmartContract != nil {
		packageHash := addressableEntity.Entity.PackageHash

		contractPackageHash, err := casper.NewHash(strings.TrimPrefix(packageHash, "package-"))
		if err != nil {
			return nil, err
		}

		contractMetadata, err := LoadContractMetadataWithoutSchema(contractPackageHash, addressableEntity.NamedKeys)
		if err != nil {
			return nil, err
		}

		return &contractMetadata, nil
	}

	return nil, ErrNotSmartContractAddressableEntity
}

func (p *EventParser) loadContractMetadatAsStoredContract(ctx context.Context, hash casper.Hash, stateRoot string) (*ContractMetadata, error) {
	contractResult, err := p.casperClient.QueryGlobalStateByStateHash(context.Background(), &stateRoot, fmt.Sprintf("hash-%s", hash), nil)
	if err != nil {
		return nil, err
	}

	if contractResult.StoredValue.Contract == nil {
		return nil, ErrExpectContractStoredValue
	}

	contract := contractResult.StoredValue.Contract
	contractMetadata, err := LoadContractMetadataWithoutSchema(contract.ContractPackageHash.Hash, contract.NamedKeys)
	if err != nil {
		return nil, err
	}

	return &contractMetadata, nil
}
