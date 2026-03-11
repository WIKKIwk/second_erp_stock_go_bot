# AGENTS.md

Bu fayl keyingi AI agent uchun amaliy handoff. Bu yerda eski tarix emas, hozirgi real ishlayotgan holat, nozik joylar va qayerdan davom ettirish kerakligi yozilgan.

## 1. Arxitektura

Ekotizim 3 qism:

1. Root repo: `/home/wikki/local.git/erpnext_stock_telegram`
   - Telegram bot
   - asosiy `core` backend
   - ERPNext API integratsiya kodi
   - mobile app uchun HTTP endpointlar

2. Mobile app repo: `/home/wikki/local.git/erpnext_stock_telegram/mobile_app`
   - Flutter client
   - alohida git repo

3. Standalone server repo: `/home/wikki/storage/local.git/mobile_server`
   - Telegram botsiz mustaqil `core/server`
   - alohida git repo

To‘g‘ri oqim:
- `Mobile App -> Core -> ERPNext`
- `Telegram Bot -> Core / ERP adapter logic`
- `Core -> ERPNext`

Asosiy maqsad:
- mobile app botga qaram bo‘lmasligi
- core alohida service bo‘lishi
- ERPNext source kodiga tegmaslik, faqat API bilan ishlash

## 2. ERP qoida

Mahalliy ERP source:
- `/home/wikki/local.git/erpnext_n1/erp`

Muhim:
- ERP source tree’ga edit kiritmang
- ERPNext bilan faqat API orqali gaplashing

## 3. Root repo holati

Path:
- `/home/wikki/local.git/erpnext_stock_telegram`

Asosiy entrypointlar:
- bot: [cmd/bot/main.go](/home/wikki/local.git/erpnext_stock_telegram/cmd/bot/main.go)
- core: [cmd/core/main.go](/home/wikki/local.git/erpnext_stock_telegram/cmd/core/main.go)
- legacy mobileapi: [cmd/mobileapi/main.go](/home/wikki/local.git/erpnext_stock_telegram/cmd/mobileapi/main.go)

Asosiy package’lar:
- bot: [internal/bot](/home/wikki/local.git/erpnext_stock_telegram/internal/bot)
- shared core logic: [internal/core](/home/wikki/local.git/erpnext_stock_telegram/internal/core)
- ERP client: [internal/erpnext](/home/wikki/local.git/erpnext_stock_telegram/internal/erpnext)
- HTTP layer: [internal/mobileapi](/home/wikki/local.git/erpnext_stock_telegram/internal/mobileapi)
- config: [internal/config](/home/wikki/local.git/erpnext_stock_telegram/internal/config)
- legacy supplier helperlar: [internal/suplier](/home/wikki/local.git/erpnext_stock_telegram/internal/suplier)

Legacy `internal/suplier` to‘liq o‘chirilmagan, lekin runtime mobile flow ERPNext’ga tayangan.

## 4. Root run buyruqlari

`Makefile`:
- [Makefile](/home/wikki/local.git/erpnext_stock_telegram/Makefile)

Muhim targetlar:
- `make core-up`
- `make core-stop`
- `make core-restart`
- `make run-core`
- `make run` yoki `make run-all` botni ham ishga tushiradi

Muhim eslatma:
- mobile/core ishlarida `make run` ishlatmang
- to‘g‘risi: `make core-up` yoki `make core-restart`

### Stale core himoyasi

Yangi fix bor:
- `Makefile` endi `.core.rev` yozadi
- `make core-up` hozirgi `git HEAD` bilan `.core.rev`ni solishtiradi
- agar eski revision ishlayotgan bo‘lsa avtomatik restart qiladi

Muhim commit:
- `c632d3a` `Restart stale core when revision changes`

Demak keyingi agent `core` eski sessionda qolib ketdimi deb alohida shubhalanib vaqt ketkazmasin:
- `make core-up` yoki `make core-restart` ishlating

## 5. Root backendning real hozirgi imkoniyatlari

Hozirgi endpoint oilalari:
- auth: login/logout/me/profile/avatar
- supplier: history/items/dispatch
- notifications: detail/comments
- push token register: `/v1/mobile/push/token`
- werka: pending/history/confirm
- admin: settings/suppliers/items/activity/werka settings

Admin fixed mobile credential:
- phone: `+998880000000`
- code: `19621978`

### So‘nggi muhim backend fixlar

- `414d40f` `Handle ERP permission errors in admin supplier detail`
- `6472c1f` `Fix ERP item supplier child-table query`
- `0579fb8` `Add admin supplier phone update API`
- `0534f3e` `Add werka history API`
- `47e58fa` `Track partial werka receipt return notes`
- `f4774ba` `Add notification detail and comment APIs`
- `c137df9` `Strip HTML from notification comments`
- `c93e600` `Support full werka return comments`
- `00a8847` `Allow full werka returns without comment`
- `c25d496` `Handle full werka returns without ERP submit`
- `e7c0e2c` `Propagate supplier acknowledgment to werka feed`
- `b7bd7b1` `Emit supplier acknowledgment events for werka`
- `7193501` `Add mobile push token registration backend`
- `8670804` `Add Firebase push sender backend`
- `31365c8` `Expose supplier dispatch amount data`
- `952b56a` `Backfill supplier acknowledgment into history notes`
- `df88db6` `Attach supplier ref to dispatch records`
- `535784a` `Proxy supplier avatars through core`
- `8c7acc8` `Reduce purchase receipt list round trips`
- `a990370` `Add FCM sender tests and work plan`

