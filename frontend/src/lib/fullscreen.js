// Cross-browser fullscreen helpers. Safari and iOS use the WebKit prefixed
// APIs, and iOS only ever fullscreens the <video> element itself.

export function fullscreenElement() {
  return (
    document.fullscreenElement ||
    document.webkitFullscreenElement ||
    document.msFullscreenElement ||
    null
  )
}

export function requestFullscreen(element) {
  if (!element) return
  const fn =
    element.requestFullscreen ||
    element.webkitRequestFullscreen ||
    element.msRequestFullscreen
  if (fn) return fn.call(element)
}

export function exitFullscreen() {
  const fn = document.exitFullscreen || document.webkitExitFullscreen || document.msExitFullscreen
  if (fn) return fn.call(document)
}

export function isIOSDevice() {
  if (typeof navigator === 'undefined') return false
  return (
    /iP(hone|ad|od)/.test(navigator.userAgent) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
  )
}
