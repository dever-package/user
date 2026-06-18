---
name: dever-user
description: Use when 修改 Dever user 组件，包括用户账号、认证、API key、credential、身份、权益、积分、token、middleware、API、Service、page JSON、权限、密钥和升级影响。
version: 0.1.0
---

# User 组件

本组件 skill 必须和 `shemic-dever` 一起使用。先遵守 Dever 框架规则，再按这里的 user 组件边界修改。

## 事实来源

- 组件源码：`backend/package/user`
- 组件声明：`backend/package/user/dever.json`
- Model：`model`
- Service：`service`
- API：`api`
- Auth context：`authctx`
- Middleware：`middleware`
- 后台页面：`front/page`

## 硬规则

- 不通过不安全的通用 action 暴露 account、token、credential、API key、identity、point 或 benefit 变更。
- 不记录 secret、token、password、API key 或 credential payload。
- 模板、page JSON 和源码里不放真实密钥。
- API handler 保持薄；auth、token、identity、point、benefit 和 API key 规则放 Service。
- Middleware 只做认证和上下文注入，避免放项目私有业务规则。
- 普通后台 list/update 页面在安全前提下使用 package/front 和 model 元信息。

## Service/API 规则

- `service/auth`：登录、token、session、credential 行为。
- `service/api_key`：API key 创建、校验、脱敏和轮换。
- `service/identity`：身份和等级行为。
- `service/point`：积分记账和日志。
- `service/benefit`：权益发放和核销。
- `authctx`：只放请求 actor / API key 上下文。

积分、权益和身份变更必须考虑事务或幂等。

## 常见检查

- 权限错误：先查站点 access、user middleware 和 page/action auth，再改 model/action 规则。
- Credential 问题：向前端返回数据前，先查 secret hash 和脱敏路径。
- API key 问题：除非现有 Service 明确支持一次性展示，否则不要返回完整 secret。
