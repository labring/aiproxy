# Realtime voice input

`GET /v1/audio/realtime?model=qwen3-asr-flash-realtime` upgrades to a WebSocket
and relays a native Qwen ASR session through the selected Aliyun channel. This is
separate from whole-file `/v1/audio/transcriptions`: audio is streamed once and
partial recognition events arrive while the microphone is recording.

Register/enable the realtime model as an AudioTranscription model (type `8`) and
associate it with an Aliyun channel. Existing token model allowlists, group/scope
routing, channel mappings, RPM/TPM checks and consumption logging still apply.
The first implementation supports `qwen3-asr-flash-realtime` and dated versions.
Existing whole-file requests and their token-accounting behavior are unchanged.

## Authentication and connection

Backend clients use `Authorization: Bearer <AI Proxy key>`. Browser clients can
use the subprotocols `aiproxy-realtime` and `aiproxy-api-key.<AI Proxy key>`; the
key protocol is removed before the normal middleware and is never echoed in the
negotiated protocol. Keys must not appear in query strings or embedded demos.
Only same-origin browser sessions are accepted. Non-browser clients without an
Origin header remain supported.

The upstream URL is built from the selected channel host and
`/api-ws/v1/realtime?model=<mapped model>`. Channel proxy/TLS settings are reused.
Provider credentials stay on the gateway. Upstream handshake failures are
returned before upgrading; an established session is never retried/replayed.

## Events and audio

Send `session.update` with `input_audio_format: "pcm"`, `sample_rate: 16000`,
`input_audio_transcription: {"language":"zh"}` and optional `server_vad`
configuration. Only mono signed PCM16 at 16 kHz is supported in this prototype.
The model cannot be changed during a session.

Send base64 PCM chunks with `input_audio_buffer.append` (100 ms recommended).
`input_audio_buffer.commit`, `input_audio_buffer.clear` and `session.finish` are
also relayed. Other client events, malformed PCM, binary frames, unsupported
formats and sample rates are rejected. Frames are limited to 64 KiB; sessions
are bounded to five minutes and 300 seconds of audio, with inactivity deadlines
and a bounded write buffer. Use `session.finish` and await `session.finished` so
the final utterance is not dropped.

Native server events pass through, especially:

- `conversation.item.input_audio_transcription.text`: replace the current item's
  preview with `text + stash`; `stash` can be revised and should be underlined.
- `conversation.item.input_audio_transcription.completed`: replace that item's
  preview with `transcript` and remove its underline.
- `session.finished`: final session acknowledgement. Includes `usage.seconds`,
  derived from PCM bytes actually forwarded upstream, rounded up. If no upstream
  usage exists, a `type: "duration"` usage with legacy `total_tokens = seconds`
  is supplied. Existing upstream token/vendor fields are preserved. Internal
  gateway accounting uses the forwarded audio duration, never a client duration.

Disconnecting/cancelling does not replay audio and still accounts for audio
already forwarded. Quota admission uses the existing request checks; this bounded
prototype does not re-check the account balance for every audio frame.

## Standalone HTML/JavaScript demo

Open `/demo/realtime-voice.html` on the gateway (localhost or HTTPS). The source is
`core/public/demos/realtime-voice.html` and `realtime-voice.js`; there are no CDN or
framework dependencies. Enter an AI Proxy key manually. It is only held in the
current page and is not written to local storage, cookies or the source files.

Click the microphone to start/stop, or hold the **right Alt / Option** key and
release it to stop. Escape/cancel restores the text replaced by this recording.
Existing text stays in place; confirmed text and the editable input survive new
partial results. Recording controls remain mounted after text arrives. The demo
has no automatic send, full-screen recording overlay or FastGPT changes.

The browser captures through MediaDevices and streams PCM using AudioWorklet.
Stopping flushes the remaining PCM, releases microphone tracks and drains final
recognition. Permission rejection, rapid key release, window blur, cancellation,
network backpressure and repeated sessions are handled explicitly.

## Verification

Run only affected tests:

```sh
go -C core test ./relay/adaptor/ali ./controller ./router -count=1
go -C core test -race ./relay/adaptor/ali ./controller -run '^TestRealtime' -count=1
```

Real browser validation uses Chrome's fake-audio device to feed a known WAV into
actual MediaDevices/AudioWorklet, with the real Aliyun upstream. It is not a mock
ASR or browser speech-recognition service. Paid live checks are opt-in; browser
credentials, private gateway data and recordings stay outside the repository.

### Opening the standalone file or selecting another gateway

The demo probes `/v1/audio/realtime` before handing off to a different gateway;
this probe sends no key. An old deployment returning 404 is shown as an unavailable
realtime endpoint rather than an unexplained same-origin error. The local test
service can be selected explicitly. Loopback aliases on the same port are
normalized to the current page origin.

When the HTML is opened directly, or another upgraded gateway is selected, use
"打开该服务的实时演示". The gateway's own page is opened and the key/model/draft
are handed over with `postMessage` to that exact origin. The receiver validates
both its opener and the declared opener origin, then severs the opener relation.
Only the non-secret opener origin briefly appears in the URL; keys are never put
in URLs or browser storage. Actual microphone streaming still occurs on the
gateway origin, so the server's same-origin WebSocket policy is unchanged.
