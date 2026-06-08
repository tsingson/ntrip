package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tsingson/ntrip/ntripio"
)

// DummyRTCMStream 模拟一个物理实体硬件（如板卡串口流）
type DummyRTCMStream struct {
	counter int
}

func (d *DummyRTCMStream) Read(p []byte) (n int, err error) {
	// 模拟硬件每秒吐出一次差分定位数据 (1Hz)
	time.Sleep(1 * time.Second)
	d.counter++

	mockData := []byte(fmt.Sprintf("[RTCM3.x Mock Frame #%d Hex:D3 00 13 3E...]", d.counter))
	n = copy(p, mockData)
	return n, nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	casterAddr := "127.0.0.1:2101"
	mountpoint := "RTK_TEST_MP"
	password := "BaseStationPass123"

	server := ntripio.NewRTKServer(casterAddr, mountpoint, password)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("[Main] 基准站收到停止信号...")
		cancel()
	}()

	hardwareStream := &DummyRTCMStream{}

	log.Printf("[Main] 启动基准站推流线程，目标挂载点: %s\n", mountpoint)
	server.PushStream(ctx, hardwareStream)

	<-ctx.Done()
	log.Println("[Main] 基准站已安全离线。")
}
