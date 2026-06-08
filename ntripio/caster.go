package ntripio

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type ConnType int

const (
	TypeClient ConnType = iota
	TypeServer
)

// RTKConn 增强型 TCP 连接封装
type RTKConn struct {
	net.Conn
	ID         string
	Type       ConnType
	Mountpoint string
	DataChan   chan []byte
	Ctx        context.Context
	Cancel     context.CancelFunc
}

type MountpointHub struct {
	mu      sync.RWMutex
	Server  *RTKConn
	Clients map[string]*RTKConn
}

type RTKCaster struct {
	addr        string
	mu          sync.RWMutex
	mountpoints map[string]*MountpointHub
	logger      *log.Logger
}

func NewRTKCaster(addr string) *RTKCaster {
	return &RTKCaster{
		addr:        addr,
		mountpoints: make(map[string]*MountpointHub),
		logger:      log.Default(),
	}
}

func (c *RTKCaster) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", c.addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	c.logger.Printf("[Caster] 监听服务已启动: %s\n", c.addr)

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		rawConn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				c.logger.Printf("[Caster] 接受连接失败: %v\n", err)
				continue
			}
		}
		go c.handleRawConnection(rawConn)
	}
}

func (c *RTKCaster) handleRawConnection(rawConn net.Conn) {
	_ = rawConn.SetReadDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(rawConn)
	firstLine, err := reader.ReadString('\n')
	if err != nil {
		rawConn.Close()
		return
	}

	parts := strings.Split(strings.TrimSpace(firstLine), " ")
	if len(parts) < 2 {
		_, _ = rawConn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		rawConn.Close()
		return
	}

	verb, path := parts[0], parts[1]
	mountpoint := strings.TrimPrefix(path, "/")
	_ = rawConn.SetReadDeadline(time.Time{}) // 重置超时

	ctx, cancel := context.WithCancel(context.Background())
	conn := &RTKConn{
		Conn:       rawConn,
		Type:       TypeClient,
		Mountpoint: mountpoint,
		DataChan:   make(chan []byte, 512), // 异步发包缓冲区
		Ctx:        ctx,
		Cancel:     cancel,
	}

	if verb == "GET" {
		conn.Type = TypeClient
		// 响应 ICY 200 格式，100%兼容老旧测绘 GNSS 终端
		_, err = rawConn.Write([]byte("ICY 200 OK\r\nNtrip-Version: 2.0\r\nServer: CustomRTK/1.0\r\n\r\n"))
		if err != nil {
			conn.Close()
			return
		}
		c.registerClient(conn)
		// 启动异步写循环，防止因当前客户端网络卡顿导致服务器主线程死锁
		go c.startWriteLoop(conn)
	} else if verb == "POST" || verb == "SOURCE" {
		conn.Type = TypeServer
		_, err = rawConn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
		if err != nil {
			conn.Close()
			return
		}
		c.registerServer(conn)
		go c.startReadServerLoop(conn, reader)
	} else {
		_, _ = rawConn.Write([]byte("HTTP/1.1 405 Method Not Allowed\r\n\r\n"))
		rawConn.Close()
	}
}

func (c *RTKCaster) registerClient(conn *RTKConn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.mountpoints[conn.Mountpoint]; !exists {
		c.mountpoints[conn.Mountpoint] = &MountpointHub{Clients: make(map[string]*RTKConn)}
	}
	hub := c.mountpoints[conn.Mountpoint]
	hub.mu.Lock()
	conn.ID = fmt.Sprintf("cli-%d", time.Now().UnixNano())
	hub.Clients[conn.ID] = conn
	hub.mu.Unlock()
	c.logger.Printf("[Caster] 移动站接入挂载点: %s, ID: %s\n", conn.Mountpoint, conn.ID)
}

func (c *RTKCaster) registerServer(conn *RTKConn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.mountpoints[conn.Mountpoint]; !exists {
		c.mountpoints[conn.Mountpoint] = &MountpointHub{Clients: make(map[string]*RTKConn)}
	}
	hub := c.mountpoints[conn.Mountpoint]
	hub.mu.Lock()
	if hub.Server != nil {
		hub.Server.Close() // 顶掉旧的基准站连接
	}
	hub.Server = conn
	hub.mu.Unlock()
	c.logger.Printf("[Caster] 基准站成功注册挂载点: %s\n", conn.Mountpoint)
}

func (c *RTKCaster) startWriteLoop(conn *RTKConn) {
	defer c.removeClient(conn)
	for {
		select {
		case <-conn.Ctx.Done():
			return
		case data, ok := <-conn.DataChan:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second)) // 3秒写超时，超时直接剔除
			if _, err := conn.Write(data); err != nil {
				return
			}
		}
	}
}

func (c *RTKCaster) startReadServerLoop(conn *RTKConn, reader *bufio.Reader) {
	defer c.removeServer(conn)
	buf := make([]byte, 2048)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second)) // 基准站60秒心跳探活
		n, err := reader.Read(buf)
		if err != nil {
			return
		}
		if n > 0 {
			c.broadcast(conn.Mountpoint, buf[:n])
		}
	}
}

func (c *RTKCaster) broadcast(mp string, data []byte) {
	c.mu.RLock()
	hub, exists := c.mountpoints[mp]
	c.mu.RUnlock()
	if !exists {
		return
	}

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	// 深拷贝数据，防止并发逻辑下底层切片数据被污染
	payload := make([]byte, len(data))
	copy(payload, data)

	for _, client := range hub.Clients {
		select {
		case client.DataChan <- payload:
		default:
			// 缓冲区满了说明终端网络极差，选择性丢弃该包防止拖慢整体转发速度
		}
	}
}

func (c *RTKCaster) removeClient(conn *RTKConn) {
	c.mu.RLock()
	hub, exists := c.mountpoints[conn.Mountpoint]
	c.mu.RUnlock()
	if exists {
		hub.mu.Lock()
		delete(hub.Clients, conn.ID)
		hub.mu.Unlock()
	}
	conn.Close()
	c.logger.Printf("[Caster] 移动站断开: %s\n", conn.ID)
}

func (c *RTKCaster) removeServer(conn *RTKConn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if hub, exists := c.mountpoints[conn.Mountpoint]; exists {
		hub.mu.Lock()
		if hub.Server == conn {
			hub.Server = nil
		}
		hub.mu.Unlock()
	}
	conn.Close()
	c.logger.Printf("[Caster] 基准站断开挂载点: %s\n", conn.Mountpoint)
}
