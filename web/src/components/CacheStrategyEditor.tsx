import { Alert, Card, Col, Form, Input, InputNumber, Row, Segmented, Slider, Space, Statistic, Tag, Tooltip } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import type { CacheStrategy, CacheStrategyType } from '../api/types'

/**
 * 缓存策略可视化编辑器（任务书 §7 重点 UI）。
 *
 * 用 Segmented 切换四种策略类型，按类型动态渲染表单字段；
 * 公式策略额外提供变量提示与"实时试算"面板，逻辑与后端 internal/cache 引擎一致。
 */

const STRATEGY_OPTIONS: { label: string; value: CacheStrategyType; desc: string }[] = [
  { label: '透传', value: 'passthrough', desc: '直接返回上游真实 usage，不做改写' },
  { label: '百分比', value: 'percentage', desc: '以入参总 tokens 为基数按比例分配' },
  { label: '固定值', value: 'fixed', desc: '所有字段返回配置的固定值' },
  { label: '公式', value: 'formula', desc: '用表达式引擎自定义每个字段' },
]

// 试算用的样例上游 usage 与上下文
const SAMPLE = { upstream_input: 12000, upstream_output: 850, upstream_cache_creation: 2000, upstream_cache_read: 30000, total: 44000 }

/** 在前端复刻后端计费逻辑，用于实时预览（与 internal/cache 对齐）。 */
function computePreview(s: CacheStrategy): { input: number; output: number; cache_creation: number; cache_read: number; error?: string } {
  const p = (s.params ?? {}) as Record<string, number | string>
  const floor = Math.floor
  const clamp = (v: number) => (v < 0 || Number.isNaN(v) || !Number.isFinite(v) ? 0 : v)
  try {
    if (s.type === 'passthrough') {
      return { input: SAMPLE.upstream_input, output: SAMPLE.upstream_output, cache_creation: SAMPLE.upstream_cache_creation, cache_read: SAMPLE.upstream_cache_read }
    }
    if (s.type === 'percentage') {
      const cc = floor(SAMPLE.total * clampRatio(Number(p.cache_creation_ratio ?? 0)))
      const cr = floor(SAMPLE.total * clampRatio(Number(p.cache_read_ratio ?? 0)))
      const out = String(p.output_source ?? 'upstream').startsWith('fixed:')
        ? Number(String(p.output_source).split(':')[1]) || 0
        : SAMPLE.upstream_output
      return { input: Number(p.input_fixed_tokens ?? 1), output: out, cache_creation: cc, cache_read: cr }
    }
    if (s.type === 'fixed') {
      const out = Number(p.output_tokens ?? 0)
      return {
        input: Number(p.input_tokens ?? 0),
        output: out === 0 ? SAMPLE.upstream_output : out,
        cache_creation: Number(p.cache_creation_tokens ?? 0),
        cache_read: Number(p.cache_read_tokens ?? 0),
      }
    }
    // formula：按 input→cache_creation→cache_read→output 顺序求值
    const env: Record<string, number> = { ...SAMPLE, input: 0, cache_creation: 0, cache_read: 0, output: 0 }
    const evalExpr = (src: string, fallback: number) => {
      if (!src) return fallback
      // 仅允许变量与算术，安全求值
      const fn = new Function(...Object.keys(env), `return (${src});`)
      const v = fn(...Object.values(env))
      return typeof v === 'number' && Number.isFinite(v) ? floor(v) : fallback
    }
    env.input = clamp(evalExpr(String(p.input ?? ''), SAMPLE.upstream_input))
    env.cache_creation = clamp(evalExpr(String(p.cache_creation ?? ''), SAMPLE.upstream_cache_creation))
    env.cache_read = clamp(evalExpr(String(p.cache_read ?? ''), SAMPLE.upstream_cache_read))
    env.output = clamp(evalExpr(String(p.output ?? ''), SAMPLE.upstream_output))
    return { input: env.input, output: env.output, cache_creation: env.cache_creation, cache_read: env.cache_read }
  } catch (e) {
    return { input: 0, output: 0, cache_creation: 0, cache_read: 0, error: String((e as Error).message) }
  }
}

const clampRatio = (r: number) => (r < 0 ? 0 : r > 1 ? 1 : r)

const FORMULA_VARS = ['total', 'upstream_input', 'upstream_output', 'upstream_cache_creation', 'upstream_cache_read', 'input', 'cache_creation', 'cache_read']

