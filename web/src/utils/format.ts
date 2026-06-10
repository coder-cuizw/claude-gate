/** 千分位整数。 */
export const fmtInt = (n: number) => n.toLocaleString('en-US')

/** 大数值用中文计量（万 / 亿）。 */
export function fmtCompact(n: number): string {
  if (n >= 1e8) return (n / 1e8).toFixed(2) + ' 亿'
  if (n >= 1e4) return (n / 1e4).toFixed(1) + ' 万'
  return fmtInt(n)
}

/** 百分比。 */
export const fmtPct = (r: number, digits = 1) => (r * 100).toFixed(digits) + '%'

/** 毫秒。 */
export const fmtMs = (n: number) => `${fmtInt(Math.round(n))} ms`

/** 本地时间（HH:mm:ss）。 */
export const fmtTime = (iso: string) =>
  new Date(iso).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })

/** 本地日期时间。 */
export const fmtDateTime = (iso: string) =>
  new Date(iso).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
