package socketio

import (
	"net/http"
	"sync"

	"github.com/robscc/go-engine.io"
)

// Socket is the socket object of socket.io.
type SocketInf interface {

	// Id returns the session id of socket.
	Id() string

	// Rooms returns the rooms name joined now.
	Rooms() []string

	// Request returns the first http request when established connection.
	Request() *http.Request

	// On registers the function f to handle an event.
	On(event string, f interface{}) error

	// Emit emits an event with given args.
	Emit(event string, args ...interface{}) error

	// Join joins the room.
	Join(room string) error

	// Leave leaves the room.
	Leave(room string) error

	// Disconnect disconnect the socket.
	Disconnect()

	// BroadcastTo broadcasts an event to the room with given args.
	BroadcastTo(room, event string, args ...interface{}) error

	Loop() error
}

type SocketWrapFunc func(SocketInf) SocketInf

type Socket struct {
	*socketHandler
	conn      engineio.Conn
	namespace string
	id        int
	mu        sync.Mutex
}

func NewSocket(conn engineio.Conn, server *Server) SocketInf {
	ret := &Socket{
		conn: conn,
	}
	ret.socketHandler = newSocketHandler(ret, server.baseHandler)
	return ret
}

func NewSocketWithWrapFactory(f SocketWrapFunc, conn engineio.Conn, server *Server) SocketInf {
	ret := &Socket{
		conn: conn,
	}
	ret.socketHandler = newSocketHandler(ret, server.baseHandler)
	return f(ret)
}

func (s *Socket) Id() string {
	return s.conn.Id()
}

func (s *Socket) Request() *http.Request {
	return s.conn.Request()
}

func (s *Socket) Emit(event string, args ...interface{}) error {
	if err := s.socketHandler.Emit(event, args...); err != nil {
		return err
	}
	if event == "disconnect" {
		s.conn.Close()
	}
	return nil
}

func (s *Socket) Disconnect() {
	s.conn.Close()
}

func (s *Socket) send(args []interface{}) error {
	packet := packet{
		Type: _EVENT,
		Id:   -1,
		NSP:  s.namespace,
		Data: args,
	}
	encoder := newEncoder(s.conn)
	return encoder.Encode(packet)
}

func (s *Socket) sendConnect() error {
	packet := packet{
		Type: _CONNECT,
		Id:   -1,
		NSP:  s.namespace,
	}
	encoder := newEncoder(s.conn)
	return encoder.Encode(packet)
}

func (s *Socket) sendId(args []interface{}) (int, error) {
	s.mu.Lock()
	packet := packet{
		Type: _EVENT,
		Id:   s.id,
		NSP:  s.namespace,
		Data: args,
	}
	s.id++
	if s.id < 0 {
		s.id = 0
	}
	s.mu.Unlock()

	encoder := newEncoder(s.conn)
	err := encoder.Encode(packet)
	if err != nil {
		return -1, nil
	}
	return packet.Id, nil
}

func (s *Socket) Loop() error {
	defer func() {
		s.LeaveAll()
		p := packet{
			Type: _DISCONNECT,
			Id:   -1,
		}
		s.socketHandler.onPacket(nil, &p)
	}()

	p := packet{
		Type: _CONNECT,
		Id:   -1,
	}
	encoder := newEncoder(s.conn)
	if err := encoder.Encode(p); err != nil {
		return err
	}
	s.socketHandler.onPacket(nil, &p)
	for {
		decoder := newDecoder(s.conn)
		var p packet
		if err := decoder.Decode(&p); err != nil {
			return err
		}
		ret, err := s.socketHandler.onPacket(decoder, &p)
		if err != nil {
			return err
		}
		switch p.Type {
		case _CONNECT:
			s.namespace = p.NSP
			s.sendConnect()
		case _BINARY_EVENT:
			fallthrough
		case _EVENT:
			if p.Id >= 0 {
				p := packet{
					Type: _ACK,
					Id:   p.Id,
					NSP:  s.namespace,
					Data: ret,
				}
				encoder := newEncoder(s.conn)
				if err := encoder.Encode(p); err != nil {
					return err
				}
			}
		case _DISCONNECT:
			return nil
		}
	}
}
