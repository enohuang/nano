// Copyright (c) nano Authors. All Rights Reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"

	"gnano/cluster/clusterpb"
	"gnano/component"
	"gnano/internal/env"
	"gnano/internal/log"
	"gnano/internal/message"
	"gnano/pipeline"
	"gnano/scheduler"
	"gnano/session"
)

// Options contains some configurations for current node
type Options struct {
	Pipeline           pipeline.Pipeline
	IsMaster           bool
	AdvertiseAddr      string
	RetryInterval      time.Duration
	ClientAddr         string
	Components         *component.Components
	Label              string
	IsWebsocket        bool
	TSLCertificate     string
	TSLKey             string
	UnregisterCallback func(Member)
	MethodDescExpand   *grpc.MethodDesc
	RemoteServiceRoute CustomerRemoteServiceRoute
}

// Node represents a node in nano cluster, which will contains a group of services.
// All services will register to cluster and messages will be forwarded to the node
// which provides respective service
type Node struct {
	Options            // current node options
	ServiceAddr string // current server service address (RPC)

	cluster   *cluster
	handler   *LocalHandler
	server    *grpc.Server
	rpcClient *rpcClient

	mu       sync.RWMutex
	sessions map[int64]*session.Session

	once          sync.Once
	keepaliveExit chan struct{}
}

func (n *Node) Startup() error {
	if n.ServiceAddr == "" {
		return errors.New("service address cannot be empty in master node")
	}
	n.sessions = map[int64]*session.Session{}
	n.cluster = newCluster(n)
	n.handler = NewHandler(n, n.Pipeline)
	components := n.Components.List()
	for _, c := range components {
		err := n.handler.register(c.Comp, c.Opts)
		if err != nil {
			return err
		}
	}

	cache()
	if err := n.initNode(); err != nil {
		return err
	}

	// Initialize all components
	for _, c := range components {
		c.Comp.Init()
	}
	for _, c := range components {
		c.Comp.AfterInit()
	}

	if n.ClientAddr != "" {
		go func() {
			if n.IsWebsocket {
				if len(n.TSLCertificate) != 0 {
					n.listenAndServeWSTLS()
				} else {
					n.listenAndServeWS()
				}
			} else {
				n.listenAndServe()
			}
		}()
	}

	return nil
}

func (n *Node) Handler() *LocalHandler {
	return n.handler
}

// 根据服务监听地址获取到节点的RPC客户端对象
func (n *Node) MemberClient(addr string) (clusterpb.MemberClient, error) {
	pool, err := n.rpcClient.getConnPool(addr)
	if err != nil {
		return nil, err
	}
	client := clusterpb.NewMemberClient(pool.Get())
	return client, nil
}

func (n *Node) initNode() error {
	// Current node is not master server and does not contains master
	// address, so running in singleton mode
	if !n.IsMaster && n.AdvertiseAddr == "" {
		return nil
	}

	listener, err := net.Listen("tcp", n.ServiceAddr)
	if err != nil {
		return err
	}

	// Initialize the gRPC server and register service
	n.server = grpc.NewServer()
	n.rpcClient = newRPCClient()

	// TODO 为当前节点拓展rpc 接口
	if n.Options.MethodDescExpand != nil {
		clusterpb.Member_ServiceDesc.Methods = append(clusterpb.Member_ServiceDesc.Methods, *n.Options.MethodDescExpand)
	}

	clusterpb.RegisterMemberServer(n.server, n)
	go func() {
		err := n.server.Serve(listener)
		if err != nil {
			log.Fatalf("Start current node failed: %v", err)
		}
	}()

	if n.IsMaster {
		clusterpb.RegisterMasterServer(n.server, n.cluster)
		member := &Member{
			isMaster: true,
			memberInfo: &clusterpb.MemberInfo{
				Label:       n.Label,
				ServiceAddr: n.ServiceAddr,
				Services:    n.handler.LocalService(),
			},
		}
		n.cluster.members = append(n.cluster.members, member)
		n.cluster.setRpcClient(n.rpcClient)
	} else {
		pool, err := n.rpcClient.getConnPool(n.AdvertiseAddr)
		if err != nil {
			return err
		}
		client := clusterpb.NewMasterClient(pool.Get())
		request := &clusterpb.RegisterRequest{
			MemberInfo: &clusterpb.MemberInfo{
				Label:       n.Label,
				ServiceAddr: n.ServiceAddr,
				Services:    n.handler.LocalService(),
			},
		}
		for {
			resp, err := client.Register(context.Background(), request)
			if err == nil {
				n.handler.initRemoteService(resp.Members)
				n.cluster.initMembers(resp.Members)
				break
			}
			log.Println("Register current node to cluster failed", err, "and will retry in", n.RetryInterval.String())
			time.Sleep(n.RetryInterval)
		}
		n.once.Do(n.keepalive)
	}
	return nil
}

