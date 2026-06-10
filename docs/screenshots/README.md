# 控制台截图

Claude 官网风格 · 明亮 / 暗黑双模式 · 共 9 个页面 × 2 套主题。
由 `web/scripts/screenshots.mjs`（Playwright）自动生成。

| 页面 | 明亮 | 暗黑 |
|------|------|------|
| 登录 | [login-light](login-light.png) | [login-dark](login-dark.png) |
| 实时大盘 | [dashboard-light](dashboard-light.png) | [dashboard-dark](dashboard-dark.png) |
| 请求明细 | [traces-light](traces-light.png) | [traces-dark](traces-dark.png) |
| 请求详情 | [trace-detail-light](trace-detail-light.png) | [trace-detail-dark](trace-detail-dark.png) |
| 分组列表 | [groups-light](groups-light.png) | [groups-dark](groups-dark.png) |
| 分组编辑（缓存策略编辑器） | [group-edit-light](group-edit-light.png) | [group-edit-dark](group-edit-dark.png) |
| 上游通道与 Key 池 | [channels-light](channels-light.png) | [channels-dark](channels-dark.png) |
| 客户 API Key | [api-keys-light](api-keys-light.png) | [api-keys-dark](api-keys-dark.png) |
| 系统设置 | [settings-light](settings-light.png) | [settings-dark](settings-dark.png) |

## 重新生成

```bash
cd web
pnpm build && pnpm preview &
PLAYWRIGHT_BROWSERS_PATH=/opt/pw-browsers node scripts/screenshots.mjs
```
