local socket = require("socket") -- 依赖 luasocket 库

local NTRIPClient = {}
NTRIPClient.__index = NTRIPClient

function NTRIPClient.new(config)
    local self = setmetatable({}, NTRIPClient)
    self.host = config.host or "127.0.0.1"
    self.port = config.port or 2101
    self.mountpoint = config.mountpoint or ""
    self.username = config.username or ""
    self.password = config.password or ""
    self.gga_interval = config.gga_interval or 5 -- GGA 上报间隔（秒）
    self.timeout = config.timeout or 2           -- 读写超时（秒）
    
    self.conn = nil
    self.is_running = false
    return self
end

-- Base64 加密工具（用于 HTTP Basic Auth）
local function base64_encode(data)
    local b = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'
    return ((data:gsub('.', function(x) 
        local r,b='',x:byte()
        for i=8,1,-1 do r=r..(b%2^i-b%2^(i-1)>0 and '1' or '0') end
        return r;
    end)..'0000'):gsub('%d%d%d?%d?%d?%d?', function(x)
        if (#x < 6) then return '' end
        local c=0
        for i=1,#x do c=c+(x:sub(i,i)=='1' and 2^(6-i) or '0') end
        return b:sub(c+1,c+1)
    end)..({ '', '==', '=' })[#data%3+1])
end

-- 建立连接与 NTRIP 握手
function NTRIPClient:connect()
    print(string.format("[RTK] Connecting to %s:%d...", self.host, self.port))
    local err
    self.conn, err = socket.connect(self.host, self.port)
    if not self.conn then
        return false, "Connection failed: " .. tostring(err)
    end

    -- 设置初始超时
    self.conn:settimeout(self.timeout)

    -- 构造 NTRIP 请求头 (兼容 1.0 / 2.0)
    local auth_str = base64_encode(self.username .. ":" .. self.password)
    local req = string.format(
        "GET /%s HTTP/1.1\r\n" ..
        "Host: %s:%d\r\n" ..
        "Ntrip-Version: Ntrip/2.0\r\n" ..
        "User-Agent: NTRIP LuaClient/1.0\r\n" ..
        "Authorization: Basic %s\r\n" ..
        "Connection: close\r\n\r\n",
        self.mountpoint, self.host, self.port, auth_str
    )

    -- 发送握手请求
    local _, s_err = self.conn:send(req)
    if s_err then return false, "Send request failed: " .. s_err end

    -- 读取响应行
    local response, r_err = self.conn:receive("*l")
    if r_err then return false, "Read response failed: " .. r_err end

    print("[RTK] Server Response: " .. response)
    -- 兼容 ICY 200 OK 或 HTTP/1.1 200 OK
    if string.find(response, "200") or string.find(response, "ICY") then
        -- 循环读取直到空行，清除余下的 HTTP Header
        while true do
            local line, l_err = self.conn:receive("*l")
            if l_err or line == "" then break end
        end
        print("[RTK] Handshake successful. Stream opened.")
        self.is_running = true
        return true
    else
        self.conn:close()
        return false, "Server rejected: " .. response
    end
end

-- 发送 GGA 语句给 Caster (用于 VRS 虚拟基准站解算)
function NTRIPClient:send_gga(gga_sentence)
    if not self.conn or not self.is_running then return false end
    
    -- 为发送设置短暂超时，防止网络拥堵导致客户端线程死锁
    self.conn:settimeout(self.timeout)
    local _, err = self.conn:send(gga_sentence .. "\r\n")
    if err then
        print("[RTK] Send GGA failed: " .. tostring(err))
        return false
    end
    print("[RTK] GGA sent successfully.")
    return true
end

-- 接收差分数据流 (非阻塞/带超时控制)
function NTRIPClient:read_rtcm()
    if not self.conn or not self.is_running then return nil, "Not connected" end

    -- 设置极短的读取超时，以便将控制权交还给主循环处理定时任务
    self.conn:settimeout(0.05) 
    
    -- 读取任意可用的数据块（最多4096字节）
    local data, err, partial = self.conn:receive(4096)
    
    if err == "timeout" then
        -- 如果是超时，返回接收到的部分数据（如果有），不判定为错误
        if partial and #partial > 0 then
            return partial
        end
        return nil, "timeout"
    elseif err then
        self.is_running = false
        self.conn:close()
        return nil, "Disconnected: " .. tostring(err)
    end

    return data
end

function NTRIPClient:close()
    self.is_running = false
    if self.conn then
        self.conn:close()
        print("[RTK] Connection closed.")
    end
end

return NTRIPClient