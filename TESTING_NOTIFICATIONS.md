# TEST GUIDE - Driver Notifications Fixed

## ✅ FIXES APPLIED

### 1. ✅ Tariff Check Logic Fixed
**File**: pkg/bot/bot.go, Line ~1784

**Before**:
```go
if !enabled[tariffID] {
    continue  // Exclude if tariff not selected
}
```

**After**:
```go
if len(enabled) > 0 && !enabled[tariffID] {
    continue  // Only exclude if tariffs ARE selected but this one isn't
}
// If no tariffs selected (empty), include by default
```

**Impact**: Drivers without selected tariffs will NOW get notifications (default behavior)

---

### 2. ✅ Debug Logging Added
**File**: pkg/bot/bot.go, Line ~1775-1830

**New Logs**:
```go
b.Log.Info("notifyDrivers: Starting driver notification",
    logger.Int64("orderID", orderID),
    logger.Int64("tariffID", tariffID),
)

b.Log.Debug("notifyDrivers: Skipping non-active driver",
    logger.Int64("driver_id", u.ID),
    logger.String("status", u.Status),
)

b.Log.Info("notifyDrivers: Driver matches route", ...)

b.Log.Info("Notification sent to driver", ...)
```

**Impact**: Can now debug why drivers aren't getting notifications

---

### 3. ✅ Database Query Loop Fixed
**File**: pkg/bot/bot.go, Line ~1814

**Before**:
```go
for id := range targetIDs {
    var teleID int64
    b.DB.QueryRow(context.Background(), "SELECT telegram_id FROM users WHERE id=$1", id).Scan(&teleID)
    if teleID != 0 {
        target.Bot.Send(...)
    }
}
```

**After**:
```go
userMap := make(map[int64]int64)
for _, u := range users {
    userMap[u.ID] = u.TelegramID
}

for id := range targetIDs {
    if teleID, ok := userMap[id]; ok && teleID != 0 {
        target.Bot.Send(...)
    }
}
```

**Impact**: No more N queries in loop, uses pre-fetched data

---

### 4. ✅ Route Selection Logging Added
**File**: pkg/bot/bot.go, Line ~1183-1195

**Added**:
```go
b.Log.Info("Driver route data ready",
    logger.Int64("from_id", session.OrderData.FromLocationID),
    logger.Int64("to_id", session.OrderData.ToLocationID),
)
```

**Impact**: Can see route selection in logs

---

## 🧪 TESTING SCENARIOS

### Scenario 1: Driver with SELECTED Tariffs & Routes

**Setup**:
```
1. Create Driver account
2. Register car (get approved → status="active")
3. Select Tariffs: "Economy" ✅
4. Select Routes: "Fergona → Margilan" ✅
```

**Test**:
```
1. Create Client order:
   From: Fergona
   To: Margilan
   Tariff: Economy
   
2. Admin approves order
3. Driver should get notification ✅ (matches tariff AND route)
```

**Expected Logs**:
```
[INFO] notifyDrivers: Starting driver notification orderID=123 tariffID=1
[INFO] notifyDrivers: Driver matches route driver_id=456
[INFO] Notification sent to driver driver_id=456 telegram_id=789
```

---

### Scenario 2: Driver WITHOUT Selected Tariffs or Routes (DEFAULT)

**Setup**:
```
1. Create Driver account
2. Register car (get approved → status="active")
3. DO NOT select tariffs
4. DO NOT select routes
```

**Test**:
```
1. Create Client order:
   From: Fergona
   To: Margilan
   Tariff: Economy
   
2. Admin approves order
3. Driver SHOULD get notification ✅ (default behavior - no filters set)
```

**Expected Logs**:
```
[INFO] notifyDrivers: Starting driver notification orderID=124 tariffID=1
[DEBUG] notifyDrivers: Driver has no routes set (default include) driver_id=789
[INFO] Notification sent to driver driver_id=789 telegram_id=999
```

---

### Scenario 3: Driver with Routes BUT Different From Order

**Setup**:
```
1. Create Driver account
2. Register car (get approved → status="active")
3. Select Tariffs: "Economy" ✅
4. Select Routes: "Quva → Andijan" ✅
```

**Test**:
```
1. Create Client order:
   From: Fergona
   To: Margilan
   Tariff: Economy
   
2. Admin approves order
3. Driver SHOULD NOT get notification ❌ (routes don't match)
```

