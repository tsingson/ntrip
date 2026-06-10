package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/tsingson/ntrip/ntripio"

	"github.com/tarm/serial"
)

// ==================== 静态内联配置 ====================
const (
	// 串口硬件配置
	DevicePort = "COM3" // Linux 或树莓派环境请修改为例如 "/dev/ttyUSB0"
	BaudRate   = 115200

	// CORS 账号配置
	CorsHost       = "rtk.qxwz.com" // 千寻位置千寻知寸服务示例
	CorsPort       = "8001"
	CorsMountPoint = "RTCM32_GGB"
	CorsUser       = "your_cors_username" // 填入你的测试账号
	CorsPassword   = "your_cors_password" // 填入你的测试密码

	// 本地文件存储路径
	LogFileName = "precise_gps_log.txt"
)

func main() {
	log.Println("=========================================")
	log.Println("   高精度 RTK 定位数据采集系统 (Go CLI)   ")
	log.Println("=========================================")

	// 1. 开启本地持久化日志文件
	logFile, err := os.OpenFile(LogFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		log.Fatalf("[FATAL] 无法初始化本地存储文件: %v", err)
	}
	defer logFile.Close()

	// 2. 初始化硬件串口
	serialConfig := &serial.Config{
		Name:        DevicePort,
		Baud:        BaudRate,
		ReadTimeout: time.Second * 3,
	}
	serialDev, err := serial.OpenPort(serialConfig)
	if err != nil {
		log.Fatalf("[FATAL] 无法打开串口设备 %s: %v", DevicePort, err)
	}
	defer serialDev.Close()
	log.Printf("[系统] 串口硬件初始化成功: %s (%d Baud)\n", DevicePort, BaudRate)

	// 3. 实例化并配置 ntripio 核心库
	ntripClient := ntripio.NewClient(ntripio.Config{
		Host:       CorsHost,
		Port:       CorsPort,
		MountPoint: CorsMountPoint,
		Username:   CorsUser,
		Password:   CorsPassword,
	})

	// 4. 使用 Context 控制并发协程生命周期
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 异步启动 CORS 数据桥接网络线程
	go ntripClient.StartBridge(ctx, serialDev)

	// 5. 主业务循环：轮询解析串口输出
	reader := bufio.NewReader(serialDev)
	lastSentGgaTime := time.Now()

	log.Println("[系统] 进入设备 NMEA 监听主循环...")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("[串口警告] 读取瞬时中断（尝试恢复中）: %v", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		line = strings.TrimSpace(line)

		// 过滤 GGA 语句（全球定位系统定位数据）
		if strings.Contains(line, "$GNGGA") || strings.Contains(line, "$GPGGA") {

			// 策略优化：每隔 5 秒将设备当前位置投递给 CORS 库，维持差分数据分发
			if time.Since(lastSentGgaTime) > 5*time.Second {
				ntripClient.SendGGA(line)
				lastSentGgaTime = time.Now()
			}

			// 通过库函数解析 RTK 状态值
			status := ntripClient.ParseRTKStatus(line)
			switch status {
			case "4": // RTK Fixed (厘米级固定解)
				log.Println("[🎉 RTK FIXED] 捕获到厘米级高精度坐标！")

				// 格式化输出带时间戳的数据
				outputRecord := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), line)
				if _, err := logFile.WriteString(outputRecord); err != nil {
					log.Printf("[存储错误] 写入失败: %v", err)
				} else {
					log.Printf("[保存] %s", line)
				}

			case "5": // RTK Float (分米级浮动解)
				log.Println("[提示] 当前处于 RTK 浮动解状态，卫星信号收敛中...")
			default:
				log.Printf("[就绪] 当前定位状态代码: %s (等待 RTK 握手...)", status)
			}
		}
	}
}
