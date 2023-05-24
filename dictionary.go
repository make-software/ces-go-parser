package ces

import (
	"bytes"
	"errors"

	"github.com/make-software/casper-go-sdk/casper"
	"github.com/make-software/casper-go-sdk/types/clvalue"
	"github.com/make-software/casper-go-sdk/types/clvalue/cltype"
	"github.com/make-software/casper-go-sdk/types/key"
)

// Dictionary value has always three parts. Data, dictionary URef and dictionary item key
// Each of the three parts has the length as prefix.
// The data for CES is stored as a CL Value. Always a List(u8). It starts with the length of the data and ends with 0e03.
type dictionary struct {
	Data clvalue.List
	Uref casper.Uref
	Key  string
}

func newDictionary(source []byte) (dictionary, error) {
	buf := bytes.NewBuffer(source)
	data, err := clvalue.FromBuffer(buf)
	if err != nil {
		return dictionary{}, err
	}

	if data.List == nil || data.List.Type.ElementsType != cltype.UInt8 {
		return dictionary{}, errors.New("can't parse dictionary event")
	}

	_ = clvalue.TrimByteSize(buf)
	urefBytes, err := clvalue.FromBufferByType(buf, cltype.NewByteArray(32))
	if err != nil {
		return dictionary{}, err
	}

	uref, err := key.NewURefFromBytes(append(urefBytes.Bytes(), key.UrefAccessReadAddWrite))
	if err != nil {
		return dictionary{}, err
	}

	dictKey, err := clvalue.FromBufferByType(buf, cltype.String)
	if err != nil {
		return dictionary{}, err
	}

	return dictionary{
		Data: *data.List,
		Uref: uref,
		Key:  dictKey.String(),
	}, nil
}

func (d dictionary) DataToBytes() []byte {
	var result []byte
	for _, one := range d.Data.Elements {
		result = append(result, one.UI8.Value())
	}
	return result
}
