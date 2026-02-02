package main

import (
	"embed"
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
	"fyne.io/fyne/v2/widget"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/wav"
)

//go:embed default_ringtone.mp3 default_bg.mp4
var embedFS embed.FS

// Flags
var (
	ringtonePath    string
	backgroundPath  string
	snoozeDuration  time.Duration
	timeoutDuration time.Duration
	showVersion     bool
)

const AppVersion = "0.1.0"

func main() {
	parseFlags()

	if showVersion {
		fmt.Printf("Alarm App version %s\n", AppVersion)
		os.Exit(0)
	}

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
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information (short)")

	flag.Parse()
}

func setupTimeout() {
	if timeoutDuration > 0 {
		go func() {
			time.Sleep(timeoutDuration)
			fmt.Println("Timeout")
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
	var f io.ReadCloser
	var err error

	if ringtonePath != "" {
		f, err = os.Open(ringtonePath)
		if err != nil {
			return nil, err
		}
	} else {
		// Use embedded default
		f, err = embedFS.Open("default_ringtone.mp3")
		if err != nil {
			return nil, fmt.Errorf("failed to open embedded ringtone: %v", err)
		}
	}

	var streamer beep.StreamSeekCloser
	var format beep.Format

	// Try MP3 first, then WAV
	// Note: beep decoders wrap the ReadCloser.
	// For re-seeking/looping, beep relies on the streamer implementation.
	// mp3.Decode returns a streamer that supports Seek if the input is a ReadSeeker?
	// Actually beep mp3 decoder buffers or seeks the underlying file.
	// fs.File supports Seek.

	// We need a ReadSeekCloser for optimal looping?
	// Beep mp3 Decode takes ReadCloser.
	// Let's assume it works.
	streamer, format, err = mp3.Decode(f)
	if err != nil {
		// If we failed, we might need to reset position if it read something
		if seeker, ok := f.(io.Seeker); ok {
			seeker.Seek(0, 0)
		}
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
	bgObj := setupBackground(backgroundPath)

	// Re-do the clock logic properly below to use BigLabel + Binding or Ticker inside.

	// Interactions
	stopAndExit := func(msg string, code int) {
		if audio != nil {
			speaker.Lock()
			audio.ctrl.Paused = true
			speaker.Unlock()
		}
		// Kill ffmpeg handled by deferred Kill in setupBackground logic
		fmt.Println(msg)
		os.Exit(code)
	}

	currentSnooze := snoozeDuration

	// -- Controls --
	snoozeLabel := canvas.NewText(fmt.Sprintf("%s", currentSnooze), color.White)
	snoozeLabel.TextSize = 40
	snoozeLabel.Alignment = fyne.TextAlignCenter

	btnDismiss := newGlassButton("Dismiss", func() {
		stopAndExit("Dismissed", 0)
	})

	btnSnooze := newGlassButton("Snooze", func() {
		stopAndExit("Snoozed "+currentSnooze.String(), 0)
	})

	btnDecrease := newGlassButton("-", func() {
		currentSnooze -= 5 * time.Minute
		if currentSnooze < 5*time.Minute {
			currentSnooze = 5 * time.Minute
		}
		snoozeLabel.Text = fmt.Sprintf("%s", currentSnooze)
		snoozeLabel.Refresh()
	})

	btnIncrease := newGlassButton("+", func() {
		currentSnooze += 5 * time.Minute
		snoozeLabel.Text = fmt.Sprintf("%s", currentSnooze)
		snoozeLabel.Refresh()
	})

	// Layout Containers
	// Clock Container (Glass)
	clockWidget := newBigClock()

	clockPanel := container.NewStack(
		newGlassPanel(),
		container.NewCenter(clockWidget),
	)

	// Refined Controls Layout
	// Row 1: Snooze Controls
	// [ - ] [ 5m ] [ + ] [ Snooze ]
	ctrlSnoozeRow := container.NewHBox(
		btnDecrease,
		container.NewCenter(snoozeLabel),
		btnIncrease,
		btnSnooze,
	)
	centeredSnoozeRow := container.NewCenter(ctrlSnoozeRow)

	// Row 2: Dismiss (Big)
	btnDismiss.Text = "DISMISS" // Uppercase and wider concept via widget? glassButton is auto-width.

	controlsPanel := container.NewStack(
		newGlassPanel(),
		container.NewVBox(
			container.NewPadded(centeredSnoozeRow),
			container.NewPadded(btnDismiss),
		),
	)

	// Padding around panels
	paddedClock := container.NewPadded(clockPanel) // Centered
	paddedControls := container.NewPadded(controlsPanel)

	// Main Layout
	// Clock in Center
	// Controls at Bottom

	content := container.NewBorder(nil, paddedControls, nil, nil, paddedClock)

	finalLayout := container.NewMax(bgObj, content)

	w.SetContent(finalLayout)
	w.ShowAndRun()
}

func setupBackground(path string) fyne.CanvasObject {
	usedPath := path
	if usedPath == "" {
		// Use embedded default
		// We must extract it to a file for ffmpeg to read it efficiently (or stream via stdin? No, we use stdin for output)
		// Actually typical ffmpeg usage needs a file or we stream INTO ffmpeg via stdin.
		// Writing to temp file is safer and easiest for ffmpeg input.
		tmpPath := extractAsset("default_bg.mp4")
		if tmpPath != "" {
			usedPath = tmpPath
			// Note: We are creating a temp file that is not explicitly deleted on exit.
			// OS cleans /tmp usually, or we track it. For this persistent app, it's acceptable.
		} else {
			return canvas.NewRectangle(color.Black)
		}
	}

	// Check extension
	ext := ""
	if len(usedPath) > 4 {
		ext = usedPath[len(usedPath)-4:]
	}
	// Simple check, can be more robust
	if ext == ".mp4" || ext == ".mkv" || usedPath[len(usedPath)-5:] == ".webm" {
		return streamVideo(usedPath)
	}

	img := canvas.NewImageFromFile(usedPath)
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
			// Thread-safe refresh
			fyne.Do(func() {
				raster.Refresh()
			})
		}
	}()

	return raster
}

// extractAsset writes an embedded file to a temporary file and returns its path.
// If the file cannot be written, it returns empty string.
func extractAsset(name string) string {
	data, err := embedFS.ReadFile(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read embedded asset %s: %v\n", name, err)
		return ""
	}

	f, err := os.CreateTemp("", "alarm_app_*_"+name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp file for %s: %v\n", name, err)
		return ""
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write temp file for %s: %v\n", name, err)
		return ""
	}

	return f.Name()
}

// -- Glass UI Helpers --

// newGlassPanel creates a semi-transparent rounded rectangle background.
func newGlassPanel() fyne.CanvasObject {
	// Fyne doesn't natively support rounded rectangles with canvas.Rectangle.
	// We can simulate it or just use a semi-transparent rectangle for now.
	// Or we can draw a raster image.
	// For simplicity and performance, let's use a semi-transparent block.
	// To make it rounded, we might need a custom widget or image.
	// Let's stick to simple rectangle but with "Glass" color.

	// Glass color: Black with alpha 0.5 (128)
	glassColor := color.NRGBA{R: 0, G: 0, B: 0, A: 100}
	rect := canvas.NewRectangle(glassColor)
	rect.CornerRadius = 20 // Fyne v2.5+ supports CornerRadius on Rectangle!
	return rect
}

// makeGlassButton creates a button-like object with glass style.
// Since we want custom style, we wrap a button and modify its theme?
// Or we perform "Custom Widget".
// Easiest: Widget.Button using a custom theme is hard for just one button.
// Better: Container with Tap handler (Clickable).
// But standard Button is robust.
// Can we style standard button? Not easily without theme.
// Let's build a simple custom "GlassButton" widget.

type glassButton struct {
	widget.BaseWidget
	Text     string
	OnTapped func()
}

func newGlassButton(text string, tapped func()) *glassButton {
	b := &glassButton{Text: text, OnTapped: tapped}
	b.ExtendBaseWidget(b)
	return b
}

func (b *glassButton) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.NRGBA{R: 255, G: 255, B: 255, A: 30})
	bg.CornerRadius = 15

	text := canvas.NewText(b.Text, color.White)
	text.Alignment = fyne.TextAlignCenter
	text.TextSize = 24
	// Make text bold?
	text.TextStyle = fyne.TextStyle{Bold: true}

	return &glassButtonRenderer{
		b:       b,
		bg:      bg,
		text:    text,
		objects: []fyne.CanvasObject{bg, text},
	}
}

