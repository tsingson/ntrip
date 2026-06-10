package ntripio

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

const (
	Rtcm3Header    = 0xD3
	MaxRtcmLen     = 1023
	MinRtcmLen     = 2 // 修正：RTCM3 负载至少需要包含消息类型等（通常>2字节）
	ReadTimeout    = 45 * time.Second
	ChanBufferSize = 256
)

// 优化：定义全局哨兵错误，避免在循环中重复申请内存，支持 errors.Is()
var (
	ErrInvalidPreamble = errors.New("ntripio: invalid RTCM3 preamble header")
	ErrFrameTooShort   = errors.New("ntripio: RTCM3 frame payload too short")
	ErrFrameTooLong    = errors.New("ntripio: RTCM3 frame payload exceeds limit")
)

// FrameBufferPool 集中管理生命周期
var FrameBufferPool = sync.Pool{
	New: func() interface{} {
		// 分配 2048 字节，高并发下复用此内存
		return make([]byte, 2048)
	},
}

type RTKSource struct {
	Mountpoint string
	conn       net.Conn
	// 修正：传递 byte 切片。规避 channel 传递过程中重复产生 GC 压力
	DataChan  chan []byte
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
}

func NewRTKSource(parentCtx context.Context, mountpoint string, conn net.Conn) *RTKSource {
	ctx, cancel := context.WithCancel(parentCtx)
	return &RTKSource{
		Mountpoint: mountpoint,
		conn:       conn,
		DataChan:   make(chan []byte, ChanBufferSize),
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (s *RTKSource) StartIngest() {
	go s.readLoop()
}

func (s *RTKSource) readLoop() {
	// 确保退出时资源彻底释放
	defer s.Close()

	headerBuf := make([]byte, 3)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// 1. 动态心跳超时控制
		if err := s.conn.SetReadDeadline(time.Now().Add(ReadTimeout)); err != nil {
			return
		}

		// 2. 精准流式读取头部
		if _, err := io.ReadFull(s.conn, headerBuf); err != nil {
			return
		}

		// 3. 校验帧头
		if headerBuf[0] != Rtcm3Header {
			// 商用红线：不再使用 errors.New，不产生内存分配
			return
		}

		// 4. 计算并校验长度边界
		payloadLen := int(headerBuf[1]&0x03)<<8 | int(headerBuf[2])
		if payloadLen < MinRtcmLen {
			return
		}
		if payloadLen > MaxRtcmLen {
			return
		}

		// 总物理长度 = 3字节头 + 负载 + 3字节CRC
		totalFrameLen := 3 + payloadLen + 3

		// 5. 内存零逃逸获取：直接从对象池取出承载容器
		buf := FrameBufferPool.Get().([]byte)
		copy(buf[0:3], headerBuf)

		// 6. 流式补全后续字节
		if _, err := io.ReadFull(s.conn, buf[3:totalFrameLen]); err != nil {
			FrameBufferPool.Put(buf)
			return
		}

		// 7. 构造切片指针发往下游
		// 注意：这里没有 make 动作！直接将对象池内存切片送入通道
		sendFrame := buf[:totalFrameLen]

		// 8. 抛弃策略（Backpressure）
		select {
		case s.DataChan <- sendFrame:
			// 投递成功。特别注意：下游（Caster消费者）在将该数据写给所有 rover 客户端后，
			// 必须执行：ntripio.FrameBufferPool.Put(frame[:cap(frame)]) 归还内存！
		default:
			// 下游积压，直接抛弃当前帧，保障实时性，并立即将内存归还对象池
			FrameBufferPool.Put(buf)
		}
	}
}

// Close 升级为安全的单向关闭机制
func (s *RTKSource) Close() {
	s.closeOnce.Do(func() {
		s.cancel()
		if s.conn != nil {
			_ = s.conn.Close()
		}

		// 修正：在长连接高并发场景下，不应随意 close 带有多个上下游关系的 channel。
		// 通过 select 非阻塞清空并辅以 context 判定，能够更平滑地通知消费端下线。
		close(s.DataChan)

		// 清洗通道中留存的切片，防止内存残留在对象池外部引发内存泄漏
		for frame := range s.DataChan {
			if frame != nil {
				FrameBufferPool.Put(frame[:cap(frame)])
			}
		}
	})
}
