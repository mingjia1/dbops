import { theme, type ThemeConfig } from 'antd'

// 主题模式: 'dark' (默认, 深空控制台) / 'light' (可切换).
export type ThemeMode = 'light' | 'dark'
const THEME_KEY = 'app.theme.mode'

export function getStoredThemeMode(): ThemeMode {
  try {
    const v = localStorage.getItem(THEME_KEY)
    return v === 'light' ? 'light' : 'dark'
  } catch {
    return 'dark'
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

// Dark variant: 深空控制台色板 (默认主题), 与 apple.css 的
// :root[data-theme="dark"] token 保持同色, 否则 Modal/Table/Select 会脱节.
export const darkTheme: ThemeConfig = {
  algorithm: theme.darkAlgorithm,
  token: {
    colorPrimary: '#3B9DFF',
    colorSuccess: '#34D399',
    colorWarning: '#FBBF24',
    colorError: '#FB7185',
    colorInfo: '#22D3EE',
    colorTextBase: '#E6EDF7',
    colorBgBase: '#070B14',
    colorBgLayout: '#070B14',
    colorBgContainer: '#0E1626',
    colorBgElevated: '#131E30',
    colorBorder: 'rgba(120, 160, 220, 0.14)',
    colorBorderSecondary: 'rgba(120, 160, 220, 0.10)',
    borderRadius: 10,
    borderRadiusLG: 12,
    borderRadiusSM: 6,
    fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display", "PingFang SC", "Helvetica Neue", Arial, "Segoe UI", sans-serif',
    fontSize: 14,
    lineHeight: 1.55,
    motionDurationMid: '0.16s',
    motionEaseInOut: 'cubic-bezier(0.2, 0, 0, 1)',
    boxShadow: '0 0 0 0.5px rgba(120,160,220,0.10), 0 1px 2px rgba(0,0,0,0.30)',
    boxShadowSecondary: '0 0 0 0.5px rgba(120,160,220,0.14), 0 4px 12px rgba(0,0,0,0.36)',
  },
  components: {
    Card: { borderRadiusLG: 12, paddingLG: 20 },
    Button: { borderRadius: 8, controlHeight: 36, fontWeight: 500 },
    Input: { borderRadius: 8, controlHeight: 36 },
    Select: { borderRadius: 8, controlHeight: 36 },
    Table: { borderRadius: 10, headerBorderRadius: 10 },
    Tabs: { titleFontSize: 15, horizontalItemGutter: 24 },
    Tag: { borderRadiusSM: 6 },
    Modal: { borderRadiusLG: 12 },
    Message: { borderRadiusLG: 10 },
  },
}

// palette 是 apple.css CSS 变量的语义别名, 不是第三套色板.
// 颜色由 :root / :root[data-theme="dark"] 统一驱动, 切换主题时图表、状态色、
// 渐变自动跟随, 组件无需各自读取 themeMode (也避开 useTheme 多实例状态不同步).
// 新增颜色请先在 apple.css 定义变量并补暗色覆写, 不要在此写字面色值.
export const palette = {
  series: {
    primary: 'var(--apple-blue)',
    success: 'var(--apple-green)',
    warning: 'var(--apple-orange)',
    danger:  'var(--apple-red)',
    info:    'var(--apple-teal)',
  },
  text: {
    healthy:   'var(--apple-status-healthy)',
    unhealthy: 'var(--apple-status-unhealthy)',
    stopped:   'var(--apple-text-tertiary)',
    warning:   'var(--apple-orange)',
  },
  gradient: {
    blueCloud:     'linear-gradient(135deg, var(--apple-blue), var(--apple-teal))',
    greenSafety:   'linear-gradient(135deg, var(--apple-green), var(--apple-teal))',
    orangeBolt:    'linear-gradient(135deg, var(--apple-orange), var(--apple-yellow))',
    purpleCluster: 'linear-gradient(135deg, var(--apple-purple), var(--apple-pink))',
  },
  accent: {
    blue:   'var(--apple-blue)',
    green:  'var(--apple-green)',
    orange: 'var(--apple-orange)',
    purple: 'var(--apple-purple)',
    red:    'var(--apple-red)',
    cyan:   'var(--apple-teal)',
    indigo: 'var(--apple-indigo)',
  },
} as const

// pickTheme 按 mode 选 light/dark antd theme. 提供给 main.tsx 动态切换.
export function pickTheme(mode: ThemeMode): ThemeConfig {
  return mode === 'dark' ? darkTheme : appleTheme
}

