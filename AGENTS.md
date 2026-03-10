# AGENTS.md

Bu fayl keyingi AI agent uchun amaliy kontekst. Qisqa emas, to‘liq yozilgan. Maqsad: loyiha nima holatda ekanini, qaysi repo nima qilayotganini, nimani sindirmaslik kerakligini va qayerdan davom ettirish kerakligini aniq qoldirish.

## 1. Loyihaning umumiy arxitekturasi

Bu ecosystem hozir 3 ta yo‘nalishga bo‘lingan:

1. `erpnext_stock_telegram` root repo
   - Telegram bot
   - root ichidagi core/backend
   - ERPNext integratsiya kodi
   - mobile app uchun backend endpointlar

2. `mobile_app/`
   - Flutter mobile client
   - alohida git repo

3. `/home/wikki/storage/local.git/mobile_server`
   - alohida standalone core/server repo
   - Telegram botsiz ham bridge/server bo‘lib ishlaydi
   - keyingi bosqichda mustaqil deploy qilinishi mumkin

To‘g‘ri arxitektura:

- `Mobile App -> Core -> ERPNext`
- `Telegram Bot -> Core or ERPNext-adapter logic`
- `Core -> ERPNext`

Maqsad:
- botga qaram bo‘lmaslik
- mobile app mustaqil ishlashi
- keyin botni faqat client/adapterga aylantirish

## 2. Muhim foydalanuvchi talablari

Foydalanuvchi aniq xohishlari:

- har bir real o‘zgarishdan keyin commit bo‘lishi kerak
- javoblar qisqa va to‘g‘ri bo‘lishi kerak
- ortiqcha falsafa kerak emas
- o‘zgarishlarni amalda tekshirish kerak
- “ishlaydi” deyilsa real tekshiruv bilan bo‘lishi kerak
- mobile app botga qaram bo‘lib qolmasligi kerak
- core alohida ishlay olishi kerak
- admin panel mobile app ichida ham bo‘lishi kerak

Muhim kommunikatsion eslatma:
- foydalanuvchi juda qo‘pol so‘z ishlatadi
- agent bunga javoban qo‘pollikni qaytarmasligi kerak
- lekin to‘g‘ri, aniq, qattiq va qisqa gapirishi kerak

## 3. ERPNext bilan ishlash bo‘yicha muhim qoida

Mahalliy ERP source:

- `/home/wikki/local.git/erpnext_n1/erp`

Muhim:
- foydalanuvchi ilgari aniq aytgan: ERP source kodini patch qilmaslik kerak
- ERP bilan faqat API orqali gaplashish kerak
- ERPNext source tree’ga edit kiritmang

## 4. Hozirgi root repo holati

Root repo:
- path: `/home/wikki/local.git/erpnext_stock_telegram`

Asosiy entrypoint’lar:
- bot: [cmd/bot/main.go](/home/wikki/local.git/erpnext_stock_telegram/cmd/bot/main.go)
- core: [cmd/core/main.go](/home/wikki/local.git/erpnext_stock_telegram/cmd/core/main.go)
- legacy mobile server entrypoint hali bor: [cmd/mobileapi/main.go](/home/wikki/local.git/erpnext_stock_telegram/cmd/mobileapi/main.go)

Asosiy package’lar:
- bot logic: [internal/bot](/home/wikki/local.git/erpnext_stock_telegram/internal/bot)
- new shared backend logic: [internal/core](/home/wikki/local.git/erpnext_stock_telegram/internal/core)
- ERPNext client: [internal/erpnext](/home/wikki/local.git/erpnext_stock_telegram/internal/erpnext)
- HTTP layer: [internal/mobileapi](/home/wikki/local.git/erpnext_stock_telegram/internal/mobileapi)
- config: [internal/config](/home/wikki/local.git/erpnext_stock_telegram/internal/config)
- old supplier flatbuffer code: [internal/suplier](/home/wikki/local.git/erpnext_stock_telegram/internal/suplier)

Muhim holat:
- bot runtime flow endi supplier lookup uchun ERP’ga tayangan
- lekin `internal/suplier` package hali repo ichida saqlanib turibdi
- u to‘liq o‘chirib tashlanmagan

