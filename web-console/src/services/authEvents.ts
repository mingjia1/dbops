type LogoutHandler = () => void

const LOGOUT_EVENT = 'auth:logout'

export function triggerLogout(): void {
  window.dispatchEvent(new CustomEvent(LOGOUT_EVENT))
}

export function onLogout(handler: LogoutHandler): () => void {
  const wrapped = () => handler()
  window.addEventListener(LOGOUT_EVENT, wrapped)
  return () => window.removeEventListener(LOGOUT_EVENT, wrapped)
}
