## 1. 简介

`@alipay/mcp-server-alipay` 是支付宝开放平台提供的 MCP Server，让你可以轻松将支付宝开放平台提供的交易创建、查询、退款等能力集成到你的 LLM 应用中，并进一步创建具备支付能力的智能工具。

以下是一个虚构的简化使用场景，用于方便理解工具能力：

> 一位插画师希望通过提供定制的原创插画服务谋取收入。传统方式下，他/她需要和每位客户反复沟通需求、确定价格，并发送支付链接，然后再人工确认支付情况，这个过程繁琐且费时。
>
> 现在，插画师利用支付宝 MCP Server 与智能 Agent 工具，通过 Agent 搭建平台，开发了一个智能聊天应用（网页或小程序）。客户只需在应用中描述自己的绘画需求（如风格偏好、插画用途、交付时间等），AI 就会自动分析需求，快速生成准确且合理的定制报价，并通过工具即时创建出专用的支付宝支付链接。
>
> 客户点击并支付后，创作者立即收到通知，进入创作环节。无需人工往返对话确认交易状态或支付情况，整个流程不仅便捷顺畅，还能显著提高交易效率和客户满意度，让插画师更专注于自己的创作本身，实现更轻松的个性化服务商业模式。

```
     最终用户设备                     Agent 运行环境
+---------------------+        +--------------------------+      +-------------------+
|                     |  交流  |   支付宝 MCP Server +    |      |                   |
|    小程序/WebApp    |<------>|   其他 MCP Server +      |<---->|     支付服务      |
|                     |  支付  |   Agent 开发工具         |      |   交易/退款/查询  |
+---------------------+        +--------------------------+      +-------------------+
     创作服务买家                     智能工具开发者                   支付宝开放平台
      (最终用户)                         (创作者)
```

