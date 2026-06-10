import type { ThemeConfig } from 'antd'
import { theme as antdTheme } from 'antd'

/**
 * Claude 官网风格的设计令牌。
 *
 * 取色参考 Anthropic 品牌色系：赤陶色（book cloth / crail）作为主强调色，
 * 暖米色（ivory / cloud）作为亮色背景，暖炭黑（slate）作为暗色背景。
 * 标题用衬线体（Fraunces）营造编辑感，正文用 Inter。
 */
export type ThemeMode = 'light' | 'dark'

/** 品牌调色板（与 antd token 解耦，供自定义组件直接引用）。 */
export const palette = {
  light: {
    // 背景层次：页面 < 容器 < 浮层
    bgLayout: '#F5F4EE', // 暖米色页面底
    bgContainer: '#FCFBF8', // 卡片
    bgElevated: '#FFFFFF', // 弹层
    bgSubtle: '#EFEDE4', // 次级填充（hover、表头）
    // 文本
    text: '#1F1E1D',
    textSecondary: '#6B6862',
    textTertiary: '#928E85',
    // 描边
    border: '#E5E2D6',
    borderStrong: '#D6D2C4',
    // 强调色（赤陶）
    accent: '#C45A35',
    accentHover: '#A94A28',
    accentSoft: '#F2E4DC',
    // 语义色（偏暖处理）
    success: '#5B8C5A',
    warning: '#C9952B',
    error: '#C0492F',
    info: '#5B7C99',
  },
  dark: {
    bgLayout: '#1A1916', // 暖炭黑页面底
    bgContainer: '#23221E', // 卡片
    bgElevated: '#2A2925', // 弹层
    bgSubtle: '#2F2E29', // 次级填充
    text: '#F2EFE6',
    textSecondary: '#B7B2A6',
    textTertiary: '#8A857A',
    border: '#3A382F',
    borderStrong: '#4A483E',
    accent: '#D97757', // 暗色下提亮的赤陶
    accentHover: '#E08B6E',
    accentSoft: '#3A2A22',
    success: '#7BAE79',
    warning: '#D9AD52',
    error: '#D9694B',
    info: '#7E9DB8',
  },
} as const

/** 通道类型 → 标签颜色（在明暗下都用语义色，让大盘对比一致）。 */
export const channelColors: Record<string, string> = {
  kiro: '#C45A35',
  official: '#5B7C99',
  relay: '#8A6FB0',
  custom: '#5B8C5A',
}

const fontStack =
  "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif"

/** 根据模式生成 antd 主题配置。 */
export function buildTheme(mode: ThemeMode): ThemeConfig {
  const c = palette[mode]
  return {
    algorithm: mode === 'dark' ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
    token: {
      colorPrimary: c.accent,
      colorInfo: c.info,
      colorSuccess: c.success,
      colorWarning: c.warning,
      colorError: c.error,

      colorBgLayout: c.bgLayout,
      colorBgContainer: c.bgContainer,
      colorBgElevated: c.bgElevated,

      colorText: c.text,
      colorTextSecondary: c.textSecondary,
      colorTextTertiary: c.textTertiary,
      colorBorder: c.border,
      colorBorderSecondary: c.border,

      fontFamily: fontStack,
      fontSize: 14,
      borderRadius: 10,
      borderRadiusLG: 14,
      borderRadiusSM: 7,
      wireframe: false,
      controlHeight: 36,
      lineWidth: 1,
      boxShadow:
        mode === 'dark'
          ? '0 4px 16px rgba(0,0,0,0.4)'
          : '0 4px 16px rgba(31,30,29,0.06)',
      boxShadowSecondary:
        mode === 'dark'
          ? '0 6px 24px rgba(0,0,0,0.5)'
          : '0 8px 28px rgba(31,30,29,0.08)',
    },
    components: {
      Layout: {
        bodyBg: c.bgLayout,
        headerBg: c.bgContainer,
        siderBg: mode === 'dark' ? '#1F1E1A' : '#EFEDE4',
        headerHeight: 60,
        headerPadding: '0 24px',
      },
      Menu: {
        itemBg: 'transparent',
        subMenuItemBg: 'transparent',
        itemSelectedBg: c.accentSoft,
        itemSelectedColor: c.accent,
        itemHoverBg: mode === 'dark' ? '#2A2925' : '#E6E3D7',
        itemColor: c.textSecondary,
        itemHoverColor: c.text,
        itemHeight: 42,
        itemBorderRadius: 9,
        iconSize: 17,
        fontSize: 14,
      },
      Card: {
        colorBgContainer: c.bgContainer,
        paddingLG: 22,
        borderRadiusLG: 14,
      },
      Button: {
        controlHeight: 36,
        fontWeight: 500,
        primaryShadow: 'none',
        defaultShadow: 'none',
      },
      Table: {
        headerBg: c.bgSubtle,
        headerColor: c.textSecondary,
        rowHoverBg: mode === 'dark' ? '#2A2925' : '#F1EFE6',
        borderColor: c.border,
        cellPaddingBlock: 13,
      },
      Segmented: {
        itemSelectedBg: c.bgElevated,
        itemSelectedColor: c.accent,
        trackBg: c.bgSubtle,
        borderRadius: 9,
      },
      Statistic: {
        contentFontSize: 28,
      },
      Input: { controlHeight: 36 },
      Select: { controlHeight: 36 },
      Tabs: { inkBarColor: c.accent, itemSelectedColor: c.accent },
      Tag: { borderRadiusSM: 6 },
      Modal: { contentBg: c.bgElevated, headerBg: c.bgElevated },
      Tooltip: {},
    },
  }
}
