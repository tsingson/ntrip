package ntripio

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/time/rate" // 需要引入官方限流库：go get golang.org/x/time/rate
)

const (
	Rtcm3Header = 0xD3
	MaxRtcmLen  = 1023
	MinRtcmLen  = 2
	ReadTimeout = 45 * time.Second

	// 商用安全策略：一个合法基准站每秒最多产生 ~5-10 个 RTCM 帧（1Hz/5Hz/10Hz 配置）
	// 设置 50 帧/秒 的硬限制，超出即视为攻击或故障，直接熔断
	MaxFramesPerSecond = 50
)

var (
	ErrInvalidPreamble = errors.New("ntripio: invalid RTCM3 preamble header")
	ErrRateLimitExceed = errors.New("ntripio: base station traffic limit exceeded")
)

var FrameBufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 2048)
	},
}

// RTKSource 升级：引入多订阅者隔离架构，彻底解决慢消费者问题
type RTKSource struct {
	Mountpoint string
	conn       net.Conn
	ctx        context.Context
	cancel     context.CancelFunc
	closeOnce  sync.Once
	limiter    *rate.Limiter

	// 订阅者注册表锁
	subMu       sync.RWMutex
	subscribers map[string]chan []byte
}

func NewRTKSource(parentCtx context.Context, mountpoint string, conn net.Conn) *RTKSource {
	ctx, cancel := context.WithCancel(parentCtx)
	return &RTKSource{
		Mountpoint:  mountpoint,
		conn:        conn,
		ctx:         ctx,
		cancel:      cancel,
		limiter:     rate.NewLimiter(rate.Limit(MaxFramesPerSecond), MaxFramesPerSecond),
		subscribers: make(map[string]chan []byte),
	}
}

// RegisterSubscriber 供下游 Caster 实例（或特定路由组合）注册自己的消费通道
func (s *RTKSource) RegisterSubscriber(id string, ch chan []byte) {
	s.subMu.Lock()
	s.subscribers[id] = ch
	s.subMu.Unlock()
}

// RemoveSubscriber 移除注销的消费者
func (s *RTKSource) RemoveSubscriber(id string) {
	s.subMu.Lock()
	delete(s.subscribers, id)
	s.subMu.Unlock()
}

func (s *RTKSource) StartIngest() {
	go s.readLoop()
}

func (s *RTKSource) readLoop() {
	defer s.Close()
	headerBuf := make([]byte, 3)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// 1. 安全熔断：速率限制检查
		if !s.limiter.Allow() {
			// 瞬时帧率过高，直接判定输入源异常，熔断连接以保护 CPU
			return
		}

		_ = s.conn.SetReadDeadline(time.Now().Add(ReadTimeout))

		if _, err := io.ReadFull(s.conn, headerBuf); err != nil {
			return
		}

		if headerBuf[0] != Rtcm3Header {
			return
		}

		payloadLen := int(headerBuf[1]&0x03)<<8 | int(headerBuf[2])
		if payloadLen < MinRtcmLen || payloadLen > MaxRtcmLen {
			return
		}

		totalFrameLen := 3 + payloadLen + 3

		// 从对象池借出内存
		buf := FrameBufferPool.Get().([]byte)
		copy(buf[0:3], headerBuf)

		if _, err := io.ReadFull(s.conn, buf[3:totalFrameLen]); err != nil {
			s.safePut(buf)
			return
		}

		// 2. 深度重构：广播分发模型（隔离慢消费者）
		s.subMu.RLock()
		if len(s.subscribers) == 0 {
			// 当前没有流动站在线，直接原地回收，拒绝无意义的内存滞留
			s.safePut(buf)
			s.subMu.RUnlock()
			continue
		}

		// 准备分发的数据
		sendFrame := buf[:totalFrameLen]

		for id, ch := range s.subscribers {
			// 为每个订阅者（如 Caster 转发实例）单独分配一份拷贝，彻底实现消费者间的内存与拥堵隔离
			// 注意：此处必须独立 make，因为每个消费者的发送时序不同，无法共用一个 Pool 缓冲
			clientBuf := make([]byte, totalFrameLen)
			copy(clientBuf, sendFrame)

			select {
			case ch <- clientBuf:
				// 发送成功，由各个消费者自行负责其内存的 lifecycle
			default:
				// 某个消费者网络阻塞卡满了自己的管道：直接丢弃该消费者的这一帧
				// 实现了“谁卡顿，谁丢帧”，绝对不影响注册表里的其他正常消费者！
				_ = id
			}
		}
		s.subMu.RUnlock()

		// 广播数据产生完毕后，立即释放 Ingress 侧的原始承载容器
		s.safePut(buf)
	}
}

// safePut 安全归还对象池，防止异常超大容量切片污染 Pool 导致常驻内存膨胀
func (s *RTKSource) safePut(buf []byte) {
	if buf != nil && cap(buf) <= 2048 {
		FrameBufferPool.Put(buf)
	}
}

func (s *RTKSource) Close() {
	s.closeOnce.Do(func() {
		s.cancel()
		if s.conn != nil {
			_ = s.conn.Close()
		}

		s.subMu.Lock()
		for id, ch := range s.subscribers {
			close(ch)
			delete(s.subscribers, id)
		}
		s.subMu.Unlock()
	})
}