**Expected Logs**:
```
[INFO] notifyDrivers: Starting driver notification orderID=125 tariffID=1
[DEBUG] notifyDrivers: Driver route doesn't match driver_id=101
```

---

### Scenario 4: Driver with Tariff NOT Selected but Order's Tariff Matches

**Setup**:
```
1. Create Driver account
2. Register car (get approved → status="active")
3. Select Tariffs: "Comfort" only ✅
4. NO routes selected
```

**Test**:
```
1. Create Client order:
   From: Fergona
   To: Margilan
   Tariff: Economy
   
2. Admin approves order
3. Driver SHOULD NOT get notification ❌ (Economy not selected)
```

**Expected Logs**:
```
[INFO] notifyDrivers: Starting driver notification orderID=126 tariffID=1
[DEBUG] notifyDrivers: Driver tariff not enabled driver_id=102 tariffID=1
```

---

## 📋 CHECKLIST - VERIFY FIXES

### Before Running Tests
- [ ] Code compiled successfully: `make build`
- [ ] Database is running: `docker-compose ps`
- [ ] All 3 bots can start: Check logs for "Bot X started"

### During Testing
- [ ] Check `.env` has correct bot tokens
- [ ] Admin ID is set in `.env`
- [ ] Database has test data (tariffs, locations)

### After Each Test
- [ ] Check bot logs: `docker-compose logs taxibot | grep notifyDrivers`
- [ ] Verify driver notification in Telegram
- [ ] Check order status changed to "active"

---

## 🔍 HOW TO DEBUG

### View Logs in Real-Time
```bash
docker-compose logs -f taxibot | grep -E "notifyDrivers|Notification sent"
```

### Check Specific Driver
```bash
docker-compose logs taxibot | grep "driver_id=<ID>"
```

### Search for Errors
```bash
docker-compose logs taxibot | grep -E "ERROR|WARN|Failed"
```

---

## 🎯 EXPECTED BEHAVIOR AFTER FIXES

### Client Order Creation
```
✅ Client creates order
✅ Order status = "pending"
✅ Admin gets notification in admin panel
✅ Client told: "Ваш заказ отправлен администратору"
```

### Admin Approves
```
✅ Admin clicks "✅ Одобрить"
✅ Order status = "pending" → "active"
✅ notifyDrivers() called with order details
✅ Debug logs show drivers being checked
✅ Matching drivers get notifications
```

### Driver Gets Notification
```
✅ Driver receives notification in Telegram
✅ Shows: Order #, Route, Price, Passengers, Time
✅ [📥 Принять заказ] button clickable
✅ Driver clicks to accept
✅ Order status = "active" → "accepted"
✅ Client gets notification: "🚖 Водитель принял ваш заказ!"
```

---

## ⚠️ KNOWN LIMITATIONS

1. **Peer Linking**: Must call notifyDrivers from admin/client bot (peers must be set)
2. **Status Check**: Drivers must be "active" (already approved) to receive notifications
3. **Route Logic**: If driver set routes, must match client order route (no partial match)
4. **Tariff Logic**: Now defaults to ALL if no tariffs selected (new behavior)

---

## 🚀 NEXT STEPS

If tests still fail:

1. **Check Logs**: Look for `notifyDrivers` logs
   - If no logs → Function not called
   - Check admin approval flow

2. **Check Driver Status**: 
   - Must be `status = "active"`
   - Check in database: `SELECT id, status FROM users WHERE role='driver'`

3. **Check Routes/Tariffs**:
   - Check if driver has routes: `SELECT * FROM routes WHERE driver_id=<ID>`
   - Check if driver has tariffs: `SELECT * FROM driver_tariffs WHERE driver_id=<ID>`

4. **Check Bot Peer Linking**:
   - Verify main.go line 70: `adminBot.Peers[bot.BotTypeDriver] = driverBot`
   - Add logging to verify peers are set

---

## 📝 TEST RESULTS TEMPLATE

```
Date: _______________
Tester: _______________

[ ] Scenario 1: Selected tariff + route ✅
    Result: _____________________
    Logs: _____________________

[ ] Scenario 2: No tariff/route (default) ✅
    Result: _____________________
    Logs: _____________________

[ ] Scenario 3: Different route ❌
    Result: _____________________
    Logs: _____________________

[ ] Scenario 4: Different tariff ❌
    Result: _____________________
    Logs: _____________________

Issues found: _____________________
```

