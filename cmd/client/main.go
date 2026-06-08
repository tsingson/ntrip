package main

import (
	"context"
	"fmt" // <-- 补上了这个关键依赖
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tsingson/ntrip/ntripio"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	casterAddr := "127.0.0.1:2101"
	mountpoint := "RTK_TEST_MP"

	client := ntripio.NewRTKClient(casterAddr, mountpoint)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("[Main] 移动站收到停止信号...")
		cancel()
	}()

	// 模拟终端内部的 GPS 板卡当前的 NMEA GGA 定位语句 (VRS 模式必选)
	mockGGAPrvider := func() string {
		now := time.Now().UTC().Format("150405.00")
		return fmt.Sprintf("$GPGGA,%s,3954.1234,N,11623.5678,E,1,08,0.9,45.2,M,-8.1,M,,*47", now)
	}

	// 接收到差分数据后的回调函数
	rtcmProcessor := func(data []byte) {
		log.Printf("[RTK 解算器] 收到差分修正数据 (%d bytes): %s\n", len(data), string(data))
	}

	log.Printf("[Main] 启动移动站拉流，请求挂载点: %s\n", mountpoint)
	client.ConnectAndStream(ctx, mockGGAPrvider, rtcmProcessor)

	<-ctx.Done()
	log.Println("[Main] 移动站已退出。")
}
