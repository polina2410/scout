import type { components, operations } from './generated/schema'

export type Photo = components['schemas']['Photo']
export type Prediction = components['schemas']['Prediction']
export type BoundingBox = components['schemas']['BoundingBox']
export type PhotoPage = components['schemas']['PhotoPage']

export type ListPhotosParams = NonNullable<
  operations['listPhotos']['parameters']['query']
>
