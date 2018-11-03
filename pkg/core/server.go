package core

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/x1957/ws-rpc/pkg/proto"
	"github.com/x1957/ws-rpc/pkg/types"
)

const (
	ReadTimeout   = 30 * time.Second      // heartbeat 10s TODO config
	WriteTimeout  = 50 * time.Millisecond // TODO config
	HeartBeatTime = 10 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

//
type WSServer struct {
	proto    proto.Proto
	port     int
	ip       string
	path     string
	handlers sync.Map
}

type WSOpts struct {
	Proto proto.Proto
	Port  int
	IP    string
	Path  string
}

type wsConn struct {
	ctx     context.Context
	cancel  context.CancelFunc
	conn    *websocket.Conn
	writeMu sync.Mutex
	// only one goroutine read, no lock needed
}

func NewDefaultWSOpts() WSOpts {
	return WSOpts{
		Proto: proto.JSONProto,
		Port:  1957,
		Path:  "/ws",
		IP:    "0.0.0.0",
	}
}

func NewWSServer(opts WSOpts) *WSServer {
	return &WSServer{
		proto: opts.Proto,
		port:  opts.Port,
		ip:    opts.IP,
		path:  opts.Path,
	}
}

func (s *WSServer) Serve() {
	http.HandleFunc(s.path, func(w http.ResponseWriter, r *http.Request) {
		s.serveWS(w, r)
	})

	err := http.ListenAndServe(fmt.Sprintf("%s:%d", s.ip, s.port), nil)
	if err != nil {
		glog.Fatalf("ListenAndServe: ", err)
	}
}

func (s *WSServer) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		glog.Errorf("upgrade %s to websocket error. %v", r.RemoteAddr, err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	wsConn := &wsConn{
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}
	glog.Infof("%s connected.", r.RemoteAddr)
	go s.read(wsConn)
}

func (s *WSServer) read(conn *wsConn) {
	// dispatcher
	// TODO handler heartbeat
	// sevver ping client
	defer func() {
		conn.cancel()
		conn.conn.Close()
		glog.Infof("connecton %s closed.", conn.conn.RemoteAddr())
	}()
	for {
		select {
		case <-conn.ctx.Done():
			return
		default:
			conn.conn.SetReadDeadline(time.Now().Add(ReadTimeout))
			_, bs, err := conn.conn.ReadMessage()
			if err != nil {
				return
			}
			if err := s.handleRequest(conn, bs); err != nil {
				// TODO write error
				s.error(conn, err)
			}
		}
	}

}

func (s *WSServer) handleRequest(conn *wsConn, req []byte) error {
	var request types.Request
	if err := s.proto.Unmarshal(req, &request); err != nil {
		return err
	}
	methodName := request.Method

	f, ok := s.handlers.Load(methodName)
	if !ok {
		return &HandlerNotExistError{methodName + " method not found."}
	}
	methodFunc := f.(reflect.Value)
	argTyp := methodFunc.Type().In(1) // arg
	arg := reflect.New(argTyp).Interface()
	if err := s.proto.Unmarshal(request.Args, arg); err != nil {
		return err
	}
	output := methodFunc.Call([]reflect.Value{reflect.ValueOf(conn.ctx), reflect.ValueOf(arg).Elem()})
	if output[1].IsNil() {
		result := output[0].Interface()
		bs, err := s.proto.Marshal(result)
		if err != nil {
			// error
			s.error(conn, err)
		}
		s.write(conn, bs)
	} else {
		// error
	}
	return nil
}

func (s *WSServer) error(conn *wsConn, err error) {
	glog.Errorf("error: %v", err)
}

func (s *WSServer) write(conn *wsConn, data []byte) error {
	conn.writeMu.Lock()
	conn.conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	if err := conn.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		conn.writeMu.Unlock()
		return err
	}
	conn.writeMu.Unlock()
	return nil
}