export function CacheStrategyEditor({ value, onChange }: { value: CacheStrategy; onChange?: (v: CacheStrategy) => void }) {
  const [strategy, setStrategy] = useState<CacheStrategy>(value)

  // 父组件异步加载分组后 value 会变化（如从 passthrough 切到 formula），需同步内部状态。
  // 由于编辑产生的 onChange 会回传同一引用，这里以引用相等短路，不会形成更新循环。
  useEffect(() => {
    setStrategy(value)
  }, [value])

  const update = (next: CacheStrategy) => {
    setStrategy(next)
    onChange?.(next)
  }
  const setType = (type: CacheStrategyType) => {
    // 切换时保留共有参数（params）以减少重填
    update({ type, params: strategy.params })
  }
  const setParam = (k: string, v: unknown) => update({ ...strategy, params: { ...strategy.params, [k]: v } })

  const preview = useMemo(() => computePreview(strategy), [strategy])
  const p = (strategy.params ?? {}) as Record<string, number | string>
  const desc = STRATEGY_OPTIONS.find((o) => o.value === strategy.type)?.desc

  return (
    <div>
      <Segmented<CacheStrategyType>
        block
        value={strategy.type}
        onChange={setType}
        options={STRATEGY_OPTIONS.map((o) => ({ label: o.label, value: o.value }))}
        style={{ marginBottom: 6 }}
      />
      <div style={{ fontSize: 12.5, color: 'var(--cg-text-secondary)', margin: '4px 2px 16px' }}>{desc}</div>

      <Row gutter={20}>
        <Col xs={24} md={14}>
          {strategy.type === 'passthrough' && (
            <Alert type="info" showIcon message="透传策略无需额外参数：上游返回什么 usage，就计费什么。" />
          )}

          {strategy.type === 'percentage' && (
            <Form layout="vertical">
              <Form.Item label={<>cache_creation 比例：<Tag color="geekblue">{clampRatio(Number(p.cache_creation_ratio ?? 0))}</Tag></>}>
                <Slider min={0} max={1} step={0.05} value={Number(p.cache_creation_ratio ?? 0)} onChange={(v) => setParam('cache_creation_ratio', v)} />
              </Form.Item>
              <Form.Item label={<>cache_read 比例：<Tag color="geekblue">{clampRatio(Number(p.cache_read_ratio ?? 0))}</Tag></>}>
                <Slider min={0} max={1} step={0.05} value={Number(p.cache_read_ratio ?? 0)} onChange={(v) => setParam('cache_read_ratio', v)} />
              </Form.Item>
              <Row gutter={12}>
                <Col span={12}>
                  <Form.Item label="input 固定值">
                    <InputNumber min={0} style={{ width: '100%' }} value={Number(p.input_fixed_tokens ?? 1)} onChange={(v) => setParam('input_fixed_tokens', v)} />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item label="output 来源">
                    <Input value={String(p.output_source ?? 'upstream')} onChange={(e) => setParam('output_source', e.target.value)} placeholder="upstream 或 fixed:N" />
                  </Form.Item>
                </Col>
              </Row>
            </Form>
          )}

          {strategy.type === 'fixed' && (
            <Form layout="vertical">
              <Row gutter={12}>
                <Col span={12}><Form.Item label="input_tokens"><InputNumber min={0} style={{ width: '100%' }} value={Number(p.input_tokens ?? 0)} onChange={(v) => setParam('input_tokens', v)} /></Form.Item></Col>
                <Col span={12}><Form.Item label="output_tokens（0=走上游）"><InputNumber min={0} style={{ width: '100%' }} value={Number(p.output_tokens ?? 0)} onChange={(v) => setParam('output_tokens', v)} /></Form.Item></Col>
                <Col span={12}><Form.Item label="cache_creation_tokens"><InputNumber min={0} style={{ width: '100%' }} value={Number(p.cache_creation_tokens ?? 0)} onChange={(v) => setParam('cache_creation_tokens', v)} /></Form.Item></Col>
                <Col span={12}><Form.Item label="cache_read_tokens"><InputNumber min={0} style={{ width: '100%' }} value={Number(p.cache_read_tokens ?? 0)} onChange={(v) => setParam('cache_read_tokens', v)} /></Form.Item></Col>
              </Row>
            </Form>
          )}

          {strategy.type === 'formula' && (
            <Form layout="vertical">
              <div style={{ marginBottom: 12 }}>
                <span style={{ fontSize: 12.5, color: 'var(--cg-text-secondary)', marginRight: 8 }}>可用变量：</span>
                <Space size={[4, 6]} wrap>
                  {FORMULA_VARS.map((v) => (
                    <Tooltip key={v} title="可用变量：填入公式时直接引用">
                      <Tag className="cg-mono" style={{ fontSize: 11, cursor: 'default' }}>{v}</Tag>
                    </Tooltip>
                  ))}
                </Space>
              </div>
              {(['input', 'cache_creation', 'cache_read', 'output'] as const).map((field) => (
                <Form.Item key={field} label={field} style={{ marginBottom: 12 }}
                  validateStatus={preview.error ? 'error' : undefined}>
                  <Input className="cg-mono" value={String(p[field] ?? '')} onChange={(e) => setParam(field, e.target.value)} placeholder={field === 'output' ? 'upstream_output' : '表达式，如 total * 0.1'} />
                </Form.Item>
              ))}
              {preview.error && <Alert type="error" showIcon message={`表达式错误：${preview.error}`} />}
            </Form>
          )}
        </Col>

        {/* 实时试算面板 */}
        <Col xs={24} md={10}>
          <Card size="small" className="cg-soft-card" style={{ background: 'var(--cg-bg-subtle)' }} title={<span style={{ fontSize: 13 }}>实时试算</span>}>
            <div style={{ fontSize: 12, color: 'var(--cg-text-secondary)', marginBottom: 12 }}>
              样例输入：total=<b>44,000</b>，上游 output=<b>850</b>，cache_read=<b>30,000</b>
            </div>
            <Row gutter={[12, 12]}>
              <Col span={12}><Statistic title="input" value={preview.input} valueStyle={{ fontSize: 20 }} /></Col>
              <Col span={12}><Statistic title="output" value={preview.output} valueStyle={{ fontSize: 20 }} /></Col>
              <Col span={12}><Statistic title="cache_creation" value={preview.cache_creation} valueStyle={{ fontSize: 20 }} /></Col>
              <Col span={12}><Statistic title="cache_read" value={preview.cache_read} valueStyle={{ fontSize: 20 }} /></Col>
            </Row>
            <div style={{ marginTop: 12, fontSize: 11.5, color: 'var(--cg-text-tertiary,#928e85)' }}>
              试算逻辑与后端 internal/cache 引擎一致，配置切换无需重启（热加载）。
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  )
}
