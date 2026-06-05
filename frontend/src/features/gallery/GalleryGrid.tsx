import { useEffect, useLayoutEffect, useRef } from 'react'
import { motion, AnimatePresence } from 'motion/react'
import { useAppSelector } from '../../store/hooks'
import { usePhotos } from './usePhotos'
import { PhotoCard } from './PhotoCard'
import { photosNearby } from '../map/mapUtils'
import styles from './GalleryGrid.module.css'
import a11y from '../../styles/a11y.module.css'

const FADE = { duration: 0.18 } as const

export function GalleryGrid() {
  const classId = useAppSelector((s) => s.filters.classId)
  const minConfidence = useAppSelector((s) => s.filters.minConfidence)
  const locationFilter = useAppSelector((s) => s.filters.locationFilter)
  const { photos, status, error, loadMoreError, hasMore, loadMore } = usePhotos(
    { classId: classId ?? undefined, minConfidence }
  )
  const visiblePhotos = locationFilter
    ? photosNearby(photos, locationFilter.x, locationFilter.y, locationFilter.radius)
    : photos

  const sentinelRef = useRef<HTMLDivElement>(null)
  const loadMoreRef = useRef(loadMore)

  useLayoutEffect(() => {
    loadMoreRef.current = loadMore
  }, [loadMore])

  useEffect(() => {
    const sentinel = sentinelRef.current
    if (!sentinel) return
    const observer = new IntersectionObserver((entries) => {
      if (entries[0]?.isIntersecting && hasMore && status !== 'loading-more') {
        loadMoreRef.current()
      }
    })
    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [hasMore, status])

  if (status === 'loading') {
    return (
      <motion.div
        className={styles.loading}
        role="status"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={FADE}
      >
        Loading…
      </motion.div>
    )
  }

  if (status === 'error') {
    return (
      <motion.div
        className={styles.error}
        role="alert"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={FADE}
      >
        {error}
      </motion.div>
    )
  }

  if (visiblePhotos.length === 0) {
    return (
      <motion.div
        className={styles.empty}
        role="status"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={FADE}
      >
        No photos found.
      </motion.div>
    )
  }

  return (
    <>
      <div className={a11y.srOnly} role="status">
        {visiblePhotos.length} photo{visiblePhotos.length === 1 ? '' : 's'} shown
      </div>
      <div className={styles.grid}>
        <AnimatePresence initial={false}>
          {visiblePhotos.map((photo, index) => (
            <motion.div
              key={photo.id}
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.22, delay: Math.min(index % 20, 8) * 0.04 }}
            >
              <PhotoCard photo={photo} />
            </motion.div>
          ))}
        </AnimatePresence>
      </div>
      {status === 'loading-more' && (
        <div className={styles.loadingMore} role="status">Loading more…</div>
      )}
      {loadMoreError && (
        <div className={styles.loadMoreError}>{loadMoreError}</div>
      )}
      <div ref={sentinelRef} className={styles.sentinel} />
    </>
  )
}
