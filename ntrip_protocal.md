

### 一、 NTRIP 协议深度解析

**NTRIP (Networked Transport of RTCM via Internet Protocol)** 是基于 **HTTP/1.1** 协议扩展而来的应用层流媒体传输协议，主要用于在互联网上高效、实时地分发 GNSS（全球导航卫星系统）差分改正数据（如 RTCM 数据）。

#### 1. 传输协议与角色架构

NTRIP 运行在 **TCP** 之上，默认端口通常为 **2101**。它定义了三个核心角色：

* **NTRIP Source (数据源/基准站)：** 负责采集卫星原始观测数据，生成 RTCM 差分流，并通过 `POST` 或 `SOURCE` 请求推送到 Caster。
* **NTRIP Caster (中心广播器/服务器)：** 核心中转站。负责基准站的管理、用户鉴权、源列表（Source Table）分发以及数据流的并发广播。
* **NTRIP Client (移动站/Rover)：** 差分数据接收端。向 Caster 发送 `GET` 请求，在维持的 TCP 长连接中持续接收差分流，并定期上报自己的 **NMEA GGA** 语句（用于 VRS 虚拟基准站技术）。

#### 2. 数据交互流程与 HTTP 字段含义

##### A. Client 请求 Source Table (获取源列表)

Client 连接 Caster，不指定挂载点时，Caster 会返回当前可用的基准站列表。

* **Client 请求:**
```http
GET / HTTP/1.1
Host: caster.example.com:2101
Ntrip-Version: Ntrip/2.0
User-Agent: NTRIP GoClient/1.0
Connection: close

```


* **Caster 响应:**
```http
HTTP/1.1 200 OK
Content-Type: text/plain
Content-Length: [长度]

[Source Table 文本数据]

```



##### B. Client 请求特定挂载点数据流 (NTRIP 1.0 vs 2.0)

* **Client 发起连接:**
```http
GET /MOUNTPOINT HTTP/1.1
Host: caster.example.com:2101
Ntrip-Version: Ntrip/2.0
Authorization: Basic dXNlcjpwYXNzd29yZA==

```


* `Authorization`: 使用 Base64 编码的 `username:password`。


* **Caster 响应 (NTRIP 1.0 标准):**
```http
ICY 200 OK

```


* **Caster 响应 (NTRIP 2.0 标准):**
```http
HTTP/1.1 200 OK
Server: NTRIP GoCaster/2.0
Content-Type: gnss/data

```


*握手成功后，TCP 连接转为**单向裸流（Raw Stream）模式**，Caster 开始源源不断地向该通道灌入二进制 RTCM 数据。*

---

### 二、 Go 语言描述：Source Table（源列表）结构体

NTRIP 源列表由多行 ASCII 文本组成，每一行代表一个数据项，以分号 `;` 分隔字段。最核心的类型是 **STR (Stream)**，代表一个具体的挂载点。

以下是严格按照 NTRIP 2.0 规范定义的 Go 语言结构体：

```go
package ntrip

// SourceTable 代表完整的源列表
type SourceTable struct {
	CAS []CASRecord // Caster 广播器信息
	NET []NETRecord // 网络/运营商信息
	STR []STRRecord // 数据流/挂载点信息（最常用）
}

// CASRecord - Caster 行 (Format: CAS;host;port;identifier;operator;nmea;country;latitude;longitude;fallbackHost;fallbackPort)
type CASRecord struct {
	Host         string  `json:"host"`
	Port         int     `json:"port"`
	Identifier   string  `json:"identifier"`
	Operator     string  `json:"operator"`
	NMEA         int     `json:"nmea"` // 0: 不接收, 1: 接收
	Country      string  `json:"country"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	FallbackHost string  `json:"fallback_host"`
	FallbackPort int     `json:"fallback_port"`
}

// NETRecord - Network 行 (Format: NET;identifier;operator;authentication;fee;web-net;web-reg;web-dl;spatial;networks)
type NETRecord struct {
	Identifier     string `json:"identifier"`
	Operator       string `json:"operator"`
	Authentication string `json:"authentication"` // N: None, B: Basic, D: Digest
	Fee            string `json:"fee"`            // N: Free, Y: Chargeable
	WebNet         string `json:"web_net"`
	WebReg         string `json:"web_reg"`
	WebDl          string `json:"web_dl"`
	Spatial        string `json:"spatial"`
	Networks       string `json:"networks"`
}

