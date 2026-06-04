import { configureStore } from '@reduxjs/toolkit'
import filtersReducer from '../features/filters/filtersSlice'
import selectedPhotoReducer from '../features/gallery/selectedPhotoSlice'

export const store = configureStore({
  reducer: {
    filters: filtersReducer,
    selectedPhoto: selectedPhotoReducer,
  },
})

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch
