import { useEffect, useState } from 'react'

// Player chrome (controls overlay) hides after 5s of inactivity and returns
// when the mouse moves inside the player box.
export function usePlayerChrome(boxRef, enabled = true) {
  const [visible, setVisible] = useState(true)

  useEffect(() => {
    if (!enabled) return
    const box = boxRef.current
    if (!box) return

    let timer
    const reveal = () => {
      setVisible(true)
      clearTimeout(timer)
      timer = setTimeout(() => setVisible(false), 5000)
    }
    reveal()
    box.addEventListener('mousemove', reveal)
    return () => {
      clearTimeout(timer)
      box.removeEventListener('mousemove', reveal)
    }
  }, [boxRef, enabled])

  return visible
}
