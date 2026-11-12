package public

import _ "embed"

//go:embed demos/realtime-voice.html
var RealtimeVoiceHTML []byte

//go:embed demos/realtime-voice.js
var RealtimeVoiceJS []byte
