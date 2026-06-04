import { GalleryGrid } from './features/gallery/GalleryGrid'
import styles from './App.module.css'

export default function App() {
  return (
    <div className={styles.layout}>
      <header className={styles.header}>
        <h1 className={styles.title}>Scout</h1>
      </header>
      <main className={styles.main}>
        <GalleryGrid />
      </main>
    </div>
  )
}
