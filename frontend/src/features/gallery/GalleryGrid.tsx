import { useEffect, useLayoutEffect, useRef } from 'react'
import { useAppSelector } from '../../store/hooks'
import { usePhotos } from './usePhotos'
import { PhotoCard } from './PhotoCard'
import { photosNearby } from '../map/mapUtils'
import styles from './GalleryGrid.module.css'

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
    return <div className={styles.loading}>Loading…</div>
  }

  if (status === 'error') {
    return <div className={styles.error}>{error}</div>
  }

  if (visiblePhotos.length === 0) {
    return <div className={styles.empty}>No photos found.</div>
  }

  return (
    <>
      <div className={styles.grid}>
        {visiblePhotos.map((photo) => (
          <PhotoCard key={photo.id} photo={photo} />
        ))}
      </div>
      {status === 'loading-more' && (
        <div className={styles.loadingMore}>Loading more…</div>
      )}
      {loadMoreError && (
        <div className={styles.loadMoreError}>{loadMoreError}</div>
      )}
      <div ref={sentinelRef} className={styles.sentinel} />
    </>
  )
}
