package cluster

import (
	"context"
	"net"

	"gnano/cluster/clusterpb"
	"gnano/internal/message"
	"gnano/mock"
	"gnano/pkg"
	"gnano/session"
)

type acceptor struct {
	sid        int64
	gateClient clusterpb.MemberClient
	session    *session.Session
	lastMid    uint64
	rpcHandler rpcHandler
	gateAddr   string
}

// Push implements the session.NetworkEntity interface
func (a *acceptor) Push(route string, v interface{}) error {
	// TODO: buffer
	data, err := message.Serialize(v)
	if err != nil {
		return err
	}
	request := &clusterpb.PushMessage{
		SessionId: a.sid,
		Route:     route,
		Data:      data,
	}
	_, err = a.gateClient.HandlePush(context.Background(), request)
	return err
}

// 新增一个RPC 消息类型 用于服务跟服务之间的远程通讯 传递数据
func (a *acceptor) RPC(route string, v interface{}, sds ...pkg.SessionData) error {
	// TODO: buffer
	data, err := message.Serialize(v)
	if err != nil {
		return err
	}
	msg := &message.Message{
		Type:  message.RPC,
		Route: route,
		Data:  data,
	}
	return a.rpcHandler(a.session, msg, true, sds...)
}

// LastMid implements the session.NetworkEntity interface
func (a *acceptor) LastMid() uint64 {
	return a.lastMid
}

// Response implements the session.NetworkEntity interface
func (a *acceptor) Response(v interface{}) error {
	return a.ResponseMid(a.lastMid, v)
}

// ResponseMid implements the session.NetworkEntity interface
func (a *acceptor) ResponseMid(mid uint64, v interface{}) error {
	// TODO: buffer
	data, err := message.Serialize(v)
	if err != nil {
		return err
	}
	request := &clusterpb.ResponseMessage{
		SessionId: a.sid,
		Id:        mid,
		Data:      data,
	}
	_, err = a.gateClient.HandleResponse(context.Background(), request)
	return err
}

// Close implements the session.NetworkEntity interface
func (a *acceptor) Close() error {
	// TODO: buffer
	request := &clusterpb.CloseSessionRequest{
		SessionId: a.sid,
	}
	_, err := a.gateClient.CloseSession(context.Background(), request)
	return err
}

func (a *acceptor) CloseHandler(ct session.CloseType) error {
	return a.Close()
}

// RemoteAddr implements the session.NetworkEntity interface
func (*acceptor) RemoteAddr() net.Addr {
	return mock.NetAddr{}
}

func (a *acceptor) Kick(kt int32, uIds ...int64) error {
	request := &clusterpb.KickRequest{
		UIds: uIds,
		Type: kt,
	}
	_, err := a.gateClient.Kick(context.Background(), request)
	return err
}

func (a *acceptor) ID() int64 {
	return a.sid
}

func (a *acceptor) RpcClientAddr() string {
	return a.gateAddr
}
