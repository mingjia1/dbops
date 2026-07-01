import { useEffect, useState } from 'react'
import { Spin } from 'antd'
import { Navigate, useLocation } from 'react-router-dom'
import { authApi } from '../services/api'

interface ProtectedRouteProps {
  children: React.ReactNode
}

const ProtectedRoute: React.FC<ProtectedRouteProps> = ({ children }) => {
  const location = useLocation()
  const token = localStorage.getItem('token')
  const [checking, setChecking] = useState(true)
  const [valid, setValid] = useState(false)

  // P1: 之前只检查 token 存在, 任意字符串 (e.g. "abc") 都能进所有页面,
  // 直到第一个 401 API 调用才被发现. 这里加最基础的格式校验:
  //  - 非空
  //  - 长度合理 (20+ 字符, 覆盖 jwt HS256 最小签名长度)
  //  - 看起来像 JWT (3 段 base64 用 . 分隔) 或后端自定义格式
  // 不做真实验签 (后端才是真理), 只挡明显占位/手敲的值.
  function isLikelyToken(s: string): boolean {
    if (s.length < 20 || s.length > 4096) return false
    // JWT: header.payload.signature (3 段). backend 也可能用别的格式,
    // 至少要求前 20 字符是 [A-Za-z0-9._-] 即可.
    return /^[A-Za-z0-9._\-]+$/.test(s)
  }

  useEffect(() => {
    if (!token || !isLikelyToken(token)) {
      setChecking(false)
      setValid(false)
      return
    }
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
  }, [token])

  if (!token || !isLikelyToken(token)) {
    return <Navigate to="/login" state={{ from: location }} replace />
  }

  if (checking) {
    return <div style={{ padding: 32, textAlign: 'center' }}><Spin size="large" /></div>
  }

  if (!valid) {
    return <Navigate to="/login" state={{ from: location }} replace />
  }

  return <>{children}</>
}

export default ProtectedRoute
