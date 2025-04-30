English | [简体中文](./README.zh.md)

# AI Proxy

Next-generation AI gateway, using OpenAI as the protocol entry point.

## Feature

- Intelligent error retry
- Channel selection based on priority and error rate
- Alert notifications
  - Channel balance warning
  - Error rate warning
  - Unauthorized channel warning
  - and more...
- Logging and auditing
  - Comprehensive request log data
  - Request and response body recording
  - Request log tracing
- Data statistics and analysis
  - Request volume statistics
  - Error volume statistics
  - RPM TPM statistics
  - Consumption statistics
  - Model statistics
  - Channel error rate analysis
  - and more...
- Rerank support
- PDF support
- STT model mapping support
- Multi-tenant system separation
- Model RPM TPM limits
- Think model support `<think>` split to `reasoning_content`
- Prompt Token Cache billing support
- Inline tiktoken, no need to download tiktoken file
- API `Swagger` documentation support `http://host:port/swagger/index.html`

## How to use

### Sealos

Use Sealos built-in model capabilities, click to [Sealos](https://usw.sealos.io/?openapp=system-aiproxy).

### FastGPT

Use AI Proxy to access models, click to [FastGPT](https://doc.tryfastgpt.ai/docs/development/modelconfig/ai-proxy/).

## Deploy

### Use Docker Compose

Copy [docker-compose.yaml](./docker-compose.yaml) to directory. default access key is `aiproxy`. default listen port is `3000`.

```bash
docker-compose up -d
```

## Envs

### Basic Configuration

- `LISTEN`: The listen address, default is `:3000`
- `ADMIN_KEY`: The admin key for the AI Proxy Service, admin key is used to admin api and relay api, default is empty
- `INTERNAL_TOKEN`: Internal token for service authentication, default is empty
- `FFMPEG_ENABLED`: Whether to enable ffmpeg, default is `false`

### Debug Options

- `DEBUG`: Enable debug mode, default is `false`
- `DEBUG_SQL`: Enable SQL debugging, default is `false`

### Database Options

- `SQL_DSN`: The database connection string, default is empty, eg: `postgres://postgres:postgres@localhost:5432/postgres`
- `LOG_SQL_DSN`: The log database connection string, default is empty, eg: `postgres://postgres:postgres@localhost:5432/postgres`
- `REDIS_CONN_STRING`: The redis connection string, default is empty, eg: `redis://localhost:6379`
- `DISABLE_AUTO_MIGRATE_DB`: Disable automatic database migration, default is `false`
- `SQL_MAX_IDLE_CONNS`: The maximum number of idle connections in the database, default is `100`
- `SQL_MAX_OPEN_CONNS`: The maximum number of open connections to the database, default is `1000`
- `SQL_MAX_LIFETIME`: The maximum lifetime of a connection in seconds, default is `60`
- `SQLITE_PATH`: The path to the sqlite database, default is `aiproxy.db`
- `SQL_BUSY_TIMEOUT`: The busy timeout for the database, default is `3000`

### Notify Options

- `NOTIFY_NOTE`: Custom notification note, default is `AI Proxy`
- `NOTIFY_FEISHU_WEBHOOK`: The feishu notify webhook url, default is empty, eg: `https://open.feishu.cn/open-apis/bot/v2/hook/xxxx`

### Model Configuration

- `DISABLE_MODEL_CONFIG`: Disable model configuration, default is `false`
- `RETRY_TIMES`: Number of retry attempts, default is `0`
- `ENABLE_MODEL_ERROR_AUTO_BAN`: Enable automatic banning of models with errors, default is `false`
- `MODEL_ERROR_AUTO_BAN_RATE`: Rate threshold for auto-banning models with errors, default is `0.3`
- `TIMEOUT_WITH_MODEL_TYPE`: Timeout settings for different model types, default is `{}`
- `DEFAULT_CHANNEL_MODELS`: Default models for each channel, default is `{}`
- `DEFAULT_CHANNEL_MODEL_MAPPING`: Model mapping for each channel, default is `{}`

### Logging Configuration

- `LOG_STORAGE_HOURS`: Hours to store logs (0 means unlimited), default is `0`
- `LOG_CONTENT_STORAGE_HOURS`: Hours to store log `content` `ip` `endpoint` `ttfb_milliseconds`, default is `0`
- `SAVE_ALL_LOG_DETAIL`: Save all log details, default is `false`
- `LOG_DETAIL_REQUEST_BODY_MAX_SIZE`: Maximum size for request body in log details, default is `128KB`
- `LOG_DETAIL_RESPONSE_BODY_MAX_SIZE`: Maximum size for response body in log details, default is `128KB`
- `LOG_DETAIL_STORAGE_HOURS`: Hours to store log details, default is `72` (3 days)
- `CLEAN_LOG_BATCH_SIZE`: Batch size for cleaning logs, cleaning interval is 1 minute, default is `2000`

### Service Control

- `DISABLE_SERVE`: Disable serving requests, default `false`
- `GROUP_MAX_TOKEN_NUM`: Maximum number of tokens per group (0 means unlimited), default is `0`
- `GROUP_CONSUME_LEVEL_RATIO`: Consumption level ratio for groups, default is `{}`
- `GEMINI_SAFETY_SETTING`: Safety setting for Gemini models, default is `BLOCK_NONE`
- `BILLING_ENABLED`: Enable billing functionality, default is `true`
- `IP_GROUPS_THRESHOLD`: IP group threshold, when the same IP is used by multiple groups, send a warning, default is `0`
- `IP_GROUPS_BAN_THRESHOLD`: IP group ban threshold, when the same IP is used by multiple groups, ban it and all groups, default is `0`
