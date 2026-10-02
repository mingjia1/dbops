import '@testing-library/jest-dom/vitest'
import { vi } from 'vitest'

window.matchMedia = window.matchMedia || vi.fn(() => ({
  matches: false,
  addListener: vi.fn(),
  removeListener: vi.fn(),
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
  dispatchEvent: vi.fn(),
}))

window.getComputedStyle = window.getComputedStyle || vi.fn(() => ({
  getPropertyValue: vi.fn(),
})) as unknown as typeof window.getComputedStyle
