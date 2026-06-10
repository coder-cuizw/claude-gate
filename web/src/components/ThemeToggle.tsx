import { Segmented, Tooltip } from 'antd'
import { DesktopOutlined, MoonOutlined, SunOutlined } from '@ant-design/icons'
import { useThemeStore, type ThemePreference } from '../store/theme'

/** 明亮 / 暗黑 / 跟随系统三态切换。 */
export function ThemeToggle({ compact = false }: { compact?: boolean }) {
  const preference = useThemeStore((s) => s.preference)
  const setPreference = useThemeStore((s) => s.setPreference)

  return (
    <Tooltip title="主题：明亮 / 暗黑 / 跟随系统">
      <Segmented<ThemePreference>
        value={preference}
        onChange={setPreference}
        size={compact ? 'small' : 'middle'}
        options={[
          { value: 'light', icon: <SunOutlined />, label: compact ? undefined : '明亮' },
          { value: 'dark', icon: <MoonOutlined />, label: compact ? undefined : '暗黑' },
          { value: 'system', icon: <DesktopOutlined />, label: compact ? undefined : '系统' },
        ]}
      />
    </Tooltip>
  )
}