// STRRecord - Stream 行，代表一个具体的挂载点 Mountpoint
// Format: STR;mountpoint;identifier;format;format-details;carrier;nav-system;network;country;latitude;longitude;nmea;solution;generator;compr-estim;authentication;fee;bitrate;misc
type STRRecord struct {
	Mountpoint    string  `json:"mountpoint"`     // 挂载点唯一名称
	Identifier    string  `json:"identifier"`     // 城市或地名简称
	Format        string  `json:"format"`         // 数据格式，如 RTCM 3.2, RTCM 3.3, CMR
	FormatDetails string  `json:"format_details"` // 详细频点信息，如 1004(1),1019(1)
	Carrier       int     `json:"carrier"`        // 载波波段：0:无, 1:单频(L1), 2:双频(L1+L2), 3:多频
	NavSystem     string  `json:"nav_system"`     // 卫星系统：GPS+GLONASS+GALILEO+BDS
	Network       string  `json:"network"`        // 所属网络机构名称
	Country       string  `json:"country"`        // 国家/地区三位代码，如 CHN, USA
	Latitude      float64 `json:"latitude"`       // 纬度
	Longitude     float64 `json:"longitude"`      // 经度
	NMEA          int     `json:"nmea"`           // 是否需要 Rover 上传 GGA：0:不需要, 1:需要(如VRS系统)
	Solution      int     `json:"solution"`       // 解算类型：0:单站, 1:网络RTK
	Generator     string  `json:"generator"`      // 硬件/软件生成器名称
	ComprEstim    string  `json:"compr_estim"`    // 压缩算法
	Authentication string `json:"authentication"` // 鉴权方式：N: 无, B: Basic, D: Digest
	Fee           string  `json:"fee"`            // 计费：N: 免费, Y: 收费
	Bitrate       int     `json:"bitrate"`        // 比特率 (bps)
	Misc          string  `json:"misc"`           // 杂项备注
}

```

---

### 三、 Go 语言：STR 的序列化、反序列化与测试

在 NTRIP 协议中，“序列化”指将 Go 结构体转换为以 `STR;` 开头、分号分隔的标准 NTRIP 协议文本行；“反序列化”指将该文本行解析回 Go 结构体。

#### 1. 实现代码 (`codec.go`)

```go
package ntrip

import (
	"fmt"
	"strconv"
	"strings"
)

// MarshalSTR 将结构体序列化为标准 NTRIP STR 文本行
func MarshalSTR(r *STRRecord) string {
	return fmt.Sprintf("STR;%s;%s;%s;%s;%d;%s;%s;%s;%.2f;%.2f;%d;%d;%s;%s;%s;%s;%d;%s",
		r.Mountpoint, r.Identifier, r.Format, r.FormatDetails, r.Carrier,
		r.NavSystem, r.Network, r.Country, r.Latitude, r.Longitude,
		r.NMEA, r.Solution, r.Generator, r.ComprEstim, r.Authentication,
		r.Fee, r.Bitrate, r.Misc)
}

// UnmarshalSTR 将标准 NTRIP STR 文本行反序列化为结构体
func UnmarshalSTR(line string) (*STRRecord, error) {
	if !strings.HasPrefix(line, "STR;") {
		return nil, fmt.Errorf("invalid STR prefix")
	}

	parts := strings.Split(line, ";")
	// 根据标准，STR 行包含 1 + 18 (Type + 18个字段) = 19 个部分
	if len(parts) < 19 {
		return nil, fmt.Errorf("insufficient fields, expected 19, got %d", len(parts))
	}

	var err error
	r := &STRRecord{
		Mountpoint:     parts[1],
		Identifier:     parts[2],
		Format:         parts[3],
		FormatDetails:  parts[4],
		NavSystem:      parts[6],
		Network:        parts[7],
		Country:        parts[8],
		Generator:      parts[13],
		ComprEstim:     parts[14],
		Authentication: parts[15],
		Fee:            parts[16],
		Misc:           parts[18],
	}

	if r.Carrier, err = strconv.Atoi(parts[5]); err != nil {
		return nil, fmt.Errorf("parse carrier err: %v", err)
	}
	if r.Latitude, err = strconv.ParseFloat(parts[9], 64); err != nil {
		return nil, fmt.Errorf("parse latitude err: %v", err)
	}
	if r.Longitude, err = strconv.ParseFloat(parts[10], 64); err != nil {
		return nil, fmt.Errorf("parse longitude err: %v", err)
	}
	if r.NMEA, err = strconv.Atoi(parts[11]); err != nil {
		return nil, fmt.Errorf("parse nmea err: %v", err)
	}
	if r.Solution, err = strconv.Atoi(parts[12]); err != nil {
		return nil, fmt.Errorf("parse solution err: %v", err)
	}
	if r.Bitrate, err = strconv.Atoi(parts[17]); err != nil {
		return nil, fmt.Errorf("parse bitrate err: %v", err)
	}

	return r, nil
}

