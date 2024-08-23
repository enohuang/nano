package nano

import (
	"gnano/cluster/clusterpb"
	"gnano/internal/runtime"
)

// 通过服务地址来获取到节点的RPC client
func MemberClientByAddr(addr string) (clusterpb.MemberClient, error) {
	return runtime.CurrentNode.MemberClient(addr)
}
