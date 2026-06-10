package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/tsingson/ntrip/ntripio" // 替换为您的实际路径
)

type ServerManager struct {
	mu      sync.RWMutex
	sources map[string]*ntripio.RTKSource
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewServerManager(parentCtx context.Context) *ServerManager {
	ctx, cancel := context.WithCancel(parentCtx)
	return &ServerManager{
		sources: make(map[string]*ntripio.RTKSource),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// HandleNewConnection 处理新接入的基准站连接（带抢占式连接排他锁）
func (sm *ServerManager) HandleNewConnection(mountpoint string, conn net.Conn) {
	sm.mu.Lock()

	// 商用核心逻辑：如果当前挂载点存在历史残留连接，必须实施强制踢出（抢占）
	if oldSource, exists := sm.sources[mountpoint]; exists {
		log.Printf("[Manager] 挂载点 [%s] 冲突，正在强制踢出旧连接...", mountpoint)
		oldSource.Close()
		delete(sm.sources, mountpoint)
	}

	// 构造全新独立生命周期的数据源
	source := ntripio.NewRTKSource(sm.ctx, mountpoint, conn)
	sm.sources[mountpoint] = source
	sm.mu.Unlock()

	log.Printf("[Manager] 基准站 [%s] 成功接入并上线", mountpoint)
	source.StartIngest()

	// 消费该基准站产生的纯净差分数据流

	go func() {
		for frame := range source.DataChan {
			// 1. 将 frame (RTCM 字节流) 并发广播分发给所有的流动站 (Rover Conn)
			// broadcastToRovers(frame)

			// 2. 【核心商用闭环】分发完毕后，必须由消费端将内存归还给对象池！
			ntripio.FrameBufferPool.Put(frame[:cap(frame)])
		}
	}()
}

func (sm *ServerManager) Stop() {
	sm.cancel()
	sm.mu.Lock()
	for mp, src := range sm.sources {
		src.Close()
		delete(sm.sources, mp)
	}
	sm.mu.Unlock()
	log.Println("[Manager] 整个 RTK Ingress 服务已优雅关闭")
}

func main() {
	log.Println("[Main] 初始化商用级 RTK 差分接收服务器...")

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	manager := NewServerManager(rootCtx)

	// 启动 TCP 监听
	listener, err := net.Listen("tcp", ":2101")
	if err != nil {
		log.Fatalf("无法绑定 RTK 服务端口: %v", err)
	}
	defer listener.Close()

	// 异步监听逻辑
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			// 此处应添加标准的 NTRIP 握手协议解析（如解析出 Mountpoint 与 密码鉴权）
			// 本重构重点解决高可用问题，暂用 mock 挂载点代入
			mockMountpoint := "BASE001"

			go manager.HandleNewConnection(mockMountpoint, conn)
		}
	}()

	// 优雅停机信号捕获
	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, syscall.SIGINT, syscall.SIGTERM)

	<-shutdownSig
	log.Println("[Main] 收到停机信号，开始执行安全释放...")
	_ = listener.Close()
	manager.Stop()
	time.Sleep(1 * time.Second) // 留给底层的 Goroutine 刷盘/清理残留内存
}
