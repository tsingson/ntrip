package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tsingson/ntrip/ntripio"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 监听 2101 标准测绘端口
	bindAddr := ":2101"
	caster := ntripio.NewRTKCaster(bindAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("[Main] 接收到停机信号，正在关闭 Caster...")
		cancel()
	}()

	log.Printf("[Main] 正在启动 RTK Caster 核心服务...\n")
	if err := caster.Start(ctx); err != nil {
		log.Fatalf("[Main] Caster 异常退出: %v\n", err)
	}

	time.Sleep(500 * time.Millisecond)
	log.Println("[Main] Caster 已完全停止。")
}
