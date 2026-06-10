import { Area, Column, Line, Pie } from '@ant-design/charts'
import { resolveMode, systemPrefersDark, useThemeStore } from '../store/theme'
import { palette } from '../theme/tokens'

/** 当前是否暗色，并据此给图表挑选配色与 G2 主题。 */
function useChartTheme() {
  const preference = useThemeStore((s) => s.preference)
  const dark = resolveMode(preference, systemPrefersDark()) === 'dark'
  const c = dark ? palette.dark : palette.light
  return { dark, c, g2Theme: dark ? 'classicDark' : 'classic' }
}

// 暖色系列调色板（与品牌一致）
const SERIES_COLORS = ['#C45A35', '#5B7C99', '#C9952B', '#5B8C5A', '#8A6FB0']

const baseAxisStyle = (color: string) => ({
  line: false,
  tick: false,
  grid: true,
  labelFill: color,
  labelFontSize: 11,
})

/** 单/多序列折线图。 */
export function LineChart({
  data,
  height = 240,
  color,
  multi,
}: {
  data: { timestamp: string; value: number; series?: string }[]
  height?: number
  color?: string
  multi?: boolean
}) {
  const { c, g2Theme } = useChartTheme()
  return (
    <Line
      className="cg-chart"
      data={data}
      height={height}
      theme={g2Theme}
      xField="timestamp"
      yField="value"
      colorField={multi ? 'series' : undefined}
      shapeField="smooth"
      scale={{ color: { range: color ? [color] : SERIES_COLORS } }}
      axis={{
        x: { ...baseAxisStyle(c.textTertiary), labelFilter: (_: unknown, i: number) => i % 6 === 0 },
        y: baseAxisStyle(c.textTertiary),
      }}
      legend={multi ? { color: { itemLabelFill: c.textSecondary, position: 'top', layout: { justifyContent: 'flex-end' } } } : false}
      style={{ lineWidth: 2 }}
      tooltip={{ title: (d: { timestamp: string }) => d.timestamp }}
    />
  )
}

/** 面积图（QPS / Token）。 */
export function AreaChart({
  data,
  height = 240,
  color = '#C45A35',
  multi,
}: {
  data: { timestamp: string; value: number; series?: string }[]
  height?: number
  color?: string
  multi?: boolean
}) {
  const { c, g2Theme } = useChartTheme()
  return (
    <Area
      className="cg-chart"
      data={data}
      height={height}
      theme={g2Theme}
      xField="timestamp"
      yField="value"
      colorField={multi ? 'series' : undefined}
      shapeField="smooth"
      stack={multi}
      scale={{ color: { range: multi ? SERIES_COLORS : [color] } }}
      style={{ fillOpacity: multi ? 0.55 : 0.18, lineWidth: 2 }}
      axis={{
        x: { ...baseAxisStyle(c.textTertiary), labelFilter: (_: unknown, i: number) => i % 6 === 0 },
        y: baseAxisStyle(c.textTertiary),
      }}
      legend={multi ? { color: { itemLabelFill: c.textSecondary, position: 'top', layout: { justifyContent: 'flex-end' } } } : false}
    />
  )
}

/** 柱状图（按通道对比）。 */
export function ColumnChart({
  data,
  xField,
  yField,
  height = 240,
}: {
  data: Record<string, unknown>[]
  xField: string
  yField: string
  height?: number
}) {
  const { c, g2Theme } = useChartTheme()
  return (
    <Column
      className="cg-chart"
      data={data}
      height={height}
      theme={g2Theme}
      xField={xField}
      yField={yField}
      colorField={xField}
      scale={{ color: { range: SERIES_COLORS } }}
      style={{ radiusTopLeft: 6, radiusTopRight: 6, maxWidth: 46 }}
      axis={{ x: baseAxisStyle(c.textTertiary), y: baseAxisStyle(c.textTertiary) }}
      legend={false}
    />
  )
}

/** 环形图（错误分布）。 */
export function DonutChart({
  data,
  angleField,
  colorField,
  height = 240,
}: {
  data: Record<string, unknown>[]
  angleField: string
  colorField: string
  height?: number
}) {
  const { c, g2Theme } = useChartTheme()
  return (
    <Pie
      className="cg-chart"
      data={data}
      height={height}
      theme={g2Theme}
      angleField={angleField}
      colorField={colorField}
      innerRadius={0.62}
      radius={0.9}
      scale={{ color: { range: SERIES_COLORS } }}
      label={false}
      legend={{ color: { itemLabelFill: c.textSecondary, position: 'right', layout: { justifyContent: 'center' } } }}
    />
  )
}
