import { createSlice, createAsyncThunk } from '@reduxjs/toolkit'
import { instanceApi, Instance } from '@/services/api'

interface InstanceState {
  instances: Instance[]
  currentInstance: Instance | null
  loading: boolean
  error: string | null
}

const initialState: InstanceState = {
  instances: [],
  currentInstance: null,
  loading: false,
  error: null,
}

export const fetchInstances = createAsyncThunk(
  'instances/fetchAll',
  async (_, { rejectWithValue }) => {
    try {
      const response = await instanceApi.list()
      return response.data
    } catch (err: any) {
      return rejectWithValue(err.message)
    }
  }
)

export const createInstance = createAsyncThunk(
  'instances/create',
  async (data: any, { rejectWithValue }) => {
    try {
      const response = await instanceApi.create(data)
      return response.data
    } catch (err: any) {
      return rejectWithValue(err.message)
    }
  }
)

export const deleteInstance = createAsyncThunk(
  'instances/delete',
  async (id: string, { rejectWithValue }) => {
    try {
      await instanceApi.delete(id)
      return id
    } catch (err: any) {
      return rejectWithValue(err.message)
    }
  }
)

export const detectVersion = createAsyncThunk(
  'instances/detectVersion',
  async (id: string, { rejectWithValue }) => {
    try {
      const response = await instanceApi.detectVersion(id)
      return response.data
    } catch (err: any) {
      return rejectWithValue(err.message)
    }
  }
)

const instanceSlice = createSlice({
  name: 'instances',
  initialState,
  reducers: {
    setCurrentInstance: (state, action) => {
      state.currentInstance = action.payload
    },
    clearError: (state) => {
      state.error = null
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchInstances.pending, (state) => {
        state.loading = true
        state.error = null
      })
      .addCase(fetchInstances.fulfilled, (state, action) => {
        state.loading = false
        state.instances = action.payload
      })
      .addCase(fetchInstances.rejected, (state, action) => {
        state.loading = false
        state.error = action.payload as string
      })
      .addCase(createInstance.fulfilled, (state, action) => {
        state.instances.push(action.payload)
      })
      .addCase(deleteInstance.fulfilled, (state, action) => {
        state.instances = state.instances.filter((i) => i.id !== action.payload)
      })
  },
})

export const { setCurrentInstance, clearError } = instanceSlice.actions
export default instanceSlice.reducer