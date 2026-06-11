
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


