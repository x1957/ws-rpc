package types

type UnKnownPongMessage struct{}

func (e UnKnownPongMessage) Error() string {
	return "Unkonw pong meesage"
}
