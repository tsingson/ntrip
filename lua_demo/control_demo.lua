-- ============================================================================
-- Lua 控制语句完整示例
-- 运行方式：lua control_demo.lua
-- ============================================================================

-- 辅助打印分隔线
local function sep(title)
    print(string.rep("=", 50))
    if title then print(title) end
    print(string.rep("=", 50))
end

-- =========================== 1. 条件分支 if ===========================
sep("1. if / elseif / else")
local score = 85

if score >= 90 then
    print("优秀")
elseif score >= 75 then
    print("良好")      -- 会执行这个分支
elseif score >= 60 then
    print("及格")
else
    print("不及格")
end

-- 单行 if 写法（仍需要 then 和 end）
local age = 20
if age >= 18 then print("已成年") end

-- Lua 中只有 false 和 nil 视为假，0 和空字符串视为真
if 0 then print("0 是真") end          -- 输出
if "" then print("空字符串是真") end    -- 输出
if nil then print("nil 是假") else print("nil 是假") end

-- =========================== 2. while 循环 ===========================
sep("2. while 循环")
local i = 1
while i <= 5 do
    io.write(i .. " ")   -- 输出 1 2 3 4 5
    i = i + 1
end
print()   -- 换行

-- 带 break 的 while（提前退出）
local n = 1
while true do
    if n > 3 then break end
    io.write("while-break:" .. n .. " ")
    n = n + 1
end
print()

-- =========================== 3. repeat-until 循环 ===========================
sep("3. repeat-until 循环（至少执行一次）")
local x = 1
repeat
    io.write(x .. " ")
    x = x + 1
until x > 5   -- 条件为真时退出，输出 1 2 3 4 5
print()

-- 注意：until 后面的条件中声明的变量在循环体内可见
local a = 10
repeat
    a = a + 1
    print("循环内 a =", a)
until a > 12   -- 先执行一次，然后判断

-- =========================== 4. 数值 for 循环 ===========================
sep("4. 数值 for 循环")
-- 语法：for var = start, end, step do ... end
-- step 默认为 1，可省略

for i = 1, 5 do
    io.write(i .. " ")   -- 1 2 3 4 5
end
print()

-- 步长 2
for i = 1, 10, 2 do
    io.write(i .. " ")   -- 1 3 5 7 9
end
print()

-- 倒序
for i = 5, 1, -1 do
    io.write(i .. " ")   -- 5 4 3 2 1
end
print()

-- 循环变量是局部变量，循环结束后失效
for i = 1, 3 do end
-- print(i)  -- 会报错，i 不存在

-- =========================== 5. 泛型 for 循环 ===========================
sep("5. 泛型 for 循环（遍历 table 和迭代器）")
-- 遍历数组（ipairs）
local fruits = {"apple", "banana", "cherry"}
for idx, value in ipairs(fruits) do
    print(string.format("fruits[%d] = %s", idx, value))
end

-- 遍历字典（pairs）
local person = {name="Li", age=30, city="Beijing"}
for key, value in pairs(person) do
    print(key, "=", value)
end

-- 遍历字符串中的字符（使用 gmatch）
local str = "Lua"
for c in string.gmatch(str, ".") do
    io.write(c .. " ")   -- L u a
end
print()

-- 使用自定义迭代器（如 io.lines 读取文件）
-- for line in io.lines("data.txt") do print(line) end  -- 实际需要文件

-- =========================== 6. break 和 return ===========================
sep("6. break 与 return 语句")
-- break 用于跳出当前循环（while/repeat/for）
for i = 1, 10 do
    if i > 3 then break end
    io.write(i .. " ")   -- 1 2 3
end
print()

-- return 用于从函数中返回值，不能出现在普通代码块顶层（除非被 do-end 包裹）
function test_return()
    for i = 1, 5 do
        if i == 3 then
            return i   -- 提前返回
        end
    end
    return 0
end
print("test_return() =", test_return())   -- 3

-- 在顶层代码中，return 前需要加 do...end 以限定块
do
    local ok = true
    if ok then
        print("提前结束脚本执行")
        return   -- 如果放在 do-end 里是合法的，但会导致脚本在此处退出，后面的代码不执行
    end
    print("这行不会执行（因为上面的 return 已退出）")
end
-- 由于上面有 return，如果去掉 do-end 结构，下面的代码将不会执行。
-- 为了演示完整性，下面不依赖上面的 return，实际测试时可以把 return 注释。

-- =========================== 7. goto 语句 ===========================
sep("7. goto 和标签（Lua 5.2+）")
-- 语法：::label:: 定义标签，goto label 跳转
-- 限制：不能跳入循环、不能跳出函数、不能跳转到变量的作用域之前

local num = 1
::start::   -- 标签
if num <= 5 then
    io.write(num .. " ")
    num = num + 1
    goto start   -- 跳回标签，实现类似循环的效果
end
print()

-- 使用 goto 模拟 continue（Lua 没有 continue）
for i = 1, 10 do
    if i % 2 == 0 then
        goto continue   -- 偶数跳过
    end
    io.write(i .. " ")  -- 输出奇数：1 3 5 7 9
    ::continue::
end
print()

-- 跳出多层循环的 goto
for i = 1, 3 do
    for j = 1, 3 do
        if i * j > 4 then
            goto found   -- 直接跳到循环外
        end
        print(string.format("i=%d, j=%d", i, j))
    end
end
::found::
print("找到满足条件的组合，退出所有循环")

-- =========================== 8. 综合练习 ===========================
sep("8. 综合示例：猜数字游戏")
math.randomseed(os.time())
local secret = math.random(1, 10)
local guess
print("猜 1-10 之间的数字，猜对为止")
repeat
    io.write("你的猜测: ")
    guess = tonumber(io.read())
    if guess == secret then
        print("恭喜！猜对了！")
    elseif guess == nil then
        print("请输入数字")
    else
        print("猜错了，再试一次")
    end
until guess == secret

print("\n所有控制语句演示结束。")




