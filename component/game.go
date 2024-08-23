package component

import (
	"gnano/component/gamepb"
	"gnano/session"
)

const (
	// Game Call Hall module
	//ROUND_OVER  牌局结束  example: Hall.RoundOver    s.RPC(ROUND_OVER,&gamepb.RoundOver{})   每一轮 游戏结束请求大厅做结算和清除动作
	RPCROUND_OVER = "Hall.RPCRoundOver"
	//TODO 钱包服务(该服务暂时还没有提供)  负责结算  下注操作 后续可以进行一次发送一个整的牌局结算信息
	// s.RPC(RPCSettle,&gamepb.Settle{})
	//每一轮结算  目前是单个玩家请求一次
	// s.RPC(RPCSettle,&gamepb.PlayerSettle{})
	RPCSETTLE = "Hall.RPCSettle"
	// Hall  Call  Game module
	//RPC_BEGIN  开始游戏  example: Xxx.RPCBegin   Xxx 对应子游戏路由前缀
	RPC_BEGIN = "RPCBegin"
	//RPC_REJOIN 登陆重新断线重连 example: Xxx.RPCReJoin
	RPC_REJOIN = "RPCReJoin"
	//  RPC_MIDWAY_ENTER 玩家中途进入游戏 只需要将玩家加入到 group 会话分组里面
	RPC_MIDWAY_ENTER = "RPCMidWayEnter"
	// Deprecated  由于需求中不要观看玩家该函数为过期函数
	RPC_WATCH_LEAVE = "RPCWatchLeave"
	//RPC_DISSOLVE 正常解散
	RPC_DISSOLVE = "RPCDissolve"
	//RPC_FORCE_DISSOLVE 强制解散
	RPC_FORCE_DISSOLVE = "RPCForceDissolve"
	//RPC_RECONNECT   预留  重新连接 example: Xxx.RPC_RECONNECT
	RPC_RECONNECT = "RPCReConnect"
	// RPC_PAUSE	玩家挂后台
	RPC_PAUSE = "RPCPause"
	// RPC_Resume 玩家切后台回来
	RPC_RESUME = "RPCResume"
)

type GameComponent interface {
	Component
	// 开始游戏
	RPCBegin([]*session.Session, *gamepb.Begin) error
	// 重新登陆断线重连 牌桌信息 总共三部分数据  1. 房间基本信息   2.玩家列表 基本信息 位置 数据   3. 牌桌数据(牌桌状态 玩家牌组)
	RPCReJoin(*session.Session, *gamepb.ReJoin) error
	// 正常解散 房间号 牌桌号
	RPCDissolve(*session.Session, *gamepb.Dissolve) error
	// 强制解散   房间号 牌桌号
	RPCForceDissolve(*session.Session, *gamepb.ForceDissolve) error
	//玩家中途进入 需要同步牌桌数据  3. 牌桌数据 将玩家放入到观看列表中
	RPCMidWayEnter(*session.Session, *gamepb.MidWayEnter) error
	//观看玩家离开
	RPCWatchLeave(*session.Session, *gamepb.WatchLeave) error
	//游戏场景内重新连接    房间号 牌桌号 玩家编号
	RPCReConnect(*session.Session, *gamepb.ReConnect) error
	// 玩家挂后台     房间号 牌桌号 玩家编号
	RPCPause(*session.Session, *gamepb.Pause) error
	// 玩家切后台回来 房间号 牌桌号 玩家编号
	RPCResume(*session.Session, *gamepb.Resume) error
}

type GameBase struct{}

// Init was called to initialize the component.
func (c *GameBase) Init() {}

// AfterInit was called after the component is initialized.
func (c *GameBase) AfterInit() {}

// BeforeShutdown was called before the component to shutdown.
func (c *GameBase) BeforeShutdown() {}

// Shutdown was called to shutdown the component.
func (c *GameBase) Shutdown() {}

func (c *GameBase) SchedName() string {
	return ""
}
