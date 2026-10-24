import { Navigate } from 'react-router-dom'

// The admin/login pages may be reached from this machine (loopback) or from a
// device on the same local network (private IPv4). Everything else (tunnels,
// public DNS) is redirected to the Access Denied page.
const LOCAL_HOSTS = new Set(['localhost', '127.0.0.1', '::1', '[::1]', '0.0.0.0'])
const PRIVATE_IPV4 = /^(10\.\d{1,3}|192\.168|172\.(1[6-9]|2\d|3[01]))\.\d{1,3}\.\d{1,3}$/

export function isLocalHost(hostname) {
  if (LOCAL_HOSTS.has(hostname)) return true
  if (PRIVATE_IPV4.test(hostname)) return true
  // mDNS name of a local device, e.g. "mylaptop.local".
  return hostname.endsWith('.local')
}

export function LocalOnly({ children }) {
  if (!isLocalHost(window.location.hostname)) {
    return <Navigate to="/denied" replace />
  }
  return children
}
