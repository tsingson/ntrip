package ntripio

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

type RTKClient struct {
	casterAddr string
	mountpoint string
	logger     *log.Logger
}

func NewRTKClient(casterAddr, mountpoint string) *RTKClient {
	return &RTKClient{
		casterAddr: casterAddr,
		mountpoint: mountpoint,
		logger:     log.Default(),
	}
}

// ConnectAndStream 维持连接，将拉取到的 RTCM 数据通过回调函数喂给上层定位解算引擎
func (c *RTKClient) ConnectAndStream(ctx context.Context, ggaProvider func() string, rnxCallback func([]byte)) {
	go func() {
		backoff := 1 * time.Second
		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn, err := net.DialTimeout("tcp", c.casterAddr, 5*time.Second)
				if err != nil {
					time.Sleep(backoff)
					if backoff < 30*time.Second {
						backoff *= 2
					}
					continue
				}
				backoff = 1 * time.Second

				// 请求挂载点
				req := fmt.Sprintf("GET /%s HTTP/1.0\r\nUser-Agent: NTRIP CustomClient/1.0\r\nAccept: */*\r\n\r\n", c.mountpoint)
				_, _ = conn.Write([]byte(req))

				// 启动定时向上游发送 GGA 坐标（支持 VRS 服务）
				vrsCtx, vrsCancel := context.WithCancel(ctx)
				go c.sendVRSGgaLoop(vrsCtx, conn, ggaProvider)

				err = c.readStream(conn, rnxCallback)
				vrsCancel()
				conn.Close()

				c.logger.Printf("[Client] 连接断开 (%v)，准备自动重连...\n", err)
				time.Sleep(1 * time.Second)
			}
		}
	}()
}

func (c *RTKClient) sendVRSGgaLoop(ctx context.Context, conn net.Conn, ggaProvider func() string) {
	if ggaProvider == nil {
		return
	}
	ticker := time.NewTicker(10 * time.Second) // 默认每 10 秒上报一次自身坐标
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			gga := ggaProvider()
			if gga != "" {
				_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
				_, err := conn.Write([]byte(gga + "\r\n"))
				if err != nil {
					return
				}
			}
		}
	}
}

func (c *RTKClient) readStream(conn net.Conn, callback func([]byte)) error {
	reader := bufio.NewReader(conn)
	// 验证 Caster 响应头
	respLine, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	if !strings.Contains(respLine, "200") && !strings.Contains(respLine, "ICY") {
		return fmt.Errorf("caster 拒绝服务: %s", strings.TrimSpace(respLine))
	}

	// 吞掉剩下的头部字段
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	// 循环回传差分数据流
	buf := make([]byte, 1024)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second)) // 15秒无数据视作掉线
		n, err := reader.Read(buf)
		if err != nil {
			return err
		}
		if n > 0 {
			callback(buf[:n])
		}
	}
}