// Shutdowns all components registered by application, that
// call by reverse order against register
func (n *Node) Shutdown() {
	// reverse call `BeforeShutdown` hooks
	components := n.Components.List()
	length := len(components)
	for i := length - 1; i >= 0; i-- {
		components[i].Comp.BeforeShutdown()
	}

	// reverse call `Shutdown` hooks
	for i := length - 1; i >= 0; i-- {
		components[i].Comp.Shutdown()
	}
	// close sendHeartbeat
	if n.keepaliveExit != nil {
		close(n.keepaliveExit)
	}
	if !n.IsMaster && n.AdvertiseAddr != "" {
		pool, err := n.rpcClient.getConnPool(n.AdvertiseAddr)
		if err != nil {
			log.Println("Retrieve master address error", err)
			goto EXIT
		}
		client := clusterpb.NewMasterClient(pool.Get())
		request := &clusterpb.UnregisterRequest{
			ServiceAddr: n.ServiceAddr,
		}
		_, err = client.Unregister(context.Background(), request)
		if err != nil {
			log.Println("Unregister current node failed", err)
			goto EXIT
		}
	}

EXIT:
	if n.server != nil {
		n.server.GracefulStop()
	}
}

// Enable current server accept connection
func (n *Node) listenAndServe() {
	listener, err := net.Listen("tcp", n.ClientAddr)
	if err != nil {
		log.Fatal(err.Error())
	}

	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err.Error())
			continue
		}

		go n.handler.handle(conn)
	}
}

func (n *Node) listenAndServeWS() {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     env.CheckOrigin,
	}

	http.HandleFunc("/"+strings.TrimPrefix(env.WSPath, "/"), func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(fmt.Sprintf("Upgrade failure, URI=%s, Error=%s", r.RequestURI, err.Error()))
			return
		}
		// 获取真实IP
		forwardes := r.Header.Get("X-Forwarded-For")
		log.Println(fmt.Sprintf("Upgrade Success, X-Forwarded-For=%s", forwardes))
		var readlRemote string
		if forwardes != "" {
			forwardes := strings.Split(forwardes, ",")
			if len(forwardes) >= 1 {
				readlRemote = strings.TrimSpace(forwardes[len(forwardes)-1])
			}
		}
		log.Println(fmt.Sprintf("Upgrade Success, readlRemote=%s", readlRemote))
		n.handler.handleWS(conn, readlRemote)
	})

	if err := http.ListenAndServe(n.ClientAddr, nil); err != nil {
		log.Fatal(err.Error())
	}
}

func (n *Node) listenAndServeWSTLS() {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     env.CheckOrigin,
	}

	http.HandleFunc("/"+strings.TrimPrefix(env.WSPath, "/"), func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(fmt.Sprintf("Upgrade failure, URI=%s, Error=%s", r.RequestURI, err.Error()))
			return
		}
		// 获取真实IP
		forwardes := r.Header.Get("X-Forwarded-For")
		log.Println(fmt.Sprintf("Upgrade Success, X-Forwarded-For=%s", forwardes))
		var readlRemote string
		if forwardes != "" {
			forwardes := strings.Split(forwardes, ",")
			if len(forwardes) >= 1 {
				readlRemote = strings.TrimSpace(forwardes[len(forwardes)-1])
			}
		}
		log.Println(fmt.Sprintf("Upgrade Success, readlRemote=%s", readlRemote))
		n.handler.handleWS(conn, readlRemote)
	})

	if err := http.ListenAndServeTLS(n.ClientAddr, n.TSLCertificate, n.TSLKey, nil); err != nil {
		log.Fatal(err.Error())
	}
}

func (n *Node) storeSession(s *session.Session) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sessions[s.ID()] = s
	if env.Debug {
		log.Println("storeSession:", len(n.sessions))
	}

}

func (n *Node) findSession(sid int64) *session.Session {
	n.mu.RLock()
	defer n.mu.RUnlock()
	s := n.sessions[sid]

	return s
}

func (n *Node) findSessionByUid(uId int64) (*session.Session, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	for _, s := range n.sessions {
		if s.UID() == uId {
			return s, true
		}
	}
	return nil, false
}