## 5. Root repo’ning run buyruqlari

Root `Makefile`:
- [Makefile](/home/wikki/local.git/erpnext_stock_telegram/Makefile)

Hozir mavjud muhim targetlar:

- `make run`
  - alias to `make run-all`
  - bot + core birga ishlaydi
  - oldingi instance’larni tozalaydi

- `make run-all`
  - core’ni ko‘taradi
  - keyin bot’ni ko‘taradi

- `make run-core`
  - faqat core’ni ko‘taradi

- `make run-bot`
  - faqat bot’ni ko‘taradi

- `make stop`
  - bot va core’ni to‘xtatadi

Muhim commit:
- root commit: `c9fb2d5`
  - `Harden make run against stale bot and core instances`

Keyingi muhim commit:
- root commit: `0f8c38f`
  - `Extract core package and add independent core entrypoint`

## 6. Root core’ning hozirgi holati

Root core endpointlari:
- health: `/healthz`
- login/logout/me/profile
- supplier history/items/dispatch
- werka pending/confirm
- admin settings
- admin suppliers

Muhim backend commitlar:
- `ad1fa80`
  - `Add minimal admin APIs and admin auth`
- `9bb782a`
  - `Use fixed mobile admin credentials`
- `efc3d4c`
  - `Fallback to first warehouse for supplier dispatch`

Admin fixed mobile credential:
- phone: `+998880000000`
- code: `19621978`

Bu credential root core’da fixed qo‘yilgan.

## 7. Supplier dispatch bo‘yicha muhim fix

Muammo bo‘lgan:
- mobile app’da `+` bosib draft Purchase Receipt yaratishda `dispatch create failed`

Sabab:
- `warehouse` bo‘sh qaytayotgan edi

Fix:
- agar default warehouse bo‘lmasa, ERP’dan birinchi warehouse avtomatik olinadi
- shu bilan `supplier items` ham warehouse bilan qaytadi
- draft yaratish `200 OK` bo‘ldi

Tekshirilgan:
- `/v1/mobile/supplier/items`
- `/v1/mobile/supplier/dispatch`

Real tekshiruvda draft yaratildi:
- `MAT-PRE-2026-00007`

## 8. Standalone core repo

Path:
- `/home/wikki/storage/local.git/mobile_server`

Remote:
- `https://github.com/WIKKIwk/mobile_server.git`

Bu repo alohida `server service` sifatida ajratilgan.

Asosiy entrypoint:
- [cmd/core/main.go](/home/wikki/storage/local.git/mobile_server/cmd/core/main.go)

Asosiy package’lar:
- [internal/core](/home/wikki/storage/local.git/mobile_server/internal/core)
- [internal/mobileapi](/home/wikki/storage/local.git/mobile_server/internal/mobileapi)
- [internal/erpnext](/home/wikki/storage/local.git/mobile_server/internal/erpnext)
- [internal/config](/home/wikki/storage/local.git/mobile_server/internal/config)
- [internal/suplier](/home/wikki/storage/local.git/mobile_server/internal/suplier)

Standalone core repo commitlar:
- `4c429e7`
  - initial standalone core service
- `ac58410`
  - `Prompt for ERP credentials on make run`
- `c1ce46d`
  - `Always prompt ERP credentials on make run`
- `7c96a91`
  - `Fallback to first warehouse for supplier dispatch`
- `fcac3b0`
  - `Use fixed mobile admin credentials`

## 9. Standalone core repo run xulqi

Path:
- [Makefile](/home/wikki/storage/local.git/mobile_server/Makefile)

`make run` xulqi:
- har safar oldingi core processlarni tozalaydi
- `.env` ni o‘chiradi
- keyin qayta so‘raydi:
  - `ERP URL`
  - `ERP API key`
  - `ERP API secret`
- keyin serverni ko‘taradi

Real test qilingan:
- prompt chiqdi
- qiymatlar qabul qilindi
- `.env` yozildi
- `http://127.0.0.1:8081/healthz` => `{"ok":true}`