```

#### 2. Go 测试代码 (`codec_test.go`)

```go
package ntrip

import (
	"testing"
)

func TestSTRCodec(t *testing.T) {
	// 1. 构造原始结构体实例
	original := &STRRecord{
		Mountpoint:     "RTCM32_VRS",
		Identifier:     "Shanghai",
		Format:         "RTCM 3.2",
		FormatDetails:  "1004(1),1019(1)",
		Carrier:        3,
		NavSystem:      "GPS+GLONASS+BDS",
		Network:        "CORS_NET",
		Country:        "CHN",
		Latitude:       31.23,
		Longitude:      121.47,
		NMEA:           1, // 需要上传 GGA
		Solution:       1, // 网络RTK
		Generator:      "RTCM_Gen_v2",
		ComprEstim:     "none",
		Authentication: "B",
		Fee:            "Y",
		Bitrate:        9600,
		Misc:           "none",
	}

	// 2. 测试序列化 (Marshal)
	rawLine := MarshalSTR(original)
	expectedLine := "STR;RTCM32_VRS;Shanghai;RTCM 3.2;1004(1),1019(1);3;GPS+GLONASS+BDS;CORS_NET;CHN;31.23;121.47;1;1;RTCM_Gen_v2;none;B;Y;9600;none"
	
	if rawLine != expectedLine {
		t.Fatalf("Marshal failed.\nGot: %s\nExp: %s", rawLine, expectedLine)
	}
	t.Logf("Serialized Output: %s", rawLine)

	// 3. 测试反序列化 (Unmarshal)
	parsed, err := UnmarshalSTR(rawLine)
	if err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	// 4. 字段等值验证
	if parsed.Mountpoint != original.Mountpoint || parsed.Latitude != original.Latitude || parsed.Bitrate != original.Bitrate {
		t.Fatalf("Fields mismatch after Unmarshal. Parsed: %+v", parsed)
	}
	t.Log("Unmarshal verification successful! Fields match perfectly.")
}

```

---

### 四、 用 Lua 语言重写：STR 定义、编解码及完整验证

在 Lua 中，我们使用表（Table）来模拟面向对象的类结构，并利用 `string.gmatch` 和 `table.concat` 来高效完成流数据的切分与拼接。

```lua
local ntrip = {}

-- 1. 创建一个新的 STR 表（模拟构造函数）
function ntrip.new_str_record(init_data)
    local obj = {
        mountpoint     = init_data.mountpoint or "",
        identifier     = init_data.identifier or "",
        format         = init_data.format or "",
        format_details = init_data.format_details or "",
        carrier        = tonumber(init_data.carrier) or 0,
        nav_system     = init_data.nav_system or "",
        network        = init_data.network or "",
        country        = init_data.country or "",
        latitude       = tonumber(init_data.latitude) or 0.0,
        longitude      = tonumber(init_data.longitude) or 0.0,
        nmea           = tonumber(init_data.nmea) or 0,
        solution       = tonumber(init_data.solution) or 0,
        generator      = init_data.generator or "",
        compr_estim    = init_data.compr_estim or "",
        authentication = init_data.authentication or "N",
        fee            = init_data.fee or "N",
        bitrate        = tonumber(init_data.bitrate) or 0,
        misc           = init_data.misc or ""
    }
    return obj
end

