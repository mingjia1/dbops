import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import App from './App'
import { getStoredThemeMode, pickTheme } from './appTheme'
import './index.css'
import './styles/apple.css'

// 读 localStorage 决定初始主题; UI 切换走 App 内的 setStoredThemeMode.
// 这里把 'dark' className 加到 <html> 让全局 CSS 变量切换生效.
const initialMode = getStoredThemeMode()
document.documentElement.dataset.theme = initialMode

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ConfigProvider locale={zhCN} theme={pickTheme(initialMode)}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </ConfigProvider>
  </React.StrictMode>,
)
