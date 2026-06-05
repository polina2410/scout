import { useMemo, useState } from 'react'
import { Stage, Layer, Circle, Line, Rect } from 'react-konva'
import type { KonvaEventObject } from 'konva/lib/Node'
import { useAppDispatch, useAppSelector } from '../../store/hooks'
import { selectPhoto } from '../gallery/selectedPhotoSlice'
import { setLocationFilter, setLocationRadius, clearLocationFilter } from '../filters/filtersSlice'
import { CLASS_COLORS, FALLBACK_COLOR } from '../gallery/bboxUtils'
import { describePredictions } from '../gallery/predictionSummary'
import { useAllPhotos } from './useAllPhotos'
import { useElementWidth } from './useElementWidth'
import { metersToCanvas, canvasToMeters, FLOOR_SIZE_M, CANVAS_SIZE_PX } from './mapUtils'
import type { Photo } from '../../api'
import styles from './MapView.module.css'
import a11y from '../../styles/a11y.module.css'

const GRID_STEP_M = 5
const MIN_MAP_PX = 240 // floor so the canvas stays usable on the smallest phones
const DOT_RADIUS_PX = 6

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

  // The map is a square sized to the available width (capped at the design size),
  // so it scales down on tablet/mobile instead of overflowing. scale (px per
  // meter) is derived from the rendered size so all geometry stays consistent.
  const [stageWrapRef, wrapWidth] = useElementWidth<HTMLDivElement>(CANVAS_SIZE_PX)
  const size = Math.max(MIN_MAP_PX, Math.min(CANVAS_SIZE_PX, wrapWidth))
  const scale = size / FLOOR_SIZE_M

  const gridLines = useMemo(
    () =>
      Array.from(
        { length: Math.floor(FLOOR_SIZE_M / GRID_STEP_M) - 1 },
        (_, i) => (i + 1) * GRID_STEP_M * scale,
      ),
    [scale],
  )

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

  // Sync React state after a drag so that subsequent wheel-zoom and re-renders
  // don't snap the stage back to its pre-drag position.
  function handleDragEnd(e: KonvaEventObject<DragEvent>) {
    setPanX(e.target.x())
    setPanY(e.target.y())
  }

  function handleStageClick(e: KonvaEventObject<MouseEvent>) {
    if (e.target !== e.target.getStage() && e.target.getClassName() !== 'Rect') return
    const stage = e.target.getStage()!
    const pos = stage.getRelativePointerPosition()!
    const { x, y } = canvasToMeters(pos.x, pos.y, scale)
    dispatch(setLocationFilter({ x, y }))
  }

  return (
    <aside className={styles.container} aria-label="Greenhouse map">
      <h2 className={styles.title}>Greenhouse Floor</h2>
      {/* The Konva canvas is mouse-only; role="img" gives screen readers a
          summary and the visually-hidden list below is the keyboard/SR path. */}
      <div
        ref={stageWrapRef}
        className={styles.stageWrap}
        role="img"
        aria-label={`Greenhouse floor map, ${photos.length} photos plotted`}
      >
        <Stage
          width={size}
          height={size}
          draggable
          onWheel={handleWheel}
          onDragEnd={handleDragEnd}
          onClick={handleStageClick}
          scaleX={zoom}
          scaleY={zoom}
          x={panX}
          y={panY}
          className={styles.stage}
        >
          <Layer>
            {/* floor background */}
            <Rect x={0} y={0} width={size} height={size} fill="#0a0a0a" />
            {/* grid lines */}
            {gridLines.flatMap((pos) => [
              <Line
                key={`h${pos}`}
                points={[0, pos, size, pos]}
                stroke="#1e1e1e"
                strokeWidth={1}
                listening={false}
              />,
              <Line
                key={`v${pos}`}
                points={[pos, 0, pos, size]}
                stroke="#1e1e1e"
                strokeWidth={1}
                listening={false}
              />,
            ])}
            {/* location filter circle */}
            {locationFilter && (
              <Circle
                x={metersToCanvas(locationFilter.x, locationFilter.y, scale).cx}
                y={metersToCanvas(locationFilter.x, locationFilter.y, scale).cy}
                radius={locationFilter.radius * scale}
                stroke="#4a9eff"
                strokeWidth={1}
                dash={[4, 4]}
                fill="rgba(74,158,255,0.06)"
                listening={false}
              />
            )}
            {/* photo dots */}
            {photos.map((photo) => {
              const { cx, cy } = metersToCanvas(photo.x, photo.y, scale)
              return (
                <Circle
                  key={photo.id}
                  x={cx}
                  y={cy}
                  radius={DOT_RADIUS_PX}
                  fill={dotColor(photo)}
                  opacity={matchesFilter(photo, classId, minConfidence) ? 1 : 0.2}
                  onClick={() => dispatch(selectPhoto(photo.id))}
                />
              )
            })}
          </Layer>
        </Stage>
      </div>
      <ul className={a11y.srOnly} aria-label="Photos on the greenhouse map">
        {photos.map((photo) => (
          <li key={photo.id}>
            <button type="button" onClick={() => dispatch(selectPhoto(photo.id))}>
              {`Photo at ${Math.round(photo.x)}m, ${Math.round(photo.y)}m — ${describePredictions(photo.predictions)}. View details.`}
            </button>
          </li>
        ))}
      </ul>
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
    </aside>
  )
}
