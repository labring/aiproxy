# MCP iCal 服务器

<div align="center">

🗓️ macOS 自然语言日历管理

[![MIT License](https://img.shields.io/badge/License-MIT-green.svg)](https://choosealicense.com/licenses/mit/)
[![Python 3.12+](https://img.shields.io/badge/python-3.12+-blue.svg)](https://www.python.org/downloads/)
[![MCP Compatible](https://img.shields.io/badge/MCP-Compatible-purple.svg)](https://modelcontextprotocol.io)

</div>

## 🌟 概述

使用自然语言改变您与 macOS 日历的交互方式！mcp-ical 服务器利用模型上下文协议 (MCP) 将您的日历管理转变为对话式体验。

```bash
您："我下周的日程安排是什么？"
Claude："让我为您查看一下..."
[显示您即将到来的一周的清晰概览]

您："明天中午和 Sarah 添加一个午餐会议"
Claude："✨ 📅 已创建：与 Sarah 的午餐 明天，下午 12:00"
```

## ✨ 功能特性

### 📅 事件创建

即时将自然语言转换为日历事件！

```text
"下周四下午 1 点在 Bistro Garden 安排团队午餐"
↓
📎 已创建：团队午餐
   📅 周四，下午 1:00
   📍 Bistro Garden
```

#### 支持的功能

- 自定义日历选择
- 位置和备注
- 智能提醒
- 重复事件

#### 高级用户示例

```text
🔄 重复事件：
"设置我的每周团队同步会议，每周一上午 9 点，提前 15 分钟提醒"

📝 详细事件：
"明天下午 2-4 点在工程日历中安排产品评审会议，
添加关于审查 Q1 指标的备注，提前 1 小时提醒我"

📱 多日历支持：
"在我的个人日历中添加下周三下午 3 点的牙医预约"
```

### 🔍 智能日程管理和可用性

通过自然查询快速访问您的日程：

```text
"我下周的日历安排是什么？"
↓
📊 显示您即将到来的事件，格式智能

"下周二我什么时候有空安排一个 2 小时的会议？"
↓
🕒 找到可用时间段：
   • 周二上午 10:00 - 下午 12:00
   • 周二下午 2:00 - 下午 4:00
```

### ✏️ 智能事件更新

自然地修改事件：

```text
之前："将明天的团队会议改到下午 3 点"
↓
之后：✨ 会议已重新安排到下午 3:00
```

#### 更新功能

- 时间和日期修改
- 日历转移
- 位置更新
- 备注添加
- 提醒调整
- 重复模式更改

### 📊 日历管理

- 查看所有可用日历
- 智能日历建议
- 与 iCloud 配置时无缝集成 Google 日历

> 💡 **专业提示**：由于您可以在自定义日历中创建事件，如果您的 Google 日历与 iCloud 日历同步，您也可以使用此 MCP 服务器在 Google 日历中创建事件！只需在创建/更新事件时指定 Google 日历即可。

## 🚀 快速开始

> 💡 **注意**：虽然这些说明重点介绍如何在 Claude 桌面版中设置 MCP 服务器，但此服务器可以与任何兼容 MCP 的客户端一起使用。有关使用不同客户端的更多详细信息，请参阅 [MCP 文档](https://modelcontextprotocol.io/quickstart/client)。

### 先决条件

- [uv 包管理器](https://github.com/astral-sh/uv)
- 配置了日历应用的 macOS
- MCP 客户端 - 推荐使用 [Claude 桌面版](https://claude.ai/download)

### 安装

虽然此 MCP 服务器可以与任何兼容 MCP 的客户端一起使用，但以下说明适用于 Claude 桌面版。

1. **克隆和设置**

    ```bash
    # 克隆仓库
    git clone https://github.com/Omar-V2/mcp-ical.git
    cd mcp-ical

    # 安装依赖项
    uv sync
    ```

2. **配置 Claude 桌面版**

    创建或编辑 `~/Library/Application\ Support/Claude/claude_desktop_config.json`：

    ```json
    {
        "mcpServers": {
            "mcp-ical": {
                "command": "uv",
                "args": [
                    "--directory",
                    "/ABSOLUTE/PATH/TO/PARENT/FOLDER/mcp-ical",
                    "run",
                    "mcp-ical"
                ]
            }
        }
    }
    ```

3. **从终端启动 Claude 以获取日历访问权限**

    > ⚠️ **重要**：必须从终端启动 Claude 才能正确请求日历权限。直接从 Finder 启动不会触发权限提示。

    在终端中运行以下命令。

    ```bash
    /Applications/Claude.app/Contents/MacOS/Claude
    ```

    > ⚠️ **警告**：或者，您可以[手动授予日历访问权限](docs/install.md#method-2-manually-grant-calendar-access)，但这涉及修改系统文件，只有在您了解相关风险的情况下才应该这样做。

4. **开始使用！**

    ```text
    试试："我下周的日程安排看起来怎么样？"
    ```

> 🔑 **注意**：当您首次使用与日历相关的命令时，macOS 将提示日历访问权限。只有按照上述指定从终端启动 Claude 时，此提示才会出现。

## 🧪 测试

> ⚠️ **警告**：测试将创建临时日历和事件。虽然清理是自动的，但只能在开发环境中运行测试。

```bash
# 安装开发依赖项
uv sync --dev

# 运行测试套件
uv run pytest tests
```

## 🐛 已知问题

### 重复事件

- 非标准重复计划可能不总是正确设置
- Claude 3.5 Sonnet 比 Haiku 效果更好
- 重复全天事件的提醒时间可能相差一天

## 🤝 贡献

欢迎反馈和贡献。您可以通过以下方式帮助：

1. Fork 仓库
2. 创建您的功能分支
3. 提交您的更改
4. 推送到分支
5. 打开 Pull Request

## 🙏 致谢

- 使用[模型上下文协议](https://modelcontextprotocol.io)构建
- macOS 日历集成使用 [PyObjC](https://github.com/ronaldoussoren/pyobjc) 构建
