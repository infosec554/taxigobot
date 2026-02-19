# Radar Go Bot — QA Test Scenarios

> Loyiha: Telegram Taxi Dispatch Bot (3 ta bot: Client, Driver, Admin)
> Yozilgan: Go + Telebot v3 + PostgreSQL + Redis

---

## Mundarija

1. [Foydalanuvchi Ro'yxatdan O'tish](#1-foydalanuvchi-royxatdan-otish)
2. [Haydovchi Ro'yxatdan O'tish](#2-haydovchi-royxatdan-otish)
3. [Buyurtma Yaratish (Mijoz)](#3-buyurtma-yaratish-mijoz)
4. [Buyurtmani Qabul Qilish (Haydovchi)](#4-buyurtmani-qabul-qilish-haydovchi)
5. [Sayohat Holati O'zgarishi](#5-sayohat-holati-ozgarishi)
6. [Admin Haydovchini Tasdiqlash](#6-admin-haydovchini-tasdiqlash)
7. [Admin Buyurtmani Tasdiqlash](#7-admin-buyurtmani-tasdiqlash)
8. [Haydovchi Tarif va Marshrut Boshqaruvi](#8-haydovchi-tarif-va-marshrut-boshqaruvi)
9. [Admin Panel — Tizim Boshqaruvi](#9-admin-panel--tizim-boshqaruvi)
10. [Davlat Raqami Validatsiyasi](#10-davlat-raqami-validatsiyasi)
11. [Bloklash va Xavfsizlik](#11-bloklash-va-xavfsizlik)
12. [Xato Holatlari (Negative Tests)](#12-xato-holatlari-negative-tests)

---

## 1. Foydalanuvchi Ro'yxatdan O'tish

### TC-001 — Yangi mijoz `/start` bosadi

| | |
|---|---|
| **Holat** | Foydalanuvchi birinchi marta botga `/start` yuboradi |
| **Pre-condition** | Foydalanuvchi DB da yo'q |
| **Qadamlar** | 1. Telegram da `/start` yuborish |
| **Kutilgan natija** | Bot telefon raqamini so'raydi, `status=pending_signup` saqlanganligi |
| **DB tekshirish** | `users` jadvalida yangi yozuv yaratilgan, `role=client`, `status=pending_signup` |

---

### TC-002 — Mijoz telefon raqamini ulashadi

| | |
|---|---|
| **Holat** | Foydalanuvchi "Telefon raqamni ulashish" tugmasini bosadi |
| **Pre-condition** | TC-001 bajarilgan, user `status=pending_signup` |
| **Qadamlar** | 1. "📱 Telefon raqamni ulashish" tugmasini bosish |
| **Kutilgan natija** | Asosiy menyu ko'rsatiladi |
| **DB tekshirish** | `phone` maydoni saqlangan, `status=active` |

---

### TC-003 — Mavjud foydalanuvchi `/start` bosadi

| | |
|---|---|
| **Holat** | DB da mavjud foydalanuvchi botga `/start` yuboradi |
| **Pre-condition** | Foydalanuvchi DB da bor, `status=active` |
| **Qadamlar** | 1. `/start` yuborish |
| **Kutilgan natija** | Asosiy menyu ko'rsatiladi, DB da duplikat yaratilmaydi |

---

### TC-004 — Bloklangan foydalanuvchi `/start` bosadi

| | |
|---|---|
| **Holat** | Admin tomonidan bloklangan user botga murojaat qiladi |
| **Pre-condition** | `status=blocked` |
| **Qadamlar** | 1. `/start` yuborish |
| **Kutilgan natija** | "🚫 Sizning hisobingiz bloklangan" xabari chiqadi |

---

## 2. Haydovchi Ro'yxatdan O'tish

### TC-010 — Yangi haydovchi `/start` bosadi (Driver Bot)

| | |
|---|---|
| **Holat** | Haydovchi Driver Bot ga `/start` yuboradi |
| **Pre-condition** | Foydalanuvchi driver botda yangi |
| **Qadamlar** | 1. Driver Bot ga `/start` yuborish |
| **Kutilgan natija** | Salom xabari + telefon raqam so'rash |

---

### TC-011 — Haydovchi telefon raqamini ulashadi

| | |
|---|---|
| **Holat** | Haydovchi telefon raqamini ulashadi |
| **Pre-condition** | Yangi haydovchi |
| **Qadamlar** | 1. Telefon raqamni ulashish |
| **Kutilgan natija** | Avtomobil markalarini tanlash menyusi chiqadi |
| **DB tekshirish** | `role=driver`, `status=pending_signup` |

---

### TC-012 — Haydovchi avtomobil markasini tanlaydi

| | |
|---|---|
| **Holat** | Ro'yxatdan o'tish jarayonida marka tanlash |
| **Pre-condition** | TC-011 bajarilgan |
| **Qadamlar** | 1. Marka tugmalaridan birini bosish (masalan: "Hyundai") |
| **Kutilgan natija** | O'sha markaga tegishli modellar ro'yxati chiqadi |
| **Session tekshirish** | `session.DriverProfile.CarBrand = "Hyundai"` |

---

### TC-013 — Haydovchi avtomobil modelini tanlaydi

| | |
|---|---|
| **Holat** | Modelni tanlash |
| **Pre-condition** | TC-012 bajarilgan |
| **Qadamlar** | 1. Model tugmasini bosish |
| **Kutilgan natija** | Davlat raqamini kiritish so'raladi |
| **Session tekshirish** | `session.State = StateLicensePlate` |

---

### TC-014 — Haydovchi "Boshqa model" tanlaydi

| | |
|---|---|
| **Holat** | Ro'yxatda yo'q model kiritish |
| **Pre-condition** | TC-012 bajarilgan |
| **Qadamlar** | 1. "🖊 Другая" tugmasini bosish → 2. Model nomini yozish |
| **Kutilgan natija** | Kiritilgan model saqlanganda davlat raqami so'raladi |
| **Session tekshirish** | `session.State = StateCarModelOther` → `StateLicensePlate` |

---

### TC-015 — Haydovchi to'g'ri davlat raqamini kiritadi

| | |
|---|---|
| **Holat** | To'g'ri format bilan davlat raqami |
| **Pre-condition** | `session.State = StateLicensePlate` |
| **Qadamlar** | 1. `А123ВС777` yozish |
| **Kutilgan natija** | "✅ Данные сохранены!" xabari, marshrut qo'shish menyusi |
| **DB tekshirish** | `driver_profiles` jadvalida yozuv yaratilgan |

---

### TC-016 — Haydovchi noto'g'ri davlat raqamini kiritadi

| | |
|---|---|
| **Holat** | Noto'g'ri format |
| **Pre-condition** | `session.State = StateLicensePlate` |
| **Qadamlar** | 1. `12345` yoki `ABCDEF` yozish |
| **Kutilgan natija** | "❌ Некорректный формат!" xato xabari, qayta kiritish so'raladi |

---

### TC-017 — Latin harflarini Kirillga konvertatsiya

| | |
|---|---|
| **Holat** | Kirill bilan o'xshash Latin harflari kiritiladi |
| **Pre-condition** | `session.State = StateLicensePlate` |
| **Qadamlar** | 1. `A123BC777` (Latin A, B, C) yozish |
| **Kutilgan natija** | Avtomatik `А123ВС777` (Kirill) ga konvertatsiya qilinib saqlanadi |

---

### TC-018 — Haydovchi ro'yxatdan o'tishni yakunlaydi

| | |
|---|---|
| **Holat** | Barcha ma'lumotlar kiritilgandan so'ng |
| **Pre-condition** | Davlat raqami saqlangan, kamida 1 marshrut va 1 tarif tanlangan |
| **Qadamlar** | 1. "✅ Далее" tugmasini bosish |
| **Kutilgan natija** | "🎉 Регистрация завершена!" + Admin xabardor qilinadi |
| **DB tekshirish** | `status=pending_review` |

---

### TC-019 — Marshutsiz ro'yxatdan o'tishni yakunlashga urinish

| | |
|---|---|
| **Holat** | Marshrut qo'shilmagan holda yakunlash |
| **Pre-condition** | Driver profili saqlangan, marshrut yo'q |
| **Qadamlar** | 1. "✅ Далее" tugmasini bosish |
| **Kutilgan natija** | "⚠️ Необходимо добавить хотя бы один маршрут!" |

---

### TC-020 — Tarifsiz ro'yxatdan o'tishni yakunlashga urinish

| | |
|---|---|
| **Holat** | Tarif tanlanmagan holda yakunlash |
| **Pre-condition** | Marshrut bor, tarif yo'q |
| **Qadamlar** | 1. "✅ Далее" tugmasini bosish |
| **Kutilgan natija** | "⚠️ Необходимо выбрать хотя бы один тариф!" |

---

## 3. Buyurtma Yaratish (Mijoz)

### TC-030 — Mijoz yangi buyurtma yaratishni boshlaydi

| | |
|---|---|
| **Holat** | Aktiv mijoz buyurtma yaratadi |
| **Pre-condition** | `status=active`, Client Bot da |
| **Qadamlar** | 1. "➕ Создать заказ" tugmasini bosish |
| **Kutilgan natija** | "Откуда?" — shaharlar ro'yxati chiqadi |
| **Session tekshirish** | `session.State = StateFrom` |

---

### TC-031 — Mijoz "Qayerdan" shaharni tanlaydi

| | |
|---|---|
| **Holat** | Jo'nab ketish shaharini tanlash |
| **Pre-condition** | TC-030 bajarilgan |
| **Qadamlar** | 1. Shahar tugmasini bosish (masalan: "Москва") |
| **Kutilgan natija** | "Куда?" — manzil shaharlari chiqadi |
| **Session tekshirish** | `session.OrderData.FromLocationID` saqlanadi |

---

### TC-032 — Mijoz "Qayerga" shaharni tanlaydi

| | |
|---|---|
| **Holat** | Boradigan shaharni tanlash |
| **Pre-condition** | TC-031 bajarilgan |
| **Qadamlar** | 1. Shahar tugmasini bosish |
| **Kutilgan natija** | Tarif tanlash menyusi chiqadi |
| **Session tekshirish** | `session.OrderData.ToLocationID` saqlanadi |

---

### TC-033 — Mijoz tarifni tanlaydi

| | |
|---|---|
| **Holat** | Tarif tanlash (Эконом, Стандарт, Комфорт, ...) |
| **Pre-condition** | TC-032 bajarilgan |
| **Qadamlar** | 1. Tarif tugmasini bosish |
| **Kutilgan natija** | Kalendar (sana tanlash) menyusi chiqadi |
| **Session tekshirish** | `session.OrderData.TariffID` saqlanadi |

---

### TC-034 — Mijoz sanani tanlaydi

| | |
|---|---|
| **Holat** | Kalendardan sana tanlash |
| **Pre-condition** | TC-033 bajarilgan |
| **Qadamlar** | 1. Kalendardagi sana tugmasini bosish |
| **Kutilgan natija** | Vaqt tanlash menyusi chiqadi |

---

### TC-035 — Mijoz vaqtni tanlaydi

| | |
|---|---|
| **Holat** | Vaqt tanlash |
| **Pre-condition** | TC-034 bajarilgan |
| **Qadamlar** | 1. Vaqt tugmasini bosish |
| **Kutilgan natija** | Buyurtma tasdiq menyusi chiqadi (narx va ma'lumotlar) |

---

### TC-036 — Mijoz buyurtmani tasdiqlaydi

| | |
|---|---|
| **Holat** | "Подтвердить" tugmasini bosish |
| **Pre-condition** | TC-035 bajarilgan |
| **Qadamlar** | 1. "✅ Подтвердить" tugmasini bosish |
| **Kutilgan natija** | Buyurtma yaratiladi, mos haydovchilarga xabar yuboriladi |
| **DB tekshirish** | `orders` jadvalida `status=active` yozuv yaratilgan |

---

### TC-037 — Mijoz buyurtmani bekor qiladi

| | |
|---|---|
| **Holat** | Tasdiq bosqichida bekor qilish |
| **Pre-condition** | TC-035 bajarilgan |
| **Qadamlar** | 1. "❌ Отмена" tugmasini bosish |
| **Kutilgan natija** | Asosiy menyuga qaytadi, buyurtma yaratilmaydi |
| **Session tekshirish** | `session.State = StateIdle` |

---

### TC-038 — Mijoz o'zining buyurtmalarini ko'radi

| | |
|---|---|
| **Holat** | Buyurtmalar tarixi |
| **Pre-condition** | Kamida 1 ta buyurtma mavjud |
| **Qadamlar** | 1. "📋 Мои заказы" tugmasini bosish |
| **Kutilgan natija** | Barcha buyurtmalar ro'yxati chiqadi |

---

### TC-039 — Kalendar navigatsiyasi

| | |
|---|---|
| **Holat** | Kalendarda oy almashtirish |
| **Pre-condition** | Kalendar ko'rsatilgan |
| **Qadamlar** | 1. "◀" yoki "▶" tugmasini bosish |
| **Kutilgan natija** | Oldingi/keyingi oy ko'rsatiladi |

---

## 4. Buyurtmani Qabul Qilish (Haydovchi)

### TC-040 — Haydovchi aktiv buyurtmalarni ko'radi

| | |
|---|---|
| **Holat** | Driver Bot da aktiv buyurtmalar ro'yxati |
| **Pre-condition** | Haydovchi `status=active`, unga mos buyurtmalar bor |
| **Qadamlar** | 1. "📦 Активные заказы" tugmasini bosish |
| **Kutilgan natija** | Haydovchining marshruti va tarifiga mos buyurtmalar chiqadi |

---

### TC-041 — Haydovchi buyurtmani qabul qiladi

| | |
|---|---|
| **Holat** | "Взять заказ" tugmasini bosish |
| **Pre-condition** | TC-040 bajarilgan, buyurtma `status=active` |
| **Qadamlar** | 1. "📥 Принять заказ" tugmasini bosish |
| **Kutilgan natija** | Admin tasdiqlash uchun xabardor qilinadi |
| **DB tekshirish** | `status=wait_confirm`, `driver_id` to'ldirilgan |

---

### TC-042 — Bir vaqtda ikki haydovchi bir buyurtmani olishga harakat qiladi

| | |
|---|---|
| **Holat** | Race condition — bir buyurtmaga ikki haydovchi |
| **Pre-condition** | 2 ta aktiv haydovchi, 1 ta buyurtma `status=active` |
| **Qadamlar** | 1. Haydovchi A "Принять заказ" bosadi → 2. Haydovchi B "Принять заказ" bosadi |
| **Kutilgan natija** | Faqat birinchi so'rov muvaffaqiyatli, ikkinchisiga xato xabari |
| **DB tekshirish** | Faqat bitta `driver_id` saqlanadi |

---

### TC-043 — Haydovchi o'z buyurtmalarini ko'radi

| | |
|---|---|
| **Holat** | Qabul qilingan buyurtmalar ro'yxati |
| **Pre-condition** | Haydovchiga tayinlangan buyurtma bor |
| **Qadamlar** | 1. "📋 Мои заказы" tugmasini bosish |
| **Kutilgan natija** | Haydovchiga tayinlangan buyurtmalar chiqadi |

---

### TC-044 — Faol bo'lmagan haydovchi buyurtmalar ko'rishga urinadi

| | |
|---|---|
| **Holat** | `status=pending_review` haydovchi |
| **Pre-condition** | Admin hali tasdiqlamagan |
| **Qadamlar** | 1. "📦 Активные заказы" tugmasini bosish |
| **Kutilgan natija** | "🚫 Доступ запрещен!" xabari chiqadi |

---

## 5. Sayohat Holati O'zgarishi

### TC-050 — Haydovchi "Chiqib ketdim" bosadi

| | |
|---|---|
| **Holat** | Buyurtma `status=taken` holatidan `on_way` ga o'tish |
| **Pre-condition** | Buyurtma `status=taken`, haydovchiga tayinlangan |
| **Qadamlar** | 1. "🚖 Выехал" tugmasini bosish |
| **Kutilgan natija** | Mijozga "🚖 Водитель выехал к вам!" xabari yuboriladi |
| **DB tekshirish** | `status=on_way`, `on_way_at` vaqt saqlanadi |

---

### TC-051 — Haydovchi "Yetib keldim" bosadi

| | |
|---|---|
| **Holat** | `status=on_way` dan `arrived` ga o'tish |
| **Pre-condition** | Buyurtma `status=on_way` |
| **Qadamlar** | 1. "📍 Прибыл" tugmasini bosish |
| **Kutilgan natija** | Mijozga "🚖 Водитель прибыл!" xabari yuboriladi |
| **DB tekshirish** | `status=arrived`, `arrived_at` vaqt saqlanadi |

---

### TC-052 — Haydovchi sayohatni boshlaydi

| | |
|---|---|
| **Holat** | `status=arrived` dan `in_progress` ga o'tish |
| **Pre-condition** | Buyurtma `status=arrived` |
| **Qadamlar** | 1. "▶ Начать поездку" tugmasini bosish |
| **Kutilgan natija** | Mijozga "▶ Поездка началась!" xabari yuboriladi |
| **DB tekshirish** | `status=in_progress`, `started_at` vaqt saqlanadi |

---

### TC-053 — Haydovchi sayohatni yakunlaydi

| | |
|---|---|
| **Holat** | `status=in_progress` dan `completed` ga o'tish |
| **Pre-condition** | Buyurtma `status=in_progress` |
| **Qadamlar** | 1. "✅ Завершить" tugmasini bosish |
| **Kutilgan natija** | Mijozga yakunlanish xabari yuboriladi |
| **DB tekshirish** | `status=completed`, `completed_at` vaqt saqlanadi |

---

### TC-054 — Noto'g'ri holatda status o'zgartirish

| | |
|---|---|
| **Holat** | Sayohat boshlanmagan buyurtmani yakunlashga urinish |
| **Pre-condition** | Buyurtma `status=active` (haydovchi hali qabul qilmagan) |
| **Qadamlar** | 1. "✅ Завершить" tugmasini bosish |
| **Kutilgan natija** | "❌ Ошибка (Возможно, статус изменился)" xabari |

---

### TC-055 — To'liq sayohat holati oqimi (Happy Path)

| | |
|---|---|
| **Holat** | Buyurtma yaratilishdan yakunlashgacha to'liq zanjir |
| **Qadamlar** | `active` → `wait_confirm` → `taken` → `on_way` → `arrived` → `in_progress` → `completed` |
| **Tekshirish** | Har bir bosqichda DB va mijoz/haydovchiga xabarlar to'g'ri ketadi |

---

## 6. Admin Haydovchini Tasdiqlash

### TC-060 — Admin yangi haydovchini ko'radi

| | |
|---|---|
| **Holat** | Yangi haydovchi ro'yxatdan o'tganda admin xabar oladi |
| **Pre-condition** | Haydovchi TC-018 ni bajargan |
| **Qadamlar** | 1. Admin Bot da xabarni ko'rish |
| **Kutilgan natija** | Haydovchi ma'lumotlari + "✅ Одобрить" / "❌ Отклонить" tugmalari |

---

### TC-061 — Admin haydovchini tasdiqlaydi

| | |
|---|---|
| **Holat** | Haydovchi so'rovi ma'qullanadi |
| **Pre-condition** | TC-060 bajarilgan |
| **Qadamlar** | 1. "✅ Одобрить" tugmasini bosish |
| **Kutilgan natija** | Haydovchiga "Sizning profilingiz tasdiqlandi" xabari yuboriladi |
| **DB tekshirish** | `status=active`, `role=driver` saqlanadi |

---

### TC-062 — Admin haydovchini rad etadi

| | |
|---|---|
| **Holat** | Haydovchi so'rovi rad qilinadi |
| **Pre-condition** | TC-060 bajarilgan |
| **Qadamlar** | 1. "❌ Отклонить" tugmasini bosish |
| **Kutilgan natija** | Haydovchiga rad xabari yuboriladi |
| **DB tekshirish** | `status=rejected` |

---

### TC-063 — Admin kutayotgan haydovchilar ro'yxatini ko'radi

| | |
|---|---|
| **Holat** | Tasdiqlanmagan haydovchilar |
| **Qadamlar** | 1. "🚖 Водители на проверке" tugmasini bosish |
| **Kutilgan natija** | `status=pending_review` haydovchilar ro'yxati chiqadi |

---

### TC-064 — Admin aktiv haydovchilar ro'yxatini ko'radi

| | |
|---|---|
| **Holat** | Barcha aktiv haydovchilar |
| **Qadamlar** | 1. "🚕 Все водители" tugmasini bosish |
| **Kutilgan natija** | `status=active, role=driver` foydalanuvchilar ro'yxati |

---

## 7. Admin Buyurtmani Tasdiqlash

### TC-070 — Admin tasdiq kutayotgan buyurtmani ko'radi

| | |
|---|---|
| **Holat** | Haydovchi buyurtmani qabul qilganda admin xabar oladi |
| **Pre-condition** | TC-041 bajarilgan |
| **Qadamlar** | 1. Admin Bot da xabarni ko'rish |
| **Kutilgan natija** | Buyurtma ma'lumotlari + "✅ Подтвердить" / "❌ Отклонить" tugmalari |

---

### TC-071 — Admin haydovchi-mijoz juftligini tasdiqlaydi

| | |
|---|---|
| **Holat** | Admin buyurtmani tasdiqlaydi |
| **Pre-condition** | TC-070 bajarilgan |
| **Qadamlar** | 1. "✅ Подтвердить" tugmasini bosish |
| **Kutilgan natija** | Haydovchi va mijozga xabar yuboriladi |
| **DB tekshirish** | `status=taken` |

---

### TC-072 — Admin barcha buyurtmalarni ko'radi

| | |
|---|---|
| **Holat** | Buyurtmalar tarixi |
| **Qadamlar** | 1. "📦 Все заказы" tugmasini bosish |
| **Kutilgan natija** | Sahifalangan buyurtmalar ro'yxati (10 ta bir sahifada) |

---

### TC-073 — Admin statistikani ko'radi

| | |
|---|---|
| **Holat** | Tizim statistikasi |
| **Qadamlar** | 1. "📊 Статистика" tugmasini bosish |
| **Kutilgan natija** | Jami foydalanuvchilar, haydovchilar, buyurtmalar, kunlik hisobot |

---

## 8. Haydovchi Tarif va Marshrut Boshqaruvi

### TC-080 — Haydovchi tarif qo'shadi

| | |
|---|---|
| **Holat** | Tarifni yoqish |
| **Pre-condition** | Haydovchi `status=active` |
| **Qadamlar** | 1. "🚕 Мои тарифы" → 2. "🔴 Эконом" tugmasini bosish |
| **Kutilgan natija** | Tarif yoqiladi, "✅ Эконом" ko'rinadi |
| **DB tekshirish** | `driver_tariffs` jadvalida yozuv yaratiladi |

---

### TC-081 — Haydovchi tarifni o'chiradi

| | |
|---|---|
| **Holat** | Tarifni o'chirish (toggle) |
| **Pre-condition** | Tarif yoqilgan holatda |
| **Qadamlar** | 1. "✅ Эконом" tugmasini bosish |
| **Kutilgan natija** | Tarif o'chadi, "🔴 Эконом" ko'rinadi |
| **DB tekshirish** | `driver_tariffs` dan yozuv o'chiriladi |

---

### TC-082 — Haydovchi marshrut qo'shadi

| | |
|---|---|
| **Holat** | Yangi marshrut qo'shish |
| **Pre-condition** | Haydovchi `status=active` |
| **Qadamlar** | 1. "📍 Мои маршруты" → 2. "➕ Добавить новый" → 3. "Откуда" tanlash → 4. "Куда" tanlash |
| **Kutilgan natija** | "✅ Маршрут добавлен!" |
| **DB tekshirish** | `driver_routes` jadvalida yozuv yaratiladi |

---

### TC-083 — Haydovchi barcha marshrutlarni tozalaydi

| | |
|---|---|
| **Holat** | Barcha marshrutlarni o'chirish |
| **Pre-condition** | Kamida 1 marshrut mavjud |
| **Qadamlar** | 1. "🗑 Очистить" tugmasini bosish |
| **Kutilgan natija** | Barcha marshrutlar o'chiriladi |
| **DB tekshirish** | `driver_routes` dan hamma yozuvlar o'chiriladi |

---

### TC-084 — Haydovchi sanada buyurtma qidiradi

| | |
|---|---|
| **Holat** | Kalendar orqali sana bo'yicha qidirish |
| **Qadamlar** | 1. "Поиск по дате" → 2. Sana tanlash |
| **Kutilgan natija** | O'sha kunga mos buyurtmalar chiqadi |

---

### TC-085 — Haydovchi bir xil marshrutni qayta qo'shishga urinadi

| | |
|---|---|
| **Holat** | Duplikat marshrut |
| **Pre-condition** | Marshrut allaqachon mavjud |
| **Qadamlar** | 1. Mavjud marshrut yo'nalishini tanlash |
| **Kutilgan natija** | Duplikat yaratilmaydi (`ON CONFLICT DO NOTHING`) |

---

## 9. Admin Panel — Tizim Boshqaruvi

### TC-090 — Admin tarif qo'shadi

| | |
|---|---|
| **Holat** | Yangi tarif yaratish |
| **Qadamlar** | 1. "⚙️ Тарифы" → 2. "➕ Добавить" → 3. Nom kiritish |
| **Kutilgan natija** | Tarif yaratiladi va ro'yxatda ko'rinadi |

---

### TC-091 — Admin tarifni o'chiradi

| | |
|---|---|
| **Holat** | Mavjud tarifni o'chirish |
| **Pre-condition** | Tarif mavjud |
| **Qadamlar** | 1. "⚙️ Тарифы" → 2. Tarif tugmasini bosish → 3. "🗑 Удалить" bosish |
| **Kutilgan natija** | Tarif o'chiriladi |

---

### TC-092 — Admin shahar qo'shadi

| | |
|---|---|
| **Holat** | Yangi shahar (location) yaratish |
| **Qadamlar** | 1. "🗺 Города" → 2. "➕ Добавить" → 3. Shahar nomi kiritish |
| **Kutilgan natija** | Shahar yaratiladi va ro'yxatda ko'rinadi |

---

### TC-093 — Admin foydalanuvchini bloklaydi

| | |
|---|---|
| **Holat** | Foydalanuvchini bloklash |
| **Qadamlar** | 1. "👥 Пользователи" → 2. Foydalanuvchi tanlash → 3. "🚫 Заблокировать" bosish |
| **Kutilgan natija** | `status=blocked` saqlanadi |

---

### TC-094 — Admin foydalanuvchini blokdan chiqaradi

| | |
|---|---|
| **Holat** | Bloklangan foydalanuvchini faollashtirish |
| **Pre-condition** | `status=blocked` |
| **Qadamlar** | 1. "🚫 Заблокированные" → 2. Foydalanuvchi → 3. "✅ Разблокировать" bosish |
| **Kutilgan natija** | `status=active` saqlanadi |

---

### TC-095 — Admin avtomobil markasi qo'shadi

| | |
|---|---|
| **Holat** | Yangi marka qo'shish |
| **Qadamlar** | 1. "🚗 Марки и модели" → 2. "➕ Добавить марку" → 3. Nom kiritish |
| **Kutilgan natija** | Marka `car_brands` jadvaliga saqlanadi |

---

## 10. Davlat Raqami Validatsiyasi

### TC-100 — To'g'ri Kirill raqamlari

| Kiritilgan | Natija |
|---|---|
| `А123ВС777` | ✅ To'g'ri |
| `К456НТ99` | ✅ To'g'ri |
| `М789РУ123` | ✅ To'g'ri (3 xonali viloyat) |
| `Т001АА11` | ✅ To'g'ri |

---

### TC-101 — Latin → Kirill konvertatsiya

| Kiritilgan (Latin) | Saqlanadigan (Kirill) | Natija |
|---|---|---|
| `A123BC777` | `А123ВС777` | ✅ Konvertatsiya |
| `K456HT99` | `К456НТ99` | ✅ Konvertatsiya |
| `M789PY123` | `М789РУ123` | ✅ Konvertatsiya |

---

### TC-102 — Noto'g'ri formatlar

| Kiritilgan | Xato sababi | Natija |
|---|---|---|
| `12345` | Raqam bilan boshlanadi | ❌ Xato |
| `АВВЕ333` | 3 ta harf, keyin 3 ta harf | ❌ Xato |
| `А12ВС77` | Faqat 2 ta raqam (3 kerak) | ❌ Xato |
| `А1234ВС77` | 4 ta raqam (3 kerak) | ❌ Xato |
| `` (bo'sh) | Bo'sh satr | ❌ Xato |
| `А123ВС1234` | 4 ta oxirgi raqam (2-3 kerak) | ❌ Xato |
| `Ф123ВС777` | `Ф` — ruxsat etilmagan harf | ❌ Xato |

---

## 11. Bloklash va Xavfsizlik

### TC-110 — Bloklangan mijoz buyurtma yarata olmaydi

| | |
|---|---|
| **Holat** | `status=blocked` mijoz |
| **Qadamlar** | 1. "➕ Создать заказ" bosish |
| **Kutilgan natija** | Xato xabari, buyurtma yaratilmaydi |

---

### TC-111 — Bloklangan haydovchi buyurtma qabul qila olmaydi

| | |
|---|---|
| **Holat** | `status=blocked` haydovchi |
| **Qadamlar** | 1. "📦 Активные заказы" bosish |
| **Kutilgan natija** | "🚫 Доступ запрещен!" |

---

### TC-112 — Admin hisobi himoyasi

| | |
|---|---|
| **Holat** | Admin Bot ga oddiy foydalanuvchi kiradi |
| **Pre-condition** | Foydalanuvchi admin emas |
| **Qadamlar** | 1. Admin Bot ga `/start` yuborish |
| **Kutilgan natija** | Login/parol so'raladi yoki xato xabari |

---

### TC-113 — Admin login/parol tekshirish

| | |
|---|---|
| **Holat** | Noto'g'ri parol kiritish |
| **Qadamlar** | 1. Admin Bot login → 2. Noto'g'ri parol kiritish |
| **Kutilgan natija** | "❌ Неверный пароль" xabari |

---

## 12. Xato Holatlari (Negative Tests)

### TC-120 — DB ulanishi uzilganda buyurtma yaratish

| | |
|---|---|
| **Holat** | PostgreSQL o'chib qolgan |
| **Qadamlar** | 1. Buyurtma yaratishga urinish |
| **Kutilgan natija** | Foydalanuvchiga tushunarli xato xabari chiqadi, bot ishdan chiqmaydi |

---

### TC-121 — Mavjud bo'lmagan buyurtma ID ga murojaat

| | |
|---|---|
| **Holat** | ID 99999999 buyurtma yo'q |
| **Qadamlar** | 1. `take_99999999` callback yuborish |
| **Kutilgan natija** | Xato xabari, bot ishdan chiqmaydi |

---

### TC-122 — Bir xil telefon raqam bilan ikki foydalanuvchi

| | |
|---|---|
| **Holat** | Bir telefon raqamni ikki kishi ishlatadi |
| **Qadamlar** | 1. Ikkinchi foydalanuvchi shu raqamni ulashadi |
| **Kutilgan natija** | Tizim muammosiz ishlaydi (telefon UNIQUE emas, faqat `telegram_id` UNIQUE) |

---

### TC-123 — Sessiya muddati tugagan holda davom etish

| | |
|---|---|
| **Holat** | Foydalanuvchi uzoq vaqt bot bilan gaplashmagan, sessiya eski |
| **Qadamlar** | 1. Eski callback ni bosish |
| **Kutilgan natija** | "Sessiya tugagan, qaytadan boshlang" yoki asosiy menyuga yo'naltirish |

---

### TC-124 — To'liq bo'lmagan buyurtma tasdiqlashga urinish

| | |
|---|---|
| **Holat** | Manzil ko'rsatilmagan buyurtma |
| **Pre-condition** | `FromLocationID=0` yoki `ToLocationID=0` |
| **Qadamlar** | 1. Bevosita `confirm_yes` callback yuborish |
| **Kutilgan natija** | Validatsiya xatosi chiqadi, buyurtma yaratilmaydi |

---

### TC-125 — Marshrut qo'shishda "Откуда" va "Куда" bir xil shahar

| | |
|---|---|
| **Holat** | Bir shahardan shu shaharga marshrut |
| **Pre-condition** | Marshrut qo'shish jarayonida |
| **Qadamlar** | 1. "Откуда" = Москва, "Куда" = Москва |
| **Kutilgan natija** | "Куда" menyusida "Откуда" shahri ko'rsatilmaydi (filtrlanadi) |

---

## Holat Oqimlari (State Machine)

```
BUYURTMA HOLATLARI:
active → wait_confirm → taken → on_way → arrived → in_progress → completed
                                                                → cancelled

HAYDOVCHI HOLATLARI:
pending_signup → pending_review → active
                               → rejected
active → blocked

FOYDALANUVCHI HOLATLARI:
pending_signup → active
active → blocked
blocked → active
```

---

## Test Muhiti

| Parametr | Qiymat |
|---|---|
| **OS** | Linux (Ubuntu 22.04) |
| **Go versiya** | 1.25.0 |
| **DB** | PostgreSQL 15 |
| **Cache** | Redis 7 |
| **Bot Framework** | Telebot v3.3.8 |

---

## Prioritetlar

| Prioritet | Test IDs |
|---|---|
| 🔴 **Kritik (P1)** | TC-036, TC-041, TC-061, TC-071, TC-053 |
| 🟠 **Yuqori (P2)** | TC-001, TC-002, TC-010 – TC-018, TC-050 – TC-054 |
| 🟡 **O'rta (P3)** | TC-030 – TC-039, TC-080 – TC-085 |
| 🟢 **Past (P4)** | TC-090 – TC-095, TC-120 – TC-125 |
