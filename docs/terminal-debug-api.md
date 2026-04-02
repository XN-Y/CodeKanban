# 终端调试 API

## 概述

为了方便调试终端输出和 AI 助手状态检测，我们提供了一个专门的调试 API 端点，可以获取终端的完整输出内容、AI 助手状态信息以及录制信息。

## API 端点

### 获取终端调试信息

**请求**

```
GET /api/v1/terminals/{sessionId}/debug
```

**路径参数**

- `sessionId` - 终端会话 ID

**响应示例**

```json
{
  "code": 200,
  "data": {
    "sessionId": "abc123",
    "projectId": "project-1",
    "worktreeId": "worktree-1",
    "status": "running",
    "rows": 24,
    "cols": 80,
    "scrollbackContent": "$ echo hello\nhello\n$ claude-code\n✻ Analyzing codebase… (esc to interrupt · 5s)\n",
    "scrollbackSize": 82,
    "scrollbackLimit": 262144,
    "aiAssistant": {
      "type": "claude-code",
      "displayName": "Claude Code",
      "command": "claude-code",
      "state": "thinking",
      "stateUpdatedAt": "2025-01-22T10:30:00Z",
      "stats": {
        "thinkingDuration": 5000000000,
        "outputDuration": 2000000000
      },
      "interrupted": false
    }
  }
}
```

**字段说明**

| 字段 | 类型 | 说明 |
|------|------|------|
| `sessionId` | string | 终端会话 ID |
| `projectId` | string | 项目 ID |
| `worktreeId` | string | 工作树 ID |
| `status` | string | 会话状态：`starting`, `running`, `closed`, `error` |
| `rows` | int | 终端行数（高度） |
| `cols` | int | 终端列数（宽度） |
| `scrollbackContent` | string | 终端的完整输出内容（scrollback buffer） |
| `scrollbackSize` | int | 当前 scrollback 内容大小（字节） |
| `scrollbackLimit` | int | scrollback 缓冲区大小限制（字节） |
| `aiAssistant` | object | AI 助手信息（如果检测到） |
| `aiAssistant.type` | string | AI 助手类型：`claude-code`, `codex`, `qwen-code` 等 |
| `aiAssistant.displayName` | string | 显示名称 |
| `aiAssistant.command` | string | 运行的命令 |
| `aiAssistant.state` | string | 当前状态：`unknown`, `thinking`, `output`, `interrupted` |
| `aiAssistant.stateUpdatedAt` | string | 状态更新时间 |
| `aiAssistant.stats` | object | 状态持续时间统计 |
| `aiAssistant.interrupted` | bool | 是否被用户中断过 |

## 使用场景

### 1. 调试 AI 助手状态检测

当您怀疑 AI 助手状态检测有问题时，可以：

1. 使用此 API 获取终端的完整输出
2. 查看 `aiAssistant.state` 字段，确认检测到的状态是否正确
3. 对比 `scrollbackContent` 中的实际输出和检测到的状态

### 2. 分析终端输出

当需要分析终端的历史输出时，可以：

1. 获取 `scrollbackContent` 字段的完整内容
2. 在本地进行文本分析或正则匹配
3. 验证输出格式是否符合预期

## 示例

### 使用 curl

```bash
# 获取终端 abc123 的调试信息
curl http://localhost:3005/api/v1/terminals/abc123/debug

# 使用 jq 格式化输出
curl http://localhost:3005/api/v1/terminals/abc123/debug | jq .

# 只查看 scrollback 内容
curl http://localhost:3005/api/v1/terminals/abc123/debug | jq -r .data.scrollbackContent

# 查看 AI 助手状态
curl http://localhost:3005/api/v1/terminals/abc123/debug | jq .data.aiAssistant
```

### 使用 JavaScript

```javascript
// 获取终端调试信息
async function getTerminalDebugInfo(sessionId) {
  const response = await fetch(`/api/v1/terminals/${sessionId}/debug`);
  const result = await response.json();

  if (result.code === 200) {
    console.log('Scrollback content:', result.data.scrollbackContent);
    console.log('AI Assistant state:', result.data.aiAssistant?.state);
  }

  return result.data;
}
```

## 注意事项

1. **性能考虑**：此 API 会返回完整的 scrollback 内容，如果终端输出很多，响应可能会比较大
2. **仅用于调试**：此 API 主要用于开发和调试，不建议在生产环境频繁调用
3. **权限控制**：此 API 不验证 projectId，任何知道 sessionId 的客户端都可以访问
4. **内容限制**：返回的内容受 `scrollbackLimit` 限制，早期的输出可能已被丢弃

## 相关文档

- [AI 助手状态配置](./ai-assistant-status-config.md)
- [终端模拟器指南](./terminal-emulator-guide.md)