func (n *Node) findSessionByUids(uIds ...int64) ([]*session.Session, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	sessions := make([]*session.Session, 0)
	for i := 0; i < len(uIds); i++ {
		for _, s := range n.sessions {
			if s.UID() == uIds[i] {
				sessions = append(sessions, s)
			}
		}
	}
	if len(sessions) <= 0 {
		return nil, false
	}
	return sessions, true
}

func (n *Node) findOrCreateSession(sid int64, gateAddr string, uId int64) (*session.Session, error) {
	n.mu.RLock()
	s, found := n.sessions[sid]
	n.mu.RUnlock()
	if !found {
		conns, err := n.rpcClient.getConnPool(gateAddr)
		if err != nil {
			return nil, err
		}
		ac := &acceptor{
			sid:        sid,
			gateClient: clusterpb.NewMemberClient(conns.Get()),
			rpcHandler: n.handler.remoteProcess,
			gateAddr:   gateAddr,
		}
		s = session.NewBindUid(ac, uId)
		ac.session = s
		n.mu.Lock()
		n.sessions[sid] = s
		n.mu.Unlock()
	}
	return s, nil
}

func (n *Node) HandleRequest(_ context.Context, req *clusterpb.RequestMessage) (*clusterpb.MemberHandleResponse, error) {
	handler, found := n.handler.localHandlers[req.Route]
	if !found {
		return nil, fmt.Errorf("service not found in current node: %v", req.Route)
	}
	s, err := n.findOrCreateSession(req.SessionId, req.GateAddr, req.UId)
	if err != nil {
		return nil, err
	}
	msg := &message.Message{
		Type:  message.Request,
		ID:    req.Id,
		Route: req.Route,
		Data:  req.Data,
	}
	n.handler.localProcess(handler, req.Id, s, msg)
	return &clusterpb.MemberHandleResponse{}, nil
}

func (n *Node) HandleNotify(_ context.Context, req *clusterpb.NotifyMessage) (*clusterpb.MemberHandleResponse, error) {
	handler, found := n.handler.localHandlers[req.Route]
	if !found {
		return nil, fmt.Errorf("service not found in current node: %v", req.Route)
	}
	s, err := n.findOrCreateSession(req.SessionId, req.GateAddr, req.UId)
	if err != nil {
		return nil, err
	}
	msg := &message.Message{
		Type:  message.Notify,
		Route: req.Route,
		Data:  req.Data,
	}
	n.handler.localProcess(handler, 0, s, msg)
	return &clusterpb.MemberHandleResponse{}, nil
}

