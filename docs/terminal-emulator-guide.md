# 终端模拟器 - 后端显示内容模拟

## 概述

在没有 xterm.js 的情况下，后端可以模拟出终端当前实际显示的内容。这对于调试和分析非常有用。

## 为什么需要终端模拟器？

### 问题

当前我们只有原始 chunks：

```
Chunk 1: "✻ Analyzing… (esc to interrupt · 5s)\r"
Chunk 2: "✻ Analyzing… (esc to interrupt · 6s)\r"
Chunk 3: "✻ Analyzing… (esc to interrupt · 7s)\r"
```

**scrollbackContent** 包含所有 chunks：
```
"✻ Analyzing… (esc to interrupt · 5s)\r✻ Analyzing… (esc to interrupt · 6s)\r✻ Analyzing… (esc to interrupt · 7s)\r"
```

**实际显示**（因为 `\r` 覆盖）：
```
"✻ Analyzing… (esc to interrupt · 7s)"
```

### 解决方案

使用终端模拟器处理控制字符（如 `\r`, `\n`, ANSI 转义序列），得到实际显示的内容。

## 使用方法

### 1. 基础用法

```go
import "code-kanban/utils/ai_assistant2"

// 创建模拟器（80列 x 24行）
emulator := ai_assistant2.NewSimpleTerminalEmulator(80, 24)

// 喂入 chunks
emulator.Write([]byte("Hello World\n"))
emulator.Write([]byte("Line 2\r"))
emulator.Write([]byte("Overwrite"))

// 获取当前显示内容
content := emulator.String()
// 输出: "Hello World\nOverwrite"
```

### 2. 模拟 Claude Code 状态更新

```go
emulator := ai_assistant2.NewSimpleTerminalEmulator(80, 24)

// Claude Code 每秒更新一次状态行
emulator.Write([]byte("✻ Analyzing codebase… (esc to interrupt · 5s)"))
emulator.Write([]byte("\r✻ Analyzing codebase… (esc to interrupt · 6s)"))
emulator.Write([]byte("\r✻ Analyzing codebase… (esc to interrupt · 7s)"))

// 获取当前行内容
currentLine := emulator.GetCurrentLine()
// 输出: "✻ Analyzing codebase… (esc to interrupt · 7s)"

// 可见行（非空行）
lines := emulator.GetVisibleLines()
// 输出: ["✻ Analyzing codebase… (esc to interrupt · 7s)"]
```

### 3. 模拟完整终端会话

```go
// 创建模拟器
emulator := ai_assistant2.NewSimpleTerminalEmulator(80, 24)

// 模拟多个 chunks 的输入
chunks := [][]byte{
    []byte("\033[2J\033[H"), // 清屏
    []byte("$ claude-code\n"),
    []byte("✻ Analyzing codebase… (esc to interrupt · 5s)\r"),
    []byte("✻ Analyzing codebase… (esc to interrupt · 6s)\r"),
}

for _, chunk := range chunks {
    emulator.Write(chunk)
}

// 获取最终显示内容
displayContent := emulator.String()
fmt.Println("Terminal actually shows:")
fmt.Println(displayContent)
```

### 4. 获取特定信息

```go
emulator := ai_assistant2.NewSimpleTerminalEmulator(80, 24)

// 处理输出...
emulator.Write(chunks...)

// 获取光标位置
x, y := emulator.GetCursorPosition()
fmt.Printf("Cursor at (%d, %d)\n", x, y)

// 获取当前行（光标所在行）
currentLine := emulator.GetCurrentLine()

// 获取所有可见行
allLines := emulator.GetVisibleLines()

// 清空屏幕
emulator.Clear()
```

## 支持的功能