### Full return mantiqi

Werka `accepted_qty = 0` bilan full return qilsa:
- ERP submit yo‘liga tiqilmaydi
- `remarks` va `Comment`ga note yoziladi
- app tarafda `rejected/cancelled` sifatida chiqadi

Qisman return note formatlari:
- `Accord Qabul:`
- `Accord Qaytarildi:`
- `Accord Sabab:`
- `Accord Izoh:`
- `Accord Supplier Tasdiq:`

## 6. Mobile app repo holati

Path:
- `/home/wikki/local.git/erpnext_stock_telegram/mobile_app`

Bu alohida git repo.

### Muhim mobile commitlar

So‘nggi muhim commitlar:
- `4ef7d6d` `Add cache-first loading for key screens`
- `a47c8d3` `Refresh key screens on incoming notifications`
- `2d6c08b` `Add Firebase messaging client integration`
- `26a134a` `Add local Android notification runtime`
- `60f5255` `Enable Android desugaring for notification plugin`
- `70627b8` `Show supplier acknowledgment badge in notifications`
- `0fa50c5` `Reset stale notification detail on account change`
- `01827e2` `Require hold on profile dock to logout`
- `08d6216` `Remove cards from supplier quantity screen`
- `bd728a6` `Simplify supplier dispatch confirmation layout`
- `3f2ef6f` `Increase supplier confirmation text hierarchy`
- `286f19c` `Confirm werka receipt completion action`
- `dbb8049` `Add full return flow to werka receipt screen`
- `a3513ef` `Show return reasons for full werka returns`
- `83279bf` `Toggle full return details cleanly`
- `eed22ed` `Fix supplier home status counts`
- `ac31c79` `Fix werka home in-progress counts`
- `51107a2` `Show supplier acknowledgment notifications for werka`
- `25111e1` `Add supplier acknowledgment action for issue receipts`
- `4acf654` `Hide comment section on clean receipts`
- `b920487` `Increase notification detail text size`
- `4bcca72` `Support Linux notification initialization`
- `f7830fb` `Show supplier acknowledgment with double-check badge`
- `2ac08bc` `Block cross-supplier stale notification details`
- `0f2144c` `Turn supplier recent into repeatable history`
- `d35fda4` `Hide issue comment box after supplier acknowledgment`
- `4e00021` `Rotate theme toggle in correct direction`
- `b2ada08` `Unify theme toggle rotation direction`
- `7d7b49e` `Correct sun-to-moon rotation direction`

### Hozirgi mobile real holat

Supplier:
- `Home` status count tuzatilgan
- `Notifications` admin activity stiliga yaqinlashgan
- `Recent` endi transaction-historyga yaqin, ustiga bosib oldingi miqdor bilan qayta jo‘natish mumkin
- `Confirm` va `Qty` cardlardan tozalangan, ixcham text ko‘rinishga o‘tgan

Werka:
- `Home` va `Notifications` ajratilgan
- `Home`da status kartalari bor
- `Notifications`da supplier ack synthetic event ko‘rinadi
- `Detail`da partial/full return flow, sabab tanlash, yakunlash confirm dialog bor

Shared:
- notification detail screen bor
- detail screen supplier/werka uchun umumiy
- supplier wrong stale detail holati kuchliroq bloklangan
- profile dock’da logout faqat 3 soniya bosib turilganda chiqadi
- theme toggle rotation direction tuzatilgan

### Avatar muammosi

Muammo:
- desktopda rasm ko‘rinsa ham APK/Android’da ko‘rinmasligi mumkin edi
- sabab ERP `localhost` URL’i telefonda ishlamasdi

Fix:
- `core` avatar proxy endpoint qiladi
- avatar URL `core` orqali beriladi

Muhim commit:
- `535784a` `Proxy supplier avatars through core`

## 7. Android APK holati

Asosiy release artifact:
- [accord.apk](/home/wikki/local.git/erpnext_stock_telegram/mobile_app/build/app/outputs/flutter-apk/accord.apk)

Muhim:
- user domain bilan build xohlaydi
- localhost build user uchun kerak emas

To‘g‘ri buyruq:
```bash
cd /home/wikki/local.git/erpnext_stock_telegram/mobile_app
make apk-domain APK_NAME=accord.apk
```

### Icon

Launcher icon yangi rasm bilan update qilingan.

Commit:
- `ddb0320` `Update app launcher icon`

## 8. Firebase / Push holati

### Hozirgi holat

