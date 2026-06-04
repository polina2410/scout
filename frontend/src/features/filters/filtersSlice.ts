import { createSlice } from '@reduxjs/toolkit'
import type { PayloadAction } from '@reduxjs/toolkit'
import type { ClassId } from './types'

interface FiltersState {
  classId: ClassId | null
  minConfidence: number
}

const initialState: FiltersState = {
  classId: null,
  minConfidence: 0,
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
    resetFilters() {
      return initialState
    },
  },
})

export const { setClassId, setMinConfidence, resetFilters } = filtersSlice.actions
export default filtersSlice.reducer
