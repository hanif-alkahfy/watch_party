import { useEffect, useState } from 'react'

const SEGMENT = /\.m4s(\?|$)/

// Real throughput of the last HLS segment downloads: end-to-end speed,
// including any tunnel in between.
function measuredMbps() {
  if (typeof performance === 'undefined') return null
  const entries = performance.getEntriesByType('resource').filter((e) => SEGMENT.test(e.name))
  let bytes = 0
  let seconds = 0
  for (const entry of entries.slice(-8)) {
    const size = entry.transferSize || 0
    if (!size) continue
    bytes += size
    seconds += (entry.duration || 0) / 1000
  }
  if (!bytes || !seconds) return null
  return (bytes * 8) / (seconds * 1e6)
}

// Browser estimate, used until something has been measured.
function estimatedMbps() {
  const conn = typeof navigator !== 'undefined' ? navigator.connection : null
  if (!conn || !conn.downlink) return null
  return conn.downlink
}

export function useConnectionSpeed() {
  const [mbps, setMbps] = useState(null)

  useEffect(() => {
    const read = () => setMbps(measuredMbps() ?? estimatedMbps())
    read()

    const conn = typeof navigator !== 'undefined' ? navigator.connection : null
    conn?.addEventListener?.('change', read)
    const timer = setInterval(read, 2000)

    return () => {
      clearInterval(timer)
      conn?.removeEventListener?.('change', read)
    }
  }, [])

  return mbps
}
