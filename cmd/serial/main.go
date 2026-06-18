package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/tarm/serial"
)

func main() {
	// 配置参数
	portName := "/dev/cu.usbserial-A5069RR4" // Windows 示例: "COM3", Linux/Mac 示例: "/dev/ttyUSB0"

	config := &serial.Config{
		Name:        portName,
		Baud:        115200,
		ReadTimeout: time.Millisecond * 200, // 【关键升级 4】增大超时，降低 CPU 负载
	}

	// 主循环：具备断线自动重连机制 【关键升级 1】
	for {
		// 【关键升级 2】每次连接前，先显式检查并确保拥有读写权限
		if !hasPermission(portName) {
			log.Printf("[权限不足或设备不存在] 5秒后重新检查 %s...", portName)
			time.Sleep(5 * time.Second)
			continue
		}

		// log.Printf("正在尝试连接串口 %s...", portName)
		port, err := serial.OpenPort(config)
		if err != nil {
			// 修正：将 log.Errorf 改为标准的 log.Printf
			log.Printf("打开串口失败: %v。 5秒后尝试重连...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// log.Println("串口连接成功，开始读取数据...")
		// fmt.Println("\n--- 接收数据流 ---")

		// 执行核心读取逻辑
		runReader(port)

		// log.Println("\n--- 连接已断开，准备重连 ---")
		time.Sleep(2 * time.Second)
	}
}

// 核心读取函数
func runReader(port io.ReadWriteCloser) {
	// 【关键升级 3】加入异常恢复，防止底层驱动 panic 导致进程退出
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[⚠️ 崩溃拦截] 读取线程发生未知错误: %v", r)
		}
		port.Close() // 确保关闭句柄
	}()

	// 【关键升级 4】扩容至 1024 字节缓冲区，应对高频突发数据，防止丢包
	buf := make([]byte, 1024)

	for {
		n, err := port.Read(buf)
		if err != nil {
			// 识别设备拔出或关闭的致命错误
			if err == io.EOF || errors.Is(err, os.ErrClosed) {
				// log.Println("\n[提示] 串口设备已断开连接。")
				break
			}
			// 忽略非致命的 ReadTimeout 超时错误，继续轮询
			continue
		}

		// 连贯打印核心：只有读到数据才打印，且不换行
		if n > 0 {
			fmt.Print(string(buf[:n]))
		}
	}
}

// 【关键升级 2】验证权限与存在性，返回 bool 闭环控制主流程
func hasPermission(portName string) bool {
	// Windows 系统跳过此检查
	if runtime.GOOS == "windows" {
		return true
	}

	// 尝试以读写模式打开文件以测试权限
	file, err := os.OpenFile(portName, os.O_RDWR, 0666)
	if err != nil {
		if os.IsPermission(err) {
			log.Printf("[⚠️ 权限错误] 当前用户没有对 %s 的读写权限！", portName)
			log.Printf("请执行: sudo chmod 666 %s 或将当前用户加入 dialout 用户组。", portName)
		} else if os.IsNotExist(err) {
			log.Printf("[⚠️ 错误] 找不到串口设备 %s，请检查 USB 是否插好。", portName)
		} else {
			log.Printf("[⚠️ 错误] 无法访问串口: %v", err)
		}
		return false // 权限不足或设备不存在，拦截连接
	}

	file.Close()
	return true // 校验通过
}
