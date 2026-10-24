package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Transcoder runs ffmpeg to produce a single-rendition HLS stream.
// PRD 3.5/13: 1200 kbps video, 128 kbps audio, 2 second fmp4 segments.
type Transcoder struct {
	ffmpegPath string
	ffprobePath string
}

func NewTranscoder(ffmpegPath string) *Transcoder {
	return &Transcoder{ffmpegPath: ffmpegPath, ffprobePath: ffprobePathFor(ffmpegPath)}
}

// ffprobePathFor picks the ffprobe binary that sits next to the ffmpeg binary.
func ffprobePathFor(ffmpegPath string) string {
	if ffmpegPath == "" || ffmpegPath == "ffmpeg" {
		return "ffprobe"
	}
	dir := filepath.Dir(ffmpegPath)
	base := strings.TrimSuffix(filepath.Base(ffmpegPath), "ffmpeg") + "ffprobe"
	return filepath.Join(dir, base)
}

// ProbeDuration returns the media duration in seconds.
func (t *Transcoder) ProbeDuration(ctx context.Context, path string) (float64, error) {
	out, err := exec.CommandContext(ctx, t.ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w", err)
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration: %w", err)
	}
	return seconds, nil
}

func (t *Transcoder) Transcode(ctx context.Context, sourcePath, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create hls dir: %w", err)
	}

	playlist := "master.m3u8"
	args := []string{
		"-y",
		"-i", sourcePath,
		"-map", "0:v:0", "-map", "0:a:0?",
		"-vf", `scale=-2:min(720\,ih)`,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-profile:v", "main",
		"-b:v", "1200k", "-maxrate", "1200k", "-bufsize", "2400k",
		"-g", "48", "-keyint_min", "48", "-sc_threshold", "0",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2",
		"-f", "hls",
		"-hls_time", "2",
		"-hls_playlist_type", "vod",
		"-hls_segment_type", "fmp4",
		"-hls_flags", "independent_segments",
		// Relative names keep the playlist URIs relative to /hls/:id/.
		"-hls_fmp4_init_filename", "init.mp4",
		"-hls_segment_filename", "seg_%05d.m4s",
		playlist,
	}

	cmd := exec.CommandContext(ctx, t.ffmpegPath, args...)
	cmd.Dir = outDir
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	return nil
}
