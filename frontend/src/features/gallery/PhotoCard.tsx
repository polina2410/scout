import { useAppDispatch } from '../../store/hooks'
import { selectPhoto } from './selectedPhotoSlice'
import { thumbnailUrl, thumbnailSrcSet, CARD_CSS_WIDTH } from './thumbnailUrl'
import { BboxCanvas } from './BboxCanvas'
import { describePredictions } from './predictionSummary'
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

  // Native <button> gives keyboard activation (Enter/Space), focus, and the
  // button role for free — no manual onKeyDown / tabIndex / role needed.
  return (
    <button
      type="button"
      className={styles.card}
      onClick={handleClick}
      aria-label={`Greenhouse photo, ${describePredictions(photo.predictions)}. View details.`}
    >
      <div className={styles.imageWrapper}>
        <img
          src={thumbnailUrl(photo.id, CARD_CSS_WIDTH, 1)}
          srcSet={thumbnailSrcSet(photo.id, CARD_CSS_WIDTH)}
          alt=""
          className={styles.image}
          loading="lazy"
          decoding="async"
        />
        <BboxCanvas predictions={photo.predictions} />
      </div>
    </button>
  )
}
