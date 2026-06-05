import { describe, it, expect } from 'vitest'
import { describePredictions } from './predictionSummary'
import type { Prediction } from '../../api'

function pred(classId: string, confidence: number): Prediction {
  return { classId, confidence, bbox: { xMin: 0, yMin: 0, xMax: 1, yMax: 1 } } as Prediction
}

describe('describePredictions', () => {
  it('returns "no detections" for an empty list', () => {
    expect(describePredictions([])).toBe('no detections')
  })

  it('labels a single detection with a rounded percentage', () => {
    expect(describePredictions([pred('mirid', 0.923)])).toBe('Mirid 92%')
  })

  it('joins multiple detections in order', () => {
    expect(describePredictions([pred('thrips', 0.75), pred('spider_mites', 0.5)])).toBe(
      'Thrips 75%, Spider Mites 50%',
    )
  })

  it('falls back to the raw classId for unknown classes', () => {
    expect(describePredictions([pred('unknown_pest', 0.4)])).toBe('unknown_pest 40%')
  })
})
