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
        {/* <picture> lets the browser negotiate WebP natively: it uses the WebP
            source only if it can render it, otherwise the JPEG <img> fallback —
            so cgo builds ship smaller WebP and no browser ever gets a format it
            can't display. The <picture> is display:contents so the <img> keeps
            sizing against .imageWrapper. */}
        <picture className={styles.picture}>
          <source type="image/webp" srcSet={thumbnailSrcSet(photo.id, CARD_CSS_WIDTH, 'webp')} />
          <img
            src={thumbnailUrl(photo.id, CARD_CSS_WIDTH, 1, 'jpeg')}
            srcSet={thumbnailSrcSet(photo.id, CARD_CSS_WIDTH, 'jpeg')}
            alt=""
            className={styles.image}
            loading="lazy"
            decoding="async"
          />
        </picture>
        <BboxCanvas predictions={photo.predictions} />
      </div>
    </button>
  )
}
