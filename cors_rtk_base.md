# rtk base location


如何获得CORS临时测试账号并用go语言开发，从串口读取当前固定位置的GPS模块数据，同时连接CORS获得当前GPS定位的准确坐标，注意go代码配置写在代码中，增加重试容错机制，最后获得当前精确坐标后写入文本文件中保存


-----


要实现高精度（厘米级）的GPS定位，通常需要使用 **RTK（载波相位差分）** 技术。高精度GPS模块通过串口输出原始的观测数据（通常是 NMEA 格式），同时通过网络连接 **CORS（连续运行基准站）** 系统，利用 NTRIP 协议接收差分修正数据，从而解算出精确坐标。

以下是完整的开发指南和 Go 语言实现方案。

---

## 一、 如何获取 CORS 临时测试账号

在中国国内，主要的 CORS 服务提供商有**千寻位置**、**六分科技**、华测导航（CORS账号）等。它们普遍提供面向开发者的免费试用机会：

1. **注册开发者账号**：访问 [千寻位置官网](https://www.qxwz.com/) 或六分科技官网，注册并完成**实名认证**。
2. **申请试用产品**：在控制台寻找 **“千寻知寸（FindCM）”** 或类似的 RTK 高精度定位服务。
3. **获取 NTRIP 凭证**：申请成功后，系统会为你生成测试用的配置信息。你必须获取以下 5 个核心参数：
* **IP 地址（Host）**（如：`rtk.qxwz.com`）
* **端口（Port）**（通常为 `8001` 或 `2101`）
* **挂载点（Mountpoint）**（常用的有 `RTCM32_GGB` 或 `AUTO`）
* **用户名（Username）**
* **密码（Password）**



---

## 二、 Go 语言开发实现

由于标准的 Go 代码无法直接处理底层的 RTK 算法解算（这通常由 GPS 芯片硬件或特定的 RTK 算法库完成），在实际开发中，**最标准且稳定的架构**是：

> **工作流：** > 1. Go 程序从串口读取 GPS 模块的定位请求（或提供初始 NMEA 位置）。
> 2. Go 程序作为 **NTRIP Client** 连接 CORS 服务器，获取差分数据。
> 3. Go 程序**将差分数据写入串口**送给 GPS 模块，模块内部的 RTK 芯片会自主完成高精度解算。
> 4. 模块解算出高精度坐标后，会通过串口输出 `$GNGGA`（或 `$GPGGA`）语句，Go 程序捕获状态为 `4`（RTK固定解）的数据并写入文件。

### 1. 准备依赖库

在项目目录下执行以下命令，安装串口库：

```bash
go get github.com/tarm/serial

```

### 2. 完整的 Go 源码

```go
package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/tarm/serial"
)

// ==================== 硬编码配置信息 ====================
const (
	// 串口配置
	SerialPort     = "COM3" // Linux下如 "/dev/ttyUSB0"
	SerialBaudRate = 115200

	// CORS (NTRIP) 服务器配置
	CorsHost       = "rtk.qxwz.com" // 示例：千寻位置
	CorsPort       = "8001"
	CorsMountPoint = "RTCM32_GGB"
	CorsUser       = "your_cors_username" // 替换为你的真实账号
	CorsPassword   = "your_cors_password" // 替换为你的真实密码

	// 输出文件
	OutputFile = "precise_gps_log.txt"
)

func main() {
	log.Println("=== 高精度 GPS RTK 数据采集程序启动 ===")

	// 1. 初始化串口
	c := &serial.Config{Name: SerialPort, Baud: SerialBaudRate, ReadTimeout: time.Second * 3}
	serialStream, err := serial.OpenPort(c)
	if err != nil {
		log.Fatalf("无法打开串口 %s: %v", SerialPort, err)
	}
	defer serialStream.Close()
	log.Printf("成功打开串口: %s\n", SerialPort)

	// 2. 创建一个通道用于在协程间传递 GPS 模块返回的 NMEA GGA 语句（CORS需要此语句来下发对应区域的差分数据）
	ggaChannel := make(chan string, 10)

	// 3. 启动异步协程：连接 CORS 服务器，接收差分数据并写入串口
	go connectCorsAndInject(serialStream, ggaChannel)

	// 4. 主循环：读取串口数据，解析高精度坐标，并定时向通道发送 GGA
	reader := bufio.NewReader(serialStream)
	file, err := os.OpenFile(OutputFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("无法创建或打开日志文件: %v", err)
	}
	defer file.Close()

	lastGgaTime := time.Now()

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("[串口错误] 读取失败(正在重试): %v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}

		line = strings.TrimSpace(line)

		// 检查是否是 GGA 语句 (包含位置和定位状态)
		if strings.Contains(line, "$GNGGA") || strings.Contains(line, "$GPGGA") {
			// 将 GGA 语句发送给 CORS 协程（每五秒更新一次即可，避免通道拥堵）
			if time.Since(lastGgaTime) > 5*time.Second {
				select {
				case ggaChannel <- line:
					lastGgaTime = time.Now()
				default:
				}
			}

			// 解析 GGA 检查是否达到 RTK 精准定位
			// 格式示例: $GNGGA,060258.00,3111.12345,N,12122.54321,E,4,12,0.8,45.1,M,...
			// 其中第 6 个字段是定位状态。4 代表 RTK 固定解（厘米级精度），5 代表 RTK 浮动解（分米级）
			fields := strings.Split(line, ",")
			if len(fields) > 6 {
				status := fields[6]
				if status == "4" { // 状态 4 代表高精度固定解
					log.Printf("[🎉 RTK FIXED] 捕获到厘米级高精度坐标!\n")
					outputLine := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), line)
					
					// 写入文本文件
					if _, err := file.WriteString(outputLine); err != nil {
						log.Printf("写入文件失败: %v\n", err)
					} else {
						log.Printf("已保存至文件: %s", line)
					}
				} else {
					log.Printf("[定位中] 当前GPS状态代码: %s (等待RTK解算变为 4...)\n", status)
				}
			}
		}
	}
}

// 异步连接 CORS 并注入差分数据到串口（带自动重试容错机制）
func connectCorsAndInject(serialStream *serial.Port, ggaChannel chan string) {
	for {
		log.Println("[CORS] 正在连接 CORS NTRIP 服务器...")
		address := fmt.Sprintf("%s:%s", CorsHost, CorsPort)
		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err != nil {
			log.Printf("[CORS错误] 连接失败: %v。5秒后重试...\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// 组装 NTRIP 协议请求头
		auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", CorsUser, CorsPassword)))
		req := fmt.Sprintf("GET /%s HTTP/1.1\r\n"+
			"User-Agent: NTRIP GoClient/1.0\r\n"+
			"Authorization: Basic %s\r\n"+
			"Accept: */*\r\n\r\n", CorsMountPoint, auth)

		_, err = conn.Write([]byte(req))
		if err != nil {
			log.Printf("[CORS错误] 发送请求头失败: %v，正在断开重连...\n", err)
			conn.Close()
			continue
		}

		// 读取响应，确保握手成功
		respReader := bufio.NewReader(conn)
		respLine, err := respReader.ReadString('\n')
		if err != nil || !strings.Contains(respLine, "ICY 200 OK") && !strings.Contains(respLine, "HTTP/1.1 200 OK") {
			log.Printf("[CORS错误] 认证失败或挂载点错误。服务器响应: %s。10秒后重试...\n", respLine)
			conn.Close()
			time.Sleep(10 * time.Second)
			continue
		}
		log.Println("[CORS] 认证成功，连接已建立！开始双向数据传输。")

		// 启动一个临时的内部协程：负责把从主线程拿到的 GGA 定位语句发给 CORS 服务器（CORS需要依据你的位置下发差分数据）
		go func(netConn net.Conn) {
			for gga := range ggaChannel {
				_, err := netConn.Write([]byte(gga + "\r\n"))
				if err != nil {
					return // 连接断开则退出该协程
				}
			}
		}(conn)

		// 接收 CORS 差分数据并灌入串口
		buffer := make([]byte, 1024)
		for {
			n, err := respReader.Read(buffer)
			if err != nil {
				log.Printf("[CORS断开] 差分流中断: %v。即将自动重连...\n", err)
				conn.Close()
				break // 跳出内循环，触发外循环重连
			}

			if n > 0 {
				// 将差分数据（RTCM格式）直接写入串口送给 GPS 模块
				_, err := serialStream.Write(buffer[:n])
				if err != nil {
					log.Printf("[串口错误] 无法写入差分数据到模块: %v\n", err)
				}
			}
		}
	}
}

```

---

## 三、 代码关键点解析与容错机制

1. **双向解算架构（NTRIP Client）**：
CORS 系统需要知道你的“大概位置”才能分发你头顶上基准站的差分流。因此代码中主循环读取到串口的 `$GNGGA` 后，会通过 `ggaChannel` 塞给网络协程，网络协程再反向发送给千寻/六分服务器。
2. **断线自动重连（网络容错）**：
使用外层的 `for { ... }` 无限循环包裹网络请求。无论是网络闪断、授权过期还是服务器主动踢掉连接，`respReader.Read` 都会抛出错误，触发 `break` 并进入下一次循环，在 5 秒后自动尝试重新建立 TCP 连接并重新认证。
3. **串口读取容错**：
串口读取使用 `bufio.NewReader`，当由于硬件松动或电平干扰导致读取单行失败时，系统不会崩溃，而是报出 `[串口错误]` 后利用 `continue` 略过坏数据，继续等待接收下一行。
4. **精确解筛选**：
通过 `strings.Split(line, ",")` 切分 GGA 语句。只有当第 6 个字段的值为 `"4"` 时，才代表芯片进入了 **RTK 固定解（Fixed）** 状态。此时的经纬度数据才是厘米级精确的，程序只会把这种状态的数据打上时间戳并写入 `precise_gps_log.txt`。