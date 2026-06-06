import { useEffect, useRef } from 'react'
import type { Prediction } from '../../api'
import { bboxToPixels, CLASS_COLORS, FALLBACK_COLOR } from './bboxUtils'
import styles from './BboxCanvas.module.css'

interface BboxCanvasProps {
  predictions: Prediction[]
}

// Each box is drawn twice: a darker, wider underlay first, then the class color
// on top. The halo keeps every box (notably the white powdery_mildew color)
// legible against both light and dark regions of the photo.
const BOX_LINE_WIDTH = 2
const BOX_HALO_WIDTH = 4
const BOX_HALO_COLOR = 'rgba(0, 0, 0, 0.55)'

export function BboxCanvas({ predictions }: BboxCanvasProps): React.ReactElement {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  // The observer below lives for the canvas's lifetime and reads predictions
  // through this ref, so a predictions change can never leave it with a stale
  // closure or trigger an observer teardown/re-setup.
  const predictionsRef = useRef(predictions)
  const drawRef = useRef<() => void>(() => {})

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    const draw = () => {
      const w = canvas.clientWidth
      const h = canvas.clientHeight
      if (w === 0 || h === 0) return

      const dpr = window.devicePixelRatio ?? 1
      canvas.width = w * dpr
      canvas.height = h * dpr

      const ctx = canvas.getContext('2d')
      if (!ctx) return

      ctx.clearRect(0, 0, canvas.width, canvas.height)
      ctx.scale(dpr, dpr)

      for (const pred of predictionsRef.current) {
        const { x, y, w: rw, h: rh } = bboxToPixels(pred.bbox, w, h)
        ctx.strokeStyle = BOX_HALO_COLOR
        ctx.lineWidth = BOX_HALO_WIDTH
        ctx.strokeRect(x, y, rw, rh)
        ctx.strokeStyle = CLASS_COLORS[pred.classId] ?? FALLBACK_COLOR
        ctx.lineWidth = BOX_LINE_WIDTH
        ctx.strokeRect(x, y, rw, rh)
      }
    }
    drawRef.current = draw
    draw()

    const observer = new ResizeObserver(() => draw())
    observer.observe(canvas)
    return () => observer.disconnect()
  }, [])

  // Redraw when predictions change; the observer stays mounted across changes.
  useEffect(() => {
    predictionsRef.current = predictions
    drawRef.current()
  }, [predictions])

  // Decorative: the bounding boxes are a visual overlay. The same detection
  // data is exposed as real text to assistive tech via the PhotoCard button's
  // aria-label and the PhotoModal sidebar, so this canvas is hidden from AT.
  return <canvas ref={canvasRef} className={styles.canvas} aria-hidden="true" />
}
