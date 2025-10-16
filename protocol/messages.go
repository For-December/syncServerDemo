package protocol

// 消息类型常量
const (
	// 客户端 -> 服务器
	MsgTypeJoin         = "join"          // 加入游戏
	MsgTypeMove         = "move"          // 移动指令
	MsgTypePositionSync = "position_sync" // 位置同步上报

	// 服务器 -> 客户端
	MsgTypeWelcome        = "welcome"         // 欢迎消息
	MsgTypePlayerJoined   = "player_joined"   // 新玩家加入
	MsgTypePlayerLeft     = "player_left"     // 玩家离开
	MsgTypeMoveCommand    = "move_command"    // 移动指令广播
	MsgTypeTimeSync       = "time_sync"       // 游戏时间同步
	MsgTypePositionUpdate = "position_update" // 位置仲裁结果
)

// JoinData 加入游戏数据
type JoinData struct {
	PlayerID string `json:"player_id"`
}

// MoveData 移动数据
type MoveData struct {
	PlayerID string  `json:"player_id"`
	VectorX  float64 `json:"vector_x"`
	VectorY  float64 `json:"vector_y"`
	GameTime int64   `json:"game_time"` // 游戏时间戳
}

// PositionData 位置数据
type PositionData struct {
	PlayerID string  `json:"player_id"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	GameTime int64   `json:"game_time"` // 对应的游戏时间
}

// PositionSyncData 位置同步上报数据（包含多个玩家的位置）
type PositionSyncData struct {
	Positions []PositionData `json:"positions"`
	GameTime  int64          `json:"game_time"`
}

// WelcomeData 欢迎数据
type WelcomeData struct {
	PlayerID  string         `json:"player_id"`
	GameTime  int64          `json:"game_time"`
	Players   []string       `json:"players"`   // 当前在线玩家
	Positions []PositionData `json:"positions"` // 当前位置
}

// PlayerJoinedData 玩家加入数据
type PlayerJoinedData struct {
	PlayerID string `json:"player_id"`
}

// PlayerLeftData 玩家离开数据
type PlayerLeftData struct {
	PlayerID string `json:"player_id"`
}

// TimeSyncData 时间同步数据
type TimeSyncData struct {
	GameTime int64 `json:"game_time"`
}

// PositionUpdateData 位置更新数据（仲裁后的结果）
type PositionUpdateData struct {
	PlayerID string  `json:"player_id"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	GameTime int64   `json:"game_time"`
}
