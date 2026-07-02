import { beforeEach, describe, expect, it } from 'vitest'
import {
  clearSecondaryPassword,
  clearSecondaryPasswordVerification,
  getDefaultMySQLCredential,
  isSecondaryPasswordEnabled,
  isSecondaryPasswordVerified,
  setDefaultMySQLCredential,
  setSecondaryPassword,
  verifySecondaryPassword,
} from './sessionSecrets'

describe('sessionSecrets', () => {
  beforeEach(() => {
    sessionStorage.clear()
    clearSecondaryPassword()
    setDefaultMySQLCredential({ username: 'root', password: '' })
  })

  it('stores only verification state for the secondary password flow', async () => {
    await setSecondaryPassword('s3cret-pass')

    const raw = sessionStorage.getItem('dbops_secondary_password_state')
    expect(raw).toBeTruthy()
    expect(raw).not.toContain('s3cret-pass')
    expect(isSecondaryPasswordEnabled()).toBe(true)
    expect(isSecondaryPasswordVerified()).toBe(false)

    await expect(verifySecondaryPassword('wrong-pass')).resolves.toBe(false)
    await expect(verifySecondaryPassword('s3cret-pass')).resolves.toBe(true)
    expect(isSecondaryPasswordVerified()).toBe(true)
  })

  it('keeps default mysql credentials out of browser storage', () => {
    setDefaultMySQLCredential({ username: 'admin', password: 'db-pass' })

    expect(getDefaultMySQLCredential()).toEqual({ username: 'admin', password: 'db-pass' })
    expect(localStorage.length).toBe(0)
    expect(sessionStorage.getItem('mysql_credential')).toBeNull()
  })

  it('clears verification state independently', async () => {
    await setSecondaryPassword('s3cret-pass')
    await verifySecondaryPassword('s3cret-pass')

    clearSecondaryPasswordVerification()

    expect(isSecondaryPasswordVerified()).toBe(false)
    expect(isSecondaryPasswordEnabled()).toBe(true)
  })
})
