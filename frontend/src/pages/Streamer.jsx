import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Pause,
  Play,
  Copy,
  Check,
  Trash2,
  Share2,
  Users,
  Pencil,
  Loader2,
  RotateCcw,
  RotateCw,
  Volume2,
  VolumeX,
  Maximize,
  Minimize,
} from 'lucide-react'
import { api, auth } from '@/lib/api'
import { useWebSocket } from '@/hooks/useWebSocket'
import { useAutoplay } from '@/hooks/useAutoplay'
import { usePlayerChrome } from '@/hooks/usePlayerChrome'
import { useFullscreen } from '@/hooks/useFullscreen'
import { Player } from '@/components/Player'
import { UploadForm } from '@/components/UploadForm'
import { StatusBadge, ConnectionSpeed } from '@/components/StatusBadge'
import { UserMenu } from '@/components/UserMenu'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Slider } from '@/components/ui/slider'
import { Progress } from '@/components/ui/progress'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

const PENDING = ['uploading', 'processing']

export function StreamerPage() {
  const [sessions, setSessions] = useState([])
  const [activeId, setActiveId] = useState(null)
  const [active, setActive] = useState(null)
  const [error, setError] = useState('')
  const [inviteUrl, setInviteUrl] = useState('')
  const [copied, setCopied] = useState(false)
  const [generating, setGenerating] = useState(false)
  const [pendingDelete, setPendingDelete] = useState(null)
  const [editing, setEditing] = useState(null)
  const [editTitle, setEditTitle] = useState('')
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [mediaReady, setMediaReady] = useState(false)
  const [volume, setVolume] = useState(1)
  const [muted, setMuted] = useState(false)
  const [restored, setRestored] = useState(false)

  const videoRef = useRef(null)
  const playerBoxRef = useRef(null)
  const appliedRef = useRef(null)
  const scrubbingRef = useRef(false)
  const navigate = useNavigate()
  const userId = auth.getUser()?.user_id

  const loadSessions = useCallback(async () => {
    const { sessions: list } = await api.sessions()
    setSessions(list)
    return list
  }, [])

  useEffect(() => {
    loadSessions().catch((e) => setError(e.message))
  }, [loadSessions])

  // Reopen the session that was active before the reload, once the list is in.
  useEffect(() => {
    if (restored || !sessions.length) return
    const stored = auth.getActiveSession(userId)
    if (stored && sessions.some((s) => s.session_id === stored)) setActiveId(stored)
    else if (stored) auth.clearActiveSession(userId)
    setRestored(true)
  }, [sessions, restored, userId])

  // Remember the active session so a refresh keeps playing it.
  useEffect(() => {
    if (!restored) return
    if (activeId) auth.setActiveSession(userId, activeId)
    else auth.clearActiveSession(userId)
  }, [activeId, restored, userId])

  // Keep polling while a session is still being transcoded.
  useEffect(() => {
    if (!sessions.some((s) => PENDING.includes(s.video_status))) return
    const timer = setInterval(() => loadSessions().catch(() => {}), 2000)
    return () => clearInterval(timer)
  }, [sessions, loadSessions])

  // The session details refresh when the status of the active session changes
  // (uploading -> ready). Depending on the whole list would re-fetch every
  // second and reset the player.
  const activeStatus = sessions.find((s) => s.session_id === activeId)?.video_status

  useEffect(() => {
    if (!activeId) {
      setActive(null)
      return
    }
    let cancelled = false
    api
      .session(activeId)
      .then((s) => {
        if (!cancelled) setActive(s)
      })
      .catch((e) => setError(e.message))
    return () => {
      cancelled = true
    }
  }, [activeId, activeStatus])

  // A new session starts from scratch: the player has to load it again.
  useEffect(() => {
    setMediaReady(false)
  }, [activeId])

  // Restore the invite link after a reload: the backend keeps it per session.
  useEffect(() => {
    if (!activeId) {
      setInviteUrl('')
      return
    }
    let cancelled = false
    api
      .currentInviteLink(activeId)
      .then(({ url, token }) => {
        if (cancelled) return
        setInviteUrl(url || `${window.location.origin}/watch?invite=${token}`)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [activeId])

  const ready = active?.video_status === 'ready'
  const { playing, position, status: wsStatus, sendCommand, expectedPosition } = useWebSocket(
    activeId,
    'admin',
  )
  const { play: autoplay, autoMuted } = useAutoplay(videoRef)
  const silent = muted || autoMuted

  // Adopt the server state: covers reloads and reconnects without fighting local control.
  useEffect(() => {
    const video = videoRef.current
    if (!video || !ready || !mediaReady) return
    const key = `${playing}:${position}`
    if (appliedRef.current === key) return
    appliedRef.current = key
    // Resume on the server timeline (position + elapsed), so a refresh lands on
    // the exact timestamp instead of the anchor of the last play command.
    const target = expectedPosition()
    if (Math.abs(video.currentTime - target) > 0.5) video.currentTime = target
    if (playing) autoplay()
    else video.pause()
  }, [playing, position, ready, mediaReady, expectedPosition, autoplay])

  // Volume and mute live on the media element itself.
  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    video.volume = volume
    video.muted = silent
  }, [volume, silent, ready])

  // Overlay chrome hides after 5s of inactivity and returns on mouse movement
  // inside the player box only.
  const controlsVisible = usePlayerChrome(playerBoxRef, ready)

  const { isFullscreen, toggleFullscreen } = useFullscreen(playerBoxRef, videoRef, ready)

  const handleUpload = async (file, onProgress) => {
    setError('')
    const result = await api.upload(file, onProgress)
    await loadSessions()
    setActiveId(result.session_id)
  }

  const handlePlay = async () => {
    const video = videoRef.current
    if (!video) return
    await autoplay()
    // Report the playhead only once it really moves: a cold media pipeline
    // starts its clock hundreds of ms after play() resolves, and a stale anchor
    // would leave every viewer ahead of the streamer (PRD 8.5).
    const from = video.currentTime
    const deadline = Date.now() + 2000
    while (video.currentTime === from && Date.now() < deadline) {
      await new Promise((r) => setTimeout(r, 30))
    }
    sendCommand('play', video.currentTime)
  }

  const handlePause = () => {
    const video = videoRef.current
    if (!video) return
    video.pause()
    sendCommand('pause', video.currentTime)
  }

  const handleTogglePlay = () => {
    if (playing) handlePause()
    else handlePlay()
  }

  const handleSkip = (delta) => {
    const video = videoRef.current
    if (!video) return
    const max = duration || video.duration || 0
    const target = Math.min(Math.max(video.currentTime + delta, 0), max || Infinity)
    video.currentTime = target
    sendCommand('seek', target)
  }

  const toggleMute = () => {
    setMuted((m) => {
      if (m) setVolume(1)
      return !m
    })
  }

  const handleSeek = (value) => {
    const video = videoRef.current
    if (!video) return
    video.currentTime = value[0]
    sendCommand('seek', value[0])
  }

  const handleDelete = async (id) => {
    try {
      await api.deleteSession(id)
      if (activeId === id) setActiveId(null)
      await loadSessions()
    } catch (e) {
      setError(e.message)
    }
  }

  const handleRename = async () => {
    if (!editing) return
    const title = editTitle.trim()
    if (!title) return
    try {
      await api.updateSessionTitle(editing.session_id, title)
      setEditing(null)
      await loadSessions()
      if (activeId === editing.session_id) {
        setActive((current) => (current ? { ...current, video_title: title } : current))
      }
    } catch (e) {
      setError(e.message)
    }
  }

  const handleGenerateInvite = async () => {
    if (!activeId) return
    setGenerating(true)
    setError('')
    try {
      // The backend builds the link with the public origin (tunnel) when set.
      const { token, url } = await api.createInviteLink(activeId)
      setInviteUrl(url || `${window.location.origin}/watch?invite=${token}`)
    } catch (e) {
      setError(e.message)
    } finally {
      setGenerating(false)
    }
  }

  const copyInvite = async () => {
    if (!inviteUrl) return
    await navigator.clipboard.writeText(inviteUrl)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const logout = async () => {
    auth.clearActiveSession(userId)
    await api.logout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="mx-auto flex min-h-screen max-w-none flex-col gap-6 px-0 py-4 sm:px-4 lg:px-6 lg:h-screen lg:overflow-hidden">
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
          {activeId && <ConnectionSpeed status={wsStatus} />}
          <UserMenu onLogout={logout} />
        </div>
      </header>

      {error && (
        <Alert variant="destructive" className="shrink-0">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <div className="grid min-h-0 min-w-0 flex-1 grid-rows-[auto] gap-4 lg:grid-rows-[minmax(0,1fr)] lg:grid-cols-[2.81fr_1fr]">
        <div className="flex min-h-0 min-w-0 flex-col">
          <Card className="relative flex min-h-0 flex-col overflow-hidden lg:flex-1">
            <CardContent className="flex min-h-0 flex-col gap-4 overflow-y-auto px-2 sm:px-(--card-spacing) lg:flex-1">
              {!active && (
                <div className="flex flex-1 flex-col items-center justify-center gap-4 text-muted-foreground">
                  <div className="rounded-full bg-muted p-4">
                    <Play className="h-8 w-8" />
                  </div>
                  <p className="text-sm">Select or upload a session</p>
                </div>
              )}

              {active && !ready && (
                <div className="flex flex-1 flex-col items-center justify-center gap-5">
                  {active.video_status === 'failed' ? (
                    <p className="text-sm text-destructive">
                      {active.error || 'Transcoding failed'}
                    </p>
                  ) : (
                    <>
                      <div className="rounded-full bg-muted p-4">
                        <Loader2 className="h-10 w-10 animate-spin text-muted-foreground" />
                      </div>
                      <Progress
                        className="w-full max-w-sm [&_[data-slot=progress-indicator]]:animate-[progress-indeterminate_1.6s_ease-in-out_infinite]"
                      />
                      <p className="text-sm text-muted-foreground">
                        Transcoding to HLS, this page updates automatically.
                      </p>
                    </>
                  )}
                </div>
              )}

              {ready && (
                <Player
                  hlsUrl={active.hls_url}
                  videoRef={videoRef}
                  containerRef={playerBoxRef}
                  playing={playing}
                  onTogglePlay={handleTogglePlay}
                  onSkip={handleSkip}
                  onToggleMute={toggleMute}
                  onToggleFullscreen={toggleFullscreen}
                  overlay={
                    <div
                      className="absolute inset-0 transition-opacity duration-300 [&_[data-slot=slider-track]]:bg-white/25 [&_[data-slot=slider-range]]:bg-white"
                      style={{
                        opacity: controlsVisible ? 1 : 0,
                        pointerEvents: controlsVisible ? 'auto' : 'none',
                      }}
                    >
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button
                            variant="secondary"
                            size="icon"
                            aria-label="Viewer invite link"
                            data-player-chrome
                            className="absolute right-3 top-3 z-10"
                            onClick={(e) => e.stopPropagation()}
                          >
                            <Share2 className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent
                          align="end"
                          className="w-80"
                          data-player-chrome
                          // The menu renders in a portal: React still bubbles its
                          // events up the tree, so stop them before they reach
                          // the player box and toggle playback.
                          onClick={(e) => e.stopPropagation()}
                        >
                          <DropdownMenuLabel className="font-normal">
                            <p className="text-sm font-medium">Viewer invite link</p>
                            <p className="text-xs text-muted-foreground">
                              Anyone with this link can watch, it does not expire
                            </p>
                          </DropdownMenuLabel>
                          <DropdownMenuSeparator />

                          {inviteUrl && (
                            <div className="flex items-center gap-1 px-1.5 pb-1">
                              <Input
                                id="invite-url"
                                name="inviteUrl"
                                readOnly
                                value={inviteUrl}
                                className="font-mono text-xs"
                              />
                              <Button
                                variant="outline"
                                size="icon"
                                aria-label="Copy invite link"
                                className="shrink-0"
                                onClick={copyInvite}
                              >
                                {copied ? (
                                  <Check className="h-4 w-4" />
                                ) : (
                                  <Copy className="h-4 w-4" />
                                )}
                              </Button>
                            </div>
                          )}

                          <div className="px-1.5 pb-1">
                            <Button
                              variant="outline"
                              size="sm"
                              className="w-fit"
                              disabled={generating}
                              onClick={handleGenerateInvite}
                            >
                              <Users className="mr-2 h-4 w-4" />
                              {inviteUrl ? 'New invite link' : 'Generate invite link'}
                            </Button>
                          </div>
                        </DropdownMenuContent>
                      </DropdownMenu>

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
                            onValueChange={(v) => {
                              scrubbingRef.current = true
                              setCurrentTime(v[0])
                            }}
                            onValueCommit={(v) => {
                              scrubbingRef.current = false
                              handleSeek(v)
                            }}
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
                            onClick={handleTogglePlay}
                            aria-label={playing ? 'Pause' : 'Play'}
                            className="text-white hover:bg-white/20 hover:text-white"
                          >
                            {playing ? (
                              <Pause className="h-5 w-5" />
                            ) : (
                              <Play className="h-5 w-5" />
                            )}
                          </Button>

                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => handleSkip(-10)}
                            aria-label="Back 10 seconds"
                            className="text-white hover:bg-white/20 hover:text-white"
                          >
                            <span className="relative">
                              <RotateCcw className="h-5 w-5" />
                              <span className="absolute inset-0 flex items-center justify-center text-[9px] font-bold">
                                10
                              </span>
                            </span>
                          </Button>

                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => handleSkip(10)}
                            aria-label="Forward 10 seconds"
                            className="text-white hover:bg-white/20 hover:text-white"
                          >
                            <span className="relative">
                              <RotateCw className="h-5 w-5" />
                              <span className="absolute inset-0 flex items-center justify-center text-[9px] font-bold">
                                10
                              </span>
                            </span>
                          </Button>

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
                  onManifestParsed={() => {
                    const video = videoRef.current
                    if (!video) return
                    setDuration(video.duration || 0)
                    if (playing) autoplay()
                  }}
                  onLoadedMetadata={(e) => {
                    setDuration(e.currentTarget.duration || 0)
                    setMediaReady(true)
                  }}
                  onTimeUpdate={(e) => {
                    if (scrubbingRef.current) return
                    setCurrentTime(e.currentTarget.currentTime)
                  }}
                  onError={(e) => setError(e.message)}
                />
              )}
            </CardContent>
          </Card>
        </div>
        <div className="flex min-h-0 min-w-0 flex-col">
          <Card className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
            <Tabs
              defaultValue="session"
              className="flex min-h-0 min-w-0 flex-1 flex-col gap-4"
            >
              <CardHeader>
                <TabsList className="w-full">
                  <TabsTrigger value="upload">Upload</TabsTrigger>
                  <TabsTrigger value="session">Session</TabsTrigger>
                </TabsList>
              </CardHeader>
              <CardContent className="flex min-h-0 min-w-0 flex-1 flex-col">
                <TabsContent value="upload" className="mt-0 min-w-0">
                  <UploadForm onUpload={handleUpload} />
                </TabsContent>
                <TabsContent
                  value="session"
                  className="mt-0 min-h-0 min-w-0 flex-1 space-y-2 overflow-x-hidden overflow-y-auto"
                >
                  {sessions.length === 0 && (
                    <p className="text-sm text-muted-foreground">No sessions yet.</p>
                  )}
                  {sessions.map((s) => (
                    <div
                      key={s.session_id}
                      className={`flex min-w-0 items-center gap-2 rounded-md border p-2 ${
                        activeId === s.session_id ? 'border-primary bg-primary/5' : ''
                      }`}
                    >
                      <button
                        type="button"
                        className="min-w-0 flex-1 text-left"
                        onClick={() => setActiveId(s.session_id)}
                      >
                        <p className="break-words text-sm font-medium">
                          {s.video_title || s.session_id}
                        </p>
                      </button>
                      {s.video_status === 'ready' ? (
                        <span className="shrink-0 text-xs tabular-nums text-muted-foreground">
                          {formatDuration(s.duration_seconds)}
                        </span>
                      ) : (
                        <StatusBadge status={s.video_status} />
                      )}
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="Rename session"
                        className="shrink-0"
                        onClick={() => {
                          // Seed the field with this session's name, otherwise the
                          // dialog keeps the title of the previously edited one.
                          setEditTitle(s.video_title || '')
                          setEditing(s)
                        }}
                      >
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="Delete session"
                        className="shrink-0"
                        onClick={() => setPendingDelete(s)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  ))}
                </TabsContent>
              </CardContent>
            </Tabs>
          </Card>
        </div>
      </div>

      <Dialog
        open={Boolean(editing)}
        onOpenChange={(open) => {
          if (!open) setEditing(null)
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Rename session</DialogTitle>
            <DialogDescription>Give this film a name your viewers will recognise.</DialogDescription>
          </DialogHeader>
          <Input
            id="session-title"
            name="sessionTitle"
            value={editTitle}
            maxLength={200}
            onChange={(e) => setEditTitle(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleRename()
            }}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditing(null)}>
              Cancel
            </Button>
            <Button onClick={handleRename} disabled={!editTitle.trim()}>
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={Boolean(pendingDelete)}
        onOpenChange={(open) => !open && setPendingDelete(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete this session?</AlertDialogTitle>
            <AlertDialogDescription>
              The video and its invite links are removed permanently.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={() => {
                handleDelete(pendingDelete.session_id)
                setPendingDelete(null)
              }}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function formatTime(seconds) {
  if (!Number.isFinite(seconds)) return '0:00'
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}

function formatDuration(seconds) {
  if (!seconds) return '—'
  const total = Math.round(seconds)
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  return h > 0
    ? `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
    : `${m}:${String(s).padStart(2, '0')}`
}
