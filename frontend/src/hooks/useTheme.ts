import { useState, useCallback } from 'react'
import { getStoredThemeMode, type ThemeMode } from '../appTheme'

const THEME_CHANGE_EVENT = 'app:theme-change'

/** Hook for managing light/dark theme toggle. */
export function useTheme() {
  const [themeMode, setThemeMode] = useState<ThemeMode>(getStoredThemeMode)

  const toggleTheme = useCallback((checked: boolean) => {
    const next: ThemeMode = checked ? 'dark' : 'light'
    setThemeMode(next)
    window.dispatchEvent(new CustomEvent<ThemeMode>(THEME_CHANGE_EVENT, { detail: next }))
  }, [])

  return { themeMode, toggleTheme }
}
