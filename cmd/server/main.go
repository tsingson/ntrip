package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/tsingson/ntrip/ntripio" // 请根据您的实际项目 go.mod 路径修改此导入
)

// ServerManager 统一管理所有在线基准站（Sources）的生命周期
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

// HandleNewBaseStation 处理基准站（NTRIP Server）的接入请求
func (sm *ServerManager) HandleNewBaseStation(mountpoint string, conn net.Conn) {
	sm.mu.Lock()

	// 商用核心安全策略：如果该挂载点已有残余连接，实施强制踢出（抢占机制）
	if oldSource, exists := sm.sources[mountpoint]; exists {
		log.Printf("[Manager] 挂载点冲突！基准站 [%s] 闪断重连，正在强制销毁旧链路...", mountpoint)
		oldSource.Close()
		delete(sm.sources, mountpoint)
	}

	// 初始化支持高并发、零内存复制、具备限流保护的全新数据源
	source := ntripio.NewRTKSource(sm.ctx, mountpoint, conn)
	sm.sources[mountpoint] = source
	sm.mu.Unlock()

	log.Printf("[Manager] 基准站 [%s] 鉴权通过，正式上线，开始摄入差分流...", mountpoint)

	// 启动数据摄入状态机循环
	source.StartIngest()

	// 监控基准站链路状态，一旦其内部报错或主动断开，在此执行注销
	go func() {
		// 阻塞等待 context 结束（由 source.Close() 触发）
		<-sm.ctx.Done()

		sm.mu.Lock()
		// 双重检查，确保没有误杀由于抢占机制刚注册进来的新连接
		if current, exists := sm.sources[mountpoint]; exists && current == source {
			delete(sm.sources, mountpoint)
			log.Printf("[Manager] 基准站 [%s] 链路已释放，安全下线", mountpoint)
		}
		sm.mu.Unlock()
	}()
}

// SubscribeRover 为新接入的流动站（Rover Client）订阅指定挂载点的 RTCM 差分流
// clientID 通常为流动站的 账号名/设备序列号，ch 为该流动站独占的发送队列
func (sm *ServerManager) SubscribeRover(mountpoint, clientID string, ch chan []byte) error {
	sm.mu.RLock()
	source, exists := sm.sources[mountpoint]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("mountpoint [%s] offline", mountpoint)
	}

	// 将流动站的独占信道注册进该基准站的广播名录中
	source.RegisterSubscriber(clientID, ch)
	log.Printf("[Caster] 流动站 [%s] 成功挂载到基准站 [%s]", clientID, mountpoint)
	return nil
}

// UnsubscribeRover 流动站断开连接时，主动注销订阅
func (sm *ServerManager) UnsubscribeRover(mountpoint, clientID string) {
	sm.mu.RLock()
	source, exists := sm.sources[mountpoint]
	sm.mu.RUnlock()

	if exists {
		source.RemoveSubscriber(clientID)
		log.Printf("[Caster] 流动站 [%s] 已取消挂载 [%s]", clientID, mountpoint)
	}
}

// Stop 优雅关闭整个中心
func (sm *ServerManager) Stop() {
	sm.cancel()
	sm.mu.Lock()
	for mp, src := range sm.sources {
		src.Close()
		delete(sm.sources, mp)
	}
	sm.mu.Unlock()
	log.Println("[Manager] 全局服务器及所有基准站链路已完成安全释放")
}

func main() {
	log.Println("[System] 正在初始化商用级高并发 RTK Caster/Server 核心组件...")

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	manager := NewServerManager(rootCtx)

	// 1. 启动基准站数据接入（Ingress）监听端口（例如 2101）
	serverListener, err := net.Listen("tcp", ":2101")
	if err != nil {
		log.Fatalf("无法绑定基准站接入端口 2101: %v", err)
	}
	defer serverListener.Close()

	go func() {
		for {
			conn, err := serverListener.Accept()
			if err != nil {
				return
			}

			go func(c net.Conn) {
				// 【商用红线补充说明】
				// 此处应添加标准的 NTRIP 握手鉴权包解析逻辑：
				// 1. 读取第一行判断是否为 "SOURCE password /MOUNTPOINT" (NTRIP 1.0)
				// 2. 校验密码是否合法
				// 3. 提取出真正的挂载点名称 (例如 "BASE001")
				// 以下使用 mock 数据进行流转：
				mountpoint := "BASE001"

				manager.HandleNewBaseStation(mountpoint, c)
			}(conn)
		}
	}()

	// 2. 模拟启动流动站（Rover Client）转发监听端口（例如 8001）
	clientListener, err := net.Listen("tcp", ":8001")
	if err != nil {
		log.Fatalf("无法绑定流动站分发端口 8001: %v", err)
	}
	defer clientListener.Close()

	go func() {
		for {
			conn, err := clientListener.Accept()
			if err != nil {
				return
			}

			go func(c net.Conn) {
				defer c.Close()

				// 【商用红线补充说明】
				// 此处应添加流动站 NTRIP Client 握手逻辑：
				// 解析 HTTP 请求头 "GET /BASE001 HTTP/1.1", 并验证 Authorization 账号密码
				// 验证通过后向流动站回应 "ICY 200 OK\r\n\r\n"
				mountpoint := "BASE001"
				roverID := fmt.Sprintf("ROVER_%d", time.Now().UnixNano())

				// 为该流动站创建独立的隔离发送缓冲区
				roverChan := make(chan []byte, 128)

				// 执行挂载订阅
				if err := manager.SubscribeRover(mountpoint, roverID, roverChan); err != nil {
					return
				}
				defer manager.UnsubscribeRover(mountpoint, roverID)

				// 启动流动站的写循环
				for {
					select {
					case <-rootCtx.Done():
						return
					case frame, ok := <-roverChan:
						if !ok {
							return
						}

						// 动态设置流动站写入超时，防止慢客户端（进入隧道的车）挂起该协程
						_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))

						// 将纯净的 RTCM 物理帧投递给无线终端
						_, err := c.Write(frame)
						if err != nil {
							return // 写入失败（终端断线），退出循环触发 defer 注销订阅
						}
					}
				}
			}(conn)
		}
	}()

	log.Println("[System] 服务器初始化完毕。基准站端口: 2101, 流动站端口: 8001")

	// 3. 监听系统信号实现优雅停机
	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, syscall.SIGINT, syscall.SIGTERM)

	<-shutdownSig
	log.Println("[System] 接收到停机指令，开始执行平滑下线...")

	_ = serverListener.Close()
	_ = clientListener.Close()
	manager.Stop()

	time.Sleep(500 * time.Millisecond) // 留出充足时间让底层 Go 协程完成 Socket 资源挥手
	log.Println("[System] 服务器安全退出。")
}
