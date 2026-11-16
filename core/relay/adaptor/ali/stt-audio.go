package ali

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/relay/meta"
)

// prepareSTTAudio checks the content rather than the filename. With FFmpeg
// enabled, browser containers are decoded to mono PCM for realtime or PCM WAV
// for chat audio. Without it, self-describing WAV and supported chat containers
// work directly; raw realtime audio requires explicit format and sample_rate.
func prepareSTTAudio(mt *meta.Meta, req *http.Request, realtime bool) ([]byte, string, int, error) {
	if err := common.ParseMultipartFormWithLimit(req); err != nil {
		return nil, "", 0, err
	}

	file, _, err := req.FormFile("file")
	if err != nil {
		return nil, "", 0, err
	}
	defer file.Close()

	data, err := common.GetBodyLimit(file, -1, config.MaxRequestBodySize)
	if err != nil {
		return nil, "", 0, err
	}

	if len(data) == 0 {
		return nil, "", 0, sttInputError("audio file is empty")
	}

	format := detectSTTFormat(data)
	declared := strings.ToLower(req.FormValue("format"))

	rate := 0
	if value := req.FormValue("sample_rate"); value != "" {
		rate, err = strconv.Atoi(value)
		if err != nil || rate < 8000 || rate > 192000 {
			return nil, "", 0, sttInputError("sample_rate must be between 8000 and 192000 Hz")
		}
	}

	rawPCM := format == "" && declared == "pcm"
	if rawPCM && (rate == 0 || len(data)%2 != 0) {
		return nil, "", 0, sttInputError(
			"raw PCM requires sample_rate and mono signed 16-bit little-endian samples",
		)
	}

	if config.FfmpegEnabled {
		return transcodeSTTAudio(mt, req, data, rawPCM, rate, realtime)
	}

	if !realtime {
		if format == "" {
			return nil, "", 0, sttInputError(
				"unrecognized audio content; enable FFMPEG_ENABLED to decode it",
			)
		}

		return data, format, rate, nil
	}

	if format == "wav" {
		pcm, wavRate, err := readPCMSTTWAV(data)
		if err != nil {
			return nil, "", 0, err
		}

		mt.Set("stt_audio_seconds", float64(len(pcm))/float64(2*wavRate))

		return pcm, "pcm", wavRate, nil
	}

	if rawPCM {
		mt.Set("stt_audio_seconds", float64(len(data))/float64(2*rate))
		return data, "pcm", rate, nil
	}
	// WebM Opus is not Ogg Opus. Renaming it or declaring format=opus does
	// not make it compatible with the DashScope realtime binary protocol.
	if format == "webm" || format == "mp4" || format == "flac" || format == "" {
		return nil, "", 0, sttInputError(
			"audio container requires transcoding for DashScope realtime; enable FFMPEG_ENABLED",
		)
	}

	if format == "ogg" {
		switch {
		case bytes.Contains(data[:min(len(data), 128)], []byte("OpusHead")):
			format = "opus"
		case bytes.Contains(data[:min(len(data), 128)], []byte("Speex   ")):
			format = "speex"
		default:
			return nil, "", 0, sttInputError(
				"Ogg must contain Opus or Speex for realtime recognition",
			)
		}
	}

	if rate == 0 {
		if format == "mp3" {
			rate = sttMP3SampleRate(data)
		}

		if format == "amr" {
			rate = 8000
		}

		if rate == 0 {
			return nil, "", 0, sttInputError(
				"sample_rate is required for this audio format when FFMPEG_ENABLED is disabled",
			)
		}
	}

	return data, format, rate, nil
}

// detectSTTFormat recognizes container signatures, ignoring misleading names
// and multipart MIME types. In particular, WebM must never be sent as MP3.
func detectSTTFormat(data []byte) string {
	switch {
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE":
		return "wav"
	case bytes.HasPrefix(data, []byte{0x1a, 0x45, 0xdf, 0xa3}):
		return "webm"
	case len(data) >= 8 && string(data[4:8]) == "ftyp":
		return "mp4"
	case bytes.HasPrefix(data, []byte("fLaC")):
		return "flac"
	case bytes.HasPrefix(data, []byte("OggS")):
		return "ogg"
	case bytes.HasPrefix(data, []byte("ID3")):
		return "mp3"
	case bytes.HasPrefix(data, []byte("#!AMR")):
		return "amr"
	case len(data) > 1 && data[0] == 0xff && data[1]&0xf6 == 0xf0:
		return "aac"
	case len(data) > 1 && data[0] == 0xff && data[1]&0xe0 == 0xe0:
		return "mp3"
	default:
		return ""
	}
}

