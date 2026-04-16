WebRTC / MJPEG Preview Prototype
================================

This document describes the prototype implemented in `internal/preview` and the options for
improving it to a full WebRTC pipeline.

Files added
- `internal/preview/mjpeg_preview.go` — a simple MJPEG streaming handler that uses
  `adb exec-out screencap -p` and streams PNG frames as `multipart/x-mixed-replace`.

Endpoint
- `GET /experiments/android/preview/mjpeg` — requires `x-api-key` header (same as other endpoints).
  Optional query param: `device` to pass an adb serial (e.g., `?device=emulator-5554`).

Usage
- Open in browser (or set `<img src="...">`):

  http://<host>:8000/experiments/android/preview/mjpeg?device=emulator-5554

Notes and next steps
- The MJPEG prototype is intentionally simple and reliable across many environments. It works
  without complex media pipelines and is easy to debug from the browser.
- For lower latency and better bandwidth use, replace this with a WebRTC publisher using Pion:
  - Capture frames with `scrcpy` (preferred) or `adb` + `ffmpeg`.
  - Encode frames into VP8/VP9/H264 and push into a Pion PeerConnection track.
  - Implement HTTP SDP signaling endpoints similar to existing handlers.
- I've kept the code well-commented to indicate where to plug a WebRTC source.

Security
- The preview endpoint is protected by the same API-key wrapper used across the server. Adjust
  permissions as needed (e.g., allow only `engineer` and `admin`).

Troubleshooting
- If you see `ERROR: ...` in the stream, check that `adb devices` lists the emulator/device
  on the host and that `adb exec-out screencap -p` works from the host shell.
