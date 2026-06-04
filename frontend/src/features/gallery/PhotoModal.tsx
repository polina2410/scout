import { useEffect, useRef } from 'react'
import { useAppDispatch, useAppSelector } from '../../store/hooks'
import { clearSelectedPhoto } from './selectedPhotoSlice'
import { usePhotoDetail } from './usePhotoDetail'
import { BboxCanvas } from './BboxCanvas'
import { CLASS_COLORS, FALLBACK_COLOR } from './bboxUtils'
import { CLASS_LABEL } from '../filters/classLabels'
import styles from './PhotoModal.module.css'

export function PhotoModal(): React.ReactElement | null {
  const dispatch = useAppDispatch()
  const photoId = useAppSelector((s) => s.selectedPhoto.photoId)
  const { photo, status, error } = usePhotoDetail(photoId)
  const closeBtnRef = useRef<HTMLButtonElement>(null)
  const previousFocusRef = useRef<Element | null>(null)

  const close = () => dispatch(clearSelectedPhoto())

  // capture focus target before opening; restore on close
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

  // Escape key closes the modal
  useEffect(() => {
    if (photoId === null) return
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') close()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [photoId]) // eslint-disable-line react-hooks/exhaustive-deps

  if (photoId === null) return null

  return (
    <div className={styles.backdrop} onClick={close}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Photo detail"
        className={styles.dialog}
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
          <div className={styles.loading}>Loading…</div>
        )}

        {status === 'error' && (
          <div className={styles.error}>{error}</div>
        )}

        {status === 'success' && photo && (
          <>
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
          </>
        )}
      </div>
    </div>
  )
}
