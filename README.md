# Go Alarm App

A high-performance, CLI-driven, full-screen alarm application for Linux. Built with Go and Fyne, featuring a modern glassmorphism UI, video background support, and embedded assets.

## Features

-   **Full-Screen UI**: Immersive alarm experience.
-   **Video Backgrounds**: Supports looping MP4, MKV, and WEBM video backgrounds via `ffmpeg`.
-   **Glassmorphism**: Stylish semi-transparent UI with a large digital clock.
-   **Embedded Assets**: Ships with a default cyberpunk-style video background and alarm ringtone.
-   **CLI Control**: detailed flags for customization (snooze duration, specific files, timeout).
-   **Output Integration**: Prints user actions (`Dismissed`, `Snoozed 10m`) to stdout for integration with scripts/cron.

## Requirements

-   **Linux** (Tested on Manjaro)
-   **Go** 1.20+
-   **FFmpeg** (Required for video background decoding)
-   **Fyne Dependencies**:
    -   `libgl1-mesa-dev`, `xorg-dev` (Debian/Ubuntu)
    -   `libgl`, `libxcursor`, `libxrandr`, `libxinerama`, `libxi` (Arch/Manjaro)

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/alarm-app.git
cd alarm-app

# Run directly
go run main.go

# Build binary
go build -o alarm main.go
```

## Usage

```bash
./alarm [flags]
```

### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--ringtone` | `-r` | Path to custom audio file (mp3/wav) | Embedded Default |
| `--background` | `-b` | Path to background image or video | Embedded Default Video |
| `--snooze` | `-s` | Default snooze duration | `5m` |
| `--timeout` | `-t` | Auto-exit timeout duration | `1m` |
| `--help` | `-h` | Show help message | |

### Examples

**Run with defaults (Embedded video & tone):**
```bash
./alarm
```

**Custom video and ringtone:**
```bash
./alarm -b ~/Videos/loops/rain.mp4 -r ~/Music/alarm.mp3
```

**Set custom snooze and timeout:**
```bash
./alarm --snooze 10m --timeout 30s
```

## Output

The application prints the exit reason to `stdout`, making it easy to chain with other commands:

-   `Dismissed`: User clicked Dismiss.
-   `Snoozed <duration>`: User clicked Snooze (e.g., `Snoozed 15m0s`).
-   `Timeout`: App closed automatically due to inactivity.
-   `canceled`: User closed window or sent signal.

## License

MIT License. See [LICENSE](LICENSE) for details.