func (b *glassButton) Tapped(_ *fyne.PointEvent) {
	if b.OnTapped != nil {
		b.OnTapped()
	}
}

type glassButtonRenderer struct {
	b       *glassButton
	bg      *canvas.Rectangle
	text    *canvas.Text
	objects []fyne.CanvasObject
}

func (r *glassButtonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.text.Resize(size)
	// padding for text inside? Text is centered via Alignment.
}

func (r *glassButtonRenderer) MinSize() fyne.Size {
	return r.text.MinSize().Add(fyne.NewSize(40, 20)) // Padding
}

func (r *glassButtonRenderer) Refresh() {
	r.text.Text = r.b.Text
	r.bg.Refresh()
	r.text.Refresh()
}

func (r *glassButtonRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *glassButtonRenderer) Destroy() {}

// -- BigClock Widget --
type bigClock struct {
	widget.BaseWidget
}

func newBigClock() *bigClock {
	c := &bigClock{}
	c.ExtendBaseWidget(c)
	return c
}

func (c *bigClock) CreateRenderer() fyne.WidgetRenderer {
	text := canvas.NewText(time.Now().Format("15:04"), color.White)
	text.TextSize = 150
	text.TextStyle = fyne.TextStyle{Bold: true}
	text.Alignment = fyne.TextAlignCenter

	go func() {
		t := time.NewTicker(1 * time.Second)
		for range t.C {
			// Thread-safe update
			fyne.Do(func() {
				text.Text = time.Now().Format("15:04")
				text.Refresh()
			})
		}
	}()

	return &bigClockRenderer{text: text}
}

type bigClockRenderer struct {
	text *canvas.Text
}

func (r *bigClockRenderer) Layout(size fyne.Size) {
	r.text.Resize(size)
}

func (r *bigClockRenderer) MinSize() fyne.Size {
	return r.text.MinSize()
}

func (r *bigClockRenderer) Refresh() {
	r.text.Refresh()
}

func (r *bigClockRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.text}
}

func (r *bigClockRenderer) Destroy() {}
