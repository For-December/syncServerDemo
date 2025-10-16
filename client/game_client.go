package client

import (
	"encoding/json"
	"log"
	"math"
	"sync"
	"syncServerDemo/gamesync"
	"syncServerDemo/protocol"
	"syncServerDemo/transport"
	"time"
)

// GameClient 游戏客户端
type GameClient struct {
	clientID       string
	playerID       string
	localTransport *transport.LocalTransport
	timeSyncer     *gamesync.TimeSynchronizer

	// 本地游戏状态
	localPlayers map[string]*LocalPlayerState
	mu           sync.RWMutex

	running  bool
	stopChan chan struct{}

	// 移动速度（单位/秒）
	moveSpeed float64
}

// LocalPlayerState 本地玩家状态
type LocalPlayerState struct {
	PlayerID       string
	X              float64
	Y              float64
	VelocityX      float64 // 当前速度向量
	VelocityY      float64
	LastUpdateTime int64 // 最后更新的游戏时间
}

// NewGameClient 创建游戏客户端
func NewGameClient(clientID, playerID string, localTransport *transport.LocalTransport) *GameClient {
	return &GameClient{
		clientID:       clientID,
		playerID:       playerID,
		localTransport: localTransport,
		timeSyncer:     gamesync.NewTimeSynchronizer(),
		localPlayers:   make(map[string]*LocalPlayerState),
		stopChan:       make(chan struct{}),
		moveSpeed:      10.0, // 10单位/秒
	}
}

// Start 启动客户端
func (c *GameClient) Start() error {
	c.running = true

	// 发送加入游戏请求
	joinMsg := transport.NewMessage(protocol.MsgTypeJoin, protocol.JoinData{
		PlayerID: c.playerID,
	})
	_ = c.localTransport.SendToServer(c.clientID, joinMsg)

	// 启动消息接收循环
	go c.messageLoop()

	// 启动位置计算和上报循环
	go c.syncLoop()

	log.Printf("[Client %s] Started for player %s", c.clientID, c.playerID)
	return nil
}

// Stop 停止客户端
func (c *GameClient) Stop() {
	c.running = false
	close(c.stopChan)
	log.Printf("[Client %s] Stopped", c.clientID)
}

// messageLoop 消息接收循环
func (c *GameClient) messageLoop() {
	ch, err := c.localTransport.GetClientChannel(c.clientID)
	if err != nil {
		log.Printf("[Client %s] Error getting channel: %v", c.clientID, err)
		return
	}

	for msg := range ch {
		c.handleMessage(msg)
	}
}

// handleMessage 处理消息
func (c *GameClient) handleMessage(msg transport.Message) {
	switch msg.GetType() {
	case protocol.MsgTypeWelcome:
		c.handleWelcome(msg)
	case protocol.MsgTypePlayerJoined:
		c.handlePlayerJoined(msg)
	case protocol.MsgTypeMoveCommand:
		c.handleMoveCommand(msg)
	case protocol.MsgTypeTimeSync:
		c.handleTimeSync(msg)
	case protocol.MsgTypePositionUpdate:
		c.handlePositionUpdate(msg)
	}
}

// handleWelcome 处理欢迎消息
func (c *GameClient) handleWelcome(msg transport.Message) {
	data, err := c.parseData(msg, &protocol.WelcomeData{})
	if err != nil {
		log.Printf("[Client %s] Error parsing welcome data: %v", c.clientID, err)
		return
	}

	welcomeData := data.(*protocol.WelcomeData)

	// 同步游戏时间
	c.timeSyncer.SetGameTime(welcomeData.GameTime)

	// 初始化本地玩家状态
	c.mu.Lock()
	for _, pos := range welcomeData.Positions {
		c.localPlayers[pos.PlayerID] = &LocalPlayerState{
			PlayerID:       pos.PlayerID,
			X:              pos.X,
			Y:              pos.Y,
			VelocityX:      0,
			VelocityY:      0,
			LastUpdateTime: pos.GameTime,
		}
	}
	c.mu.Unlock()

	log.Printf("[Client %s] Welcomed! Game time: %d, Players: %v",
		c.clientID, welcomeData.GameTime, welcomeData.Players)
}

// handlePlayerJoined 处理玩家加入
func (c *GameClient) handlePlayerJoined(msg transport.Message) {
	data, err := c.parseData(msg, &protocol.PlayerJoinedData{})
	if err != nil {
		return
	}

	joinedData := data.(*protocol.PlayerJoinedData)

	c.mu.Lock()
	if _, exists := c.localPlayers[joinedData.PlayerID]; !exists {
		c.localPlayers[joinedData.PlayerID] = &LocalPlayerState{
			PlayerID:       joinedData.PlayerID,
			X:              0,
			Y:              0,
			VelocityX:      0,
			VelocityY:      0,
			LastUpdateTime: c.timeSyncer.GetGameTime(),
		}
	}
	c.mu.Unlock()

	log.Printf("[Client %s] Player %s joined", c.clientID, joinedData.PlayerID)
}

