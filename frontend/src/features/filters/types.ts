export const CLASS_IDS = [
  'powdery_mildew',
  'mirid',
  'whitefly_aphid',
  'miner_tuta',
  'thrips',
  'spider_mites',
] as const

export type ClassId = (typeof CLASS_IDS)[number]
