import { useEffect, useState } from 'react'
import Spin from 'antd/es/spin'
import { Navigate, useLocation } from 'react-router-dom'
import { authApi } from '../services/api'

interface ProtectedRouteProps {
  children: React.ReactNode
}

const ProtectedRoute: React.FC<ProtectedRouteProps> = ({ children }) => {
  const location = useLocation()
  const [checking, setChecking] = useState(true)
  const [valid, setValid] = useState(false)

  useEffect(() => {
    // P1-B: 单轨鉴权 —— 会话是否存在由后端 cookie 会话 (/auth/me) 说了算,
    // 前端不再解析本地 token (历史版本遗留的 token 直接清掉).
    localStorage.removeItem('token')
    authApi.me()
      .then((res: any) => {
        if (res?.data) {
          localStorage.setItem('user', JSON.stringify(res.data))
          setValid(true)
        } else {
          setValid(false)
        }
      })
      .catch(() => setValid(false))
      .finally(() => setChecking(false))
  }, [])

  if (checking) {
    return <div style={{ padding: 32, textAlign: 'center' }}><Spin size="large" /></div>
  }

  if (!valid) {
    return <Navigate to="/login" state={{ from: location }} replace />
  }

  return <>{children}</>
}

export default ProtectedRoute