| 功能 | 支持 | 说明 |
|------|------|------|
| `\r` (回车) | ✅ | 光标回到行首 |
| `\n` (换行) | ✅ | 光标移到下一行 |
| `\b` (退格) | ✅ | 光标左移 |
| ANSI 颜色 | ✅ | 自动移除颜色代码 |
| 滚动 | ✅ | 自动滚动缓冲区 |
| UTF-8 | ✅ | 正确处理多字节字符 |
| 清屏 | ⚠️ | 简化版 |
| 光标移动序列 | ❌ | 暂不支持 |

## 实际应用场景

### 场景 1：调试状态检测

```go
// 1. 加载录制文件
assistantType, records, _ := ai_assistant2.LoadChunkRecordsFromFile("debug.json")

// 2. 创建模拟器和状态追踪器
emulator := ai_assistant2.NewSimpleTerminalEmulator(80, 24)
tracker := ai_assistant2.NewStatusTracker()
tracker.Activate(assistantType)

// 3. 逐个处理 chunks
for i, record := range records {
    // 更新模拟器
    emulator.Write(record.ChunkBytes)

    // 检测状态
    state, _, changed := tracker.Process(record.ChunkBytes)

    if changed {
        // 获取当前实际显示的内容
        displayContent := emulator.GetCurrentLine()
        fmt.Printf("[%d] State changed to: %s\n", i, state)
        fmt.Printf("     Display shows: %s\n", displayContent)
    }
}
```

### 场景 2：分析误判

```go
// 找出状态检测误判的 chunks
for i, record := range records {
    emulator.Write(record.ChunkBytes)
    currentDisplay := emulator.GetCurrentLine()

    // 检查是否包含 "esc to interrupt" 但状态不是 thinking
    if strings.Contains(currentDisplay, "esc to interrupt") &&
       record.DetectedState != ai_assistant2.AIAssistantStateThinking {
        fmt.Printf("❌ False negative at chunk %d:\n", i)
        fmt.Printf("   Display: %s\n", currentDisplay)
        fmt.Printf("   Detected: %s (should be thinking)\n", record.DetectedState)
    }
}
```

### 场景 3：调试 API 集成

可以在调试 API 中添加模拟显示内容：

```go
// 在 DebugInfo 中添加字段
type DebugInfo struct {
    // ... 现有字段
    DisplayContent string `json:"displayContent"` // 模拟显示内容
}

// 在 GetDebugInfo 中计算
func (s *Session) GetDebugInfo() *DebugInfo {
    // ... 现有代码

    // 模拟终端显示
    emulator := ai_assistant2.NewSimpleTerminalEmulator(s.cols, s.rows)
    for _, chunk := range scrollback {
        emulator.Write(chunk)
    }
    info.DisplayContent = emulator.String()

    return info
}
```

## 性能考虑

- **内存**：O(rows × cols)，默认 24 × 80 = 1920 字符
- **时间**：O(n)，n 为输入字节数
- **适用场景**：调试、测试、分析（不建议用于生产环境的实时渲染）

## 限制

1. **简化版实现**：不支持完整的 VT100/ANSI 转义序列
2. **无颜色保留**：颜色代码被移除，只保留文本
3. **无复杂光标移动**：不支持 `\x1b[H` 等光标定位序列

如果需要完整的终端模拟功能，推荐使用 `github.com/hinshun/vt10x`。

## 与前端 xterm.js 的对比

| | 后端模拟器 | xterm.js |
|---|---|---|
| **用途** | 调试、分析 | 用户交互 |
| **功能** | 基础控制字符 | 完整 VT100/ANSI |
| **性能** | 低开销 | 浏览器渲染 |
| **颜色** | 不保留 | 完整支持 |
| **输出** | 纯文本 | 富文本 + 样式 |

## 总结

后端终端模拟器让我们能够：

✅ **看到实际显示**：不是原始 chunks，而是用户看到的内容
✅ **调试状态检测**：基于实际显示内容验证检测逻辑
✅ **分析误判**：找出被覆盖或隐藏的内容导致的问题
✅ **测试准确性**：无需启动浏览器即可测试

这对于调试 AI 助手状态检测特别有用！
