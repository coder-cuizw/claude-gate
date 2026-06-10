import { App, Button, Card, Col, Divider, Form, Input, InputNumber, Row, Segmented, Select, Slider, Switch } from 'antd'
import { SaveOutlined } from '@ant-design/icons'
import { ThemeToggle } from '../components/ThemeToggle'

/** 系统设置：并发/限流、采样、存储、告警、主题（任务书 §2.1 / §5.6 / §6）。 */
export function Settings() {
  const { message } = App.useApp()

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <Card title="并发与连接池" className="cg-soft-card" styles={{ body: { padding: 18 } }} extra={<span style={{ fontSize: 12, color: 'var(--cg-text-secondary)' }}>16 核 / 32 GB 推荐基线</span>}>
            <Form layout="vertical">
              <Row gutter={12}>
                <Col span={12}><Form.Item label="全局并发上限"><InputNumber min={0} style={{ width: '100%' }} defaultValue={6000} /></Form.Item></Col>
                <Col span={12}><Form.Item label="每通道并发上限"><InputNumber min={0} style={{ width: '100%' }} defaultValue={2000} /></Form.Item></Col>
                <Col span={12}><Form.Item label="落盘 Worker 数"><InputNumber min={1} style={{ width: '100%' }} defaultValue={16} /></Form.Item></Col>
                <Col span={12}><Form.Item label="上游连接池上限"><InputNumber min={1} style={{ width: '100%' }} defaultValue={256} /></Form.Item></Col>
              </Row>
              <Form.Item label="全链路超时（秒）"><InputNumber min={1} style={{ width: '100%' }} defaultValue={600} /></Form.Item>
            </Form>
          </Card>
        </Col>

        <Col xs={24} lg={12}>
          <Card title="落盘采样与存储" className="cg-soft-card" styles={{ body: { padding: 18 } }}>
            <Form layout="vertical">
              <Form.Item label="成功请求采样率">
                <Slider min={0} max={0.1} step={0.005} defaultValue={0.01} marks={{ 0: '0%', 0.01: '1%', 0.05: '5%', 0.1: '10%' }} tooltip={{ formatter: (v) => `${((v ?? 0) * 100).toFixed(1)}%` }} />
              </Form.Item>
              <Form.Item label="错误请求 100% 留存" valuePropName="checked"><Switch defaultChecked /></Form.Item>
              <Row gutter={12}>
                <Col span={12}><Form.Item label="S3 Bucket"><Input defaultValue="claude-gate" /></Form.Item></Col>
                <Col span={12}><Form.Item label="Body TTL（天）"><InputNumber min={1} style={{ width: '100%' }} defaultValue={30} /></Form.Item></Col>
              </Row>
            </Form>
          </Card>
        </Col>

        <Col xs={24} lg={12}>
          <Card title="告警 Webhook" className="cg-soft-card" styles={{ body: { padding: 18 } }}>
            <Form layout="vertical">
              <Form.Item label="告警渠道">
                <Segmented options={['飞书', '钉钉', 'Slack']} defaultValue="飞书" />
              </Form.Item>
              <Form.Item label="Webhook URL"><Input placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/..." /></Form.Item>
              <Form.Item label="磁盘水位告警阈值">
                <Slider min={50} max={95} defaultValue={85} marks={{ 50: '50%', 85: '85%', 95: '95%' }} tooltip={{ formatter: (v) => `${v}%` }} />
              </Form.Item>
            </Form>
          </Card>
        </Col>

        <Col xs={24} lg={12}>
          <Card title="外观与本地化" className="cg-soft-card" styles={{ body: { padding: 18 } }}>
            <Form layout="vertical">
              <Form.Item label="主题模式" extra="支持明亮 / 暗黑 / 跟随系统三态自适应">
                <ThemeToggle />
              </Form.Item>
              <Form.Item label="界面语言">
                <Select defaultValue="zh-CN" options={[{ label: '简体中文', value: 'zh-CN' }, { label: 'English', value: 'en-US' }]} style={{ width: 200 }} />
              </Form.Item>
              <Divider style={{ margin: '12px 0' }} />
              <div style={{ fontSize: 12.5, color: 'var(--cg-text-secondary)' }}>
                claude-gate v0.1.0 · 对外协议恒为 Anthropic Messages
              </div>
            </Form>
          </Card>
        </Col>
      </Row>

      <div>
        <Button type="primary" icon={<SaveOutlined />} onClick={() => message.success('系统设置已保存（演示）')}>
          保存设置
        </Button>
      </div>
    </div>
  )
}
