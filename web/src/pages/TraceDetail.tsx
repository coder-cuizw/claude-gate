import { App, Button, Card, Col, Descriptions, Form, Modal, Row, Select, Switch, Table, Tabs, Tag } from 'antd'
import { ArrowLeftOutlined, RedoOutlined, ThunderboltOutlined } from '@ant-design/icons'
import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ChannelTag, StatusTag } from '../components/tags'
import { useGroups, useTrace } from '../api/queries'
import type { Usage } from '../api/types'
import { fmtDateTime, fmtInt, fmtMs } from '../utils/format'

function CodeBlock({ value }: { value: unknown }) {
  return <div className="cg-code">{JSON.stringify(value, null, 2)}</div>
}

function UsageTable({ billed, upstream }: { billed: Usage; upstream: Usage }) {
  const rows = [
    { key: 'input', label: 'Input', b: billed.input_tokens, u: upstream.input_tokens },
    { key: 'output', label: 'Output', b: billed.output_tokens, u: upstream.output_tokens },
    { key: 'cc', label: 'Cache 创建', b: billed.cache_creation_tokens, u: upstream.cache_creation_tokens },
    { key: 'cr', label: 'Cache 读取', b: billed.cache_read_tokens, u: upstream.cache_read_tokens },
  ]
  return (
    <Table
      size="small"
      pagination={false}
      dataSource={rows}
      columns={[
        { title: '字段', dataIndex: 'label' },
        { title: '计费值（返回客户）', dataIndex: 'b', align: 'right', render: (v: number) => <b>{fmtInt(v)}</b> },
        { title: '上游真实值', dataIndex: 'u', align: 'right', render: (v: number) => <span style={{ color: 'var(--cg-text-secondary)' }}>{fmtInt(v)}</span> },
      ]}
    />
  )
}

export function TraceDetailPage() {
  const { traceId = '' } = useParams()
  const navigate = useNavigate()
  const { message } = App.useApp()
  const { data: t, isLoading } = useTrace(traceId)
  const groups = useGroups()
  const [replayOpen, setReplayOpen] = useState(false)
  const [form] = Form.useForm()

  if (isLoading || !t) {
    return <Card loading className="cg-soft-card" />
  }

  const doReplay = () => {
    setReplayOpen(false)
    message.success('已发起复现，新的 trace 正在执行（演示）')
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/traces')}>
          返回列表
        </Button>
        <span className="cg-mono" style={{ fontSize: 13, color: 'var(--cg-text-secondary)' }}>{t.trace_id}</span>
        <StatusTag success={t.is_success} code={t.status_code} />
        <Button type="primary" icon={<RedoOutlined />} style={{ marginLeft: 'auto' }} onClick={() => setReplayOpen(true)}>
          一键复现
        </Button>
      </div>

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={14}>
          <Card title="请求概览" className="cg-soft-card" styles={{ body: { padding: 18 } }}>
            <Descriptions column={2} size="small" colon={false} labelStyle={{ color: 'var(--cg-text-secondary)' }}>
              <Descriptions.Item label="时间">{fmtDateTime(t.request_at)}</Descriptions.Item>
              <Descriptions.Item label="分组">{t.group_name}</Descriptions.Item>
              <Descriptions.Item label="通道"><ChannelTag type={t.channel_type} /></Descriptions.Item>
              <Descriptions.Item label="模型"><span className="cg-mono" style={{ fontSize: 12 }}>{t.model}</span></Descriptions.Item>
              <Descriptions.Item label="客户 Key">{t.api_key_name}</Descriptions.Item>
              <Descriptions.Item label="上游 Key">{t.upstream_key_name}</Descriptions.Item>
              <Descriptions.Item label="类型">
                {t.is_streaming ? <Tag icon={<ThunderboltOutlined />} color="processing">流式</Tag> : <Tag>非流</Tag>}
              </Descriptions.Item>
              <Descriptions.Item label="首字 TTFT">
                {t.is_streaming
                  ? <span style={{ color: 'var(--cg-accent,#C45A35)', fontWeight: 600 }}>{fmtMs(t.ttft_ms)}</span>
                  : <span style={{ color: 'var(--cg-text-secondary)' }}>{fmtMs(t.ttft_ms)}（非流）</span>}
              </Descriptions.Item>
              <Descriptions.Item label="总耗时">{(t.duration_ms / 1000).toFixed(1)} s</Descriptions.Item>
              {!t.is_success && (
                <Descriptions.Item label="错误" span={2}>
                  <Tag color="error">{t.error_type}</Tag>
                  <span style={{ color: 'var(--cg-text-secondary)' }}>{t.error_message}</span>
                </Descriptions.Item>
              )}
            </Descriptions>
          </Card>
        </Col>
        <Col xs={24} lg={10}>
          <Card title="Usage 计量（计费值 vs 上游真实值）" className="cg-soft-card" styles={{ body: { padding: 18 } }}>
            <UsageTable billed={t.billed_usage} upstream={t.upstream_usage} />
          </Card>
        </Col>
      </Row>

      <Card className="cg-soft-card" styles={{ body: { padding: 18 } }}>
        <Tabs
          items={[
            { key: 'req', label: '请求 Body', children: <CodeBlock value={t.request_body} /> },
            { key: 'resp', label: '响应 Body', children: <CodeBlock value={t.response_body} /> },
            { key: 'meta', label: 'Meta（headers / 连接）', children: <CodeBlock value={t.meta} /> },
          ]}
        />
      </Card>

      <Modal
        title="一键复现请求"
        open={replayOpen}
        onCancel={() => setReplayOpen(false)}
        onOk={doReplay}
        okText="发起复现"
        destroyOnClose
      >
        <Form form={form} layout="vertical" initialValues={{ target_group_id: t.group_id, dry_run: false }}>
          <Form.Item name="target_group_id" label="目标分组（可指向不同通道做对比复现）">
            <Select options={(groups.data ?? []).map((g) => ({ label: `${g.name}（${g.channel_type}）`, value: g.id }))} />
          </Form.Item>
          <Form.Item name="override_model" label="覆盖模型（可选）">
            <Select allowClear placeholder="保持原模型" options={[
              { label: 'claude-3-5-sonnet-20241022', value: 'claude-3-5-sonnet-20241022' },
              { label: 'claude-3-5-haiku-20241022', value: 'claude-3-5-haiku-20241022' },
            ]} />
          </Form.Item>
          <Form.Item name="dry_run" label="Dry Run（仅解析不发送）" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
