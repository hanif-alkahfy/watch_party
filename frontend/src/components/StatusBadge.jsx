import { Signal } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { useConnectionSpeed } from '@/hooks/useConnectionSpeed'

const STYLES = {
  uploading: 'bg-amber-500/15 text-amber-700 dark:text-amber-400 border-amber-500/30',
  processing: 'bg-blue-500/15 text-blue-700 dark:text-blue-400 border-blue-500/30',
  ready: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400 border-emerald-500/30',
  failed: 'bg-red-500/15 text-red-700 dark:text-red-400 border-red-500/30',
}

const SPEED_STYLES = {
  good: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400 border-emerald-500/30',
  warn: 'bg-amber-500/15 text-amber-700 dark:text-amber-400 border-amber-500/30',
  bad: 'bg-red-500/15 text-red-700 dark:text-red-400 border-red-500/30',
}

export function StatusBadge({ status }) {
  const style = STYLES[status] || SPEED_STYLES[status] || SPEED_STYLES.bad
  return (
    <Badge variant="outline" className={style}>
      {status}
    </Badge>
  )
}

// Signal icon + live speed, coloured by connection status and throughput.
export function ConnectionSpeed({ status }) {
  const mbps = useConnectionSpeed()
  const connected = status === 'connected'

  const level = !connected ? 'bad' : mbps == null ? 'warn' : mbps >= 5 ? 'good' : mbps >= 1 ? 'warn' : 'bad'
  const text = !connected
    ? status
    : mbps == null
      ? 'no data'
      : mbps >= 10
        ? `${Math.round(mbps)} Mbps`
        : `${mbps.toFixed(1)} Mbps`

  return (
    <Badge variant="outline" className={SPEED_STYLES[level]}>
      <Signal className="mr-1.5 h-3.5 w-3.5" />
      {text}
    </Badge>
  )
}
