package proto

type Proto interface {
	Marshaler(interface{}) ([]byte, error)
	Unmarshaler([]byte, interface{}) error
}
