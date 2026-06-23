import { theme, type ThemeConfig } from 'antd'

// 主题模式: 'light' (默认) / 'dark'.
export type ThemeMode = 'light' | 'dark'
const THEME_KEY = 'app.theme.mode'

export function getStoredThemeMode(): ThemeMode {
  try {
    const v = localStorage.getItem(THEME_KEY)
    return v === 'dark' ? 'dark' : 'light'
  } catch {
    return 'light'
  }
}

export function setStoredThemeMode(m: ThemeMode) {
  try {
    localStorage.setItem(THEME_KEY, m)
  } catch {
    /* ignore (e.g. private mode) */
  }
}

// Apple-inspired antd theme tokens.
export const appleTheme: ThemeConfig = {
  algorithm: theme.defaultAlgorithm,
  token: {
    colorPrimary: '#0071E3',
    colorSuccess: '#34C759',
    colorWarning: '#FF9500',
    colorError: '#FF3B30',
    colorInfo: '#5AC8FA',
    colorTextBase: '#1D1D1F',
    colorBgBase: '#FFFFFF',
    colorBgLayout: '#F5F5F7',
    colorBorder: 'rgba(60, 60, 67, 0.10)',
    colorBorderSecondary: 'rgba(60, 60, 67, 0.06)',
    borderRadius: 10,
    borderRadiusLG: 14,
    borderRadiusSM: 6,
    fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display", "PingFang SC", "Helvetica Neue", Arial, "Segoe UI", sans-serif',
    fontSize: 14,
    fontSizeHeading1: 28,
    fontSizeHeading2: 22,
    fontSizeHeading3: 18,
    lineHeight: 1.55,
    motionDurationMid: '0.28s',
    motionEaseInOut: 'cubic-bezier(0.32, 0.72, 0, 1)',
    boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
    boxShadowSecondary: '0 2px 8px rgba(0,0,0,0.06)',
  },
  components: {
    Card: {
      borderRadiusLG: 14,
      paddingLG: 20,
    },
    Button: {
      borderRadius: 8,
      controlHeight: 36,
      fontWeight: 500,
    },
    Input: {
      borderRadius: 8,
      controlHeight: 36,
    },
    Select: {
      borderRadius: 8,
      controlHeight: 36,
    },
    Table: {
      borderRadius: 10,
      headerBg: '#FAFAFC',
      headerColor: '#6E6E73',
      headerBorderRadius: 10,
    },
    Tabs: {
      titleFontSize: 15,
      horizontalItemGutter: 24,
    },
    Tag: {
      borderRadiusSM: 6,
    },
    Modal: {
      borderRadiusLG: 14,
    },
    Message: {
      borderRadiusLG: 10,
    },
  },
}

// Dark variant: algorithm=dark + 暗色 token 覆写.
// 同一组 palette (本文件下方定义) 在两套主题下都能用.
export const darkTheme: ThemeConfig = {
  algorithm: theme.darkAlgorithm,
  token: {
    colorPrimary: '#0A84FF',
    colorSuccess: '#30D158',
    colorWarning: '#FF9F0A',
    colorError: '#FF453A',
    colorInfo: '#64D2FF',
    colorTextBase: '#F5F5F7',
    colorBgBase: '#000000',
    colorBgLayout: '#1C1C1E',
    colorBgContainer: '#2C2C2E',
    borderRadius: 10,
    borderRadiusLG: 14,
    borderRadiusSM: 6,
    fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display", "PingFang SC", "Helvetica Neue", Arial, "Segoe UI", sans-serif',
    fontSize: 14,
    lineHeight: 1.55,
    motionDurationMid: '0.28s',
    motionEaseInOut: 'cubic-bezier(0.32, 0.72, 0, 1)',
    boxShadow: '0 1px 2px rgba(0,0,0,0.4)',
    boxShadowSecondary: '0 2px 8px rgba(0,0,0,0.5)',
  },
  components: {
    Card: { borderRadiusLG: 14, paddingLG: 20 },
    Button: { borderRadius: 8, controlHeight: 36, fontWeight: 500 },
    Input: { borderRadius: 8, controlHeight: 36 },
    Select: { borderRadius: 8, controlHeight: 36 },
    Table: { borderRadius: 10, headerBorderRadius: 10 },
    Tabs: { titleFontSize: 15, horizontalItemGutter: 24 },
    Tag: { borderRadiusSM: 6 },
    Modal: { borderRadiusLG: 14 },
    Message: { borderRadiusLG: 10 },
  },
}

// palette 集中存放硬编码颜色, 各页面引用而不是写死.
// 命名按用途 (series / gradient / status) 而不是字面颜色,
// 这样未来切暗主题时一处改全局生效.
export const palette = {
  series: {
    primary: '#1890ff',     // 接收/INSERT
    success: '#52c41a',     // 发送/UPDATE-success/SELECT
    warning: '#fa8c16',     // 慢查询/UPDATE-warning
    danger:  '#f5222d',     // DELETE/异常
    info:    '#13c2c2',
  },
  text: {
    healthy:   '#3f8600',
    unhealthy: '#cf1322',
    stopped:   '#8c8c8c',
    warning:   '#fa8c16',
  },
  gradient: {
    blueCloud:    'linear-gradient(135deg,#0071E3,#5AC8FA)',
    greenSafety:  'linear-gradient(135deg,#34C759,#30D158)',
    orangeBolt:   'linear-gradient(135deg,#FF9500,#FFCC00)',
    purpleCluster: 'linear-gradient(135deg,#AF52DE,#FF2D55)',
  },
  accent: {
    blue:   '#0071E3',
    green:  '#34C759',
    orange: '#FF9500',
    purple: '#AF52DE',
    red:    '#FF3B30',
    cyan:   '#5AC8FA',
  },
} as const

// pickTheme 按 mode 选 light/dark antd theme. 提供给 main.tsx 动态切换.
export function pickTheme(mode: ThemeMode): ThemeConfig {
  return mode === 'dark' ? darkTheme : appleTheme
}

