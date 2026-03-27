package protocol

import "encoding/json"

// EncodeJSON and DecodeJSON provide the shared message codec shape used by protocols.
func EncodeJSON[T any](v T) []byte {
	data, _ := json.Marshal(v)
	return data
}

func DecodeJSON[T any](data []byte) (T, error) {
	var v T
	err := json.Unmarshal(data, &v)
	return v, err
}
