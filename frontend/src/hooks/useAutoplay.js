import { useCallback, useEffect, useRef, useState } from 'react'

/**
 * Plays the video even when the browser blocks autoplay with sound (which
 * happens after a reload, since the user gesture is gone). The element is
 * muted for the attempt and unmuted again on the next user interaction.
 *
 * Returns `play` and `autoMuted`: the page keeps the mute button in sync with
 * what the element really does.
 */
export function useAutoplay(videoRef) {
  const [autoMuted, setAutoMuted] = useState(false)
  const autoMutedRef = useRef(false)

  useEffect(() => {
    // The video element may not exist yet (it is created with the session), so
    // the listener looks it up when it fires instead of capturing it here.
    const unmute = () => {
      const video = videoRef.current
      if (!video || !autoMutedRef.current) return
      autoMutedRef.current = false
      setAutoMuted(false)
      video.muted = false
    }
    window.addEventListener('pointerdown', unmute)
    window.addEventListener('keydown', unmute)
    return () => {
      window.removeEventListener('pointerdown', unmute)
      window.removeEventListener('keydown', unmute)
    }
  }, [videoRef])

  const play = useCallback(async () => {
    const video = videoRef.current
    if (!video) return false
    try {
      await video.play()
      return true
    } catch {
      // Autoplay with sound was blocked: play muted and unmute on interaction.
      video.muted = true
      autoMutedRef.current = true
      setAutoMuted(true)
      try {
        await video.play()
        return true
      } catch {
        return false
      }
    }
  }, [videoRef])

  return { play, autoMuted }
}
