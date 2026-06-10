import { palette } from '../theme/tokens'

/** Claude 风格的赤陶色光芒标记 + 字标。 */
export function Logo({ size = 26, showText = true, dark }: { size?: number; showText?: boolean; dark?: boolean }) {
  const accent = dark ? palette.dark.accent : palette.light.accent
  const textColor = dark ? palette.dark.text : palette.light.text
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 11 }}>
      <Sunburst size={size} color={accent} />
      {showText && (
        <span
          className="cg-serif"
          style={{ fontSize: size * 0.74, fontWeight: 600, color: textColor, letterSpacing: '-0.02em' }}
        >
          claude<span style={{ color: accent }}>·</span>gate
        </span>
      )}
    </div>
  )
}

/** 16 道光芒的星芒图形，呼应 Claude 视觉。 */
export function Sunburst({ size = 26, color = '#C45A35' }: { size?: number; color?: string }) {
  const spokes = 12
  const cx = size / 2
  const cy = size / 2
  const rOuter = size / 2
  const rInner = size * 0.13
  const lines = Array.from({ length: spokes }, (_, i) => {
    const a = (Math.PI * 2 * i) / spokes
    return {
      x1: cx + Math.cos(a) * rInner,
      y1: cy + Math.sin(a) * rInner,
      x2: cx + Math.cos(a) * rOuter,
      y2: cy + Math.sin(a) * rOuter,
    }
  })
  return (
    <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} aria-hidden>
      {lines.map((l, i) => (
        <line
          key={i}
          x1={l.x1}
          y1={l.y1}
          x2={l.x2}
          y2={l.y2}
          stroke={color}
          strokeWidth={size * 0.085}
          strokeLinecap="round"
        />
      ))}
    </svg>
  )
}
