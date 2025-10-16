package server

import (
	"encoding/json"
	"log"
	"sync"
	"syncServerDemo/gamesync"
	"syncServerDemo/protocol"
	"syncServerDemo/transport"
	"time"
)

// GameServer 游戏服务器
type GameServer struct {
	transport  transport.Transport
	timeSyncer *gamesync.TimeSynchronizer
	arbitrator *gamesync.PositionArbitrator

	players map[string]*PlayerState // 玩家状态
	mu      sync.RWMutex

	positionReports map[string]map[string]protocol.PositionData // [playerID][reporterID]position
	reportMu        sync.RWMutex

	running  bool
	stopChan chan struct{}
}

// PlayerState 玩家状态
type PlayerState struct {
	PlayerID string
	X        float64
	Y        float64
	LastSync int64 // 最后同步时间
}

// NewGameServer 创建游戏服务器
func NewGameServer(transport transport.Transport) *GameServer {
	return &GameServer{
		transport:       transport,
		timeSyncer:      gamesync.NewTimeSynchronizer(),
		arbitrator:      gamesync.NewPositionArbitrator(1.0), // 1.0单位的误差容忍
		players:         make(map[string]*PlayerState),
		positionReports: make(map[string]map[string]protocol.PositionData),
		stopChan:        make(chan struct{}),
	}
}

// Start 启动服务器
func (s *GameServer) Start() error {
	s.running = true

	// 启动消息处理协程
	go s.messageLoop()

	// 启动时间同步协程
	go s.timeSyncLoop()

	// 启动位置仲裁协程
	go s.arbitrationLoop()

	log.Println("Game server started")
	return nil
}

// Stop 停止服务器
func (s *GameServer) Stop() {
	s.running = false
	close(s.stopChan)
	s.transport.Close()
	log.Println("Game server stopped")
}

// messageLoop 消息处理循环
func (s *GameServer) messageLoop() {
	for s.running {
		clientID, msg, err := s.transport.Receive()
		if err != nil {
			if s.running {
				log.Printf("Error receiving message: %v", err)
			}
			break
		}

		s.handleMessage(clientID, msg)
	}
}

// handleMessage 处理消息
func (s *GameServer) handleMessage(clientID string, msg transport.Message) {
	switch msg.GetType() {
	case protocol.MsgTypeJoin:
		s.handleJoin(clientID, msg)
	case protocol.MsgTypeMove:
		s.handleMove(clientID, msg)
	case protocol.MsgTypePositionSync:
		s.handlePositionSync(clientID, msg)
	default:
		log.Printf("Unknown message type: %s", msg.GetType())
	}
}

// handleJoin 处理加入游戏
func (s *GameServer) handleJoin(clientID string, msg transport.Message) {
	data, err := s.parseData(msg, &protocol.JoinData{})
	if err != nil {
		log.Printf("Error parsing join data: %v", err)
		return
	}

	joinData := data.(*protocol.JoinData)
	playerID := joinData.PlayerID

	s.mu.Lock()
	s.players[playerID] = &PlayerState{
		PlayerID: playerID,
		X:        0,
		Y:        0,
		LastSync: s.timeSyncer.GetGameTime(),
	}

	// 获取当前所有玩家
	players := make([]string, 0, len(s.players))
	positions := make([]protocol.PositionData, 0, len(s.players))
	for _, p := range s.players {
		players = append(players, p.PlayerID)
		positions = append(positions, protocol.PositionData{
			PlayerID: p.PlayerID,
			X:        p.X,
			Y:        p.Y,
			GameTime: p.LastSync,
		})
	}
	s.mu.Unlock()

	// 发送欢迎消息
	welcomeMsg := transport.NewMessage(protocol.MsgTypeWelcome, protocol.WelcomeData{
		PlayerID:  playerID,
		GameTime:  s.timeSyncer.GetGameTime(),
		Players:   players,
		Positions: positions,
	})
	s.transport.Send(clientID, welcomeMsg)

	// 广播新玩家加入
	joinedMsg := transport.NewMessage(protocol.MsgTypePlayerJoined, protocol.PlayerJoinedData{
		PlayerID: playerID,
	})
	s.transport.Broadcast(joinedMsg, clientID)

	log.Printf("Player %s joined the game", playerID)
}