Demak standalone bridge Telegram botsiz mustaqil ishlaydi.

## 10. Mobile app repo

Path:
- `/home/wikki/local.git/erpnext_stock_telegram/mobile_app`

Bu alohida git repo.

Muqim commitlardan ba’zilari:
- `e98cc17`
  - remote Android test + APK flow
- `61886f8`
  - Accord Vision branding
- `4b7a848`
  - release Android manifest’ga internet permission
- `deb473e`
  - session persistence + pull-to-refresh
- `9452fee`
  - supplier home’ni minimal qilish
- `78c0214`
  - notification visualini soddalashtirish + home navigation fix
- `dc5640d`
  - SVG dock iconlarni kattalashtirish
- `c442805`
  - minimal admin panel
- `afc1232`
  - admin create flows + dock visible
- `2b5ea94`
  - admin profile’da ham dock ko‘rinishi

Hozirgi mobile app state:
- supplier flow ishlaydi
- werka flow ishlaydi
- admin minimal panel ishlaydi
- app session persist bo‘ladi
- app resume bo‘lganda auto-refresh bor
- error state’larda retry bor
- doimiy domain bilan ishlaydi

## 11. Mobile app current important behavior

### Session persistence

Session root:
- [app_session.dart](/home/wikki/local.git/erpnext_stock_telegram/mobile_app/lib/src/core/session/app_session.dart)

Holat:
- token va profile `shared_preferences`da saqlanadi
- app qayta ochilganda role’ga qarab route tanlanadi

### Auto recovery

Mobile API wrapper:
- [mobile_api.dart](/home/wikki/local.git/erpnext_stock_telegram/mobile_app/lib/src/core/api/mobile_api.dart)

Holat:
- 401 bo‘lsa app stored `phone + code` bilan o‘zi qayta login qiladi
- keyin request’ni yana bir marta uradi
- server restart bo‘lsa tiklanish osonlashadi

### Error state recovery

Screens:
- supplier home
- recent
- notifications
- item picker
- werka home
- profile

Holat:
- `Qayta urinish`
- pull-to-refresh
- app resume bo‘lsa auto-refresh

### SVG dock icons

Assetlar:
- [home-fill.svg](/home/wikki/local.git/erpnext_stock_telegram/mobile_app/assets/icons/home-fill.svg)
- [home-line.svg](/home/wikki/local.git/erpnext_stock_telegram/mobile_app/assets/icons/home-line.svg)
- [notification-3-fill.svg](/home/wikki/local.git/erpnext_stock_telegram/mobile_app/assets/icons/notification-3-fill.svg)
- [notification-3-line.svg](/home/wikki/local.git/erpnext_stock_telegram/mobile_app/assets/icons/notification-3-line.svg)
- [repeat-2-fill.svg](/home/wikki/local.git/erpnext_stock_telegram/mobile_app/assets/icons/repeat-2-fill.svg)
- [account-circle-fill.svg](/home/wikki/local.git/erpnext_stock_telegram/mobile_app/assets/icons/account-circle-fill.svg)
- [account-circle-line.svg](/home/wikki/local.git/erpnext_stock_telegram/mobile_app/assets/icons/account-circle-line.svg)

Supplier dock:
- [supplier_dock.dart](/home/wikki/local.git/erpnext_stock_telegram/mobile_app/lib/src/features/supplier/presentation/widgets/supplier_dock.dart)

Dark/light qoidasi:
- light theme => fill icon
- dark theme => line icon

### Profile

Profile screen:
- [profile_screen.dart](/home/wikki/local.git/erpnext_stock_telegram/mobile_app/lib/src/features/shared/presentation/profile_screen.dart)

Holat:
- supplier, werka va admin uchun ishlaydi
- avatar upload
- nickname
- phone read-only
- theme switch
- bottom dock ko‘rinadi

## 12. APK holati

Release APK path:
- [accord-vision.apk](/home/wikki/local.git/erpnext_stock_telegram/mobile_app/build/app/outputs/flutter-apk/accord-vision.apk)

Branding:
- app name `Accord Vision`
- icon qo‘yilgan
- internet permission release manifest’da bor

