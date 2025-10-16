package gamesync

import (
	"sync"
	"time"
)

// TimeSynchronizer 游戏时间同步器
// 确保所有客户端使用相同的游戏时间基准
type TimeSynchronizer struct {
	startTime time.Time // 游戏开始的真实时间
	mu        sync.RWMutex
}

// NewTimeSynchronizer 创建时间同步器
func NewTimeSynchronizer() *TimeSynchronizer {
	return &TimeSynchronizer{
		startTime: time.Now(),
	}
}

// GetGameTime 获取当前游戏时间（毫秒）
func (ts *TimeSynchronizer) GetGameTime() int64 {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	elapsed := time.Since(ts.startTime)
	return elapsed.Milliseconds()
}

// Reset 重置游戏时间
func (ts *TimeSynchronizer) Reset() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.startTime = time.Now()
}

// SetGameTime 设置游戏时间（用于同步）
func (ts *TimeSynchronizer) SetGameTime(gameTime int64) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.startTime = time.Now().Add(-time.Duration(gameTime) * time.Millisecond)
}
