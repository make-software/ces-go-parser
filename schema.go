package ces

import (
	"bytes"
	"database/sql/driver"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/make-software/casper-go-sdk/casper"
	"github.com/make-software/casper-go-sdk/types/clvalue"
	"github.com/make-software/casper-go-sdk/types/clvalue/cltype"
	"github.com/make-software/ces-go-parser/utils"
)

var ErrInvalidSchemaFormat = errors.New("invalid schema format")

type SchemaData struct {
	ParamName string
	ParamType cltype.CLType
}

type Schemas map[EventName][]SchemaData

func NewSchemasFromBytes(rawSchemas []byte) (Schemas, error) {
	buf := bytes.NewBuffer(rawSchemas)
	// For all the ces events schema for parsing CLType has next representation
	cesEventCLTypeParsingSchema := cltype.Map{Key: cltype.String, Val: &cltype.List{ElementsType: &cltype.Tuple2{
		Inner1: cltype.String,
		Inner2: &cltype.Dynamic{},
	}}}
	data, err := clvalue.FromBufferByType(buf, &cesEventCLTypeParsingSchema)
	if err != nil {
		return nil, err
	}
	result := make(map[EventName][]SchemaData, data.Map.Len())
	for name, schema := range data.Map.Map() {
		var schemaList []SchemaData
		for _, event := range schema.List.Elements {
			schemaList = append(schemaList, SchemaData{
				ParamName: event.Tuple2.Inner1.String(),
				ParamType: event.Tuple2.Inner2.GetType(),
			})
		}
		result[name] = schemaList
	}

	return result, nil
}

func (t *SchemaData) MarshalJSON() ([]byte, error) {
	temp := struct {
		Name  string `json:"name"`
		Bytes []byte `json:"bytes"`
	}{
		Name:  t.ParamName,
		Bytes: t.ParamType.Bytes(),
	}

	return json.Marshal(temp)
}

func (t *SchemaData) UnmarshalJSON(data []byte) error {
	temp := struct {
		ParamName string `json:"name"`
		ParamType string `json:"bytes"`
	}{}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(temp.ParamType)
	if err != nil {
		return err
	}

	resulType, err := cltype.FromBuffer(bytes.NewBuffer(decodedBytes))
	if err != nil {
		return err
	}

	t.ParamType = resulType
	t.ParamName = temp.ParamName
	return nil
}

type EventPayload map[string]interface{}

func (e EventPayload) StringParam(key string) (string, error) {
	rawParam, ok := e[key]
	if !ok {
		return "", fmt.Errorf("failed to get '%s' param from ces event payload", key)
	}

	value, ok := rawParam.(string)
	if !ok {
		return "", fmt.Errorf("failed to assert '%s' param type to string from ces event payload: type: %T", key, rawParam)
	}

	return value, nil
}

func (e EventPayload) HashParam(key string) (casper.Hash, error) {
	value, err := e.StringParam(key)
	if err != nil {
		return casper.Hash{}, err
	}

	return casper.NewHash(value)
}

func (e EventPayload) Map() map[string]interface{} {
	return map[string]interface{}(e)
}

func (t Schemas) ParseEventPayload(event Event) (EventPayload, error) {
	return t.ParseEventRawDataPayload(event.Name, event.RawData)
}

func (t Schemas) ParseEventRawDataPayload(name, rawData string) (EventPayload, error) {
	rawEventData, err := hex.DecodeString(rawData)
	if err != nil {
		return nil, err
	}

	result, err := ParseEventDataFromSchemaBytes(t[name], bytes.NewBuffer(rawEventData))
	if err != nil {
		return nil, err
	}

	eventData := make(map[string]interface{}, len(result))
	for key, res := range result {
		eventData[key] = utils.CLValueToJSONValue(res)
	}

	return eventData, nil
}

func (t Schemas) Value() (driver.Value, error) {
	marshaled, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	return marshaled, nil
}

// Scan rewrite behaviour for selecting Hash from db
func (t *Schemas) Scan(value interface{}) error {
	bv, err := driver.String.ConvertValue(value)
	if err != nil {
		return ErrInvalidSchemaFormat
	}

	v, ok := bv.([]byte)
	if !ok {
		return ErrInvalidSchemaFormat
	}

	var schemas Schemas
	if err := json.Unmarshal(v, &schemas); err != nil {
		return err
	}

	*t = schemas
	return nil
}