-- 2. 序列化实现：将 Lua Table 转化为标准的以分号分隔的 NTRIP 字符串
function ntrip.marshal_str(r)
    local parts = {
        "STR",
        r.mountpoint,
        r.identifier,
        r.format,
        r.format_details,
        string.format("%d", r.carrier),
        r.nav_system,
        r.network,
        r.country,
        string.format("%.2f", r.latitude),
        string.format("%.2f", r.longitude),
        string.format("%d", r.nmea),
        string.format("%d", r.solution),
        r.generator,
        r.compr_estim,
        r.authentication,
        r.fee,
        string.format("%d", r.bitrate),
        r.misc
    }
    return table.concat(parts, ";")
end

-- 辅助函数：按分号安全切分字符串（包含空字段的处理）
local function split_by_semicolon(input)
    local fields = {}
    local pattern = "([^;]*);"
    local last_pos = 1
    for part in string.gmatch(input .. ";", pattern) do
        table.insert(fields, part)
    end
    return fields
end

-- 3. 反序列化实现：解析文本行回 Lua Table
function ntrip.unmarshal_str(line)
    if not line or string.sub(line, 1, 4) ~= "STR;" then
        return nil, "Invalid STR prefix"
    end

    local parts = split_by_semicolon(line)
    if #parts < 19 then
        return nil, string.format("Insufficient fields, expected 19, got %d", #parts)
    end

    -- 组装并强制做数值转换转换
    local r = ntrip.new_str_record({
        mountpoint     = parts[2],
        identifier     = parts[3],
        format         = parts[4],
        format_details = parts[5],
        carrier        = parts[6],
        nav_system     = parts[7],
        network        = parts[8],
        country        = parts[9],
        latitude       = parts[10],
        longitude      = parts[11],
        nmea           = parts[12],
        solution       = parts[13],
        generator      = parts[14],
        compr_estim    = parts[15],
        authentication = parts[16],
        fee            = parts[17],
        bitrate        = parts[18],
        misc           = parts[19]
    })
    return r
end


-- =================================================================
-- 4. 测试与验证脚本
-- =================================================================

print("=== [Lua Test] Starting NTRIP STR Codec Test ===")

-- 创建原始测试数据
local original_record = ntrip.new_str_record({
    mountpoint     = "RTCM33_VRS",
    identifier     = "Beijing",
    format         = "RTCM 3.3",
    format_details = "1005(1),1074(1)",
    carrier        = 3,
    nav_system     = "GPS+BDS",
    network        = "CORS_CHINA",
    country        = "CHN",
    latitude       = 39.90,
    longitude      = 116.40,
    nmea           = 1,
    solution       = 1,
    generator      = "LuaCaster_v1",
    compr_estim    = "none",
    authentication = "B",
    fee            = "Y",
    bitrate        = 19200,
    misc           = "test_data"
})

-- 测试序列化
local serialized_line = ntrip.marshal_str(original_record)
print("\n[Serialized Output]:")
print(serialized_line)

local expected = "STR;RTCM33_VRS;Beijing;RTCM 3.3;1005(1),1074(1);3;GPS+BDS;CORS_CHINA;CHN;39.90;116.40;1;1;LuaCaster_v1;none;B;Y;19200;test_data"
assert(serialized_line == expected, "Serialization mismatch error!")

-- 测试反序列化
local parsed_record, err = ntrip.unmarshal_str(serialized_line)
if err then
    print("Error during deserialization: " .. tostring(err))
    os.exit(1)
end

-- 验证转换完的字段精度及正确性
print("\n[Deserialization Verification]:")
print("Mountpoint: ", parsed_record.mountpoint)  --> RTCM33_VRS
print("Latitude:   ", parsed_record.latitude)    --> 39.9
print("Bitrate:    ", parsed_record.bitrate)     --> 19200
print("Auth Type:  ", parsed_record.authentication)--> B

-- 自动化断言验证
assert(parsed_record.mountpoint == original_record.mountpoint, "Mountpoint corrupted")
assert(parsed_record.latitude == original_record.latitude, "Latitude precision lost")
assert(parsed_record.bitrate == original_record.bitrate, "Bitrate corrupted")

print("\n🎉 [Lua Test] SUCCESS! All encoder/decoder assertions passed perfectly.")

```



