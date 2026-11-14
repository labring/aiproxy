# Aliyun speech-to-text upstream protocols

Clients can keep using `POST /v1/audio/transcriptions` with multipart `model` and
`file` fields. The Aliyun adaptor selects the upstream protocol on the selected
channel, converts the audio/request and returns a Whisper-compatible transcript.

## Channel configuration

Use the channel's visual configuration editor or its existing `configs` JSON:

```json
{
  "stt_upstream_protocol": "auto",
  "stt_model_protocols": {
    "my-public-asr-alias": "openai_chat_audio",
    "private-whisper-deployment": "openai_transcription"
  }
}
```

| Value | Upstream |
| --- | --- |
| `auto` | Detect `qwen3-asr-flash` and dated variants; retain DashScope realtime for existing/custom realtime deployments |
| `dashscope_realtime` | DashScope `/api-ws/v1/inference` task-based WebSocket protocol, including Paraformer and Fun-ASR realtime |
| `openai_chat_audio` | `/compatible-mode/v1/chat/completions` with Base64 `input_audio`; synchronous response normalized to `{text, usage}` |
| `openai_transcription` | Forward multipart uploads to a Whisper-compatible upstream |

Resolution order is an exact override for the public model, an exact override for
the mapped upstream model, the channel default, then automatic detection. An
explicit `auto` override restores detection even when the channel default is
another protocol. Selection is repeated when a request changes channels.

Leave the Aliyun base URL empty for the default DashScope host. For an
OpenAI-compatible deployment exposing `/v1`, set the full base URL, for example
`https://asr.example.com/v1`; HTTP STT requests append `/chat/completions` or
`/audio/transcriptions` to that base without adding `compatible-mode/v1` again.

Models must still be registered/enabled and associated with the channel. Protocol
selection does not bypass model authorization or change model configuration.

## Audio, output and usage

Enable the existing `FFMPEG_ENABLED=true` option and install `ffmpeg` to decode
browser WebM/M4A recordings. Realtime input becomes mono PCM16 at 16 kHz (8 kHz
for 8k models); chat audio becomes PCM WAV. Conversion uses the request context,
a 30-second conversion deadline, pipe-only input protocols, and a decoded-output
size limit of `MAX_REQUEST_BODY_SIZE`. File names and MIME declarations are not
treated as proof of the actual encoding.

Without FFmpeg, mono PCM16 WAV is read from its RIFF headers. Raw mono PCM16
requires `format=pcm` and `sample_rate`. MP3 rates are read from MPEG frame
headers; other supported compressed realtime inputs require `sample_rate`.
WebM/M4A inputs fail with an actionable conversion error instead of being sent as
MP3. Chat audio can forward supported recognizable containers without decoding.

The realtime and chat audio bridges support synchronous `response_format=json`
(the default) and `text`. Unsupported streaming/timestamp output options fail
before calling upstream. Chat audio maps `language` to `asr_options.language`
and `prompt` to ASR context. A silent recording may legitimately produce empty
text; connectivity tests should use known speech and assert a non-empty result.

`usage.seconds` is audio duration. Qwen token usage remains in `input_tokens`,
`output_tokens`, `total_tokens` and `input_token_details`; consumers billing by
seconds must not interpret token totals as duration. Realtime usage is reported
with `type=duration` and measured/upstream seconds, including when preflight
duration accounting is disabled.

Successful OpenAI-compatible JSON responses and `transcript.text.done` SSE events
also include `usage.seconds`, even if the upstream usage only contains token
fields. Valid upstream seconds, including zero, are preserved. Missing, null or
negative seconds are filled from server-measured request audio duration; when no
duration is available the response explicitly includes `seconds: 0`. Existing
total_tokens and vendor-specific usage fields remain unchanged.

Token-priced consumers use independent input_tokens and output_tokens. Chat
prompt_tokens/completion_tokens are normalized to these fields; explicitly
present canonical counters (including zero) win. Missing/invalid components are
zero, never derived from totals, seconds, or transcript text. Without upstream usage, only server-measured duration is reported with
type=duration. Input/output/total tokens are never estimated from audio or
transcript text. Upstream total_tokens is retained as a statistic and never
used to infer input/output charges. Configure separate input/output prices per million tokens for
models billed by tokens; duration-priced models continue using seconds. Plain-text/subtitle formats do not gain a JSON usage envelope.

DashScope realtime billing is retained from `result-generated` events because
`task-finished` normally has `usage=null`. The duration is cumulative per task:
keep the largest reported value instead of summing it across sentence events.
Only when upstream duration is absent does the bridge use measured audio length.

Realtime connections honor the selected channel's HTTP/HTTPS or SOCKS proxy and
TLS verification settings. HTTP/1.1 is negotiated independently for WebSocket
upgrades. The model request timeout covers connection setup and recognition;
failed upgrades preserve upstream authentication/rate-limit status and errors.

## Asynchronous and session-based models

`paraformer-v2`, `fun-asr`, SenseVoice and Qwen `*-filetrans` use native file
transcription APIs. They cannot be sent to the realtime endpoint. Automatic
selection rejects these models with a clear error. Persistent asynchronous
submission/polling and storage integration are outside this synchronous bridge.
Qwen session-based realtime ASR also uses a different protocol.

## Verification

Run the affected adaptor tests:

```sh
go -C core test ./relay/adaptor/ali -count=1
```

Live checks are opt-in and incur upstream usage. They read an existing Aliyun
channel in memory, invoke the local production relay and do not change saved dev
configuration. No credentials are stored in fixtures or printed:

```sh
AIPROXY_STT_DEV_ENV_FILE=/path/to/dev.env \
AIPROXY_STT_LIVE_AUDIO_FILE=/path/to/known-speech.webm \
AIPROXY_STT_LIVE_CHANNEL_ID=24 \
go -C core test ./relay/adaptor/ali -run '^TestAliSTTLive$' -count=1 -v
```