Muhim:
- eski APK bilan chalkashmaslik uchun foydalanuvchiga ko‘pincha:
  1. eski appni o‘chirish
  2. yangi APK’ni o‘rnatish
  tavsiya qilingan

## 13. Domain va external access

Doimiy URL:
- `https://core.wspace.sbs`

Muhim:
- mobile app release build shu URL bilan build qilingan
- domain hozir Cloudflare tunnel orqali root repo’dagi core’ga boradi

Bir vaqtlar bo‘lgan muammo:
- domain DNS only bo‘lib qolgan
- keyin proxied/tunnel to‘g‘rilangan
- tunnel process detach bo‘lmay tushib qolayotgan edi
- keyin detached start fix qilingan

## 14. Hozirgi eng muhim pending ishlar

Admin panel bo‘yicha hali to‘liq tugamagan narsalar:

1. Admin mobile UI’da:
- supplier qo‘shish end-to-end qo‘shildi, lekin bu flow hali APK’da ko‘rib chiqilmagan bo‘lishi mumkin
- werka page qo‘shildi, lekin premium polish hali yo‘q

2. Hali qilinmagan katta funksiyalar:
- supplier block/unblock
- supplier delete/remove
- supplier code regenerate
- supplierni item’ga bog‘lash UI
- admin home’da blocked supplier count

Foydalanuvchi aynan shularni xohlagan:
- supplier listdan supplier detail
- mahsulotga bog‘lash
- code regenerate
- block/unblock
- remove
- home’da blocked count

Bu hali to‘liq implement qilingan deb hisoblamang.

## 15. Muhim xavf / nozik nuqtalar

1. Root repo’da untracked fayl bor:
- `Gemini_Generated_Image_x2c5d2x2c5d2x2c5.png`

Buni tasodifan commit/push qilib yubormang.

2. `.env` hech qachon commit qilinmasin.
- root repo’da ham
- mobile_server repo’da ham

3. Standalone core repo remote ulangan, lekin push auth har doim tayyor emas.
- GitHub auth yo‘qligida push yiqilishi mumkin

4. `internal/suplier` ichidagi eski FB-related kodlar root repo’da hali turibdi.
- runtime flow undan maksimal ajratilgan
- lekin to‘liq cleanup hali tugamagan

## 16. Keyingi AI nimadan boshlashi kerak

Agar keyingi agent davom ettirsa, eng to‘g‘ri ketma-ketlik:

1. Root repo cleanligini tekshirish:
```bash
cd /home/wikki/local.git/erpnext_stock_telegram
git status --short
```

2. Mobile repo cleanligini tekshirish:
```bash
cd /home/wikki/local.git/erpnext_stock_telegram/mobile_app
git status --short
```

3. Standalone core repo cleanligini tekshirish:
```bash
cd /home/wikki/storage/local.git/mobile_server
git status --short
```

4. Agar admin supplier-management taskini davom ettirsa:
- root backenddagi admin API’larni tekshirish
- mobile admin UI compile/test
- keyin `supplier block/unblock`, `regen code`, `assign items` ni bosqichma-bosqich qo‘shish

5. Har bir real o‘zgarishdan keyin commit qilish

## 17. Tavsiya etiladigan keyingi commit yo‘nalishlari

Tartibli davom ettirish uchun:

1. `Add admin dock and management screens`
2. `Add supplier blocking and code rotation`
3. `Add supplier item assignment from admin app`
4. `Add blocked supplier summary to admin home`

## 18. Qisqa yakun

Hozir loyiha holati:
- root repo’da bot + core birga
- core alohida entrypoint bilan ajratilgan
- standalone `mobile_server` repo tayyor
- mobile app supplier/werka/admin bilan ishlaydi
- doimiy domain bor
- APK release build bor

Lekin:
- admin supplier management hali faqat minimal darajada
- block/regenerate/assign/remove kabi to‘liq admin operatsiyalar hali tugallanmagan

Shu faylning maqsadi:
- keyingi AI agent taxmin bilan emas
- amaldagi real holatdan davom etsin
- qaysi repo nima qilayotganini chalkashtirmasin
