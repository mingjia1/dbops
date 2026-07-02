const secondaryPasswordStateKey = 'dbops_secondary_password_state'
const secondaryPasswordVerifiedKey = 'dbops_credential_verified'
const secondaryPasswordIterations = 150000

type SecondaryPasswordState = {
  salt: string
  hash: string
  iterations: number
}

type MySQLCredential = {
  username: string
  password: string
}

let mysqlCredentialState: MySQLCredential = { username: 'root', password: '' }

const textEncoder = new TextEncoder()

const bytesToBase64 = (bytes: Uint8Array) => {
  let binary = ''
  bytes.forEach((b) => { binary += String.fromCharCode(b) })
  return btoa(binary)
}

const base64ToBytes = (value: string) => {
  const binary = atob(value)
  const out = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i += 1) out[i] = binary.charCodeAt(i)
  return out
}

const derivePasswordHash = async (password: string, salt: Uint8Array, iterations: number) => {
  const keyMaterial = await crypto.subtle.importKey(
    'raw',
    textEncoder.encode(password),
    'PBKDF2',
    false,
    ['deriveBits']
  )
  const normalizedSalt = new Uint8Array(salt)
  const derived = await crypto.subtle.deriveBits(
    { name: 'PBKDF2', salt: normalizedSalt, iterations, hash: 'SHA-256' },
    keyMaterial,
    256
  )
  return bytesToBase64(new Uint8Array(derived))
}

const readSecondaryPasswordState = (): SecondaryPasswordState | null => {
  const raw = sessionStorage.getItem(secondaryPasswordStateKey)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as SecondaryPasswordState
    if (!parsed?.salt || !parsed?.hash || !parsed?.iterations) return null
    return parsed
  } catch {
    return null
  }
}

export const isSecondaryPasswordEnabled = () => readSecondaryPasswordState() !== null

export const isSecondaryPasswordVerified = () => sessionStorage.getItem(secondaryPasswordVerifiedKey) === '1'

export const clearSecondaryPasswordVerification = () => {
  sessionStorage.removeItem(secondaryPasswordVerifiedKey)
}

export const setSecondaryPassword = async (password: string) => {
  const salt = crypto.getRandomValues(new Uint8Array(16))
  const hash = await derivePasswordHash(password, salt, secondaryPasswordIterations)
  const state: SecondaryPasswordState = {
    salt: bytesToBase64(salt),
    hash,
    iterations: secondaryPasswordIterations,
  }
  sessionStorage.setItem(secondaryPasswordStateKey, JSON.stringify(state))
  clearSecondaryPasswordVerification()
}

export const clearSecondaryPassword = () => {
  sessionStorage.removeItem(secondaryPasswordStateKey)
  clearSecondaryPasswordVerification()
}

export const verifySecondaryPassword = async (password: string) => {
  const state = readSecondaryPasswordState()
  if (!state) return true
  const hash = await derivePasswordHash(password, base64ToBytes(state.salt), state.iterations)
  const matched = hash === state.hash
  if (matched) {
    sessionStorage.setItem(secondaryPasswordVerifiedKey, '1')
  }
  return matched
}

export const getDefaultMySQLCredential = (): MySQLCredential => ({ ...mysqlCredentialState })

export const setDefaultMySQLCredential = (value: MySQLCredential) => {
  mysqlCredentialState = { ...value }
}
