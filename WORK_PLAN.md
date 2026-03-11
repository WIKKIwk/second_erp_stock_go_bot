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
  Current blocker: local ERP `http://localhost:8000` hozir ishlamayapti. Amaliy checklist: `HARD_TEST.md`.
