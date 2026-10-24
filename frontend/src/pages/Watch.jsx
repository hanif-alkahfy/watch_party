import { useEffect, useRef, useState } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { Maximize, Minimize, Volume2, VolumeX } from 'lucide-react'
import { api, auth } from '@/lib/api'
import { useWebSocket } from '@/hooks/useWebSocket'
import { useAutoplay } from '@/hooks/useAutoplay'
import { usePlayerChrome } from '@/hooks/usePlayerChrome'
import { useFullscreen } from '@/hooks/useFullscreen'
import { Player } from '@/components/Player'
import { ConnectionSpeed } from '@/components/StatusBadge'
import { UserMenu } from '@/components/UserMenu'
import { Button } from '@/components/ui/button'
import { Slider } from '@/components/ui/slider'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

const DRIFT_TOLERANCE = 0.5 // seconds, PRD F-SYNC-03
const PENDING = ['uploading', 'processing']

export function WatchPage() {
  const [params] = useSearchParams()
  const inviteToken = params.get('invite')
  const sessionId = params.get('session')

  const [token, setToken] = useState(null)
  const [resolvedSessionId, setResolvedSessionId] = useState(sessionId)
  const [session, setSession] = useState(null)
  const [error, setError] = useState('')
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [volume, setVolume] = useState(1)
  const [muted, setMuted] = useState(false)

  const videoRef = useRef(null)
  const playerBoxRef = useRef(null)
  const redeemingRef = useRef(null)
  const navigate = useNavigate()

  // Redeem an invite link if present, or load the saved viewer token.
  // In React development mode StrictMode runs effects twice; we guard with a ref
  // so the link is only sent to the server once.
  useEffect(() => {
    if (inviteToken) {
      if (redeemingRef.current === inviteToken) return
      redeemingRef.current = inviteToken

      api
        .exchangeInvite(inviteToken)
        .then(({ token: vt, session_id: sid }) => {
          auth.setViewerToken(sid, vt)
          setToken(vt)
          setResolvedSessionId(sid)
          // The link stays in the address bar: it is multi-use, so it can be
          // re-opened or shared again.
        })
        .catch((e) => {
          setError(e.message)
        })
      return
    }

    if (!sessionId) {
      setError('Missing session id or invite link')
      return
    }

    setResolvedSessionId(sessionId)
    const stored = auth.getViewerToken(sessionId)
    const activeToken = stored || auth.getToken()
    if (!activeToken) {
      setError('This invite link is invalid or has expired.')
      return
    }
    setToken(activeToken)
  }, [inviteToken, sessionId])

  // Fetch session details once authenticated.
  useEffect(() => {
    if (!resolvedSessionId || !token) return
    let cancelled = false
    api
      .session(resolvedSessionId, token)
      .then((s) => {
        if (!cancelled) setSession(s)
      })
      .catch((e) => {
        if (!cancelled) setError(e.message)
      })
    return () => {
      cancelled = true
    }
  }, [resolvedSessionId, token])

  // Poll while the streamer is still transcoding.
  useEffect(() => {
    if (!session || !PENDING.includes(session.video_status) || !token) return
    const timer = setInterval(() => {
      api
        .session(session.session_id, token)
        .then(setSession)
        .catch(() => {})
    }, 2000)
    return () => clearInterval(timer)
  }, [session, token])

  const ready = session?.video_status === 'ready'
  const { playing, position, status: wsStatus, expectedPosition } = useWebSocket(
    resolvedSessionId,
    'viewer',
    token,
  )
  const { play: autoplay, autoMuted } = useAutoplay(videoRef)
  const silent = muted || autoMuted

  // Apply the streamer's play/pause/seek. The target is the server's timeline
  // (position + elapsed), not the raw position, so a viewer joining a running
  // session lands where the session actually is.
  useEffect(() => {
    const video = videoRef.current
    if (!video || !ready) return
    if (playing) {
      const target = expectedPosition()
      if (Math.abs(video.currentTime - target) > 0.3) video.currentTime = target
      autoplay()
    } else {
      video.pause()
    }
  }, [playing, position, ready, expectedPosition, autoplay])

  // Resume once the browser has buffered enough.
  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    const onCanPlay = () => {
      if (playing) autoplay()
    }
    video.addEventListener('canplay', onCanPlay)
    return () => video.removeEventListener('canplay', onCanPlay)
  }, [playing, ready, autoplay])

  // Drift correction: resync when we fall more than 0.5s behind the streamer.
  useEffect(() => {
    if (!ready) return
    const timer = setInterval(() => {
      const video = videoRef.current
      if (!video || video.readyState < 3) return
      const expected = expectedPosition()
      if (Math.abs(video.currentTime - expected) > DRIFT_TOLERANCE) {
        video.currentTime = expected
      }
    }, 1000)
    return () => clearInterval(timer)
  }, [ready, expectedPosition])

  const logout = async () => {
    await api.logout()
    navigate('/login', { replace: true })
  }

  // Volume and mute live on the media element itself.
  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    video.volume = volume
    video.muted = silent
  }, [volume, silent, ready])

  const { isFullscreen, toggleFullscreen } = useFullscreen(playerBoxRef, videoRef, ready)

  const toggleMute = () => {
    setMuted((m) => {
      if (m) setVolume(1)
      return !m
    })
  }

  const controlsVisible = usePlayerChrome(playerBoxRef, ready)

  return (
    <div className="mx-auto flex min-h-screen max-w-6xl flex-col gap-6 px-0 py-4 sm:px-4 lg:px-6 lg:h-screen lg:overflow-hidden">
      <header className="flex shrink-0 items-center justify-between px-3 sm:px-0">
        <div className="flex min-w-0 items-center gap-2">
          <img
            src="/logo.svg"
            alt=""
            aria-hidden="true"
            className="h-7 w-7 shrink-0 sm:h-8 sm:w-8"
          />
          <h1 className="truncate text-xl font-semibold">Watch Party</h1>
        </div>
        <div className="flex items-center gap-3">
          {resolvedSessionId && <ConnectionSpeed status={wsStatus} />}
          <UserMenu onLogout={logout} />
        </div>
      </header>

      {error && (
        <Alert variant="destructive" className="shrink-0">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <Card className="flex min-h-0 flex-col overflow-hidden lg:flex-1">
        <CardHeader>
          <div className="flex items-center gap-2">
            <CardTitle>Now playing</CardTitle>
            {session?.video_title && (
              <span className="truncate text-sm text-muted-foreground">
                {session.video_title}
              </span>
            )}
          </div>
        </CardHeader>
        <CardContent className="flex min-h-0 flex-col gap-4 px-2 sm:px-(--card-spacing) lg:flex-1">
          {session && !ready && (
            <p className="text-sm text-muted-foreground">
              {session.video_status === 'failed'
                ? session.error || 'This video is unavailable'
                : 'The streamer is still preparing this video.'}
            </p>
          )}

          {ready && (
            <>
              <Player
                hlsUrl={session.hls_url}
                  videoRef={videoRef}
                  containerRef={playerBoxRef}
                  token={token}
                  playing={playing}
                  onToggleMute={toggleMute}
                  onToggleFullscreen={toggleFullscreen}
                  onTimeUpdate={(e) => setCurrentTime(e.currentTarget.currentTime)}
                  onLoadedMetadata={(e) => setDuration(e.currentTarget.duration || 0)}
                  onError={(e) => setError(e.message)}
                  overlay={
                    <div
                      className="absolute inset-0 transition-opacity duration-300 [&_[data-slot=slider-track]]:bg-white/25 [&_[data-slot=slider-range]]:bg-white"
                      style={{
                        opacity: controlsVisible ? 1 : 0,
                        pointerEvents: controlsVisible ? 'auto' : 'none',
                      }}
                    >
                      <div
                        data-player-chrome
                        className="absolute inset-x-0 bottom-0 flex flex-col gap-2 bg-gradient-to-t from-black/85 via-black/35 to-transparent p-3"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <div className="flex items-center gap-2">
                          <Slider
                            value={[currentTime]}
                            max={duration || 100}
                            step={0.1}
                            disabled
                            aria-label="Playback position"
                            className="flex-1"
                          />
                          <span className="text-xs tabular-nums text-white/90">
                            {formatTime(currentTime)} / {formatTime(duration)}
                          </span>
                        </div>

                        <div className="flex items-center gap-1">
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={toggleMute}
                            aria-label={silent ? 'Unmute' : 'Mute'}
                            className="text-white hover:bg-white/20 hover:text-white"
                          >
                            {silent || volume === 0 ? (
                              <VolumeX className="h-5 w-5" />
                            ) : (
                              <Volume2 className="h-5 w-5" />
                            )}
                          </Button>

                          <Slider
                            value={[silent ? 0 : volume]}
                            max={1}
                            step={0.05}
                            onValueChange={(v) => {
                              setVolume(v[0])
                              setMuted(v[0] === 0)
                            }}
                            aria-label="Volume"
                            className="w-20"
                          />

                          <div className="flex-1" />

                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={toggleFullscreen}
                            aria-label={isFullscreen ? 'Exit fullscreen' : 'Fullscreen'}
                            className="text-white hover:bg-white/20 hover:text-white"
                          >
                            {isFullscreen ? (
                              <Minimize className="h-5 w-5" />
                            ) : (
                              <Maximize className="h-5 w-5" />
                            )}
                          </Button>
                        </div>
                      </div>
                    </div>
                  }
                />
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function formatTime(seconds) {
  if (!Number.isFinite(seconds)) return '0:00'
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}