// handleMoveCommand 处理移动指令（客户端计算移动）
func (c *GameClient) handleMoveCommand(msg transport.Message) {
	data, err := c.parseData(msg, &protocol.MoveData{})
	if err != nil {
		return
	}

	moveData := data.(*protocol.MoveData)

	c.mu.Lock()
	defer c.mu.Unlock()

	player, exists := c.localPlayers[moveData.PlayerID]
	if !exists {
		return
	}

	// 先根据旧速度计算到当前时间的位置
	c.updatePlayerPosition(player, moveData.GameTime)

	// 设置新的速度向量
	player.VelocityX = moveData.VectorX * c.moveSpeed
	player.VelocityY = moveData.VectorY * c.moveSpeed

	log.Printf("[Client %s] Player %s moving with velocity (%.2f, %.2f)",
		c.clientID, moveData.PlayerID, player.VelocityX, player.VelocityY)
}

// handleTimeSync 处理时间同步
func (c *GameClient) handleTimeSync(msg transport.Message) {
	data, err := c.parseData(msg, &protocol.TimeSyncData{})
	if err != nil {
		return
	}

	timeSyncData := data.(*protocol.TimeSyncData)

	// 微调本地时间
	localTime := c.timeSyncer.GetGameTime()
	diff := timeSyncData.GameTime - localTime

	// 如果差异超过100ms，才进行调整
	if math.Abs(float64(diff)) > 100 {
		c.timeSyncer.SetGameTime(timeSyncData.GameTime)
		log.Printf("[Client %s] Time synced: %d (diff: %d ms)", c.clientID, timeSyncData.GameTime, diff)
	}
}

// handlePositionUpdate 处理位置仲裁结果
func (c *GameClient) handlePositionUpdate(msg transport.Message) {
	data, err := c.parseData(msg, &protocol.PositionUpdateData{})
	if err != nil {
		return
	}

	updateData := data.(*protocol.PositionUpdateData)

	c.mu.Lock()
	defer c.mu.Unlock()

	player, exists := c.localPlayers[updateData.PlayerID]
	if !exists {
		return
	}

	// 计算误差
	localX, localY := c.predictPosition(player, updateData.GameTime)
	errorX := updateData.X - localX
	errorY := updateData.Y - localY
	distance := math.Sqrt(errorX*errorX + errorY*errorY)

	// 如果误差较大，进行校正
	if distance > 0.5 {
		player.X = updateData.X
		player.Y = updateData.Y
		player.LastUpdateTime = updateData.GameTime
		log.Printf("[Client %s] Position corrected for %s: (%.2f, %.2f), error: %.2f",
			c.clientID, updateData.PlayerID, updateData.X, updateData.Y, distance)
	}
}

// syncLoop 同步循环：定期上报位置
func (c *GameClient) syncLoop() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.reportPositions()
		case <-c.stopChan:
			return
		}
	}
}

// reportPositions 上报所有玩家的位置
func (c *GameClient) reportPositions() {
	gameTime := c.timeSyncer.GetGameTime()

	c.mu.Lock()
	positions := make([]protocol.PositionData, 0, len(c.localPlayers))
	for _, player := range c.localPlayers {
		x, y := c.predictPosition(player, gameTime)
		positions = append(positions, protocol.PositionData{
			PlayerID: player.PlayerID,
			X:        x,
			Y:        y,
			GameTime: gameTime,
		})
	}
	c.mu.Unlock()

	if len(positions) > 0 {
		syncMsg := transport.NewMessage(protocol.MsgTypePositionSync, protocol.PositionSyncData{
			Positions: positions,
			GameTime:  gameTime,
		})
		_ = c.localTransport.SendToServer(c.clientID, syncMsg)
	}
}

// Move 发起移动
func (c *GameClient) Move(vectorX, vectorY float64) {
	gameTime := c.timeSyncer.GetGameTime()
	moveMsg := transport.NewMessage(protocol.MsgTypeMove, protocol.MoveData{
		PlayerID: c.playerID,
		VectorX:  vectorX,
		VectorY:  vectorY,
		GameTime: gameTime,
	})
	_ = c.localTransport.SendToServer(c.clientID, moveMsg)
}

// updatePlayerPosition 更新玩家位置到指定游戏时间
func (c *GameClient) updatePlayerPosition(player *LocalPlayerState, targetTime int64) {
	deltaTime := float64(targetTime-player.LastUpdateTime) / 1000.0 // 转换为秒

	player.X += player.VelocityX * deltaTime
	player.Y += player.VelocityY * deltaTime
	player.LastUpdateTime = targetTime
}

// predictPosition 预测玩家在指定时间的位置
func (c *GameClient) predictPosition(player *LocalPlayerState, targetTime int64) (float64, float64) {
	deltaTime := float64(targetTime-player.LastUpdateTime) / 1000.0

	x := player.X + player.VelocityX*deltaTime
	y := player.Y + player.VelocityY*deltaTime

	return x, y
}

// GetPlayerPosition 获取玩家当前位置
func (c *GameClient) GetPlayerPosition(playerID string) (x, y float64, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	player, exists := c.localPlayers[playerID]
	if !exists {
		return 0, 0, false
	}

	gameTime := c.timeSyncer.GetGameTime()
	x, y = c.predictPosition(player, gameTime)
	return x, y, true
}

// parseData 解析消息数据
func (c *GameClient) parseData(msg transport.Message, target interface{}) (interface{}, error) {
	dataBytes, err := json.Marshal(msg.GetData())
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(dataBytes, target)
	if err != nil {
		return nil, err
	}

	return target, nil
}