// readPCMSTTWAV handles RIFF chunks including padding and metadata. Only mono
// PCM16 can be passed to realtime without decoding; other WAV codecs fail early.
func readPCMSTTWAV(data []byte) ([]byte, int, error) {
	rate := 0

	var pcm []byte
	for offset := 12; offset+8 <= len(data); {
		size64 := int64(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))

		start := offset + 8
		if size64 > int64(len(data)-start) {
			return nil, 0, sttInputError("invalid WAV chunk size")
		}

		// #nosec G115 -- Nonnegative size64 is bounded by the remaining bytes, whose length fits int.
		size := int(size64)

		chunk := data[start : start+size]
		switch string(data[offset : offset+4]) {
		case "fmt ":
			if len(chunk) < 16 || binary.LittleEndian.Uint16(chunk[:2]) != 1 ||
				binary.LittleEndian.Uint16(chunk[2:4]) != 1 ||
				binary.LittleEndian.Uint16(chunk[14:16]) != 16 {
				return nil, 0, sttInputError(
					"WAV must be mono PCM16; enable FFMPEG_ENABLED for other encodings",
				)
			}

			rate = int(binary.LittleEndian.Uint32(chunk[4:8]))
		case "data":
			pcm = chunk
		}

		offset = start + size + size%2
	}

	if rate < 8000 || rate > 192000 || len(pcm) == 0 || len(pcm)%2 != 0 {
		return nil, 0, sttInputError("WAV has invalid or missing PCM audio")
	}

	return pcm, rate, nil
}

// sttAudioBuffer caps decoded output as well as the uploaded container size.
type sttAudioBuffer struct {
	buffer bytes.Buffer
	limit  int64
}

func (b *sttAudioBuffer) Write(p []byte) (int, error) {
	if int64(b.Len())+int64(len(p)) > b.limit {
		return 0, errors.New("decoded audio too large")
	}
	return b.buffer.Write(p)
}

func (b *sttAudioBuffer) Len() int      { return b.buffer.Len() }
func (b *sttAudioBuffer) Bytes() []byte { return b.buffer.Bytes() }

// sttMP3SampleRate reads an MPEG frame header after an optional ID3v2 tag.
// The rate comes from audio bytes rather than a guessed 24 kHz default.
func sttMP3SampleRate(data []byte) int {
	start := 0
	if len(data) >= 10 && string(data[:3]) == "ID3" {
		size := 0
		for _, b := range data[6:10] {
			if b&0x80 != 0 {
				return 0
			}

			size = size<<7 | int(b)
		}

		start = 10 + size
	}

	for i := start; i+4 <= len(data) && i-start < 65536; i++ {
		if data[i] != 0xff || data[i+1]&0xe0 != 0xe0 || data[i+1]&6 == 0 || data[i+2]>>4 == 15 {
			continue
		}

		version := data[i+1] >> 3 & 3

		index := data[i+2] >> 2 & 3
		if version == 1 || index == 3 {
			continue
		}

		rate := []int{44100, 48000, 32000}[index]
		if version == 2 {
			rate /= 2
		}

		if version == 0 {
			rate /= 4
		}

		return rate
	}

	return 0
}

// transcodeSTTAudio decodes only stdin into bounded mono audio. Cancellation,
// subprocess lifetime and output size are bounded independently of input size.
func transcodeSTTAudio(
	mt *meta.Meta,
	req *http.Request,
	data []byte,
	rawPCM bool,
	rate int,
	realtime bool,
) ([]byte, string, int, error) {
	targetRate := 16000
	if strings.Contains(strings.ToLower(mt.ActualModel), "8k") {
		targetRate = 8000
	}

	args := []string{
		"-hide_banner",
		"-loglevel",
		"error",
		"-nostdin",
		"-protocol_whitelist",
		"pipe",
	}
	if rawPCM {
		args = append(args, "-f", "s16le", "-ar", strconv.Itoa(rate), "-ac", "1")
	}

	args = append(
		args,
		"-i",
		"pipe:0",
		"-map",
		"0:a:0",
		"-vn",
		"-threads",
		"1",
		"-ac",
		"1",
		"-ar",
		strconv.Itoa(targetRate),
		"-c:a",
		"pcm_s16le",
	)

	outputFormat := "wav"
	if realtime {
		outputFormat = "s16le"
	}

	args = append(args, "-f", outputFormat, "pipe:1")

	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
	defer cancel()

	// #nosec G702 -- Fixed binary and flags, rate is a validated integer; audio is only read from stdin.
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stdin = bytes.NewReader(data)
	output := &sttAudioBuffer{limit: config.MaxRequestBodySize}

	cmd.Stdout = output
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, "", 0, ctx.Err()
		}

		return nil, "", 0, sttInputError(
			"cannot decode audio with FFmpeg, or decoded audio exceeds MAX_REQUEST_BODY_SIZE",
		)
	}

	if output.Len() == 0 {
		return nil, "", 0, sttInputError("audio contains no samples")
	}

	if realtime {
		mt.Set("stt_audio_seconds", float64(output.Len())/float64(2*targetRate))
		return output.Bytes(), "pcm", targetRate, nil
	}

	return output.Bytes(), "wav", targetRate, nil
}
