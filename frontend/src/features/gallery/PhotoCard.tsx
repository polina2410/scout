import { useAppDispatch } from '../../store/hooks'
import { selectPhoto } from './selectedPhotoSlice'
import { thumbnailUrl, thumbnailSrcSet, CARD_CSS_WIDTH } from './thumbnailUrl'
import type { Photo } from '../../api'
import styles from './PhotoCard.module.css'

interface PhotoCardProps {
  photo: Photo
}

export function PhotoCard({ photo }: PhotoCardProps) {
  const dispatch = useAppDispatch()

  function handleClick() {
    dispatch(selectPhoto(photo.id))
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      dispatch(selectPhoto(photo.id))
    }
  }

  return (
    <article
      className={styles.card}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      tabIndex={0}
      role="button"
      aria-label={`View photo ${photo.id}`}
    >
      <div className={styles.imageWrapper}>
        <img
          src={thumbnailUrl(photo.id, CARD_CSS_WIDTH, 1)}
          srcSet={thumbnailSrcSet(photo.id, CARD_CSS_WIDTH)}
          alt={`Photo ${photo.id}`}
          className={styles.image}
          loading="lazy"
          decoding="async"
        />
      </div>
    </article>
  )
}
