import { useState, useEffect, useCallback } from 'react'

const API_BASE = '/api/v1'

async function fetchSettings(): Promise<Record<string, string>> {
  const token = localStorage.getItem('token')
  const res = await fetch(`${API_BASE}/settings`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!res.ok) return {}
  const json = await res.json()
  return json?.data || {}
}

async function saveSetting(key: string, value: string): Promise<void> {
  const token = localStorage.getItem('token')
  await fetch(`${API_BASE}/settings/${key}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({ value }),
  })
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
