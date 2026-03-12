# Work Plan

- [x] FCM senderni tugatish
  App yopiq bo'lsa ham push keladigan qilish.

- [x] Push kelsa auto-refresh
  Supplier, werka va admin screenlar notification kelganda o'zi yangilansin.

- [x] Cache-first data
  Internet sust bo'lsa ham oxirgi foydali data darrov ko'rinsin.

- [ ] API payloadni yengillashtirish
  Ortiqcha fieldlarni kamaytirish va list endpointlarni tezlatish.

- [ ] Hard test
  Supplier -> werka, werka -> supplier, foreground/background/closed holatlarda to'liq tekshirish.
  Desktop/API oqimi o'tdi.
  Android supplier push bo'yicha backend delivery va system notification creation ADB bilan tasdiqlandi.
  Qolgan asosiy blok: user-visible closed/background notification presentation va `werka` token/device verification.
  APK: `/home/wikki/local.git/erpnext_stock_telegram/mobile_app/build/app/outputs/flutter-apk/accord.apk`
  Amaliy checklist: `HARD_TEST.md`.