// handleMove 处理移动指令
func (s *GameServer) handleMove(clientID string, msg transport.Message) {
	data, err := s.parseData(msg, &protocol.MoveData{})
	if err != nil {
		log.Printf("Error parsing move data: %v", err)
		return
	}

	moveData := data.(*protocol.MoveData)

	// 服务器只转发移动指令，不计算位置
	broadcastMsg := transport.NewMessage(protocol.MsgTypeMoveCommand, moveData)
	s.transport.Broadcast(broadcastMsg, "")

	log.Printf("Broadcasting move command from %s: vector(%.2f, %.2f) at time %d",
		moveData.PlayerID, moveData.VectorX, moveData.VectorY, moveData.GameTime)
}

// handlePositionSync 处理位置同步上报
func (s *GameServer) handlePositionSync(clientID string, msg transport.Message) {
	data, err := s.parseData(msg, &protocol.PositionSyncData{})
	if err != nil {
		log.Printf("Error parsing position sync data: %v", err)
		return
	}

	syncData := data.(*protocol.PositionSyncData)

	s.reportMu.Lock()
	for _, pos := range syncData.Positions {
		if s.positionReports[pos.PlayerID] == nil {
			s.positionReports[pos.PlayerID] = make(map[string]protocol.PositionData)
		}
		s.positionReports[pos.PlayerID][clientID] = pos
	}
	s.reportMu.Unlock()

	log.Printf("Received position sync from %s for %d players at game time %d",
		clientID, len(syncData.Positions), syncData.GameTime)
}

// timeSyncLoop 时间同步循环
func (s *GameServer) timeSyncLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			gameTime := s.timeSyncer.GetGameTime()
			syncMsg := transport.NewMessage(protocol.MsgTypeTimeSync, protocol.TimeSyncData{
				GameTime: gameTime,
			})
			s.transport.Broadcast(syncMsg, "")
		case <-s.stopChan:
			return
		}
	}
}

// arbitrationLoop 位置仲裁循环
func (s *GameServer) arbitrationLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.performArbitration()
		case <-s.stopChan:
			return
		}
	}
}

// performArbitration 执行位置仲裁
func (s *GameServer) performArbitration() {
	s.reportMu.Lock()
	reports := s.positionReports
	s.positionReports = make(map[string]map[string]protocol.PositionData)
	s.reportMu.Unlock()

	if len(reports) == 0 {
		return
	}

	for playerID, reportMap := range reports {
		positions := make([]protocol.PositionData, 0, len(reportMap))
		for _, pos := range reportMap {
			positions = append(positions, pos)
		}

		// 仲裁位置
		arbitratedPos := s.arbitrator.Arbitrate(positions)
		if arbitratedPos != nil {
			// 更新服务器状态
			s.mu.Lock()
			if player, exists := s.players[playerID]; exists {
				player.X = arbitratedPos.X
				player.Y = arbitratedPos.Y
				player.LastSync = arbitratedPos.GameTime
			}
			s.mu.Unlock()

			// 广播仲裁结果
			updateMsg := transport.NewMessage(protocol.MsgTypePositionUpdate, protocol.PositionUpdateData{
				PlayerID: arbitratedPos.PlayerID,
				X:        arbitratedPos.X,
				Y:        arbitratedPos.Y,
				GameTime: arbitratedPos.GameTime,
			})
			s.transport.Broadcast(updateMsg, "")

			log.Printf("Arbitrated position for %s: (%.2f, %.2f) based on %d reports",
				playerID, arbitratedPos.X, arbitratedPos.Y, len(positions))
		}
	}
}

// parseData 解析消息数据
func (s *GameServer) parseData(msg transport.Message, target interface{}) (interface{}, error) {
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

// GetPlayerCount 获取在线玩家数
func (s *GameServer) GetPlayerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.players)
}
