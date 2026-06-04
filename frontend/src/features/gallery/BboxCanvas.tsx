import { useEffect, useRef } from 'react'
import type { Prediction } from '../../api'
import { bboxToPixels, CLASS_COLORS, FALLBACK_COLOR } from './bboxUtils'
import styles from './BboxCanvas.module.css'

interface BboxCanvasProps {
  predictions: Prediction[]
}

export function BboxCanvas({ predictions }: BboxCanvasProps): React.ReactElement {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    function draw() {
      if (!canvas) return
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

      for (const pred of predictions) {
        const { x, y, w: rw, h: rh } = bboxToPixels(pred.bbox, w, h)
        ctx.strokeStyle = CLASS_COLORS[pred.classId] ?? FALLBACK_COLOR
        ctx.lineWidth = 2
        ctx.strokeRect(x, y, rw, rh)
      }
    }

    draw()

    const observer = new ResizeObserver(() => draw())
    observer.observe(canvas)
    return () => observer.disconnect()
  }, [predictions])

  return <canvas ref={canvasRef} className={styles.canvas} />
}
