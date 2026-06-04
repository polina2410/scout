import { createSlice } from '@reduxjs/toolkit'
import type { PayloadAction } from '@reduxjs/toolkit'
import type { ClassId } from './types'

interface LocationFilter {
  x: number
  y: number
  radius: number
}

interface FiltersState {
  classId: ClassId | null
  minConfidence: number
  locationFilter: LocationFilter | null
}

const initialState: FiltersState = {
  classId: null,
  minConfidence: 0,
  locationFilter: null,
}

const filtersSlice = createSlice({
  name: 'filters',
  initialState,
  reducers: {
    setClassId(state, action: PayloadAction<ClassId | null>) {
      state.classId = action.payload
    },
    setMinConfidence(state, action: PayloadAction<number>) {
      state.minConfidence = Math.min(1, Math.max(0, action.payload))
    },
    setLocationFilter(state, action: PayloadAction<{ x: number; y: number }>) {
      const radius = state.locationFilter?.radius ?? 5
      state.locationFilter = { x: action.payload.x, y: action.payload.y, radius }
    },
    setLocationRadius(state, action: PayloadAction<number>) {
      if (state.locationFilter) {
        state.locationFilter.radius = action.payload
      }
    },
    clearLocationFilter(state) {
      state.locationFilter = null
    },
    resetFilters() {
      return initialState
    },
  },
})

export const {
  setClassId,
  setMinConfidence,
  setLocationFilter,
  setLocationRadius,
  clearLocationFilter,
  resetFilters,
} = filtersSlice.actions
export default filtersSlice.reducer
