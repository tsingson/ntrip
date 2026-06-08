package ntripio

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

type RTKServer struct {
	casterAddr string
	mountpoint string
	password   string
	logger     *log.Logger
}

func NewRTKServer(casterAddr, mountpoint, password string) *RTKServer {
	return &RTKServer{
		casterAddr: casterAddr,
		mountpoint: mountpoint,
		password:   password,
		logger:     log.Default(),
	}
}

// PushStream 维持一个长连接，稳定从数据源（如输入流或串口）推流至 Caster
func (s *RTKServer) PushStream(ctx context.Context, rStream io.Reader) {
	dataChan := make(chan []byte, 128)

	// 数据采集协程
	go func() {
		buf := make([]byte, 1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := rStream.Read(buf)
				if err != nil {
					s.logger.Printf("[Server] 数据源读取异常: %v\n", err)
					time.Sleep(1 * time.Second)
					continue
				}
				if n > 0 {
					chunk := make([]byte, n)
					copy(chunk, buf[:n])
					dataChan <- chunk
				}
			}
		}
	}()

	// 网络传输与自动重连协程
	go func() {
		backoff := 1 * time.Second
		for {
			select {
			case <-ctx.Done():
				return
			default:
				s.logger.Printf("[Server] 正在连接 Caster: %s...\n", s.casterAddr)
				conn, err := net.DialTimeout("tcp", s.casterAddr, 5*time.Second)
				if err != nil {
					s.logger.Printf("[Server] 连接失败，%v 后重试...", backoff)
					time.Sleep(backoff)
					if backoff < 30*time.Second {
						backoff *= 2
					}
					continue
				}
				backoff = 1 * time.Second // 成功连接后重置退避时间

				// 发送 NTRIP Server 握手头
				handshake := fmt.Sprintf("POST /%s HTTP/1.1\r\nUser-Agent: NTRIP CustomServer/1.0\r\nAuthorization: Basic %s\r\n\r\n", s.mountpoint, s.password)
				_, _ = conn.Write([]byte(handshake))

				// 传输数据
				err = s.transmit(conn, dataChan, ctx)
				if err != nil {
					s.logger.Printf("[Server] 传输中断: %v, 即将重新触发握手\n", err)
				}
				conn.Close()
			}
		}
	}()
}

func (s *RTKServer) transmit(conn net.Conn, dataChan chan []byte, ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case data := <-dataChan:
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, err := conn.Write(data)
			if err != nil {
				return err
			}
		}
	}
}
