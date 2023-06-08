package ces

import (
	"bytes"
	"encoding/hex"
	"strings"

	"github.com/make-software/casper-go-sdk/casper"
	"github.com/make-software/casper-go-sdk/types/clvalue"
	"github.com/make-software/casper-go-sdk/types/clvalue/cltype"
)

type ParseResult struct {
	Error error
	Event Event
}

type Event struct {
	ContractHash        casper.Hash
	ContractPackageHash casper.Hash
	Data                map[string]casper.CLValue
	Name                string
	TransformID         uint
	EventID             uint
}

// ParseEventNameAndData parse provided rawEvent according to event schema, return EventName and EventData
func ParseEventNameAndData(eventHex string, schemas Schemas) (EventName, map[string]casper.CLValue, error) {
	decoded, err := hex.DecodeString(eventHex)
	if err != nil {
		return "", nil, err
	}

	dictionary, err := newDictionary(decoded)
	if err != nil {
		return "", nil, err
	}
	payload := bytes.NewBuffer(dictionary.DataToBytes())
	eventNameWithPrefix, err := clvalue.FromBufferByType(payload, cltype.String)
	if err != nil {
		return "", nil, err
	}

	if !strings.HasPrefix(eventNameWithPrefix.String(), eventPrefix) {
		return "", nil, ErrNoEventPrefixInEvent
	}

	eventName := strings.TrimPrefix(eventNameWithPrefix.String(), eventPrefix)
	schema, ok := schemas[eventName]
	if !ok {
		return "", nil, ErrEventNameNotInSchema
	}

	eventData, err := ParseEventDataFromSchemaBytes(schema, payload)
	if err != nil {
		return "", nil, err
	}

	return eventName, eventData, nil
}

func ParseEventDataFromSchemaBytes(schemas []SchemaData, buf *bytes.Buffer) (map[EventName]casper.CLValue, error) {
	result := make(map[EventName]casper.CLValue, len(schemas))
	var (
		one casper.CLValue
		err error
	)
	for _, item := range schemas {
		one, err = clvalue.FromBufferByType(buf, item.ParamType)
		if err != nil {
			return nil, err
		}
		result[item.ParamName] = one
	}
	return result, nil
}
