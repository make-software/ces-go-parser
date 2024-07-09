package utils

import (
	"github.com/make-software/casper-go-sdk/types/clvalue"
	"github.com/make-software/casper-go-sdk/types/clvalue/cltype"
)

// TODO: Refactore and move to casper go sdk
func CLValueToJSONValue(value clvalue.CLValue) interface{} {
	switch value.Type.GetTypeID() {
	case cltype.TypeIDBool:
		return value.Bool.Value()
	case cltype.TypeIDI32:
		return value.I32.Value()
	case cltype.TypeIDI64:
		return value.I64.Value()
	case cltype.TypeIDU8:
		return value.UI8.Value()
	case cltype.TypeIDU32:
		return value.UI32.Value()
	case cltype.TypeIDU64:
		return value.UI64.Value()
	case cltype.TypeIDU128:
		return value.UI128.Value().String()
	case cltype.TypeIDU256:
		return value.UI256.Value().String()
	case cltype.TypeIDU512:
		return value.UI512.Value().String()
	case cltype.TypeIDUnit:
		return value.Unit.String()
	case cltype.TypeIDString:
		return value.StringVal.String()
	case cltype.TypeIDKey:
		return value.Key.String()
	case cltype.TypeIDURef:
		return value.Uref.String()
	case cltype.TypeIDOption:
		if value.Option.IsEmpty() {
			return nil
		}
		return CLValueToJSONValue(*value.Option.Inner)
	case cltype.TypeIDList:
		var data []interface{}
		for _, one := range value.List.Elements {
			data = append(data, CLValueToJSONValue(one))
		}
		return data
	case cltype.TypeIDByteArray:
		return value.ByteArray.String()
	case cltype.TypeIDResult:
		if value.Result.IsSuccess {
			return CLValueToJSONValue(value.Result.Inner)
		}
		return nil
	case cltype.TypeIDMap:
		res := make(map[string]interface{}, value.Map.Len())
		for key, value := range value.Map.Map() {
			res[key] = CLValueToJSONValue(value)
		}
		return res
	case cltype.TypeIDTuple1:
		return CLValueToJSONValue(value.Tuple1.Value())
	case cltype.TypeIDTuple2:
		return [2]interface{}{CLValueToJSONValue(value.Tuple2.Inner1), CLValueToJSONValue(value.Tuple2.Inner2)}
	case cltype.TypeIDTuple3:
		return [3]interface{}{CLValueToJSONValue(value.Tuple3.Inner1), CLValueToJSONValue(value.Tuple3.Inner2), CLValueToJSONValue(value.Tuple3.Inner3)}
	case cltype.TypeIDPublicKey:
		return value.PublicKey.String()
	default:
		return nil
	}
}
