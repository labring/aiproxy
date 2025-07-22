package audio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/labring/aiproxy/core/common/config"
	log "github.com/sirupsen/logrus"
)

var (
	ErrAudioDurationNAN = errors.New("audio duration is N/A")
	re                  = regexp.MustCompile(`time=(\d+:\d+:\d+\.\d+)`)
)

func GetAudioDuration(ctx context.Context, audio io.Reader) (float64, error) {
	if !config.FfmpegEnabled {
		return 0, nil
	}

	ffprobeCmd := exec.CommandContext(
		ctx,
		"ffprobe",
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		"-i", "-",
	)
	ffprobeCmd.Stdin = audio

	output, err := ffprobeCmd.Output()
	if err != nil {
		return 0, err
	}

	str := strings.TrimSpace(string(output))

	if str == "" || str == "N/A" {
		seeker, ok := audio.(io.Seeker)
		if !ok {
			return 0, ErrAudioDurationNAN
		}

		_, err := seeker.Seek(0, io.SeekStart)
		if err != nil {
			return 0, ErrAudioDurationNAN
		}

		return getAudioDurationFallback(ctx, audio)
	}

	duration, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0, err
	}

	return duration, nil
}

func getAudioDurationFallback(ctx context.Context, audio io.Reader) (float64, error) {
	if !config.FfmpegEnabled {
		return 0, nil
	}

	ffmpegCmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-i", "-",
		"-f", "null", "-",
	)
	ffmpegCmd.Stdin = audio

	var stderr bytes.Buffer

	ffmpegCmd.Stderr = &stderr

	err := ffmpegCmd.Run()
	if err != nil {
		return 0, err
	}

	log.Debugf("ffmpeg -i - -f null -\n%s", stderr.Bytes())

	// Parse the time from ffmpeg output
	// Example: size=N/A time=00:00:05.52 bitrate=N/A speed= 785x
	return parseTimeFromFfmpegOutput(stderr.String())
}

func GetAudioDurationFromFilePath(ctx context.Context, filePath string) (float64, error) {
	if !config.FfmpegEnabled {
		return 0, nil
	}

	ffprobeCmd := exec.CommandContext(
		ctx,
		"ffprobe",
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		"-i", filePath,
	)

	output, err := ffprobeCmd.Output()
	if err != nil {
		return 0, err
	}

	str := strings.TrimSpace(string(output))

	if str == "" || str == "N/A" {
		return getAudioDurationFromFilePathFallback(ctx, filePath)
	}

	duration, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0, err
	}

	return duration, nil
}

func getAudioDurationFromFilePathFallback(ctx context.Context, filePath string) (float64, error) {
	if !config.FfmpegEnabled {
		return 0, nil
	}

	ffmpegCmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-i", filePath,
		"-f", "null", "-",
	)

	var stderr bytes.Buffer

	ffmpegCmd.Stderr = &stderr

	err := ffmpegCmd.Run()
	if err != nil {
		return 0, err
	}

	log.Debugf("ffmpeg -i %s -f null -\n%s", filePath, stderr.Bytes())

	// Parse the time from ffmpeg output
	return parseTimeFromFfmpegOutput(stderr.String())
}

// parseTimeFromFfmpegOutput extracts and converts time from ffmpeg output to seconds
func parseTimeFromFfmpegOutput(output string) (float64, error) {
	// Find all matches of time pattern
	matches := re.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return 0, ErrAudioDurationNAN
	}

	// Get the last time match (as per the instruction)
	match := matches[len(matches)-1]
	if len(match) < 2 {
		return 0, ErrAudioDurationNAN
	}

	// Convert time format HH:MM:SS.MS to seconds
	timeStr := match[1]

	parts := strings.Split(timeStr, ":")
	if len(parts) != 3 {
		return 0, errors.New("invalid time format")
	}

	hours, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}

	minutes, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, err
	}

	seconds, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, err
	}

	duration := hours*3600 + minutes*60 + seconds

	return duration, nil
}
