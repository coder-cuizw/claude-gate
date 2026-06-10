import { Button, Card, Form, Input, Typography } from 'antd'
import { LockOutlined, MailOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { Logo, Sunburst } from '../components/Logo'
import { ThemeToggle } from '../components/ThemeToggle'
import { useAuthStore } from '../store/auth'
import { resolveMode, systemPrefersDark, useThemeStore } from '../store/theme'

export function Login() {
  const navigate = useNavigate()
  const login = useAuthStore((s) => s.login)
  const preference = useThemeStore((s) => s.preference)
  const dark = resolveMode(preference, systemPrefersDark()) === 'dark'

  const onFinish = (v: { email: string; password: string }) => {
    // 演示环境：任意账号即可登录，签发一个本地 token
    login('demo-token', { email: v.email || 'admin@claude-gate.io', role: 'admin' })
    navigate('/dashboard')
  }

  return (
    <div className="cg-login-bg">
      <div style={{ position: 'fixed', top: 20, right: 24 }}>
        <ThemeToggle />
      </div>

      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 }}>
        <div className="cg-fade-in" style={{ width: 412, maxWidth: '100%' }}>
          {/* 品牌区 */}
          <div style={{ textAlign: 'center', marginBottom: 26 }}>
            <div style={{ display: 'inline-flex', marginBottom: 18 }}>
              <Sunburst size={52} color={dark ? '#D97757' : '#C45A35'} />
            </div>
            <Typography.Title level={2} className="cg-serif" style={{ margin: 0, fontWeight: 600 }}>
              欢迎回到 claude·gate
            </Typography.Title>
            <Typography.Paragraph style={{ color: 'var(--cg-text-secondary)', marginTop: 8, marginBottom: 0, fontSize: 14.5 }}>
              面向 Claude 系列模型的可编程中转网关 · 控制台
            </Typography.Paragraph>
          </div>

          <Card className="cg-soft-card" style={{ borderRadius: 16 }} styles={{ body: { padding: 28 } }}>
            <Form layout="vertical" requiredMark={false} onFinish={onFinish} initialValues={{ email: 'admin@claude-gate.io', password: 'demo-password' }}>
              <Form.Item name="email" label="邮箱" rules={[{ required: true, message: '请输入邮箱' }]}>
                <Input size="large" prefix={<MailOutlined style={{ color: 'var(--cg-text-secondary)' }} />} placeholder="admin@claude-gate.io" />
              </Form.Item>
              <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]} style={{ marginBottom: 18 }}>
                <Input.Password size="large" prefix={<LockOutlined style={{ color: 'var(--cg-text-secondary)' }} />} placeholder="••••••••••" />
              </Form.Item>
              <Button type="primary" htmlType="submit" size="large" block style={{ fontWeight: 600 }}>
                登录控制台
              </Button>
            </Form>

            <div style={{ marginTop: 18, textAlign: 'center', fontSize: 12.5, color: 'var(--cg-text-tertiary, #928e85)' }}>
              演示环境：任意账号密码即可进入
            </div>
          </Card>

          <div style={{ marginTop: 22, display: 'flex', justifyContent: 'center', opacity: 0.7 }}>
            <Logo size={18} dark={dark} />
          </div>
        </div>
      </div>
    </div>
  )
}
