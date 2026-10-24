# Watch Party

Stream an MP4 to a group and watch it together: the admin uploads, the video is
transcoded to HLS, and every viewer stays in sync over a WebSocket.

Built from [`context/watch-party-simple-prd.md`](context/watch-party-simple-prd.md).

## Requirements

- Go 1.27+
- Node 20+
- `ffmpeg` on `PATH` (the transcoder shells out to it)
- `cloudflared` on `PATH` (only needed for the tunnel, see the launcher)

## Run it

### All-in-one (backend + frontend + public tunnel)

```bash
cd backend
go run ./cmd/launcher
```

The launcher builds the frontend, starts the API (which serves the built
bundle) and opens a free Cloudflare quick tunnel. The tunnel URL is handed to
the API as `PUBLIC_BASE_URL`, so every invite link points at the public address
(`https://<random>.trycloudflare.com/watch?invite=...`) instead of localhost.

It opens the admin UI in your browser and then streams the logs of every
service to the console, prefixed by service:

```
[tunnel] 2026-10-22T03:24:07Z INF |  https://acne-shape.trycloudflare.com   |
[backend] 2026/10/22 10:24:08 watchparty listening on 0.0.0.0:3001
```

`Ctrl+C` stops the API and the tunnel and closes the window. The same logs are
written to `.launcher/{tunnel,backend,build}.log` (fresh per run).

### Access from other devices on the same network

The API binds to `0.0.0.0:3001` and the launcher prints the LAN address:

```
LAN:  http://192.168.1.6:3001
```

Open that address on a phone or another computer on the same Wi-Fi to use the
admin UI or to open an invite link. Invite links created from the LAN address
keep that address, so a viewer on the same network can open them; links created
from localhost still point at the tunnel. The admin and login pages load from
this machine, from private IPv4 addresses and from `*.local` hostnames only, so
a tunnel still exposes just the viewer page.

### Manual

**Backend** (port 3001):

```bash
cd backend
cp .env.example .env          # then set JWT_SECRET (>= 32 chars, e.g. openssl rand -base64 48)
go run ./cmd/server
```

**Frontend** (port 5173, proxies `/api`, `/hls` and `/ws` to the backend):

```bash
cd frontend
npm install
npm run dev
```

Open http://localhost:5173, sign in with the seeded admin account (`admin@mail.com` / `admin123`
from `.env`), upload an MP4, click "Generate invite link", and share the `/watch?invite=...` link with a viewer.
**Viewers do not need an account.** Each invite link is multi-use, stays valid
for 30 days and is redeemed for a session-scoped viewer token in each viewer's
browser. The link is stored per session, so it survives a page reload and can
be shared with as many people as needed.

The admin and login pages only load from this machine or from the local
network (private IPv4, `*.local`); anything else is sent to the "Access
Denied" page, so a tunnel only exposes the viewer page.

Default admin credentials: `admin@mail.com`, password `admin123` (configured via `.env`).

## How it works

- **Upload** → the file is stored under `media/source/`, then `ffmpeg` transcodes it
  to HLS (H.264 1200k, AAC 128k, 2s segments) into `media/hls/<session>/`.
  The session turns `ready` when the playlist is written.
- **Playback** → the browser plays `master.m3u8` through hls.js. The playlist and
  segments are served by the backend and require the JWT, so hls.js sends it via
  `xhrSetup`.
- **Sync** → the server keeps `position` + `updated_at` per session. A client
  computes the expected position as `position + (now - updated_at)` and re-seeks
  when it drifts more than 0.5s (PRD 8.5). Only the session owner may send
  `play` / `pause` / `seek`; viewers get `error: forbidden`.
- The streamer reports its playhead only after it starts moving, so a cold media
  pipeline cannot pull the server timeline ahead of the real playhead.
- A reload keeps the playback state: the server timeline is restored, and the
  browser's autoplay block (no user gesture after a refresh) is worked around
  by starting muted and unmuting on the first click.

## Keyboard shortcuts

On the admin page:

| Key            | Action                        |
| -------------- | ----------------------------- |
| `Space`        | play / pause                  |
| `F`            | fullscreen / exit fullscreen  |
| `M`            | mute / unmute                 |
| `←` / `→`      | skip 10 seconds back / forward |
| double click   | fullscreen / exit fullscreen  |

Viewers get `F`, `M` and the double click: they do not drive the playback, the
streamer does.

## Layout

```
backend/
  cmd/server/           entrypoint
  internal/config/      .env loading and validation
  internal/domain/      models and validation
  internal/repository/  SQLite (users, sessions) + embedded schema
  internal/service/     auth, sessions, ffmpeg transcoder
  internal/transport/   HTTP routes, HLS files, WebSocket hub
frontend/
  src/pages/            Login, Streamer (admin), Watch (viewer)
  src/hooks/            useWebSocket (sync), useHLS (hls.js)
  src/components/       Player, UploadForm, badges, shadcn/ui
```

## Not included

- Refresh tokens: the PRD mentions them in the NFR table but defines no
  endpoint, model or client flow, so the app issues a single 24h JWT.
- Docker: optional in the PRD, not set up here.
- One HLS rendition only (no adaptive bitrate ladder).
- The server timeline is not clamped to the video duration; a session that is
  left playing past the end keeps counting.
