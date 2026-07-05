import { useState, useEffect, useCallback } from 'react'

const API_BASE = '/api/v1'

const buildAuthHeaders = (): Record<string, string> => {
  const token = localStorage.getItem('token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

async function readJson(res: Response): Promise<any> {
  try {
    return await res.json()
  } catch {
    return null
  }
}

async function fetchSettings(): Promise<Record<string, string>> {
  const res = await fetch(`${API_BASE}/settings`, {
    credentials: 'include',
    headers: buildAuthHeaders(),
  })
  if (!res.ok) return {}
  const json = await res.json()
  return json?.data || {}
}

async function saveSetting(key: string, value: string): Promise<void> {
  const res = await fetch(`${API_BASE}/settings/${key}`, {
    method: 'PUT',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...buildAuthHeaders() },
    body: JSON.stringify({ value }),
  })
  const json = await readJson(res)
  if (!res.ok || (json?.code && json.code !== 200)) {
    throw new Error(json?.message || `failed to save setting ${key}`)
  }
}

export function usePlatformSetting(key: string, defaultValue: string = '') {
  const [value, setValue] = useState<string>(defaultValue)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchSettings().then((all) => {
      if (key in all) setValue(all[key])
      setLoading(false)
    }).catch(() => setLoading(false))
  }, [key])

  const save = useCallback(async (newValue: string) => {
    await saveSetting(key, newValue)
    setValue(newValue)
  }, [key])

  return { value, setValue, save, loading }
}

export function usePlatformSettings() {
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(true)

  const reload = useCallback(async () => {
    setLoading(true)
    try {
      const all = await fetchSettings()
      setSettings(all)
    } catch { /* ignore */ }
    setLoading(false)
  }, [])

  useEffect(() => { reload() }, [reload])

  const save = useCallback(async (key: string, value: string) => {
    await saveSetting(key, value)
    setSettings((prev) => ({ ...prev, [key]: value }))
  }, [])

  const saveAll = useCallback(async (entries: Record<string, string>) => {
    for (const [k, v] of Object.entries(entries)) {
      await saveSetting(k, v)
    }
    setSettings((prev) => ({ ...prev, ...entries }))
  }, [])

  return { settings, loading, reload, save, saveAll }
}
