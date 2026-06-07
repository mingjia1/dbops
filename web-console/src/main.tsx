import React, { useEffect, useState } from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import App from './App'
import { getStoredThemeMode, setStoredThemeMode, pickTheme, type ThemeMode } from './appTheme'
import './index.css'
import './styles/apple.css'

// P0: 之前 ConfigProvider 在 main.tsx 顶层 mount 时定死 pickTheme(initialMode),
// Dashboard 切主题只改 <html data-theme> + localStorage, ConfigProvider 永不重渲 →
// antd 组件仍是 light algorithm, 切暗只 50% 生效.
// 修: ThemeRoot 内部 state 管 themeMode, 用 useEffect 同步 <html data-theme>,
// ConfigProvider theme={pickTheme(mode)} 重渲. Dashboard Switch 通过
// 'app:theme-change' CustomEvent 通知 ThemeRoot.
function ThemeRoot() {
  const [mode, setMode] = useState<ThemeMode>(getStoredThemeMode)
  useEffect(() => {
    document.documentElement.dataset.theme = mode
  }, [mode])
  useEffect(() => {
    const onChange = (e: Event) => {
      const next = (e as CustomEvent<ThemeMode>).detail
      setMode(next)
      setStoredThemeMode(next)
    }
    window.addEventListener('app:theme-change', onChange)
    return () => window.removeEventListener('app:theme-change', onChange)
  }, [])
  return (
    <ConfigProvider locale={zhCN} theme={pickTheme(mode)}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </ConfigProvider>
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ThemeRoot />
  </React.StrictMode>,
)
