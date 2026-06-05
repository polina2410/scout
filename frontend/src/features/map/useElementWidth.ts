import { useLayoutEffect, useRef, useState } from 'react'

/**
 * Tracks the live content width of an element via ResizeObserver.
 * Used to size the Konva map canvas responsively (Konva needs explicit
 * pixel dimensions — it cannot use CSS percentages). Returns a ref to attach
 * and the current width; `initial` seeds the first render before measurement.
 */
export function useElementWidth<T extends HTMLElement>(initial: number) {
  const ref = useRef<T>(null)
  const [width, setWidth] = useState(initial)

  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const update = () => setWidth(el.clientWidth)
    update()
    const observer = new ResizeObserver(update)
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  return [ref, width] as const
}
