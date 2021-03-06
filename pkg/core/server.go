package core

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"sync/atomic"

	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/x1957/ws-rpc/pkg/proto"
	"github.com/x1957/ws-rpc/pkg/types"
	"github.com/x1957/ws-rpc/pkg/util"
)

const (
	ReadTimeout   = 30 * time.Second      // heartbeat 10s TODO config
	WriteTimeout  = 50 * time.Millisecond // TODO config
	HeartBeatTime = 10 * time.Second
	ConnKey       = "WSConn"
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
	gpool    *gpool
}

type WSOpts struct {
	Proto proto.Proto
	Port  int
	IP    string
	Path  string
}

type WsConn struct {
	Ctx      context.Context
	Cancel   context.CancelFunc
	conn     *websocket.Conn
	writeMu  sync.Mutex
	proto    proto.Proto
	lastPong int64
	lastPing int64
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
		gpool: newGpool(4096), // TODO config
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

	wsConn := &WsConn{
		conn:  conn,
		proto: s.proto,
	}

	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, ConnKey, wsConn) // *WsConn
	wsConn.Ctx = ctx
	wsConn.Cancel = cancel
	glog.Infof("%s connected.", r.RemoteAddr)
	go s.read(wsConn)
}

func (s *WSServer) read(conn *WsConn) {
	defer func() {
		conn.Cancel()
		conn.conn.Close()
		glog.Infof("connecton %s closed.", conn.conn.RemoteAddr())
	}()
	// TODO handle ping here
	s.gpool.run(func() {
		conn.heartbeat()
	})

	for {
		select {
		case <-conn.Ctx.Done():
			return
		default:
			conn.conn.SetReadDeadline(time.Now().Add(ReadTimeout))
			_, bs, err := conn.conn.ReadMessage()
			if err != nil {
				return
			}

			if err := s.gpool.run(func() {
				// run in goroutine pool
				if err := s.handleRequest(conn, bs); err != nil {
					// TODO Write error
					s.error(conn, err)
				}
			}); err != nil {
				s.error(conn, err)
			}
		}
	}

}

func (s *WSServer) handleRequest(conn *WsConn, req []byte) error {
	var request types.Request
	if err := s.proto.Unmarshal(req, &request); err != nil {
		return err
	}
	if request.Method == "pong" {
		var pong types.PingPong
		if err := s.proto.Unmarshal(request.Args, &pong); err != nil {
			return &types.UnKnownPongMessage{}
		}
		atomic.StoreInt64(&conn.lastPong, pong.Ts)
		return nil
	}
	methodName := request.Method

	f, ok := s.handlers.Load(methodName)
	if !ok {
		return &HandlerNotExistError{methodName + " method not found."}
	}
	ht := f.(handlerType)
	methodFunc := ht.method
	argTyp := methodFunc.Type().In(1) // arg
	arg := reflect.New(argTyp).Interface()
	if err := s.proto.Unmarshal(request.Args, arg); err != nil {
		return err
	}
	output := methodFunc.Call([]reflect.Value{reflect.ValueOf(conn.Ctx), reflect.ValueOf(arg).Elem()})

	if ht.methodType == Normal {
		if output[1].IsNil() {
			// output
			// TODO response pool
			var resp types.Response
			resp.Status = 0
			resp.ClientId = request.ClientId
			bs, err := s.proto.Marshal(output[0].Interface())

			if err != nil {
				// error
				return err
			}
			resp.Data = bs
			resp.Method = request.Method
			respBs, err := s.proto.Marshal(resp)
			if err != nil {
				return err
			}
			conn.Write(respBs)
		} else {
			// error
		}
	} else if ht.methodType == Stream {
		if !output[0].IsNil() {
			// error
		}
	}
	return nil
}

func (s *WSServer) error(conn *WsConn, err error) {
	glog.Errorf("%s error: %v", conn.conn.RemoteAddr(), err)
}

func (conn *WsConn) Write(data []byte) error {
	conn.writeMu.Lock()
	conn.conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	if err := conn.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		conn.writeMu.Unlock()
		return err
	}
	conn.writeMu.Unlock()
	return nil
}

func (conn *WsConn) WriteObj(data interface{}) error {
	bs, err := conn.proto.Marshal(data)
	if err != nil {
		return err
	}
	return conn.Write(bs)
}

func (conn *WsConn) heartbeat() {
	// send heart beat
	tick := time.NewTicker(HeartBeatTime)
	var ping types.Response
	ping.Method = "ping"
	ping.Status = 0
	var ts types.PingPong
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			// check last pong
			lastPong := atomic.LoadInt64(&conn.lastPong)
			ts.Ts = util.Timestamp()
			if (lastPong > 0 && ts.Ts-lastPong > 20*1000) || (conn.lastPing > 0 && conn.lastPong == 0) {
				glog.Errorf("Do not recv pong meesage, close client: %s", conn.conn.RemoteAddr().String())
				conn.writeMu.Lock()
				conn.conn.WriteMessage(websocket.CloseMessage, []byte{})
				conn.writeMu.Unlock()
				conn.Cancel()
				return
			}
			// send ping
			pingData, _ := conn.proto.Marshal(ts)
			ping.Data = pingData
			if err := conn.WriteObj(ping); err != nil {
				glog.Errorf("Write ping message for %s error. %v", conn.conn.RemoteAddr(), err)
				conn.Cancel()
				return
			}
			conn.lastPing = ts.Ts
		case <-conn.Ctx.Done():
			return
		}
	}
}
