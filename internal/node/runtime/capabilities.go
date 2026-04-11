package runtime

import "fmt"

type ProtocolCapabilities struct {
	KVPut    bool `json:"kvPut"`
	KVGet    bool `json:"kvGet"`
	KVDelete bool `json:"kvDelete"`
}

type KVPut interface {
	Put(key, value string)
}

type KVGet interface {
	Get(key string) (string, bool)
}

type KVGetMetadata struct {
	Value   string
	Version int64
	Origin  string
}

type KVGetWithMetadata interface {
	GetWithMetadata(key string) (string, int64, string, bool)
}

type KVDelete interface {
	Delete(key string)
}

/*
? Why not make the interface protocol specific??

* through capabilites we want to know what APIs protocol supports without knowing the protocol itself, so that we can use it in the test cases without importing the protocol package
* if later we implement CRDT that also implemntsa PUT and others, then it will means we will be internally using same type of interface for both protocol
*/

func PutAction(proto any, args ...string) error {
	if writable, ok := proto.(KVPut); ok {
		if len(args) == 0 {
			return nil
		}
		if len(args) != 2 {
			return fmt.Errorf("kv_put expects key and value")
		}

		key := args[0]
		value := args[1]
		writable.Put(key, value)
		return nil
	}
	return fmt.Errorf("kv_write not supported by protocol")
}

func GetAction(proto any, args ...string) (string, bool, error) {
	if readable, ok := proto.(KVGet); ok {
		if len(args) == 0 {
			return "", false, nil
		}
		if len(args) != 1 {
			return "", false, fmt.Errorf("kv_get expects key")
		}

		key := args[0]
		value, found := readable.Get(key)
		return value, found, nil
	}
	return "", false, fmt.Errorf("kv_read not supported by protocol")
}

func GetActionWithMetadata(proto any, args ...string) (KVGetMetadata, bool, error) {
	if readable, ok := proto.(KVGetWithMetadata); ok {
		if len(args) == 0 {
			return KVGetMetadata{}, false, nil
		}
		if len(args) != 1 {
			return KVGetMetadata{}, false, fmt.Errorf("kv_get expects key")
		}

		key := args[0]
		value, version, origin, found := readable.GetWithMetadata(key)
		return KVGetMetadata{Value: value, Version: version, Origin: origin}, found, nil
	}

	value, found, err := GetAction(proto, args...)
	if err != nil {
		return KVGetMetadata{}, false, err
	}

	return KVGetMetadata{Value: value}, found, nil
}

func DeleteAction(proto any, args ...string) error {
	if deletable, ok := proto.(KVDelete); ok {
		if len(args) == 0 {
			return nil
		}
		if len(args) != 1 {
			return fmt.Errorf("kv_delete expects key")
		}

		key := args[0]
		deletable.Delete(key)
		return nil
	}
	return fmt.Errorf("kv_delete not supported by protocol")
}

func CheckCapabilities(proto any) ProtocolCapabilities {
	_, _, getErr := GetAction(proto)

	return ProtocolCapabilities{
		KVPut:    PutAction(proto) == nil,
		KVGet:    getErr == nil,
		KVDelete: DeleteAction(proto) == nil,
	}
}

func CheckProtocol(proto any) ProtocolCapabilities {
	return CheckCapabilities(proto)
}

/*
each functionality that a protocol has is defined as interfance and before calling in the
protocol we will check it through here.
*/

/*
see what we can do is there are checks in the protocols
in capabilities.go and before making any KV request or anything like
that first it is checked that that protocol has that inteface or not
and if not then ignore the request, what we can do instead is in the
controlplane,

see the current architecture is such that it is hot swappable...
before implementing this we should implement dynamic setting up of
protocol from controlplane and. when the controlplane sets a protcol
for a cluster, it also recievees the response of the capabolities
of the protocol in the form of contract. the contract is then consumed
my frontend whre it renders form based on what things are availabe...
what do u say?? this flow is better right?? but we will also need a
contract in rutnime that will check all the capabilities of the protocol
as soon as it is loaded in runtime and return some kind of struct or interafce... that should be the first step
*/
