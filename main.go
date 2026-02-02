package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"os/exec"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/wav"
)

// Flags
var (
	ringtonePath    string
	backgroundPath  string
	snoozeDuration  time.Duration
	timeoutDuration time.Duration
)

func main() {
	parseFlags()

	// 1. Setup Timeout
	setupTimeout()

	// 2. Setup Audio
	ctrl, err := setupAudio()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Audio error: %v\n", err)
	}

	// 3. Setup UI
	setupUI(ctrl)
}

func parseFlags() {
	flag.StringVar(&ringtonePath, "ringtone", "", "Path to audio file")
	flag.StringVar(&ringtonePath, "r", "", "Path to audio file (short)")

	flag.StringVar(&backgroundPath, "background", "", "Path to background image")
	flag.StringVar(&backgroundPath, "b", "", "Path to background image (short)")

	flag.DurationVar(&snoozeDuration, "snooze", 5*time.Minute, "Default snooze duration")
	flag.DurationVar(&snoozeDuration, "s", 5*time.Minute, "Default snooze duration (short)")

	flag.DurationVar(&timeoutDuration, "timeout", 1*time.Minute, "Timeout duration")
	flag.DurationVar(&timeoutDuration, "t", 1*time.Minute, "Timeout duration (short)")

	flag.Parse()
}

func setupTimeout() {
	if timeoutDuration > 0 {
		go func() {
			time.Sleep(timeoutDuration)
			fmt.Println("err timeout")
			os.Exit(2)
		}()
	}
}

// Global control for audio to stop it
type audioControl struct {
	streamer beep.StreamSeeker
	format   beep.Format
	ctrl     *beep.Ctrl
}

func setupAudio() (*audioControl, error) {
	if ringtonePath == "" {
		return nil, nil // No audio
	}

	f, err := os.Open(ringtonePath)
	if err != nil {
		return nil, err
	}
	// Note: We are not closing f, as the streamer needs it.

	var streamer beep.StreamSeekCloser
	var format beep.Format

	// Try MP3 first, then WAV
	streamer, format, err = mp3.Decode(f)
	if err != nil {
		f.Seek(0, 0)
		streamer, format, err = wav.Decode(f)
		if err != nil {
			return nil, fmt.Errorf("unsupported audio format")
		}
	}

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	loop := beep.Loop(-1, streamer)
	ctrl := &beep.Ctrl{Streamer: loop, Paused: false}

	speaker.Play(ctrl)

	return &audioControl{
		streamer: streamer,
		format:   format,
		ctrl:     ctrl,
	}, nil
}

func setupUI(audio *audioControl) {
	a := app.New()
	w := a.NewWindow("Alarm")
	w.SetFullScreen(true)

	// Background
	// Background
	bgObj := setupBackground(backgroundPath)

	// Digital Clock
	// Use Data Binding for thread-safe updates from goroutine
	clockData := binding.NewString()
	clockData.Set(time.Now().Format("15:04:05"))

	clockLabel := widget.NewLabelWithData(clockData)
	clockLabel.Alignment = fyne.TextAlignCenter

	// Ticker
	go func() {
		t := time.NewTicker(1 * time.Second)
		for range t.C {
			clockData.Set(time.Now().Format("15:04:05"))
		}
	}()

	// Interactions
	stopAndExit := func(msg string, code int) {
		if audio != nil {
			speaker.Lock()
			audio.ctrl.Paused = true
			speaker.Unlock()
		}
		fmt.Println(msg)
		os.Exit(code)
	}

	btnDismiss := widget.NewButton("Dismiss", func() {
		stopAndExit("canceled", 0)
	})

	// Snooze
	currentSnooze := snoozeDuration

	snoozeLabel := widget.NewLabel(fmt.Sprintf("Snooze: %s", currentSnooze))

	btnDecrease := widget.NewButton("-", func() {
		currentSnooze -= 5 * time.Minute
		if currentSnooze < 5*time.Minute {
			currentSnooze = 5 * time.Minute
		}
		snoozeLabel.SetText(fmt.Sprintf("Snooze: %s", currentSnooze))
	})

	btnIncrease := widget.NewButton("+", func() {
		currentSnooze += 5 * time.Minute
		snoozeLabel.SetText(fmt.Sprintf("Snooze: %s", currentSnooze))
	})

	btnSnooze := widget.NewButton("Snooze", func() {
		stopAndExit(currentSnooze.String(), 0)
	})

	snoozeContainer := container.NewHBox(btnDecrease, snoozeLabel, btnIncrease, btnSnooze)
	centeredSnooze := container.NewCenter(snoozeContainer)

	// Layout
	controls := container.NewVBox(
		btnDismiss,
		centeredSnooze,
	)

	// Clock in center
	content := container.NewBorder(nil, controls, nil, nil, container.NewCenter(clockLabel))

	finalLayout := container.NewMax(bgObj, content)

	w.SetContent(finalLayout)
	w.ShowAndRun()
}

func setupBackground(path string) fyne.CanvasObject {
	if path == "" {
		return canvas.NewRectangle(color.Black)
	}

	// Check extension
	ext := ""
	if len(path) > 4 {
		ext = path[len(path)-4:]
	}
	// Simple check, can be more robust
	if ext == ".mp4" || ext == ".mkv" || path[len(path)-5:] == ".webm" {
		return streamVideo(path)
	}

	img := canvas.NewImageFromFile(path)
	img.FillMode = canvas.ImageFillContain
	return img
}

func streamVideo(path string) fyne.CanvasObject {
	// Fixed internal resolution for processing
	const w, h = 1280, 720

	// Create a raster that will hold the current frame
	// frameSize := w * h * 4 // RGBA

	// Start ffmpeg
	cmd := exec.Command("ffmpeg",
		"-stream_loop", "-1",
		"-i", path,
		"-f", "image2pipe",
		"-pix_fmt", "rgba",
		"-vcodec", "rawvideo",
		"-s", fmt.Sprintf("%dx%d", w, h),
		"-",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FFmpeg stdout error: %v\n", err)
		return canvas.NewRectangle(color.Black)
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "FFmpeg start error: %v\n", err)
		return canvas.NewRectangle(color.Black)
	}

	// Image to hold data
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	raster := canvas.NewRaster(func(rw, rh int) image.Image {
		return img
	})

	// Goroutine to read frames
	go func() {
		defer cmd.Process.Kill()
		for {
			// Read exactly frameSize bytes
			_, err := io.ReadFull(stdout, img.Pix)
			if err != nil {
				break
			}
			raster.Refresh()
		}
	}()

	return raster
}
