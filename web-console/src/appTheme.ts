import { theme, type ThemeConfig } from 'antd'

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