关于本工具的更多介绍和使用指南，包括准备收款商户身份等前置流程，请参考支付宝开放平台上的 [支付 MCP 服务文档](https://opendocs.alipay.com/open/0go80l) 。

## 2. 使用和配置

要使用工具的大部分支付能力，你需要先成为支付宝开放平台的收款商户，获取商户私钥。
之后，你可以直接在主流的 MCP Client 上使用支付宝 MCP Server：

### 在 Cursor 中使用

在 Cursor 项目中的 `.cursor/mcp.json` 加入如下配置：

```json
{
  "mcpServers": {
    "mcp-server-alipay": {
      "command": "npx",
      "args": ["-y", "@alipay/mcp-server-alipay"],
      "env": {
        "AP_APP_ID": "2014...222",
        "AP_APP_KEY": "MIIE...DZdM=",
        "AP_PUB_KEY": "MIIB...DAQAB",
        "AP_RETURN_URL": "https://success-page",
        "AP_NOTIFY_URL": "https://your-own-server",
        "...其他参数": "...其他值"
      }
    },
    "其他工具": { 
      "...": "..."
    }
  }
}
```

### 在 Cline 中使用

在你的 Cline 设置中找到 `cline_mcp_settings.json` 配置文件，并加入如下配置：

```json
{
  "mcpServers": {
    "mcp-server-alipay": {
      "command": "npx",
      "args": ["-y", "@alipay/mcp-server-alipay"],
      "env": {
        "AP_APP_ID": "2014...222",
        "AP_APP_KEY": "MIIE...DZdM=",
        "AP_PUB_KEY": "MIIB...DAQAB",
        "AP_RETURN_URL": "https://success-page",
        "AP_NOTIFY_URL": "https://your-own-server",
        "...其他参数": "...其他值"
      },
      "disable": false,
      "autoApprove": []
    },
    "其他工具": { 
      "...": "..."
    }
  }
}
```

### 在其他 MCP Client 中使用

你也可以在任何其它 MCP Client 中使用，合理配置 Server 进程启动方式 `npx -y @alipay/mcp-server-alipay`，并按下文介绍设置环境参数即可。

### 所有参数

支付宝 MCP Server 通过环境变量接收参数。参数和默认值包括:

```shell
# 支付宝开放平台配置

AP_APP_ID=2014...222                    # 商户在开放平台申请的应用 ID（APPID）。必需。
AP_APP_KEY=MIIE...DZdM=                 # 商户在开放平台申请的应用私钥。必需。
AP_PUB_KEY=MIIB...DAQAB                 # 用于验证支付宝服务端数据签名的支付宝公钥，在开放平台获取。必需。
AP_RETURN_URL=https://success-page      # 网页支付完成后对付款用户展示的「同步结果返回地址」。
AP_NOTIFY_URL=https://your-own-server   # 支付完成后，用于告知开发者支付结果的「异步结果通知地址」。
AP_ENCRYPTION_ALGO=RSA2                 # 商户在开放平台配置的参数签名方式。可选值为 "RSA2" 或 "RSA"。缺省值为 "RSA2"。
AP_CURRENT_ENV=prod                     # 连接的支付宝开放平台环境。可选值为 "prod"（线上环境）或 "sandbox"（沙箱环境）。缺省值为 "prod"。

# MCP Server 配置

AP_SELECT_TOOLS=all                      # 允许使用的工具。可选值为 "all" 或逗号分隔的工具名称列表。工具名称包括 `mobilePay`, `webPagePay`, `queryPay`, `refundPay`, `refundQuery`。缺省值为 "all"。
AP_LOG_ENABLED=true                      # 是否在 $HOME/mcp-server-alipay.log 中记录日志。默认值为 true。
```

## 3. 使用 MCP Inspector 调试

你可以使用 MCP Inspector 来调试和了解支付宝 MCP Server 的功能：

1. 通过 `export` 设置各环境变量；
2. 执行 `npx -y @modelcontextprotocol/inspector npx -y @alipay/mcp-server-alipay` 启动 MCP Inspector；
3. 在 MCP Inspector WebUI 中调试即可。

## 4. 支持的能力

以下表格列出了所有可用的支付工具能力：

| 名称 | `AP_SELECT_TOOLS` 中的工具名称 | 描述 | 参数 | 输出 |
|-------|--------------------------|------|------|------|
| `create-mobile-alipay-payment` | `mobilePay` | 创建一笔支付宝订单，返回带有支付链接的 Markdown 文本，该链接在手机浏览器中打开后可跳转到支付宝或直接在浏览器中支付。本工具适用于移动网站或移动 App。 | - outTradeNo: 商户订单号，最长 64 个字符<br>- totalAmount: 支付金额，单位：元，最小 0.01<br>- orderTitle: 订单标题，最长 256 个字符 | - url: 支付链接的 markdown 文本 |
| `create-web-page-alipay-payment` | `webPagePay` | 创建一笔支付宝订单，返回带有支付链接的 Markdown 文本，该链接在电脑浏览器中打开后会展示支付二维码，用户可扫码支付。本工具适用于桌面网站或电脑客户端。 | - outTradeNo: 商户订单号，最长 64 个字符<br>- totalAmount: 支付金额，单位：元，最小 0.01<br>- orderTitle: 订单标题，最长 256 个字符 | - url: 支付链接的 markdown 文本 |
| `query-alipay-payment` | `queryPay` | 查询一笔支付宝订单，并返回带有订单信息的文本 | - outTradeNo: 商户订单号，最长 64 个字符 | - tradeStatus: 订单的交易状态<br>- totalAmount: 订单的交易金额<br>- tradeNo: 支付宝交易号 |
| `refund-alipay-payment` | `refundPay` | 对交易发起退款，并返回退款状态和退款金额 | - outTradeNo: 商户订单号，最长 64 个字符<br>- refundAmount: 退款金额，单位：元，最小 0.01<br>- outRequestNo: 退款请求号，最长 64 个字符<br>- refundReason: 退款原因，最长 256 个字符（可选） | - tradeNo: 支付宝交易号<br>- refundResult: 退款结果 |
| `query-alipay-refund` | `refundQuery` | 查询一笔支付宝退款，并返回退款状态和退款金额 | - outRequestNo: 退款请求号，最长 64 个字符<br>- outTradeNo: 商户订单号，最长 64 个字符 | - tradeNo: 支付宝交易号<br>- refundAmount: 退款金额<br>- refundStatus: 退款状态 |

## 5. 如何选择合适的支付方式

在开发过程中，为了让 LLM 能更准确地选择合适的支付方式，建议在 Prompt 中清晰说明你的产品使用场景：

- **扫码支付（`webPagePay`）**：适用于用户在电脑屏幕上看到支付界面的场景。如果您的应用或网站主要运行在桌面端（PC），你可以在 Prompt 中说明："我的应用是桌面软件/PC网站，需要在电脑上展示支付二维码"。

- **手机支付（`mobilePay`）**：适用于用户在手机浏览器内发起支付的场景。如果您的应用是手机H5页面或移动端网站，你可以在 Prompt 中说明："我的页面是手机网页，需要直接在手机上唤起支付宝支付"。

我们会在未来提供更多适合 AI 应用的支付方式，敬请期待。

## 6. 注意事项

- 支付宝 MCP 服务目前处于发布早期阶段，相关能力和配套设施正在持续完善中。如有问题反馈、使用体验或建议，欢迎在 [支付宝开发者社区](https://open.alipay.com/portal/forum) 参与讨论。
- 部署和使用智能体服务时，请务必妥善保管自己的商户私钥，防止泄露。如需要，可参考 [支付宝开放平台-如何修改密钥](https://opendocs.alipay.com/support/01rav9) 的说明让已有密钥失效。
- 在开发任何使用 MCP Server 的智能体服务，并提供给用户使用时，请了解必要的安全知识，防范 AI 应用特有的 Prompt 攻击、MCP Server 任意命令执行等安全风险。
- 更多注意事项和最佳实践，请参考支付宝开放平台上 [关于支付 MCP 服务](https://opendocs.alipay.com/open/0go80l) 的说明。

## 7. 使用协议

本工具是支付宝开放平台能力的组成部分。使用期间，请遵守 [支付宝开放平台开发者服务协议](https://ds.alipay.com/fd-ifz2dlhv/index.html) 及相关商业行为法规。
