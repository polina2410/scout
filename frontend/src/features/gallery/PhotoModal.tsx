import { useEffect, useRef } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import { useAppDispatch, useAppSelector } from '../../store/hooks'
import { clearSelectedPhoto } from './selectedPhotoSlice'
import { usePhotoDetail } from './usePhotoDetail'
import { BboxCanvas } from './BboxCanvas'
import { CLASS_COLORS, FALLBACK_COLOR } from './bboxUtils'
import { CLASS_LABEL } from '../filters/classLabels'
import styles from './PhotoModal.module.css'

const SPRING = { duration: 0.25, ease: [0.16, 1, 0.3, 1] } as const

export function PhotoModal(): React.ReactElement {
  const dispatch = useAppDispatch()
  const photoId = useAppSelector((s) => s.selectedPhoto.photoId)
  const { photo, status, error } = usePhotoDetail(photoId)
  const closeBtnRef = useRef<HTMLButtonElement>(null)
  const previousFocusRef = useRef<Element | null>(null)

  const close = () => dispatch(clearSelectedPhoto())

  useEffect(() => {
    if (photoId !== null) {
      previousFocusRef.current = document.activeElement
      closeBtnRef.current?.focus()
    } else {
      if (previousFocusRef.current instanceof HTMLElement) {
        previousFocusRef.current.focus()
      }
      previousFocusRef.current = null
    }
  }, [photoId])

  useEffect(() => {
    if (photoId === null) return
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') close()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [photoId]) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <AnimatePresence>
      {photoId !== null && (
        <motion.div
          className={styles.backdrop}
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.2 }}
          onClick={close}
        >
          <motion.div
            role="dialog"
            aria-modal="true"
            aria-label="Photo detail"
            className={styles.dialog}
            initial={{ opacity: 0, scale: 0.96, y: 12 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 12 }}
            transition={SPRING}
            onClick={(e) => e.stopPropagation()}
          >
            <button
              ref={closeBtnRef}
              className={styles.closeBtn}
              aria-label="Close"
              onClick={close}
            >
              ×
            </button>

            {status === 'loading' && (
              <motion.div
                className={styles.loading}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
              >
                Loading…
              </motion.div>
            )}

            {status === 'error' && (
              <motion.div
                className={styles.error}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
              >
                {error}
              </motion.div>
            )}

            {status === 'success' && photo && (
              <motion.div
                className={styles.successContent}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ duration: 0.2, delay: 0.05 }}
              >
                <div className={styles.imageSection}>
                  <div className={styles.imageWrapper}>
                    <img
                      src={photo.originalUrl}
                      alt={`Photo ${photo.id}`}
                      className={styles.image}
                    />
                    <BboxCanvas predictions={photo.predictions} />
                  </div>
                </div>
                <div className={styles.sidebar}>
                  <h2 className={styles.sidebarTitle}>Detections</h2>
                  {photo.predictions.length === 0 ? (
                    <p className={styles.noDetections}>No detections.</p>
                  ) : (
                    <ul className={styles.predList}>
                      {photo.predictions.map((pred, i) => (
                        <li key={i} className={styles.predItem}>
                          <span
                            className={styles.dot}
                            style={{ background: CLASS_COLORS[pred.classId] ?? FALLBACK_COLOR }}
                          />
                          <span className={styles.predClass}>
                            {CLASS_LABEL[pred.classId as keyof typeof CLASS_LABEL] ?? pred.classId}
                          </span>
                          <span className={styles.predConf}>
                            {Math.round(pred.confidence * 100)}%
                          </span>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              </motion.div>
            )}
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
