package main

import (
	"fmt"
	"log"
	"math"
	"syncServerDemo/client"
	"syncServerDemo/server"
	"syncServerDemo/transport"
	"time"
)

func main() {
	fmt.Println("=== 多人游戏同步框架演示 ===")
	fmt.Println("架构说明:")
	fmt.Println("1. 使用游戏时间同步机制（非帧同步）")
	fmt.Println("2. 客户端驱动：移动计算由客户端执行")
	fmt.Println("3. 服务器只负责转发指令和仲裁位置")
	fmt.Println("4. 支持多数投票的位置仲裁")
	fmt.Println("5. 网络传输层可替换（当前使用本地内存实现）")
	fmt.Println()

	// 创建本地传输层
	localTransport := transport.NewLocalTransport()

	// 创建游戏服务器
	gameServer := server.NewGameServer(localTransport)
	err := gameServer.Start()
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer gameServer.Stop()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建多个客户端（模拟多个玩家）
	clients := make([]*client.GameClient, 0)
	playerIDs := []string{"Alice", "Bob", "Charlie"}

	for i, playerID := range playerIDs {
		clientID := fmt.Sprintf("client_%d", i)

		// 注册客户端到传输层
		localTransport.Register(clientID)

		// 创建客户端
		gameClient := client.NewGameClient(clientID, playerID, localTransport)
		gameClient.Start()
		clients = append(clients, gameClient)

		time.Sleep(100 * time.Millisecond)
	}

	// 等待所有客户端加入
	time.Sleep(500 * time.Millisecond)

	fmt.Println("\n=== 开始游戏演示 ===")

	// Alice向右移动
	fmt.Println("\n[动作] Alice向右移动 (1, 0)")
	clients[0].Move(1, 0)
	time.Sleep(1 * time.Second)

	// Bob向上移动
	fmt.Println("\n[动作] Bob向上移动 (0, 1)")
	clients[1].Move(0, 1)
	time.Sleep(1 * time.Second)

	// Charlie斜向移动
	fmt.Println("\n[动作] Charlie斜向移动 (0.707, 0.707)")
	clients[2].Move(0.707, 0.707)
	time.Sleep(1 * time.Second)

	// Alice停止
	fmt.Println("\n[动作] Alice停止移动")
	clients[0].Move(0, 0)
	time.Sleep(1 * time.Second)

	// 打印所有客户端看到的世界状态
	fmt.Println("\n=== 各客户端的世界视图 ===")
	for i, c := range clients {
		fmt.Printf("\nClient %d 视角:\n", i)
		for _, pid := range playerIDs {
			x, y, ok := c.GetPlayerPosition(pid)
			if ok {
				fmt.Printf("  %s: (%.2f, %.2f)\n", pid, x, y)
			}
		}
	}

	// 验证一致性
	fmt.Println("\n=== 验证一致性 ===")
	checkConsistency(clients, playerIDs)

	// 再运行一段时间
	time.Sleep(2 * time.Second)

	// 最终状态
	fmt.Println("\n=== 最终状态 ===")
	for i, c := range clients {
		fmt.Printf("\nClient %d:\n", i)
		for _, pid := range playerIDs {
			x, y, ok := c.GetPlayerPosition(pid)
			if ok {
				fmt.Printf("  %s: (%.2f, %.2f)\n", pid, x, y)
			}
		}
	}

	// 停止所有客户端
	for _, c := range clients {
		c.Stop()
	}

	fmt.Println("\n=== 演示结束 ===")
	fmt.Println("\n关键技术点总结:")
	fmt.Println("✓ 游戏时间同步 - 所有客户端使用统一的游戏时间基准")
	fmt.Println("✓ 客户端预测 - 每个客户端独立计算玩家位置")
	fmt.Println("✓ 服务器仲裁 - 定期收集上报，通过多数投票确定真实位置")
	fmt.Println("✓ 位置校正 - 客户端根据仲裁结果修正本地状态")
	fmt.Println("✓ 可替换传输层 - 当前用本地内存，可轻松替换为TCP/UDP/WebSocket")
}

// checkConsistency 检查各客户端视图的一致性
func checkConsistency(clients []*client.GameClient, playerIDs []string) {
	for _, pid := range playerIDs {
		positions := make([][2]float64, 0)
		for _, c := range clients {
			x, y, ok := c.GetPlayerPosition(pid)
			if ok {
				positions = append(positions, [2]float64{x, y})
			}
		}

		if len(positions) < 2 {
			continue
		}

		// 计算最大偏差
		maxDeviation := 0.0
		for i := 0; i < len(positions); i++ {
			for j := i + 1; j < len(positions); j++ {
				dx := positions[i][0] - positions[j][0]
				dy := positions[i][1] - positions[j][1]
				deviation := math.Sqrt(dx*dx + dy*dy)
				if deviation > maxDeviation {
					maxDeviation = deviation
				}
			}
		}

		if maxDeviation < 1.0 {
			fmt.Printf("✓ %s: 一致性良好 (最大偏差: %.4f)\n", pid, maxDeviation)
		} else {
			fmt.Printf("⚠ %s: 存在较大偏差 (最大偏差: %.4f)\n", pid, maxDeviation)
		}
	}
}
