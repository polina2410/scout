import { useAppDispatch, useAppSelector } from '../../store/hooks'
import { setClassId, setMinConfidence, resetFilters } from './filtersSlice'
import { CLASS_IDS } from './types'
import { CLASS_COLORS } from '../gallery/bboxUtils'
import { CLASS_LABEL } from './classLabels'
import styles from './FilterBar.module.css'

export function FilterBar(): React.ReactElement {
  const dispatch = useAppDispatch()
  const classId = useAppSelector((s) => s.filters.classId)
  const minConfidence = useAppSelector((s) => s.filters.minConfidence)
  const isFiltered = classId !== null || minConfidence > 0

  return (
    <div className={styles.bar}>
      <div className={styles.classes}>
        <button
          className={`${styles.classBtn} ${classId === null ? styles.active : ''}`}
          aria-pressed={classId === null}
          onClick={() => dispatch(setClassId(null))}
        >
          All
        </button>
        {CLASS_IDS.map((id) => (
          <button
            key={id}
            className={`${styles.classBtn} ${classId === id ? styles.active : ''}`}
            aria-pressed={classId === id}
            style={{ '--accent': CLASS_COLORS[id] } as React.CSSProperties}
            onClick={() => dispatch(setClassId(id))}
          >
            {CLASS_LABEL[id]}
          </button>
        ))}
      </div>

      <div className={styles.confidence}>
        <label htmlFor="confidence-slider" className={styles.label}>
          Min confidence: {Math.round(minConfidence * 100)}%
        </label>
        <input
          id="confidence-slider"
          type="range"
          min={0}
          max={100}
          step={1}
          value={Math.round(minConfidence * 100)}
          onChange={(e) => dispatch(setMinConfidence(Number(e.target.value) / 100))}
          className={styles.slider}
        />
      </div>

      {isFiltered && (
        <button className={styles.resetBtn} onClick={() => dispatch(resetFilters())}>
          Reset
        </button>
      )}
    </div>
  )
}
