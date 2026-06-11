-- ============================================================================
-- Lua Table 完整示例与演示
-- 运行方式：lua table_demo.lua
-- ============================================================================

-- 辅助打印分隔线
local function separator(title)
    print(string.rep("=", 50))
    if title then print(title) end
    print(string.rep("=", 50))
end

-- =========================== 1. Table 的创建 ===========================
separator("1. Table 创建")
local empty = {}                         -- 空表
print("空表:", empty)

-- 数组风格（索引从 1 开始）
local arr = {10, 20, 30, 40}
print("数组: arr[1]=" .. arr[1], "arr[3]=" .. arr[3])

-- 字典风格
local dict = {name="Lua", version="5.4", year=2024}
print("字典: name=" .. dict.name, "year=" .. dict["year"])

-- 混合风格
local mixed = {"apple", "banana", count=2, price=3.5}
print("混合: mixed[1]=" .. mixed[1], "mixed.count=" .. mixed.count)

-- 使用表达式作为键
local key = "foo"
local expr_key = { [key] = "bar", [10] = "ten" }
print("表达式键: foo=" .. expr_key.foo, "[10]=" .. expr_key[10])

-- =========================== 2. 访问与修改元素 ===========================
separator("2. 访问与修改")
local t = {name="Alice", age=30}
t.age = 31                     -- 修改
t["city"] = "Beijing"          -- 新增
print("修改后:", "name="..t.name, "age="..t.age, "city="..t.city)

-- 访问不存在的键返回 nil
print("不存在的键:", t.none)    -- nil

-- =========================== 3. 删除元素 ===========================
separator("3. 删除元素")
local t2 = {a=1, b=2, c=3}
t2.b = nil                     -- 删除 b
print("删除 b 后:", t2.a, t2.b, t2.c)   -- 1 nil 3

