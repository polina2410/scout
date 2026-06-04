import { createSlice } from '@reduxjs/toolkit'
import type { PayloadAction } from '@reduxjs/toolkit'

interface SelectedPhotoState {
  photoId: string | null
}

const initialState: SelectedPhotoState = {
  photoId: null,
}

const selectedPhotoSlice = createSlice({
  name: 'selectedPhoto',
  initialState,
  reducers: {
    selectPhoto(state, action: PayloadAction<string>) {
      state.photoId = action.payload
    },
    clearSelectedPhoto(state) {
      state.photoId = null
    },
  },
})

export const { selectPhoto, clearSelectedPhoto } = selectedPhotoSlice.actions
export default selectedPhotoSlice.reducer
