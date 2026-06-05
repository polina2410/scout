import type { Prediction } from '../../api'
import { CLASS_LABEL } from '../filters/classLabels'

/**
 * Builds a human-readable, screen-reader-friendly summary of a photo's
 * detections, e.g. "Mirid 92%, Thrips 75%" or "no detections".
 * Used for accessible labels where the bounding-box overlay is the only
 * visual carrier of this information.
 */
export function describePredictions(predictions: Prediction[]): string {
  if (predictions.length === 0) return 'no detections'
  return predictions
    .map((p) => {
      const label = CLASS_LABEL[p.classId as keyof typeof CLASS_LABEL] ?? p.classId
      return `${label} ${Math.round(p.confidence * 100)}%`
    })
    .join(', ')
}
