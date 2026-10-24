import { useCallback, useEffect, useRef, useState } from 'react'
import { auth } from '@/lib/api'

const MAX_BACKOFF = 10000

/**
 * Live sync channel for one watch session.
 * Mirrors the PRD message contract: { type, payload, ts }.
 */
export function useWebSocket(sessionID, role, token) {
  const [playing, setPlaying] = useState(false)
  const [position, setPosition] = useState(0)
  const [updatedAt, setUpdatedAt] = useState(0)
  const [status, setStatus] = useState('connecting')

  const wsRef = useRef(null)
  const retryRef = useRef(0)
  const timerRef = useRef(null)
  const skewRef = useRef(0) // server clock - browser clock
  const closedRef = useRef(false)
  const fatalRef = useRef(false)
  // Commands sent before the socket is open are kept and flushed on connect.
  const pendingRef = useRef([])

  useEffect(() => {
    if (!sessionID) return
    closedRef.current = false
    fatalRef.current = false

    const connect = () => {
      const bearer = token ?? auth.getToken()
      if (!bearer) return

      const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
      const ws = new WebSocket(
        `${proto}://${window.location.host}/ws/${sessionID}?token=${encodeURIComponent(bearer)}&role=${role}`,
      )
      wsRef.current = ws
      setStatus(retryRef.current === 0 ? 'connecting' : 'reconnecting')

      ws.onopen = () => {
        retryRef.current = 0
        setStatus('connected')
        // Flush the commands that were sent while the socket was connecting.
        const queued = pendingRef.current
        pendingRef.current = []
        for (const raw of queued) ws.send(raw)
      }

      ws.onmessage = (event) => {
        let msg
        try {
          msg = JSON.parse(event.data)
        } catch {
          return
        }
        if (msg.ts) skewRef.current = msg.ts - Date.now()

        if (msg.type === 'snapshot') {
          setPlaying(Boolean(msg.payload.playing))
          setPosition(Number(msg.payload.position) || 0)
          setUpdatedAt(Number(msg.payload.updated_at) || 0)
        } else if (msg.type === 'play' || msg.type === 'pause' || msg.type === 'seek') {
          setPlaying(Boolean(msg.payload.playing))
          setPosition(Number(msg.payload.position) || 0)
          setUpdatedAt(Date.now() + skewRef.current)
        } else if (msg.type === 'error') {
          if (msg.payload.fatal) fatalRef.current = true
          setStatus(`error: ${msg.payload.error}`)
        }
      }

      ws.onclose = () => {
        if (closedRef.current) return
        if (fatalRef.current) return // server refused us; reconnecting cannot help
        setStatus('reconnecting')
        const delay = Math.min(1000 * 2 ** retryRef.current, MAX_BACKOFF)
        retryRef.current += 1
        timerRef.current = setTimeout(connect, delay)
      }

      ws.onerror = () => ws.close()
    }

    connect()

    return () => {
      closedRef.current = true
      clearTimeout(timerRef.current)
      wsRef.current?.close()
    }
  }, [sessionID, role, token])

  const sendCommand = useCallback(
    (type, position = null) => {
      if (role !== 'admin') return
      const raw = JSON.stringify({ type, payload: position !== null ? { position } : {}, ts: Date.now() })
      const ws = wsRef.current
      if (!ws || ws.readyState !== WebSocket.OPEN) {
        // The socket is still connecting: keep the commands for the open
        // event, so a click right after a reload is not lost.
        pendingRef.current = [...pendingRef.current, raw].slice(-10)
        return
      }
      ws.send(raw)
    },
    [role],
  )

  // Where the server thinks the playhead is right now.
  const expectedPosition = useCallback(() => {
    if (!playing) return position
    return position + (Date.now() + skewRef.current - updatedAt) / 1000
  }, [playing, position, updatedAt])

  return { playing, position, updatedAt, status, sendCommand, expectedPosition }
}
