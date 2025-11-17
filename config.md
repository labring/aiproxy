# YAML Configuration Guide

AIProxy now supports YAML configuration files for managing channels, model configurations, and system options.

## Configuration Priority

The configuration system follows this priority order (highest to lowest):

1. **Environment Variables** (highest priority)
2. **YAML Configuration File** (medium priority)
3. **Database** (lowest priority)

This means:
- Values set via environment variables will always take precedence
- YAML configuration will override database values
- Database values are used as defaults when no other configuration is provided

## Configuration File Location

By default, AIProxy looks for `config.yaml` in the current working directory.

You can specify a custom location using the `CONFIG_FILE_PATH` environment variable:

```bash
export CONFIG_FILE_PATH=/path/to/your/config.yaml
```

## Configuration File Structure

The YAML configuration file has three main sections. The channel and modelconfig structures directly correspond to the database model types, making it easy to understand and maintain.

### 1. Channels Configuration

Define your API provider channels:

```yaml
channels:
  - name: "openai-primary"
    type_name: "openai"  # Human-readable type name (recommended)
    # OR use numeric type:
    # type: 1  # OpenAI channel type
    key: "sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    base_url: "https://api.openai.com"
    models:
      - "gpt-4"
      - "gpt-3.5-turbo"
    model_mapping:
      "gpt-4": "gpt-4-0613"
    status: 1  # 1=Enabled, 2=Disabled
    priority: 0
    balance: 100.0
    balance_threshold: 10.0
    enabled_auto_balance_check: true
    sets:
      - "default"
```

#### Channel Type Names

You can use either `type_name` (human-readable string) or `type` (numeric code). Using `type_name` is recommended for better readability:

**Supported Type Names:**
- `openai`: OpenAI API
- `azure` / `azure2`: Azure OpenAI
- `anthropic` / `claude`: Anthropic Claude
- `gemini` / `google gemini`: Google Gemini
- `gemini-openai` / `google gemini (openai)`: Google Gemini via OpenAI API
- `zhipu`: Zhipu AI
- `ali` / `aliyun`: Alibaba Cloud
- `baidu`: Baidu Wenxin
- `baiduv2` / `baidu v2`: Baidu Wenxin v2
- `xunfei`: iFlytek Spark
- `tencent`: Tencent Hunyuan
- `moonshot`: Moonshot AI
- `deepseek`: DeepSeek
- `aws`: AWS Bedrock
- `vertexai` / `vertex`: Google Vertex AI
- `xai`: xAI Grok
- `groq`: Groq
- `mistral`: Mistral AI
- `cohere`: Cohere
- `openrouter`: OpenRouter
- And many more... (see `core/model/yaml_integration.go` for the complete list)

**Numeric Channel Types:**
- `1`: OpenAI
- `3`: Azure
- `14`: Anthropic/Claude
- `24`: Google Gemini
- See `core/model/chtype.go` for complete list

### 2. Model Configurations

Define model-specific settings:

```yaml
modelconfigs:
  - model: "gpt-4"
    owner: "openai"
    type_name: "chat"  # Human-readable type name (recommended)
    # OR use numeric type:
    # type: 1  # ChatCompletions
    rpm: 3500  # Requests per minute
    tpm: 80000  # Tokens per minute
    retry_times: 3
    timeout_config:
      request_timeout: 300
      stream_request_timeout: 600
    warn_error_rate: 0.5
    max_error_rate: 0.8
    price:
      input: 0.03  # Price per 1000 input tokens
      output: 0.06  # Price per 1000 output tokens
    config:
      max_context_tokens: 8192
      max_output_tokens: 4096
      vision: false
      tool_choice: true

  - model: "text-embedding-3-small"
    owner: "openai"
    type_name: "embedding"  # Embedding model
    rpm: 3000
    tpm: 1000000
    price:
      input: 0.00002
      output: 0
```

#### Model Type Names

You can use either `type_name` (human-readable string) or `type` (numeric code). Using `type_name` is recommended for better readability:

**Supported Type Names:**
- `chat` / `chatcompletions`: Chat completion models
- `completion` / `completions`: Text completion models
- `embedding` / `embeddings`: Embedding models
- `moderation` / `moderations`: Moderation models
- `image` / `imagegenerations`: Image generation models
- `imageedit` / `imageedits`: Image editing models
- `audio` / `speech` / `audiospeech`: Text-to-speech models
- `transcription` / `audiotranscription`: Audio transcription models
- `translation` / `audiotranslation`: Audio translation models
- `rerank`: Reranking models
- `pdf` / `parsepdf`: PDF parsing models
- `anthropic`: Anthropic-specific models
- And more... (see `core/model/yaml_integration.go` for the complete list)

