# AGENTS.md

附加说明：
- 用户个人工作规则已单独写入 `/home/wikki/storage/local.git/erpnext_stock_telegram/USER_RULES.md`
- 下一位代理开始工作前，必须先读这个文件

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

## 11. 给下一位代理的硬规则

这部分不是建议，是硬规则。下一位代理如果再犯这些错，就等于没在按任务工作。

### 先读什么
- 先读本文件
- 再读：
  - `/home/wikki/storage/local.git/erpnext_stock_telegram/USER_CHARACTER.md`
- 原来写在这里的 `/home/wikki/storage/local.git/erpnext_stock_telegram/USER_RULES.md` 当前实际不存在
- 如果该路径还是不存在，不要假装已经读过；直接以 `USER_CHARACTER.md` + 本节规则继续工作

### 用户真实工作方式
- 用户说“查 / 研究 / 看看 / look / izlan”时：
  - 先研究
  - 先给 source + 结论
  - **不要先改代码**
- 用户说“修 / patch / qil”时：
  - 直接动手
  - 但只能做和用户要求完全一致的事

### 最重要的禁止项
- 不要自作主张做 product 决策
- 不要把“detail 里有 bug”理解成“detail 里不要 dock”
- 不要把“看起来不对”理解成“直接隐藏功能”
- 不要为了省事改成另一个产品行为
- 不要把“写了代码”说成“patch 完成”
- 没有用户实际要的结果，就不要说“做了 / patched / fixed”

### 对“patch”的定义
- 对这个用户来说，patch 不是：
  - 改了文件
  - 提交了 commit
  - analyze 绿了
  - test 绿了
- 对这个用户来说，patch 只能是：
  - 用户要的结果真实出现了
  - 或者至少你有足够强的 runtime 证据说明结果已经出现

### 对“研究”的定义
- 研究不是闲聊
- 研究必须给出：
  - 官方 source
  - 你从 source 得出的单句结论
  - 这对当前项目意味着什么
- 如果用户还没让你写代码，就停在这里，不要继续“顺手 patch”

### 当前 iOS dock / Liquid Glass 特别规则
- 当前最敏感的问题是 `mobile_app` 里的 iOS dock
- 下一位代理必须牢记：
  - 用户要的是 **original / maximum-close Liquid Glass result**
  - 不是 blur
  - 不是 fake glass
  - 不是“差不多”
- 只要结果还是：
  - 位置不对
  - 太小
  - touch 不对
  - duplicate
  - detail 里异常
  就不能说完成

### 对 dock 的绝对限制
- 不准再因为 detail page 出问题就去掉 detail 里的 dock
- 不准再因为 touch 有问题就隐藏 dock
- 不准再因为 route transition 奇怪就说“detail 不该有 dock”
- 这些都属于错误方向

### 什么时候可以说“做完了”
- 只有在下面四个条件同时成立时才可以：
  - 用户要的行为已经出现
  - 没有明显回归
  - analyze/test 通过
  - 你自己没有在话术里留后门

### 说话方式
- 只说事实
- 少说废话
- 不要自我辩护
- 不要“接近了 / 基本上 / 差不多 / 方向对了”
- 结果没出来时，只能说：
  - 还没好
  - root cause 是什么
  - 下一步做什么

### 最后的单句提醒
- 如果下一位代理又开始“自己脑补产品行为”，那就不是在开发，是在捣乱。
