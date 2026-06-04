import { useState } from 'react'
import { Stage, Layer, Circle, Line, Rect } from 'react-konva'
import type { KonvaEventObject } from 'konva/lib/Node'
import { useAppDispatch, useAppSelector } from '../../store/hooks'
import { selectPhoto } from '../gallery/selectedPhotoSlice'
import { setLocationFilter, setLocationRadius, clearLocationFilter } from '../filters/filtersSlice'
import { CLASS_COLORS, FALLBACK_COLOR } from '../gallery/bboxUtils'
import { useAllPhotos } from './useAllPhotos'
import { metersToCanvas, canvasToMeters, FLOOR_SIZE_M, CANVAS_SIZE_PX } from './mapUtils'
import type { Photo } from '../../api'
import styles from './MapView.module.css'

const SCALE = CANVAS_SIZE_PX / FLOOR_SIZE_M // 10 px/m
const GRID_STEP_M = 5
const GRID_STEP_PX = GRID_STEP_M * SCALE

const GRID_LINES = Array.from(
  { length: Math.floor(FLOOR_SIZE_M / GRID_STEP_M) - 1 },
  (_, i) => (i + 1) * GRID_STEP_PX,
)

function dotColor(photo: Photo): string {
  if (photo.predictions.length === 0) return '#555'
  const top = photo.predictions.reduce((a, b) => (a.confidence > b.confidence ? a : b))
  return CLASS_COLORS[top.classId] ?? FALLBACK_COLOR
}

function matchesFilter(photo: Photo, classId: string | null, minConfidence: number): boolean {
  if (!classId && minConfidence === 0) return true
  return photo.predictions.some(
    (p) => (classId ? p.classId === classId : true) && p.confidence >= minConfidence,
  )
}

export function MapView(): React.ReactElement {
  const dispatch = useAppDispatch()
  const classId = useAppSelector((s) => s.filters.classId)
  const minConfidence = useAppSelector((s) => s.filters.minConfidence)
  const locationFilter = useAppSelector((s) => s.filters.locationFilter)
  const { photos } = useAllPhotos()

  const [zoom, setZoom] = useState(1)
  const [panX, setPanX] = useState(0)
  const [panY, setPanY] = useState(0)

  function handleWheel(e: KonvaEventObject<WheelEvent>) {
    e.evt.preventDefault()
    const scaleBy = 1.1
    const stage = e.target.getStage()!
    const pointer = stage.getPointerPosition()!
    const newZoom = e.evt.deltaY < 0 ? zoom * scaleBy : zoom / scaleBy
    const clamped = Math.min(Math.max(newZoom, 0.5), 8)
    const mousePointTo = { x: (pointer.x - panX) / zoom, y: (pointer.y - panY) / zoom }
    setPanX(pointer.x - mousePointTo.x * clamped)
    setPanY(pointer.y - mousePointTo.y * clamped)
    setZoom(clamped)
  }

  function handleStageClick(e: KonvaEventObject<MouseEvent>) {
    if (e.target !== e.target.getStage() && e.target.getClassName() !== 'Rect') return
    const stage = e.target.getStage()!
    const pos = stage.getRelativePointerPosition()!
    const { x, y } = canvasToMeters(pos.x, pos.y, SCALE)
    dispatch(setLocationFilter({ x, y }))
  }

  return (
    <div className={styles.container}>
      <p className={styles.title}>Greenhouse Floor</p>
      <Stage
        width={CANVAS_SIZE_PX}
        height={CANVAS_SIZE_PX}
        draggable
        onWheel={handleWheel}
        onClick={handleStageClick}
        scaleX={zoom}
        scaleY={zoom}
        x={panX}
        y={panY}
        className={styles.stage}
      >
        <Layer>
          {/* floor background */}
          <Rect
            x={0}
            y={0}
            width={CANVAS_SIZE_PX}
            height={CANVAS_SIZE_PX}
            fill="#0a0a0a"
          />
          {/* grid lines */}
          {GRID_LINES.flatMap((pos) => [
            <Line
              key={`h${pos}`}
              points={[0, pos, CANVAS_SIZE_PX, pos]}
              stroke="#1e1e1e"
              strokeWidth={1}
              listening={false}
            />,
            <Line
              key={`v${pos}`}
              points={[pos, 0, pos, CANVAS_SIZE_PX]}
              stroke="#1e1e1e"
              strokeWidth={1}
              listening={false}
            />,
          ])}
          {/* location filter circle */}
          {locationFilter && (
            <Circle
              x={metersToCanvas(locationFilter.x, locationFilter.y, SCALE).cx}
              y={metersToCanvas(locationFilter.x, locationFilter.y, SCALE).cy}
              radius={locationFilter.radius * SCALE}
              stroke="#4a9eff"
              strokeWidth={1}
              dash={[4, 4]}
              fill="rgba(74,158,255,0.06)"
              listening={false}
            />
          )}
          {/* photo dots */}
          {photos.map((photo) => {
            const { cx, cy } = metersToCanvas(photo.x, photo.y, SCALE)
            return (
              <Circle
                key={photo.id}
                x={cx}
                y={cy}
                radius={6}
                fill={dotColor(photo)}
                opacity={matchesFilter(photo, classId, minConfidence) ? 1 : 0.2}
                onClick={() => dispatch(selectPhoto(photo.id))}
              />
            )
          })}
        </Layer>
      </Stage>
      <div className={styles.controls}>
        <label htmlFor="radius-slider" className={styles.label}>
          Radius: {locationFilter?.radius ?? 5} m
        </label>
        <input
          id="radius-slider"
          type="range"
          min={1}
          max={20}
          step={1}
          value={locationFilter?.radius ?? 5}
          onChange={(e) => dispatch(setLocationRadius(Number(e.target.value)))}
          className={styles.slider}
          disabled={!locationFilter}
        />
        {locationFilter && (
          <button className={styles.clearBtn} onClick={() => dispatch(clearLocationFilter())}>
            Clear
          </button>
        )}
      </div>
    </div>
  )
}