-- =========================== 4. Table 的长度 ===========================
separator("4. 长度运算符 #")
local seq = {10,20,30,40}
print("连续数组长度:", #seq)          -- 4

local gap = {10,20,nil,40}          -- 有空洞
print("有空洞的数组长度（不确定）:", #gap)  -- 可能是 2 或 4，不推荐依赖

local dict_only = {x=1, y=2}
print("纯字典长度:", #dict_only)      -- 0（通常）

-- =========================== 5. 遍历 Table ===========================
separator("5. 遍历方式")
local demo = {"hello", "world", lang="Lua", score=100}

-- pairs: 遍历所有键值对（顺序随机）
print("pairs 遍历：")
for k, v in pairs(demo) do
    print(" ", k, v)
end

-- ipairs: 只遍历连续整数键（从1开始，直到遇到 nil）
print("ipairs 遍历：")
for i, v in ipairs(demo) do
    print(" ", i, v)
end

-- 数字 for 循环（要求知道长度）
local arr2 = {5,6,7,8}
print("数字 for 循环：")
for i = 1, #arr2 do
    print(" ", i, arr2[i])
end

-- =========================== 6. Table 作为数组的操作 ===========================
separator("6. 数组操作（table.insert/remove/sort/concat）")
local nums = {3,1,4,1,5}
table.insert(nums, 9)          -- 尾部插入
table.insert(nums, 2, 99)      -- 索引2处插入
print("插入后:", table.concat(nums, ", "))

table.remove(nums)             -- 删除最后一个
table.remove(nums, 2)          -- 删除索引2
print("删除后:", table.concat(nums, ", "))

table.sort(nums)               -- 升序排序
print("升序排序:", table.concat(nums, ", "))
table.sort(nums, function(a,b) return a>b end)
print("降序排序:", table.concat(nums, ", "))

-- =========================== 7. Table 作为字典的操作 ===========================
separator("7. 字典操作")
local dict2 = {name="Lua", version="5.4"}

-- 检查键是否存在
if dict2.name then print("存在 name 键") end
if dict2["none"] == nil then print("none 键不存在") end

-- 获取所有键
local keys = {}
for k in pairs(dict2) do table.insert(keys, k) end
print("所有键:", table.concat(keys, ", "))

-- 获取所有值
local vals = {}
for _, v in pairs(dict2) do table.insert(vals, v) end
print("所有值:", table.concat(vals, ", "))

-- =========================== 8. 元表 (Metatable) ===========================
separator("8. 元表示例（__index, __add, __tostring）")
local point = {x=10, y=20}
local mt = {
    __index = function(t, k) return "default_" .. k end,
    __add = function(a, b) return {x = a.x + b.x, y = a.y + b.y} end,
    __tostring = function(t) return string.format("Point(%d,%d)", t.x, t.y) end
}
setmetatable(point, mt)

print("访问不存在的 z:", point.z)               -- 触发 __index
local p2 = {x=1, y=2}
setmetatable(p2, mt)
local p3 = point + p2                          -- 触发 __add
print("point + p2 =", p3)                      -- 触发 __tostring

-- =========================== 9. 全局环境 _G ===========================
separator("9. 全局环境 _G")
foo = "bar"                                    -- 全局变量
print("_G.foo =", _G.foo)                      -- bar
print('_G["print"] 就是 print 函数:', _G["print"] == print)  -- true

-- 动态调用全局函数
local func_name = "print"
_G[func_name]("通过 _G 动态调用")

-- =========================== 10. 浅拷贝与深拷贝 ===========================
separator("10. 浅拷贝与深拷贝")
local original = {a=1, b={c=2, d=3}}

-- 浅拷贝
function shallow_copy(t)
    local copy = {}
    for k, v in pairs(t) do copy[k] = v end
    return copy
end

-- 深拷贝（处理循环引用）
function deep_copy(orig)
    local copies = {}
    local function _copy(obj)
        if type(obj) ~= "table" then return obj end
        if copies[obj] then return copies[obj] end
        local new = {}
        copies[obj] = new
        for k, v in pairs(obj) do
            new[_copy(k)] = _copy(v)
        end
        return new
    end
    return _copy(orig)
end

local shallow = shallow_copy(original)
local deep = deep_copy(original)

original.b.c = 999
print("修改原表后：")
print("  原表 original.b.c =", original.b.c)      -- 999
print("  浅拷贝 shallow.b.c =", shallow.b.c)      -- 999（被影响）
print("  深拷贝 deep.b.c =", deep.b.c)            -- 2（未受影响）

-- =========================== 11. Table 实现面向对象 ===========================
separator("11. 面向对象模拟")
local Animal = {}
Animal.__index = Animal

function Animal:new(name)
    local obj = {name = name}
    setmetatable(obj, self)
    return obj
end

function Animal:speak()
    print(self.name .. " makes a sound.")
end

-- 继承 Dog
local Dog = setmetatable({}, Animal)
Dog.__index = Dog

function Dog:speak()
    print(self.name .. " barks.")
end

local a = Animal:new("Generic")
a:speak()           --> Generic makes a sound.
local d = Dog:new("Buddy")
d:speak()           --> Buddy barks.

-- =========================== 12. 综合示例：简易缓存 ===========================
separator("12. 综合示例：带过期时间的缓存")
local Cache = {}
Cache.__index = Cache

function Cache:new(ttl)
    local obj = { data = {}, ttl = ttl or 60 }
    setmetatable(obj, self)
    return obj
end

function Cache:set(key, value)
    self.data[key] = {
        value = value,
        expire = os.time() + self.ttl
    }
end

function Cache:get(key)
    local entry = self.data[key]
    if entry and entry.expire > os.time() then
        return entry.value
    else
        self.data[key] = nil
        return nil
    end
end

function Cache:dump()
    for k, v in pairs(self.data) do
        print(string.format("  %s = %s (expires at %s)", k, v.value, os.date("%H:%M:%S", v.expire)))
    end
end

local cache = Cache:new(2)   -- 2秒过期
cache:set("user", "Alice")
print("第一次 get:", cache:get("user"))   --> Alice
os.execute("sleep 3")                    -- 等待3秒（Windows 下可用 timeout /t 3）
print("等待3秒后 get:", cache:get("user")) --> nil

print("\n所有示例运行完毕！")