**Numeric Model Types:**
- `1`: ChatCompletions
- `2`: Completions
- `3`: Embeddings
- `4`: Moderations
- `5`: ImagesGenerations
- See `core/relay/mode/define.go` for complete list

#### Model Config Keys

Common configuration keys:
- `max_context_tokens`: Maximum context window size
- `max_output_tokens`: Maximum output tokens
- `vision`: Whether the model supports vision/image inputs
- `tool_choice`: Whether the model supports function calling

### 3. System Options

Configure system-wide options:

```yaml
options:
  # Log retention (in hours)
  LogStorageHours: "168"  # 7 days
  RetryLogStorageHours: "72"  # 3 days
  LogDetailStorageHours: "24"  # 1 day

  # Log settings
  SaveAllLogDetail: "false"
  LogDetailRequestBodyMaxSize: "10000"
  LogDetailResponseBodyMaxSize: "10000"

  # Rate limiting
  IPGroupsThreshold: "100"  # Requests per minute
  IPGroupsBanThreshold: "200"

  # Retry settings
  RetryTimes: "3"

  # Error rate alerts
  DefaultWarnNotifyErrorRate: "0.5"

  # Usage alerts
  UsageAlertThreshold: "100"
```

#### Available Options

- `LogStorageHours`: How long to keep logs (hours)
- `RetryLogStorageHours`: How long to keep retry logs (hours)
- `LogDetailStorageHours`: How long to keep detailed logs (hours)
- `CleanLogBatchSize`: Batch size for log cleanup operations
- `IPGroupsThreshold`: Request rate limit per IP
- `IPGroupsBanThreshold`: Ban threshold for IP
- `SaveAllLogDetail`: Whether to save all request/response details
- `LogDetailRequestBodyMaxSize`: Max size of request body to log
- `LogDetailResponseBodyMaxSize`: Max size of response body to log
- `DisableServe`: Disable API serving (for maintenance)
- `RetryTimes`: Number of retry attempts
- `DefaultChannelModels`: Default models for new channels (JSON array)
- `GroupMaxTokenNum`: Max tokens per group
- `DefaultWarnNotifyErrorRate`: Default error rate warning threshold
- `UsageAlertThreshold`: Usage alert threshold
- `FuzzyTokenThreshold`: Fuzzy token matching threshold

## Example: Complete Configuration

See `config.example.yaml` for a complete example configuration file.

## Usage

1. Create a `config.yaml` file in your project root or specify a custom location via `CONFIG_FILE_PATH`

2. Start AIProxy as usual:
   ```bash
   ./aiproxy
   ```

3. The configuration will be loaded in this order:
   - Database values (if any)
   - YAML configuration (overrides database)
   - Environment variables (overrides everything)

## Updating Configuration at Runtime

Changes to the YAML configuration file require restarting the application to take effect.

However, you can still use the web UI or API to modify configurations at runtime, which will be stored in the database.

## Migration from Database-only Configuration

You can extract your current database configuration and convert it to YAML format:

1. Export your channels via the web UI
2. Export your model configs via the web UI
3. Convert to YAML format following the structure in `config.example.yaml`

## Notes

- All values in the `options` section must be strings (they will be parsed according to their type)
- Channel and model config IDs are optional in YAML - if omitted, the system will auto-generate them
- When both YAML and database contain the same configuration, YAML takes precedence
- New configurations from YAML that don't exist in the database will be automatically added

## Troubleshooting

### Configuration not loading

- Check the log output on startup - it will show how many channels/models were loaded from YAML
- Verify the YAML syntax is correct (use a YAML validator)
- Ensure the file path is correct (check `CONFIG_FILE_PATH` environment variable)

### Environment variables not overriding YAML

Environment variables override YAML values through the `config.SetXxx()` functions which check for environment variables on every call. Make sure you're using the correct environment variable names (see `core/common/config/env.go` for the list).

### Values reverting to database values

If you modify configuration through the web UI or API, those changes will be written to the database. On next restart, YAML will override those database values again. Use YAML for persistent configuration and the web UI for temporary changes.