func (n *Node) HandleRPC(_ context.Context, req *clusterpb.RPCMessage) (*clusterpb.MemberHandleResponse, error) {
	handler, found := n.handler.localHandlers[req.Route]
	if !found {
		return nil, fmt.Errorf("service not found in current node: %v", req.Route)
	}

	// 是否需要使用旧的sessionId 设置为本地新建的会话对象sessionId
	if len(req.Sessions) <= 0 {
		req.Sessions = make(map[int64]string, 0)
		req.Sessions[req.SessionId] = req.GateAddr
	}

	// 为多个会话编号生成会话对象
	var sessions = make([]*session.Session, 0)
	for k, v := range req.Sessions {
		s, err := n.findOrCreateSession(k, v, req.UId)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	msg := &message.Message{
		Type:  message.RPC,
		Route: req.Route,
		Data:  req.Data,
	}
	n.handler.localProcess(handler, 0, sessions[0], msg, sessions...)
	return &clusterpb.MemberHandleResponse{}, nil
}

func (n *Node) HandlePush(_ context.Context, req *clusterpb.PushMessage) (*clusterpb.MemberHandleResponse, error) {
	s := n.findSession(req.SessionId)
	if s == nil {
		return &clusterpb.MemberHandleResponse{}, fmt.Errorf("session not found: %v", req.SessionId)
	}
	err := s.Push(req.Route, req.Data)
	if err != nil && errors.Is(err, ErrBrokenPipe) {
		return &clusterpb.MemberHandleResponse{}, nil
	}

	return &clusterpb.MemberHandleResponse{}, err
}

func (n *Node) HandleResponse(_ context.Context, req *clusterpb.ResponseMessage) (*clusterpb.MemberHandleResponse, error) {
	s := n.findSession(req.SessionId)
	if s == nil {
		return &clusterpb.MemberHandleResponse{}, fmt.Errorf("session not found: %v", req.SessionId)
	}
	err := s.ResponseMID(req.Id, req.Data)
	if err != nil && (errors.Is(err, ErrBrokenPipe) || errors.Is(err, ErrSessionOnNotify)) {
		return &clusterpb.MemberHandleResponse{}, nil
	}
	return &clusterpb.MemberHandleResponse{}, err
}

func (n *Node) NewMember(_ context.Context, req *clusterpb.NewMemberRequest) (*clusterpb.NewMemberResponse, error) {
	log.Println("NewMember member", req.String())
	n.handler.addRemoteService(req.MemberInfo)
	n.cluster.addMember(req.MemberInfo)
	return &clusterpb.NewMemberResponse{}, nil
}

func (n *Node) DelMember(_ context.Context, req *clusterpb.DelMemberRequest) (*clusterpb.DelMemberResponse, error) {
	log.Println("DelMember member", req.String())
	n.handler.delMember(req.ServiceAddr)
	n.cluster.delMember(req.ServiceAddr)
	return &clusterpb.DelMemberResponse{}, nil
}

// SessionClosed implements the MemberServer interface
// 当网关中连接对象断开连接后 进行其它节点中的会话对象移除
func (n *Node) SessionClosed(_ context.Context, req *clusterpb.SessionClosedRequest) (*clusterpb.SessionClosedResponse, error) {
	n.mu.Lock()
	s, found := n.sessions[req.SessionId]
	delete(n.sessions, req.SessionId)
	n.mu.Unlock()
	if found {
		scheduler.PushTask(func() { session.Lifetime.Close(s) })
	}
	// 还存在其它相同玩家编号的会话对象需要移除
	// 会同步移除掉其它节点中相同玩家编号的session
	us, found := n.findSessionByUid(req.UId)
	if found {
		n.mu.Lock()
		delete(n.sessions, us.ID())
		n.mu.Unlock()
	}
	return &clusterpb.SessionClosedResponse{}, nil
}

// CloseSession implements the MemberServer interface
func (n *Node) CloseSession(_ context.Context, req *clusterpb.CloseSessionRequest) (*clusterpb.CloseSessionResponse, error) {
	n.mu.Lock()
	s, found := n.sessions[req.SessionId]
	delete(n.sessions, req.SessionId)
	n.mu.Unlock()
	if found {
		// 节点主动发起会话关闭
		s.Close()
	}
	return &clusterpb.CloseSessionResponse{}, nil
}

func (n *Node) Kick(_ context.Context, req *clusterpb.KickRequest) (*clusterpb.KickResponse, error) {
	var ss []*session.Session
	var found bool
	// 服务器维护踢人
	if req.Type == message.RepairKick {
		n.mu.Lock()
		ss = make([]*session.Session, 0)
		for _, v := range n.sessions {
			ss = append(ss, v)
		}
		n.sessions = make(map[int64]*session.Session)
		n.mu.Unlock()
	} else {
		// 非服务器维护踢人
		ss, found = n.findSessionByUids(req.UIds...)
		if found {
			n.mu.Lock()
			for i := 0; i < len(ss); i++ {
				delete(n.sessions, ss[i].ID())
			}
			n.mu.Unlock()
		}
	}

	// 批量踢人
	for i := 0; i < len(ss); i++ {
		if ss[i] != nil {
			//  gate   调用的是agent.go 下的
			ss[i].Kick(req.Type, ss[i].UID())
		}
	}
	return &clusterpb.KickResponse{}, nil
}

func (n *Node) SwithMode(_ context.Context, req *clusterpb.SwithModeRequest) (*clusterpb.SwithModeResponse, error) {
	env.Mode = env.ModeType(req.Mode)
	return &clusterpb.SwithModeResponse{}, nil
}

// ticker send heartbeat register info to master
func (n *Node) keepalive() {
	if n.keepaliveExit == nil {
		n.keepaliveExit = make(chan struct{})
	}
	if n.AdvertiseAddr == "" || n.IsMaster {
		return
	}
	heartbeat := func() {
		pool, err := n.rpcClient.getConnPool(n.AdvertiseAddr)
		if err != nil {
			log.Println("rpcClient master conn", err)
			return
		}
		masterCli := clusterpb.NewMasterClient(pool.Get())
		if _, err := masterCli.Heartbeat(context.Background(), &clusterpb.HeartbeatRequest{
			MemberInfo: &clusterpb.MemberInfo{
				Label:       n.Label,
				ServiceAddr: n.ServiceAddr,
				Services:    n.handler.LocalService(),
			},
		}); err != nil {
			log.Println("Member send heartbeat error", err)
		}
	}
	go func() {
		ticker := time.NewTicker(env.Heartbeat)
		for {
			select {
			case <-ticker.C:
				heartbeat()
			case <-n.keepaliveExit:
				log.Println("Exit member node heartbeat ")
				ticker.Stop()
				return
			}
		}
	}()
}
