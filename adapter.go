package socketio

import "sync"

// BroadcastAdaptor is the adaptor to handle broadcasts.
type BroadcastAdaptor interface {

	// Join causes the socket to join a room.
	Join(room string, socket SocketInf) error

	// Leave causes the socket to leave a room.
	Leave(room string, socket SocketInf) error

	// Send will send an event with args to the room. If "ignore" is not nil, the event will be excluded from being sent to "ignore".
	Send(ignore SocketInf, room, event string, args ...interface{}) error
}

var newBroadcast = newBroadcastDefault

type broadcast struct {
	m map[string]map[string]SocketInf
	sync.RWMutex
}

func newBroadcastDefault() BroadcastAdaptor {
	return &broadcast{
		m: make(map[string]map[string]SocketInf),
	}
}

func (b *broadcast) Join(room string, socket SocketInf) error {
	b.Lock()
	sockets, ok := b.m[room]
	if !ok {
		sockets = make(map[string]SocketInf)
	}
	sockets[socket.Id()] = socket
	b.m[room] = sockets
	b.Unlock()
	return nil
}

func (b *broadcast) Leave(room string, socket SocketInf) error {
	b.Lock()
	defer b.Unlock()
	sockets, ok := b.m[room]
	if !ok {
		return nil
	}
	delete(sockets, socket.Id())
	if len(sockets) == 0 {
		delete(b.m, room)
		return nil
	}
	b.m[room] = sockets
	return nil
}

func (b *broadcast) Send(ignore SocketInf, room, event string, args ...interface{}) error {
	b.RLock()
	sockets := b.m[room]
	for id, s := range sockets {
		if ignore != nil && ignore.Id() == id {
			continue
		}
		s.Emit(event, args...)
	}
	b.RUnlock()
	return nil
}
