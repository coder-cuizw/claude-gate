import { App, Button, Card, Col, Divider, Form, Input, Row } from 'antd'
import { LockOutlined } from '@ant-design/icons'
import { ThemeToggle } from '../components/ThemeToggle'
import { useChangePassword } from '../api/queries'
import { useAuthStore } from '../store/auth'

/** 系统设置：管理员改密 + 外观主题。运行时参数（并发/采样/存储）以服务端 .env 为准。 */
export function Settings() {
  const { message } = App.useApp()
  const [form] = Form.useForm()
  const changePwd = useChangePassword()
  const logout = useAuthStore((s) => s.logout)

  const submit = async () => {
    const v = await form.validateFields()
    if (v.new_password !== v.confirm_password) {
      message.error('两次输入的新密码不一致')
      return
    }
    try {
      await changePwd.mutateAsync({ old_password: v.old_password, new_password: v.new_password })
      message.success('密码修改成功，请重新登录')
      form.resetFields()
      setTimeout(logout, 800)
    } catch (e: unknown) {
      const msg = (e as { message?: string })?.message || '修改失败'
      message.error(msg)
    }
  }

  return (
    <Row gutter={[16, 16]}>
      <Col xs={24} lg={14}>
        <Card title="修改管理员密码" className="cg-soft-card" styles={{ body: { padding: 18 } }}>
          <Form form={form} layout="vertical" autoComplete="off">
            <Form.Item
              label="当前密码"
              name="old_password"
              rules={[{ required: true, message: '请输入当前密码' }]}
            >
              <Input.Password prefix={<LockOutlined />} placeholder="当前密码" autoComplete="current-password" />
            </Form.Item>
            <Form.Item
              label="新密码"
              name="new_password"
              rules={[
                { required: true, message: '请输入新密码' },
                { min: 8, message: '新密码至少 8 位' },
              ]}
            >
              <Input.Password prefix={<LockOutlined />} placeholder="至少 8 位" autoComplete="new-password" />
            </Form.Item>
            <Form.Item
              label="确认新密码"
              name="confirm_password"
              dependencies={['new_password']}
              rules={[
                { required: true, message: '请再次输入新密码' },
                ({ getFieldValue }) => ({
                  validator(_, value) {
                    if (!value || getFieldValue('new_password') === value) return Promise.resolve()
                    return Promise.reject(new Error('两次输入不一致'))
                  },
                }),
              ]}
            >
              <Input.Password prefix={<LockOutlined />} placeholder="再次输入新密码" autoComplete="new-password" />
            </Form.Item>
            <Divider style={{ margin: '12px 0' }} />
            <Button type="primary" loading={changePwd.isPending} onClick={submit}>
              保存并重新登录
            </Button>
          </Form>
        </Card>
      </Col>

      <Col xs={24} lg={10}>
        <Card title="外观" className="cg-soft-card" styles={{ body: { padding: 18 } }}>
          <Form layout="vertical">
            <Form.Item label="主题模式" extra="支持明亮 / 暗黑 / 跟随系统三态自适应">
              <ThemeToggle />
            </Form.Item>
            <Divider style={{ margin: '12px 0' }} />
            <div style={{ fontSize: 12.5, color: 'var(--cg-text-secondary)' }}>
              claude-gate v0.1.0 · 对外协议恒为 Anthropic Messages
            </div>
            <div style={{ fontSize: 12, color: 'var(--cg-text-secondary)', marginTop: 8 }}>
              运行时参数（并发上限、采样率、存储等）以服务端 .env 与 yaml 配置为准，请联系运维调整。
            </div>
          </Form>
        </Card>
      </Col>
    </Row>
  )
}
