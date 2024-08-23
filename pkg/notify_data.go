package pkg

// 会话数据
type SessionData struct {
	// 会话编号
	SessionId int64
	// 请求到当前节点的会话地址
	Addr string
}
