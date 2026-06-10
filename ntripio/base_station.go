package ntripio

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

// Config 存储 CORS 连接所需的配置参数
type Config struct {
	Host       string
	Port       string
	MountPoint string
	Username   string
	Password   string
}

// Client 负责管理与 CORS 服务器的连接和数据交互
type Client struct {
	cfg        Config
	ggaChannel chan string
}

// NewClient 创建一个新的 NTRIP 客户端实例
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:        cfg,
		ggaChannel: make(chan string, 10),
	}
}

// SendGGA 供外部调用，用于定时向 CORS 更新当前位置（触发差分下发）
func (c *Client) SendGGA(gga string) {
	select {
	case c.ggaChannel <- gga:
	default:
		// 通道满时抛弃旧数据，确保不阻塞主业务
	}
}

// ParseRTKStatus 解析 GGA 语句并返回其定位状态
// 状态定义：0=未定位, 1=单点定位, 2=伪距差分, 4=RTK固定解(厘米级), 5=RTK浮动解
func (c *Client) ParseRTKStatus(ggaLine string) string {
	fields := strings.Split(ggaLine, ",")
	if len(fields) > 6 {
		return fields[6] // GGA 语句的第 7 个字段（索引为6）是状态位
	}
	return "0"
}

// StartBridge 启动差分数据桥接，包含自动重连与容错机制
// ctx 用于外部控制生命周期，target 设备（如串口）用来写入差分并读取 NMEA
func (c *Client) StartBridge(ctx context.Context, target io.Writer) {
	for {
		select {
		case <-ctx.Done():
			log.Println("[ntripio] 收到停止信号，退出网络线程")
			return
		default:
		}

		log.Printf("[ntripio] 正在连接 CORS 服务器 (%s:%s)...", c.cfg.Host, c.cfg.Port)

		conn, err := net.DialTimeout("tcp", net.JoinHostPort(c.cfg.Host, c.cfg.Port), 5*time.Second)
		if err != nil {
			log.Printf("[ntripio 错误] 连接失败: %v。5秒后自动重试...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if err := c.handleHandshake(conn); err != nil {
			log.Printf("[ntripio 错误] 协议握手失败: %v。10秒后重试...", err)
			conn.Close()
			time.Sleep(10 * time.Second)
			continue
		}

		log.Println("[ntripio] 认证成功，建立差分数据泵...")

		// 构建专门用于当前连接的取消机制
		connCtx, cancelConn := context.WithCancel(ctx)

		// 1. 启动上行流协程：将 GGA 位置信息上报给 CORS
		go c.upstreamPump(connCtx, conn)

		// 2. 本地执行下行流：接收 CORS 差分数据并直接注入目标设备（串口）
		c.downstreamPump(conn, target)

		// 当 downstreamPump 退出时，说明网络连接已断开
		cancelConn()
		conn.Close()
		log.Println("[ntripio] 连接断开，触发重连流程...")
		time.Sleep(2 * time.Second)
	}
}

// 处理 NTRIP 1.0 的标准 HTTP 握手认证
func (c *Client) handleHandshake(conn net.Conn) error {
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.cfg.Username, c.cfg.Password)))
	req := fmt.Sprintf("GET /%s HTTP/1.1\r\n"+
		"User-Agent: NTRIP GoClient/2.0\r\n"+
		"Authorization: Basic %s\r\n"+
		"Accept: */*\r\n\r\n", c.cfg.MountPoint, auth)

	if _, err := conn.Write([]byte(req)); err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	respLine, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	if !strings.Contains(respLine, "ICY 200 OK") && !strings.Contains(respLine, "HTTP/1.1 200 OK") {
		return fmt.Errorf("服务器拒绝访问: %s", strings.TrimSpace(respLine))
	}
	return nil
}

// 上行数据泵：向 CORS 发送 GGA
func (c *Client) upstreamPump(ctx context.Context, conn net.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		case gga := <-c.ggaChannel:
			_, err := conn.Write([]byte(gga + "\r\n"))
			if err != nil {
				return // 网络写入失败，退出协程以触发外层重连
			}
		}
	}
}

// 下行数据泵：从 CORS 接收 RTCM 差分数据并写入串口设备
func (c *Client) downstreamPump(conn net.Conn, target io.Writer) {
	buffer := make([]byte, 2048)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			break // 读取失败（连接断开），退出下行泵
		}
		if n > 0 {
			if _, err := target.Write(buffer[:n]); err != nil {
				log.Printf("[ntripio 警告] 无法将差分数据灌入串口: %v", err)
			}
		}
	}
}
