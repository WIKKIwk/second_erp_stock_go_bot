# AGENTS.md

本文件只保留“下一位代理继续开发时真正需要知道”的当前上下文，不记录无关历史。

## 1. 当前主仓库

移动端前端主仓库：
- `/home/wikki/local.git/erpnext_stock_telegram/mobile_app`

移动端后端主仓库：
- `/home/wikki/local.git/erpnext_stock_telegram/mobile_server`

ERP 自定义模块仓库：
- `/home/wikki/storage/local.git/erpnext_n1/erp/apps/accord_state_core`

根仓库：
- `/home/wikki/local.git/erpnext_stock_telegram`

重要原则：
- 新的 mobile backend 变更只写进 `mobile_server`
- 不修改 ERPNext core 源码树
- ERP 端只通过 API 或独立 custom app 扩展

## 2. 当前 Git 状态

### mobile_app
- 分支：`main`
- 状态：`ahead 1`
- 最新提交：`9600d59` `Harden app session restore and logout reset`
- 未跟踪文件：
  - `android/app/google-services.json`
  - `flutter_01.png`

### mobile_server
- 分支：`main`
- 状态：clean
- 最新提交：`efa6a3f` `Make mobile_server run bring up domain tunnel`

### accord_state_core
- 分支：`main`
- 状态：clean
- 最新提交：`359f39d` `Use int-based delivery state fields`

## 3. 当前最重要的真实架构

### ERP / server / mobile 三层职责
- `ERP Delivery Note` 是状态真相来源
- `mobile_server` 负责读写 ERP 状态，不再依赖 comment 作为业务真相
- `mobile_app` 负责渲染与单角色 store，同一个角色的多个页面必须共享同一个 truth

### Delivery Note 自定义字段
由 `accord_state_core` 负责创建：
- `accord_flow_state`
- `accord_customer_state`
- `accord_customer_reason`
- `accord_delivery_actor`

当前约定：
- `accord_flow_state`
  - `0` = none
  - `1` = submitted
  - `2` = returned
- `accord_customer_state`
  - `0` = pending
  - `1` = confirmed
  - `2` = rejected

说明：
- `accord_delivery_actor` 在 live ERP 中仍为 `Data`，因为 Frappe 不允许直接把旧字段从 `Data` 改成 `Int`
- 当前实际写入值是字符串 `"1"`，语义仍然表示 werka

### Customer 当前真实规则
- Werka 发给 Customer 时，`Delivery Note` 会在 Werka 阶段直接 submit
- Customer confirm / reject 只修改 ERP 字段，不再触发 stock submit
- `pending/confirmed/rejected` 只看 ERP 字段，不看 comment

## 4. mobile_app 当前最重要的变更

### 单角色 store 架构
已经开始并部分完成：
- Customer 使用 `CustomerStore`
- Werka 使用 `WerkaStore`
- Supplier 使用 `SupplierStore`
- Admin 使用 `AdminStore`

目标：
- 同一角色的 home / status / detail / notifications 不再各自维护一套状态
- count 必须与列表来自同一 source

### 本次最新修复：会话恢复与 logout 硬重置
最新提交：
- `9600d59` `Harden app session restore and logout reset`

当前真实行为：
1. app 冷启动不再直接根据本地旧 profile 强行跳到 role home
2. 新增入口页：
   - `mobile_app/lib/src/features/auth/presentation/app_entry_screen.dart`
3. 如果本地有 session：
   - 会先尝试用 `MobileApi.instance.profile()` 验证
   - 然后再进入对应 role 页面
4. logout 现在会清理：
   - `AppSession`
   - role stores
   - runtime mutation stores
   - unread / hidden 通知状态
   - notification snapshot
   - notification cache
   - profile avatar cache
   - `last_login_phone`
   - `last_login_code`

核心文件：
- `mobile_app/lib/src/core/session/app_session.dart`
- `mobile_app/lib/src/core/session/app_runtime_reset.dart`
- `mobile_app/lib/src/features/auth/presentation/app_entry_screen.dart`
- `mobile_app/lib/src/core/api/mobile_api_auth_profile.dart`

验证结果：
- `flutter analyze` 绿色
- `flutter test` 全绿

## 5. mobile_server 当前最重要的真实状态

### 运行方式
启动：
```bash
cd /home/wikki/local.git/erpnext_stock_telegram/mobile_server
make run
```

停止：
```bash
make stop
```

健康检查：
```bash
curl -sS http://127.0.0.1:8081/healthz
curl -sS https://core.wspace.sbs/healthz
```

预期：
- 两个都返回 `200`

### 重要说明
`mobile_server/Makefile` 已修复：
- `make run` 不再只是本地 core
- 现在会同时拉起本地 core 与 domain tunnel
- 相关提交：
  - `efa6a3f` `Make mobile_server run bring up domain tunnel`

### 已修复的关键后端问题
- Delivery Note list query 之前请求了不被 Frappe list 接口接受的字段（如 `remarks`、`items`），导致 `417`
  - 修复提交：`509fc17`
- Werka 创建 customer shipment 时，过去是“先 submit 再 best-effort 写 state”，会导致真实 submit 了但 `accord_flow_state=0`
  - 现在改成：
    1. create draft
    2. write state
    3. submit
  - 修复提交：`6e2d659`

## 6. ERP 自定义模块当前状态

仓库：
- `/home/wikki/storage/local.git/erpnext_n1/erp/apps/accord_state_core`

已完成：
- app 已创建并已安装到本地 ERP site
- `Delivery Note` state fields 已自动创建

关键文件：
- `accord_state_core/accord_state_core/state/delivery_note_state.py`

当前不要做的事：
- 不修改 ERPNext core 源码
- 如果需要更多业务逻辑，应继续在 `accord_state_core` 中扩展

## 7. APK 与域名

正式 APK 生成命令：
```bash
cd /home/wikki/local.git/erpnext_stock_telegram/mobile_app
make apk-domain APK_NAME=accord.apk
```

输出：
- `/home/wikki/local.git/erpnext_stock_telegram/mobile_app/build/app/outputs/flutter-apk/accord.apk`

当前要求：
- release APK 只能用域名构建
- 不要再用 `127.0.0.1` / `localhost` 做 release APK
- 当前域名：
  - `https://core.wspace.sbs`

## 8. 当前活跃 issue

### ERP_mobile
- `#20` `Rebuild mobile role state flow around single-store architecture`

### customfield_for_server
- `#1` Delivery Note 字段化状态模型
- `#2` ERP 侧高性能过滤/聚合优化

## 9. 当前最合理的下一步

1. 用真实 Android 设备做 hard test：
   - login
   - app kill 后重新打开
   - logout 后重新打开
   - role 切换后旧 UI / 旧通知 / 旧缓存不能残留
2. 验证：
   - Customer home / pending / confirmed / notifications
   - Werka home / recent / status
   - Supplier home / recent / notifications / status
   - Admin home / activity
3. 如果 role store 仍有漂移，再继续按 `#20` 收口

## 10. 极简结论

现在最关键的事实只有三条：
- ERP 字段已经是状态真相来源，comment 不能再作为业务真相
- mobile_app 已开始单角色 store 化，且最新会话恢复 / logout 硬重置已完成
- 下一阶段主要是“真机 hard test + 按结果继续收口”，不是再回去补旧架构
