import React from 'react'
import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import CapabilityMatrix from './CapabilityMatrix'

const renderWithRouter = (ui: React.ReactElement) =>
  render(<BrowserRouter>{ui}</BrowserRouter>)

describe('CapabilityMatrix', () => {
  it('renders the matrix title', () => {
    renderWithRouter(<CapabilityMatrix />)
    expect(screen.getByText('引擎能力矩阵')).toBeTruthy()
  })

  it('shows mysql protocol flavors', () => {
    renderWithRouter(<CapabilityMatrix />)
    for (const flavor of ['MYSQL', 'MARIADB', 'PERCONA']) {
      expect(screen.getByText(flavor)).toBeTruthy()
    }
  })

  it('shows tiered onboarding flavors', () => {
    renderWithRouter(<CapabilityMatrix />)
    for (const flavor of ['OCEANBASE', 'TIDB', 'GAUSSDB-MYSQL', 'POLARDB-MYSQL', 'TDSQL-MYSQL']) {
      expect(screen.getByText(flavor)).toBeTruthy()
    }
  })

  it('displays capability columns', () => {
    renderWithRouter(<CapabilityMatrix />)
    expect(screen.getByText('主从复制搭建')).toBeTruthy()
    expect(screen.getByText('故障切换')).toBeTruthy()
    expect(screen.getByText('集群架构部署')).toBeTruthy()
  })
})
