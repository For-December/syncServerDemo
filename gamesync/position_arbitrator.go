package gamesync

import (
	"math"
	"syncServerDemo/protocol"
)

// PositionArbitrator 位置仲裁器
// 使用多数投票机制确定玩家的真实位置
type PositionArbitrator struct {
	epsilon float64 // 位置相似度阈值
}

// NewPositionArbitrator 创建位置仲裁器
func NewPositionArbitrator(epsilon float64) *PositionArbitrator {
	return &PositionArbitrator{
		epsilon: epsilon,
	}
}

// Arbitrate 仲裁位置
// 输入：多个客户端上报的同一玩家的位置
// 输出：仲裁后的位置
func (pa *PositionArbitrator) Arbitrate(positions []protocol.PositionData) *protocol.PositionData {
	if len(positions) == 0 {
		return nil
	}

	if len(positions) == 1 {
		return &positions[0]
	}

	// 聚类：将相似的位置分组
	clusters := pa.clusterPositions(positions)

	// 找到最大的簇（多数投票）
	maxCluster := clusters[0]
	for _, cluster := range clusters {
		if len(cluster) > len(maxCluster) {
			maxCluster = cluster
		}
	}

	// 计算簇的平均位置
	return pa.averagePosition(maxCluster)
}

// clusterPositions 将位置聚类
func (pa *PositionArbitrator) clusterPositions(positions []protocol.PositionData) [][]protocol.PositionData {
	var clusters [][]protocol.PositionData
	used := make([]bool, len(positions))

	for i, pos := range positions {
		if used[i] {
			continue
		}

		cluster := []protocol.PositionData{pos}
		used[i] = true

		for j := i + 1; j < len(positions); j++ {
			if used[j] {
				continue
			}

			if pa.isSimilar(pos, positions[j]) {
				cluster = append(cluster, positions[j])
				used[j] = true
			}
		}

		clusters = append(clusters, cluster)
	}

	return clusters
}

// isSimilar 判断两个位置是否相似
func (pa *PositionArbitrator) isSimilar(p1, p2 protocol.PositionData) bool {
	distance := math.Sqrt(math.Pow(p1.X-p2.X, 2) + math.Pow(p1.Y-p2.Y, 2))
	return distance <= pa.epsilon
}

// averagePosition 计算平均位置
func (pa *PositionArbitrator) averagePosition(positions []protocol.PositionData) *protocol.PositionData {
	if len(positions) == 0 {
		return nil
	}

	var sumX, sumY float64
	var sumTime int64
	playerID := positions[0].PlayerID

	for _, pos := range positions {
		sumX += pos.X
		sumY += pos.Y
		sumTime += pos.GameTime
	}

	count := float64(len(positions))
	return &protocol.PositionData{
		PlayerID: playerID,
		X:        sumX / count,
		Y:        sumY / count,
		GameTime: sumTime / int64(len(positions)),
	}
}
