package ces

import (
	"bytes"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/make-software/casper-go-sdk/types/clvalue"
	"github.com/make-software/casper-go-sdk/types/clvalue/cltype"
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
