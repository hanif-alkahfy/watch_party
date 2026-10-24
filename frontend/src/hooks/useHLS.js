import { useEffect, useRef } from 'react'
import Hls from 'hls.js'
import { auth } from '@/lib/api'

/**
 * Loads an HLS stream into a <video> element.
 * The playlist and segments are fetched with the JWT in the Authorization header,
 * so the backend must accept them (PRD 9.4).
 */
export function useHLS(videoRef, hlsUrl, { onManifestParsed, onError, token } = {}) {
  const callbacks = useRef({ onManifestParsed, onError })
  callbacks.current = { onManifestParsed, onError }
  // Exposed so the player can re-arm the loader (e.g. after a fullscreen exit).
  const hlsRef = useRef(null)

  useEffect(() => {
    const video = videoRef.current
    if (!video || !hlsUrl) return

    if (!Hls.isSupported()) {
      callbacks.current.onError?.(new Error('HLS is not supported in this browser'))
      return
    }

    const bearer = token ?? auth.getToken()
    const hls = new Hls({
      maxBufferLength: 15,
      maxMaxBufferLength: 25,
      maxBufferSize: 60 * 1000 * 1000,
      maxStarvationDelay: 4,
      backBufferLength: 15,
      // hls.js reads xhrSetup when the loader is built, so it must be passed here.
      xhrSetup: (xhr) => {
        if (bearer) xhr.setRequestHeader('Authorization', `Bearer ${bearer}`)
      },
    })
    hlsRef.current = hls

    hls.on(Hls.Events.MANIFEST_PARSED, () => callbacks.current.onManifestParsed?.())
    hls.on(Hls.Events.ERROR, (_event, data) => {
      if (!data.fatal) return
      if (data.type === Hls.ErrorTypes.NETWORK_ERROR) hls.startLoad()
      else if (data.type === Hls.ErrorTypes.MEDIA_ERROR) hls.recoverMediaError()
      else callbacks.current.onError?.(new Error(data.details || 'HLS playback failed'))
    })

    hls.loadSource(hlsUrl)
    hls.attachMedia(video)

    return () => {
      hlsRef.current = null
      hls.destroy()
    }
  }, [videoRef, hlsUrl, token])

  return hlsRef
}
