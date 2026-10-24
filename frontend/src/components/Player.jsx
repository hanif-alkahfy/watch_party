import { useEffect, useRef, useState } from 'react'
import { Pause, Play } from 'lucide-react'
import { cn } from 'cn'
import { useHLS } from '@/hooks/useHLS'

function ratioValue(ratio) {
  const [w, h] = ratio.split('/').map((n) => parseFloat(n))
  return w && h ? w / h : 16 / 9
}

// useMediaQuery tells the player whether the height is a real constraint
// (desktop: the card is pinned to the viewport) or the width is (mobile).
function useMediaQuery(query) {
  const [matches, setMatches] = useState(false)
  useEffect(() => {
    const mql = window.matchMedia(query)
    const onChange = () => setMatches(mql.matches)
    onChange()
    mql.addEventListener('change', onChange)
    return () => mql.removeEventListener('change', onChange)
  }, [query])
  return matches
}

export function Player({
  hlsUrl,
  videoRef,
  containerRef,
  token,
  overlay,
  notice,
  playing,
  onTogglePlay,
  onSkip,
  onToggleMute,
  onToggleFullscreen,
  onManifestParsed,
  onError,
  onTimeUpdate,
  onLoadedMetadata,
  muted = false,
}) {
  const fallbackRef = useRef(null)
  const ref = videoRef || fallbackRef
  const [ratio, setRatio] = useState('16 / 9')
  const [flash, setFlash] = useState(0)
  const areaRef = useRef(null)
  const [area, setArea] = useState(null)

  const hlsRef = useHLS(ref, hlsUrl, { onManifestParsed, onError, token })

  // Measure the space the player gets, so the box can be sized to the film's
  // native aspect ratio and still fit that space.
  useEffect(() => {
    const area = areaRef.current
    if (!area) return
    const measure = () => {
      const rect = area.getBoundingClientRect()
      setArea({ w: rect.width, h: rect.height })
    }
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(area)
    return () => observer.disconnect()
  }, [])

  // Clicking the picture toggles playback and flashes a large centre icon.
  // Clicks on the overlay chrome (controls, share menu) are ignored.
  // A double click opens the fullscreen instead, so the single click waits
  // briefly to see whether a second one follows.
  const clickTimer = useRef(null)

  const handleClick = (e) => {
    if (e.target instanceof Element && e.target.closest('[data-player-chrome]')) return
    if (!onTogglePlay) return
    if (clickTimer.current) clearTimeout(clickTimer.current)
    clickTimer.current = setTimeout(() => {
      clickTimer.current = null
      onTogglePlay()
      setFlash((n) => n + 1)
    }, 220)
  }

  const handleDoubleClick = (e) => {
    if (e.target instanceof Element && e.target.closest('[data-player-chrome]')) return
    if (clickTimer.current) {
      clearTimeout(clickTimer.current)
      clickTimer.current = null
    }
    onToggleFullscreen?.()
  }

  useEffect(() => () => clearTimeout(clickTimer.current), [])

  // Keyboard shortcuts: F fullscreen, Space play/pause, M mute, arrows skip.
  // They are ignored while a text field has the focus.
  useEffect(() => {
    const onKey = (e) => {
      if (e.metaKey || e.ctrlKey || e.altKey) return
      const target = e.target
      if (target instanceof HTMLElement) {
        if (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)) {
          return
        }
        // Space on a focused button is the button's own activation, so the
        // shortcut would toggle twice.
        if (e.key === ' ' && ['BUTTON', 'A'].includes(target.tagName)) return
      }
      switch (e.key) {
        case 'f':
        case 'F':
          onToggleFullscreen?.()
          break
        case ' ':
          if (!onTogglePlay) return
          e.preventDefault()
          onTogglePlay()
          break
        case 'm':
        case 'M':
          onToggleMute?.()
          break
        case 'ArrowLeft':
          if (!onSkip) return
          e.preventDefault()
          onSkip(-10)
          break
        case 'ArrowRight':
          if (!onSkip) return
          e.preventDefault()
          onSkip(10)
          break
        default:
          break
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onTogglePlay, onSkip, onToggleMute, onToggleFullscreen])

  // Leaving fullscreen on Apple devices leaves the media surface broken: the
  // element is paused by the OS and the compositor keeps painting a single
  // frame, so playback looks frozen (one frame per second). Re-arm the
  // pipeline on exit: restart the loader, force playback to resume and force a
  // repaint of the stuck frame.
  useEffect(() => {
    const video = ref.current
    if (!video) return
    // default assume playing; iOS may not fire onEnter
    let wasPlaying = true
    let resumeAttempts = 0

    const onEnter = () => {
      wasPlaying = !video.paused
    }
    const tryResume = () => {
      if (!video.paused) return
      if (resumeAttempts >= 3) return // avoid infinite loop
      const promise = video.play()
      if (promise) {
        promise.catch(() => {})
      }
      resumeAttempts++
      setTimeout(() => {
        if (video.paused) tryResume()
      }, 250)
    }
    const nudge = () => {
      setTimeout(() => {
        if (video.seeking || video.readyState < 2) return
        try { video.currentTime += 0.04 } catch {}
      }, 400)
    }
    const onExit = () => {
      // Always try to resume – the OS may have paused the surface.
      hlsRef.current?.startLoad()
      const resume = () => {
        if (video.paused) video.play().catch(() => {})
      }
      resume()
      // Retry once the surface has stabilised (≈ ½ second).
      setTimeout(() => {
        if (video.paused) resume()
        // Small nudge to force the compositor to re‑render the stuck frame.
        if (!video.seeking && video.readyState >= 2) {
          try { video.currentTime += 0.04 } catch {}
        }
      }, 500)
    }
    const onChange = () => {
      const el = document.fullscreenElement || document.webkitFullscreenElement
      if (el) onEnter()
      else onExit()
    }

    document.addEventListener('fullscreenchange', onChange)
    document.addEventListener('webkitfullscreenchange', onChange)
    // iOS native fullscreen player reports its own events on the video.
    video.addEventListener('webkitbeginfullscreen', onEnter)
    video.addEventListener('webkitendfullscreen', onExit)

    return () => {
      document.removeEventListener('fullscreenchange', onChange)
      document.removeEventListener('webkitfullscreenchange', onChange)
      video.removeEventListener('webkitbeginfullscreen', onEnter)
      video.removeEventListener('webkitendfullscreen', onExit)
    }
  }, [ref, hlsRef])

  // The box keeps the film's native ratio. Desktop pins the card to the viewport
  // height, so the box is the smaller of "width" and "height x ratio"; on
  // mobile the height follows the width, so the film is never stretched.
  const heightConstrained = useMediaQuery('(min-width: 1024px)')
  const fitWidth = area
    ? heightConstrained && area.h > 0
      ? Math.min(area.w, area.h * ratioValue(ratio))
      : area.w
    : undefined

  return (
    <div ref={areaRef} className="flex min-h-0 w-full items-start justify-center lg:flex-1">
      <div
        ref={containerRef}
        onClick={handleClick}
        onDoubleClick={handleDoubleClick}
        style={fitWidth ? { width: fitWidth } : undefined}
        className="relative flex items-center justify-center"
      >
        <video
          ref={ref}
          className={cn('w-full max-w-full rounded-lg bg-black', onTogglePlay && 'cursor-pointer')}
          style={{ aspectRatio: ratio }}
          playsInline
          muted={muted}
          preload="auto"
          controls={false}
          onTimeUpdate={onTimeUpdate}
          onLoadedMetadata={(e) => {
            const video = e.currentTarget
            if (video.videoWidth && video.videoHeight) {
              setRatio(`${video.videoWidth} / ${video.videoHeight}`)
            }
            onLoadedMetadata?.(e)
          }}
        />

        {flash > 0 && (
          <div
            key={flash}
            className="pointer-events-none absolute inset-0 flex items-center justify-center"
          >
            <div className="animate-[player-flash_1s_ease-out_forwards] rounded-full bg-black/40 p-4">
              {playing ? (
                <Pause className="h-12 w-12 text-white" />
              ) : (
                <Play className="h-12 w-12 text-white" />
              )}
            </div>
          </div>
        )}

        {overlay}

        {/* Always visible notices (e.g. "click to start") sit on the picture itself. */}
        {notice}
      </div>
    </div>
  )
}
