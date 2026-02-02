package main

import (
	"flag"
	"fmt"
	"image/color"
	"os"
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
	var bgObj fyne.CanvasObject
	if backgroundPath != "" {
		img := canvas.NewImageFromFile(backgroundPath)
		img.FillMode = canvas.ImageFillContain
		bgObj = img
	} else {
		bgObj = canvas.NewRectangle(color.Black)
	}

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
