import { configureStore } from '@reduxjs/toolkit'
import instanceReducer from './instanceSlice'

export const store = configureStore({
  reducer: {
    instances: instanceReducer,
  },
})

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch