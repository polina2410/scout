import { GalleryGrid } from './features/gallery/GalleryGrid'
import { PhotoModal } from './features/gallery/PhotoModal'
import { FilterBar } from './features/filters/FilterBar'
import { MapView } from './features/map/MapView'
import styles from './App.module.css'

export default function App() {
  return (
    <div className={styles.layout}>
      <header className={styles.header}>
        <h1 className={styles.title}>Scout</h1>
      </header>
      <FilterBar />
      <div className={styles.content}>
        <MapView />
        <main className={styles.main}>
          <GalleryGrid />
        </main>
      </div>
      <PhotoModal />
    </div>
  )
}
