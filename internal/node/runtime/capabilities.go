package runtime

import "fmt"

type KVPut interface {
	Put(key, value string)
}

type KVGet interface {
	Get(key string) (string, bool)
}

type KVDelete interface {
	Delete(key string)
}
/*
? Why not make the interface protocol specific??

* through capabilites we want to know what APIs protocol supports without knowing the protocol itself, so that we can use it in the test cases without importing the protocol package
* if later we implement CRDT that also implemntsa PUT and others, then it will means we will be internally using same type of interface for both protocol
*/


func PutAction(proto any, key, value string) error {
	if writable, ok := proto.(KVPut); ok {
		writable.Put(key, value)
		return nil
	}
	return fmt.Errorf("kv_write not supported by protocol")
}

func GetAction(proto any, key string) (string, bool, error) {
	if readable, ok := proto.(KVGet); ok {
		value, found := readable.Get(key)
		return value, found, nil
	}
	return "", false, fmt.Errorf("kv_read not supported by protocol")
}

func DeleteAction(proto any, key string) error {
	if deletable, ok := proto.(KVDelete); ok {
		deletable.Delete(key)
		return nil
	}
	return fmt.Errorf("kv_delete not supported by protocol")
}

/*
each functionality that a protocol has is defined as interfance and before calling in the 
protocol we will check it through here.
*/ 
