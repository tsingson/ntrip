local NTRIPClient = require("rtk_client")

-- 模拟的一段标准 NMEA-0183 GGA 语句 (由于机车在移动，实际应用中应由 GPS 芯片通过串口实时解析传入)
local mock_gga = "$GNGGA,072027.00,3112.1234,N,12123.5678,E,1,12,0.8,25.3,M,0.0,M,,*63"

-- 1. 初始化客户端配置
local client = NTRIPClient.new({
    host = "127.0.0.1",       -- NTRIP Caster IP
    port = 2101,              -- 端口
    mountpoint = "RTCM33",    -- 挂载点
    username = "test_user",   -- 账号
    password = "secret_password", -- 密码
    gga_interval = 5,         -- 5秒回传一次GGA
    timeout = 2
})

-- 2. 启动连接
local success, msg = client:connect()
if not success then
    print("Error: " .. msg)
    os.exit(1)
end

-- 3. 主事件循环 (Event Loop)
local last_gga_time = 0

print("\n--- Starting Data Recv Loop (Press Ctrl+C to stop) ---")
while client.is_running do
    local current_time = os.time()

    -- 【定时任务】：定周期向上行发送 GGA 语句
    if current_time - last_gga_time >= client.gga_interval then
        local ok = client:send_gga(mock_gga)
        if ok then
            last_gga_time = current_time
        else
            print("[Test] Failed to send GGA, loop breaking...")
            break
        end
     matrimonial
    end

    -- 【实时任务】：尝试读取下行的 RTCM 差分数据
    local rtcm_data, err = client:read_rtcm()
    
    if rtcm_data then
        -- 成功拿到差分数据
        print(string.format("[Recv] Got RTCM Data! Size: %d bytes. First Hex: 0x%02X", 
            #rtcm_data, string.byte(rtcm_data, 1)))
        
        -------------------------------------------------------------
        -- 【商用落地业务提示】：
        -- 在实际硬件中，这里应当通过物理串口（UART）将 rtcm_data 直接写入 GPS/GNSS 芯片
        -- uart.write(1, rtcm_data)
        -------------------------------------------------------------
        
    elseif err ~= "timeout" then
        -- 发生了非超时的严重网络错误（如断线）
        print("[Test] Connection lost error: " .. tostring(err))
        break
    end

    -- 极其微小的休眠，防止死循环导致单核 CPU 占用率飙到 100%
    socket.sleep(0.01)
end

-- 4. 资源清理
client:close()
print("[Test] Client safely stopped.")