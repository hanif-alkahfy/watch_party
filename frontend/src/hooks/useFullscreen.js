import { useCallback, useEffect, useState } from 'react'
import {
  exitFullscreen,
  fullscreenElement,
  isIOSDevice,
  requestFullscreen,
} from '@/lib/fullscreen'

// Fullscreen for the player box, with Safari/iOS fallbacks.
export function useFullscreen(boxRef, videoRef, enabled = true) {
  const [isFullscreen, setIsFullscreen] = useState(false)

  useEffect(() => {
    if (!enabled) return
    const video = videoRef.current

    const onChange = () => setIsFullscreen(Boolean(fullscreenElement()))
    // iOS fires these on the video element for the native fullscreen player.
    const onBegin = () => setIsFullscreen(true)
    const onEnd = () => setIsFullscreen(false)

    document.addEventListener('fullscreenchange', onChange)
    document.addEventListener('webkitfullscreenchange', onChange)
    video?.addEventListener('webkitbeginfullscreen', onBegin)
    video?.addEventListener('webkitendfullscreen', onEnd)

    return () => {
      document.removeEventListener('fullscreenchange', onChange)
      document.removeEventListener('webkitfullscreenchange', onChange)
      video?.removeEventListener('webkitbeginfullscreen', onBegin)
      video?.removeEventListener('webkitendfullscreen', onEnd)
    }
  }, [videoRef, enabled])

  const toggleFullscreen = useCallback(() => {
    const box = boxRef.current
    const video = videoRef.current

    if (isFullscreen) {
      if (isIOSDevice() && video?.webkitExitFullscreen) {
        video.webkitExitFullscreen()
        return
      }
      exitFullscreen()
      return
    }

    // iOS Safari cannot fullscreen a container: only the video element can.
    if (isIOSDevice() && video?.webkitEnterFullscreen) {
      video.webkitEnterFullscreen()
      return
    }

    requestFullscreen(box)
  }, [isFullscreen, boxRef, videoRef])

  return { isFullscreen, toggleFullscreen }
}
