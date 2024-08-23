package option

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/viper"

	nano "gnano"
	"gnano/component"
	"gnano/serialize/json"
	"gnano/serialize/protobuf"
)

const PROTOBUF = "protobuf"

const JSON = "json"

// common 公共注册
func common() []nano.Option {
	options := make([]nano.Option, 0)
	enableWs := viper.GetBool("network.enable_ws")
	if enableWs {
		options = append(options, nano.WithIsWebsocket(enableWs))
		options = append(options, nano.WithWSPath(viper.GetString("network.ws_path")))
		options = append(options, nano.WithCheckOriginFunc(func(_ *http.Request) bool { return true }))
	}

	if clientAddr := viper.GetString("network.client_addr"); clientAddr != "" {
		options = append(options, nano.WithClientAddr(viper.GetString("network.client_addr")))
	}

	switch viper.GetString("serializer.format") {
	case PROTOBUF:
		options = append(options, nano.WithSerializer(protobuf.NewSerializer()))
	case JSON:
		options = append(options, nano.WithSerializer(json.NewSerializer()))
	default:
		options = append(options, nano.WithSerializer(protobuf.NewSerializer()))
	}

	if viper.GetBool("debug.enable") {
		options = append(options, nano.WithDebugMode())
	}
	//设置snowflake 节点  https://github.com/bwmarrin/snowflake
	options = append(options, nano.WithNodeId(viper.GetUint64("snowflake_node")))
	version := viper.GetString("app.version")

	heartbeat := viper.GetUint("core.heartbeat")
	if heartbeat > 0 {
		options = append(options, nano.WithHeartbeatInterval(time.Duration(heartbeat)*time.Second))
	}

	connArrayMaxSize := viper.GetUint("grpc.conn_array_max_size")
	if connArrayMaxSize > 0 {
		options = append(options, nano.WithConnArrayMaxSize(connArrayMaxSize))
	}

	forceUpdate := viper.GetBool("app.force")
	fmt.Printf("\t\t\t\t\t\t\t 服务器版本: %s, 是否强制更新: %t, 当前心跳时间间隔: %d秒 \n", version, forceUpdate, heartbeat)
	return options
}

// NodeOptions 单节点启动注册
func NodeOptions(components *component.Components) []nano.Option {
	options := common()
	options = append(options, nano.WithComponents(components))
	return options
}

// ClusterOptions 集群启动注册
func ClusterOptions(components *component.Components) []nano.Option {
	options := common()
	if !viper.GetBool("cluster.is_master") {
		options = append(options, nano.WithAdvertiseAddr(viper.GetString("cluster.advertise_addr")))
	} else {
		options = append(options, nano.WithMaster())
	}
	options = append(options, nano.WithComponents(components))
	return options
}
