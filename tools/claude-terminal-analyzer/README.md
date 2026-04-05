# Claude Code Terminal Analyzer

一个专门用于分析 Claude Code 终端调试信息的工具，可以：
- 使用虚拟终端模拟器重放终端输出
- 检测 AI 助手状态的转换
- 生成详细的 JSON 和 HTML 分析报告

## 功能特性

- ✅ 支持从 URL 或本地文件加载调试数据
- ✅ 使用虚拟终端（vt10x）精确模拟终端行为
- ✅ 复用项目 `ai_assistant` 包的检测逻辑，确保一致性
- ✅ 自动检测 Claude Code 的状态：
  - `thinking` - AI 正在思考
  - `executing` - AI 正在执行任务
  - `waiting_approval` - 等待审批
  - `replying` - AI 正在回复
  - `waiting_input` - 等待用户输入
- ✅ 记录每次状态转换时的终端完整显示
- ✅ 生成美观的 HTML 可视化报告
- ✅ 输出详细的 JSON 数据报告
- ✅ 统计各阶段耗时

## 安装

```bash
# 编译
go build -o claude-terminal-analyzer.exe

# 或使用提供的脚本
build.bat   # Windows
./build.sh  # Linux/Mac
```

## 使用方法

### 基本用法

```bash
# 从本地文件分析
./claude-terminal-analyzer -source debug.json

# 从 API URL 分析
./claude-terminal-analyzer -source http://localhost:3007/api/v1/terminals/xxxxx/debug

# 指定输出文件
./claude-terminal-analyzer -source debug.json -json report.json
```

### 命令行参数

- `-source` - **必需**，数据源（文件路径或 URL）
- `-json` - JSON 报告输出路径（默认: analysis.json）
- `-type` - AI 类型（默认: claude，可选: claude / codex）
- `-sync2026` - 是否启用 CSI `?2026` 同步输出缓冲（默认: true）
- `-codex-filter` - 是否启用 Codex diff 渲染误判过滤（默认: true）

## 输出示例

### 控制台输出

```
============================================================
📊 ANALYSIS SUMMARY
============================================================
Session ID:      jlBr0XXAJoUJng9N
Total Chunks:    261
State Changes:   15
Unique States:   [thinking executing completed waiting]
Current State:   waiting_input
State Updated:   2025-11-23 17:14:59
Thinking Time:   2.71s
Executing Time:  0.00s
Waiting Time:    16.76s
============================================================

🔄 STATE TRANSITIONS:
  1. [Chunk #0] THINKING
  2. [Chunk #15] EXECUTING - 测试所有功能
  3. [Chunk #89] THINKING
  4. [Chunk #102] EXECUTING - 测试所有功能
  ...
```

### HTML 报告

生成的 HTML 报告包含：
- 📊 统计卡片：显示各项指标
- 🎯 时间轴：展示状态变化
- 💬 消息详情：显示每个状态的具体内容
- 📝 原始数据：可展开查看原始终端输出

### JSON 报告

```json
{
  "summary": {
    "totalChunks": 261,
    "stateChanges": 15,
    "uniqueStates": ["thinking", "executing", "waiting"],
    "sessionId": "jlBr0XXAJoUJng9N",
    "currentState": "waiting_input",
    "totalThinkingMs": 2706,
    "totalExecutingMs": 0,
    "totalWaitingMs": 16763
  },
  "stateChanges": [
    {
      "index": 0,
      "state": "thinking",
      "indicator": "∴ Thought for 4s",
      "message": "",
      "cleanedText": "..."
    }
  ]
}
```

## 工作原理

工具模拟真实的终端行为：

1. **创建虚拟终端**：使用 `vt10x` 包创建终端模拟器
2. **逐块喂入数据**：将每个 scrollback chunk 依次写入虚拟终端
3. **读取终端显示**：从虚拟终端读取当前的可见内容
4. **状态检测**：使用项目的 `ai_assistant` 包检测状态
5. **记录转换**：当状态变化时，记录终端快照

这种方式确保：
- ✅ 完全模拟真实终端的渲染行为
- ✅ 检测逻辑与主项目完全一致
- ✅ 结果稳定可重复

## 典型应用场景

1. **性能分析**：了解 AI 在各个阶段的耗时
2. **状态追踪**：查看 AI 的工作流程
3. **问题诊断**：分析卡顿或异常的原因
4. **使用统计**：统计 AI 的使用模式
5. **行为验证**：验证状态检测逻辑是否正确

## 技术实现

- **虚拟终端模拟**：使用 `github.com/hinshun/vt10x` 包
- **状态检测**：复用 `code-kanban/utils/ai_assistant2` 包的检测逻辑
- **报告生成**：
  - JSON：使用 Go 标准库的 `encoding/json`
  - HTML：使用内嵌模板生成美观的可视化报告

## 开发

```bash
# 运行测试
go test ./...

# 格式化代码
go fmt ./...
```

## 许可证

MIT License
