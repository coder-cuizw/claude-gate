import { Avatar, Dropdown, Layout, Menu, Tag } from 'antd'
import {
  ApiOutlined,
  AppstoreOutlined,
  BarChartOutlined,
  ClusterOutlined,
  KeyOutlined,
  LogoutOutlined,
  ProfileOutlined,
  SettingOutlined,
} from '@ant-design/icons'
import { useMemo } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { Logo } from './Logo'
import { ThemeToggle } from './ThemeToggle'
import { useAuthStore } from '../store/auth'
import { resolveMode, systemPrefersDark, useThemeStore } from '../store/theme'

const { Sider, Header, Content } = Layout

const navItems = [
  { key: '/dashboard', icon: <BarChartOutlined />, label: '实时大盘' },
  { key: '/traces', icon: <ProfileOutlined />, label: '请求明细' },
  { key: '/groups', icon: <AppstoreOutlined />, label: '分组配置' },
  { key: '/channels', icon: <ClusterOutlined />, label: '上游通道' },
  { key: '/api-keys', icon: <KeyOutlined />, label: '客户 Key' },
  { key: '/settings', icon: <SettingOutlined />, label: '系统设置' },
]

const pageTitles: Record<string, string> = {
  '/dashboard': '实时大盘',
  '/traces': '请求明细',
  '/groups': '分组配置',
  '/channels': '上游通道与 Key 池',
  '/api-keys': '客户 API Key',
  '/settings': '系统设置',
}

export function AppLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const preference = useThemeStore((s) => s.preference)
  const dark = resolveMode(preference, systemPrefersDark()) === 'dark'

  const selectedKey = useMemo(() => {
    const match = navItems.find((i) => location.pathname.startsWith(i.key))
    return match?.key ?? '/dashboard'
  }, [location.pathname])

  const title = useMemo(() => {
    const k = Object.keys(pageTitles).find((p) => location.pathname.startsWith(p))
    return k ? pageTitles[k] : 'claude-gate'
  }, [location.pathname])

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        width={232}
        breakpoint="lg"
        collapsedWidth={0}
        style={{ borderRight: '1px solid var(--cg-border)', position: 'sticky', top: 0, height: '100vh' }}
      >
        <div style={{ height: 60, display: 'flex', alignItems: 'center', padding: '0 20px' }}>
          <Logo dark={dark} />
        </div>
        <Menu
          mode="inline"
          selectedKeys={[selectedKey]}
          onClick={(e) => navigate(e.key)}
          items={navItems}
          style={{ background: 'transparent', border: 'none', padding: '8px 12px' }}
        />
        <div style={{ position: 'absolute', bottom: 16, left: 0, right: 0, padding: '0 20px', color: 'var(--cg-text-secondary)', fontSize: 12 }}>
          <Tag bordered={false} color="default" style={{ borderRadius: 6 }}>
            v0.1.0 · 演示数据
          </Tag>
        </div>
      </Sider>

      <Layout>
        <Header
          className="cg-header-blur"
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            borderBottom: '1px solid var(--cg-border)',
            position: 'sticky',
            top: 0,
            zIndex: 10,
            background: 'color-mix(in srgb, var(--cg-bg-container) 82%, transparent)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 12 }}>
            <span className="cg-serif" style={{ fontSize: 19, fontWeight: 600 }}>
              {title}
            </span>
            <span style={{ color: 'var(--cg-text-secondary)', fontSize: 13 }}>Claude 中转网关控制台</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
            <ThemeToggle />
            <Dropdown
              menu={{
                items: [
                  { key: 'role', icon: <ApiOutlined />, label: `角色：${user?.role ?? 'admin'}`, disabled: true },
                  { type: 'divider' },
                  { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', danger: true },
                ],
                onClick: ({ key }) => {
                  if (key === 'logout') {
                    logout()
                    navigate('/login')
                  }
                },
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 9, cursor: 'pointer' }}>
                <Avatar size={32} style={{ background: 'var(--cg-accent)', fontWeight: 600 }}>
                  {(user?.email ?? 'A')[0].toUpperCase()}
                </Avatar>
                <span style={{ fontSize: 13, color: 'var(--cg-text-secondary)' }}>{user?.email ?? 'admin@claude-gate'}</span>
              </div>
            </Dropdown>
          </div>
        </Header>

        <Content style={{ padding: 24 }}>
          <div className="cg-fade-in" style={{ maxWidth: 1320, margin: '0 auto' }}>
            <Outlet />
          </div>
        </Content>
      </Layout>
    </Layout>
  )
}
