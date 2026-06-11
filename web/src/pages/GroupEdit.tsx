import { App, Button, Card, Col, Form, Input, InputNumber, List, Row, Select, Space, Switch, Tag } from 'antd'
import { ArrowLeftOutlined, HolderOutlined, SaveOutlined } from '@ant-design/icons'
import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { CacheStrategyEditor } from '../components/CacheStrategyEditor'
import { ChannelTag } from '../components/tags'
import { useChannels, useGroup, useSaveGroup } from '../api/queries'
import type { CacheStrategy, TransformerItem } from '../api/types'

const TRANSFORMER_LABELS: Record<string, string> = {
  model_mapper: '模型别名映射',
  tool_call_normalizer: '工具调用规整',
  system_prompt_injector: 'System Prompt 注入',
  streaming_event_fixer: '流式事件修复',
}

export function GroupEdit() {
  const { id = '1' } = useParams()
  const navigate = useNavigate()
  const { message } = App.useApp()
  const group = useGroup(Number(id))
  const channels = useChannels()
  const saveGroup = useSaveGroup()
  const [form] = Form.useForm()
  const [strategy, setStrategy] = useState<CacheStrategy>({ type: 'passthrough' })
  const [transformers, setTransformers] = useState<TransformerItem[]>([])

  useEffect(() => {
    if (group.data) {
      const g = group.data
      setStrategy(g.cache_strategy)
      setTransformers(g.transformer_config ?? [])
      form.setFieldsValue({
        name: g.name,
        description: g.description,
        channel_id: g.channel_id,
        enabled: g.enabled,
        rate_limit_config: g.rate_limit_config ?? {},
        retry_config: g.retry_config ?? {},
      })
    }
  }, [group.data, form])

  if (group.isLoading || !group.data) return <Card loading className="cg-soft-card" />
  const g = group.data

  const toggleTransformer = (name: string, enabled: boolean) =>
    setTransformers((list) => list.map((t) => (t.name === name ? { ...t, enabled } : t)))

  const onSave = async () => {
    const v = await form.validateFields()
    try {
      await saveGroup.mutateAsync({
        id: g.id,
        name: v.name,
        description: v.description,
        channel_id: v.channel_id,
        enabled: v.enabled,
        cache_strategy: strategy,
        transformer_config: transformers,
        rate_limit_config: v.rate_limit_config,
        retry_config: v.retry_config,
      })
      message.success('分组配置已保存')
    } catch (e) {
      message.error(e instanceof Error ? e.message : '保存失败')
    }
  }

  return (
    <Form form={form} layout="vertical">
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/groups')}>返回</Button>
          <span className="cg-serif" style={{ fontSize: 18, fontWeight: 600 }}>{g.name}</span>
          <ChannelTag type={g.channel_type} />
          <Button type="primary" icon={<SaveOutlined />} style={{ marginLeft: 'auto' }} loading={saveGroup.isPending} onClick={onSave}>
            保存配置
          </Button>
        </div>

        <Row gutter={[16, 16]}>
          <Col xs={24} lg={14}>
            <Card title="基础信息" className="cg-soft-card" styles={{ body: { padding: 18 } }}>
              <Row gutter={12}>
                <Col span={12}><Form.Item name="name" label="分组名称" rules={[{ required: true }]}><Input /></Form.Item></Col>
                <Col span={12}>
                  <Form.Item name="channel_id" label="绑定通道" extra="请认准 base_url，避免同类型通道选错">
                    <Select
                      showSearch
                      optionFilterProp="label"
                      options={(channels.data ?? []).map((c) => ({
                        label: `${c.name}（${c.type}） · ${c.base_url || '—'}`,
                        value: c.id,
                      }))}
                    />
                  </Form.Item>
                </Col>
              </Row>
              <Form.Item name="description" label="描述"><Input.TextArea rows={2} /></Form.Item>
              <Form.Item name="enabled" label="启用分组" valuePropName="checked"><Switch /></Form.Item>
            </Card>
          </Col>

          <Col xs={24} lg={10}>
            <Card title="限流与重试" className="cg-soft-card" styles={{ body: { padding: 18 } }} extra={<span style={{ fontSize: 12, color: 'var(--cg-text-secondary)' }}>留空或 0 = 不启用（纯透传）</span>}>
              <Row gutter={12}>
                <Col span={12}><Form.Item name={['rate_limit_config', 'rpm']} label="RPM（每分钟请求）" extra="0 = 不限流"><InputNumber min={0} placeholder="不限" style={{ width: '100%' }} /></Form.Item></Col>
                <Col span={12}><Form.Item name={['rate_limit_config', 'tpm']} label="TPM（每分钟 tokens）" extra="0 = 不限流"><InputNumber min={0} placeholder="不限" style={{ width: '100%' }} /></Form.Item></Col>
                <Col span={12}><Form.Item name={['retry_config', 'max_retries']} label="最大重试次数" extra="0 = 失败不重试"><InputNumber min={0} placeholder="不重试" style={{ width: '100%' }} /></Form.Item></Col>
                <Col span={12}><Form.Item name={['retry_config', 'backoff_ms']} label="重试退避 (ms)" extra="仅在重试次数 > 0 时生效"><InputNumber min={0} placeholder="0" style={{ width: '100%' }} /></Form.Item></Col>
              </Row>
            </Card>
          </Col>

          <Col span={24}>
            <Card title={<span>缓存计费策略 <Tag color="var(--cg-accent)" style={{ marginLeft: 6 }}>核心</Tag></span>} className="cg-soft-card" styles={{ body: { padding: 18 } }}>
              <CacheStrategyEditor value={strategy} onChange={setStrategy} />
            </Card>
          </Col>

          <Col span={24}>
            <Card
              title="Transformer 改写流水线"
              className="cg-soft-card"
              styles={{ body: { padding: 8 } }}
              extra={<span style={{ fontSize: 12.5, color: 'var(--cg-text-secondary)' }}>顺序即执行顺序，前序输出是后序输入</span>}
            >
              <List
                dataSource={transformers}
                locale={{ emptyText: '该分组未启用任何 Transformer' }}
                renderItem={(t, idx) => (
                  <List.Item actions={[<Switch key="sw" size="small" checked={t.enabled} onChange={(v) => toggleTransformer(t.name, v)} />]} style={{ padding: '12px 16px' }}>
                    <List.Item.Meta
                      avatar={<HolderOutlined style={{ color: 'var(--cg-text-tertiary,#928e85)', cursor: 'grab', marginTop: 4 }} />}
                      title={
                        <Space>
                          <span style={{ fontWeight: 500 }}>{idx + 1}. {TRANSFORMER_LABELS[t.name] ?? t.name}</span>
                          <Tag className="cg-mono" style={{ fontSize: 11 }}>{t.name}</Tag>
                        </Space>
                      }
                      description={t.params ? <span className="cg-mono" style={{ fontSize: 11.5, color: 'var(--cg-text-secondary)' }}>{JSON.stringify(t.params)}</span> : <span style={{ color: 'var(--cg-text-tertiary,#928e85)' }}>无参数</span>}
                    />
                  </List.Item>
                )}
              />
            </Card>
          </Col>
        </Row>
      </div>
    </Form>
  )
}