Client taraf:
- `google-services.json` local joylangan
- `firebase_core`
- `firebase_messaging`
- Android gradle google-services plugin
- login bo‘lganda FCM token register qilinadi

Backend taraf:
- push token storage bor
- FCM sender bor
- event paytida tokenlarga FCM push yuborishga urinadi

Muhim commitlar:
- `7193501` `Add mobile push token registration backend`
- `8670804` `Add Firebase push sender backend`

### Muhim local-only fayllar

Commit qilmang:
- `/home/wikki/local.git/erpnext_stock_telegram/oneni-ami-firebase-adminsdk-fbsvc-cd184690f6.json`
- `/home/wikki/local.git/erpnext_stock_telegram/mobile_app/android/app/google-services.json`
- `.env`

Bu fayllar local secret/config.

### Push bo‘yicha real ehtiyotkor eslatma

Kod ulanib bo‘lgan, lekin keyingi agent real device verification qilishi kerak:
- app o‘rnatiladi
- login qilinadi
- permission `Allow`
- token backendga yozilganini tekshirish
- keyin supplier/werka event bilan real push kelishini sinash

Desktop `flutter run -d linux` FCM background push uchun authoritative test emas.
Haqiqiy test Android device/APK bilan bo‘lishi kerak.

## 9. Performance reja holati

[WORK_PLAN.md](/home/wikki/local.git/erpnext_stock_telegram/WORK_PLAN.md)

Hozir progress:
- [x] FCM senderni tugatish
- [x] Push kelsa auto-refresh
- [x] Cache-first data
- [ ] API payloadni yengillashtirish
- [ ] Hard test

### 2-bosqich: auto-refresh

`RefreshHub` qo‘shilgan.
Push/event kelganda refresh bo‘ladigan screenlar:
- supplier home
- supplier notifications
- supplier recent
- werka home
- werka notifications
- admin home
- admin activity

Commit:
- `a47c8d3` `Refresh key screens on incoming notifications`

### 3-bosqich: cache-first

`JsonCacheStore` qo‘shilgan.
Key screenlar oldin cached data ko‘rsatib, keyin network refresh qiladi.

Commit:
- `4ef7d6d` `Add cache-first loading for key screens`

### 4-bosqich: API payload optimization

Boshlangan.
Hozir list endpointlardagi eng yirik round-trip kamaytirilgan:
- `ListPendingPurchaseReceipts`
- `ListSupplierPurchaseReceipts`
- `ListTelegramPurchaseReceipts`

Inline fieldlar yetarli bo‘lsa har item uchun alohida `GetPurchaseReceipt` qilmaydi.

Commit:
- `8c7acc8` `Reduce purchase receipt list round trips`

Bu bosqich hali tugamagan.

## 10. Nozik joylar

1. Root repo dirty bo‘lishi mumkin:
- `AGENTS.md`
- `.core.rev`
- Firebase service account JSON
- userning boshqa local fayllari

2. Mobile repo dirty bo‘lishi mumkin:
- `android/app/google-services.json` untracked turadi
- buni commit qilmang

3. `core` eski sessionda qolib ketishi tarixi bor.
- endi `make core-up` HEAD-aware
- lekin shunga qaramay `git rev-parse HEAD` va `cat .core.rev` bilan tekshirish foydali

4. `desktop preview` va `real APK` holatini aralashtirmang.
- Android push, avatar, permission, FCM faqat real APK/device bilan to‘liq ishonchli tekshiriladi

5. Cross-supplier stale detail bugi uchun mobile-side guard qo‘shilgan.
- lekin agent har safar real supplier A / supplier B bilan verify qilsa yaxshi

## 11. Keyingi agent nimadan boshlasin

1. Root repo holatini tekshirsin:
```bash
cd /home/wikki/local.git/erpnext_stock_telegram
git status --short
```

2. Mobile repo holatini tekshirsin:
```bash
cd /home/wikki/local.git/erpnext_stock_telegram/mobile_app
git status --short
```

3. `core` revision to‘g‘riligini tekshirsin:
```bash
cd /home/wikki/local.git/erpnext_stock_telegram
git rev-parse HEAD
cat .core.rev
make core-up
curl -sS http://127.0.0.1:8081/healthz
```

4. Agar performance davom ettirilsa:
- `4. API payloadni yengillashtirish`ni bitirish
- keyin `5. Hard test`

5. Agar push verification qilinsa:
- Android appni yangi `accord.apk` bilan o‘rnatish
- login qilish
- permission berish
- `data/mobile_push_tokens.json` ni tekshirish
- supplier/werka event bilan closed-app pushni real test qilish

## 12. Qisqa yakun

Loyiha hozir ishlayotgan holatga yaqin:
- core alohida service
- mobile app supplier/werka/admin flow bilan
- local push/runtime va FCM scaffold bor
- stale core restart himoyasi bor
- cache-first va auto-refresh bor

Lekin hali to‘liq tugamagan asosiy ishlar:
- API payload optimizationni to‘liq bitirish
- real Android closed-app pushni hard verify qilish
- end-to-end hard test qilish
