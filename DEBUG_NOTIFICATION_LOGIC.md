# BUG ANALYSIS - Driver Notification Logic

## 🔍 SORUN: Client Order → Admin Approval → Driver Notification

### Current Flow (Now):

```
1. CLIENT CREATES ORDER
   ├─ Status: "pending" (line 1131)
   ├─ Saved to DB
   ├─ Admin notified (line 1148: b.notifyAdmin())
   └─ Client: "⏳ Ваш заказ отправлен администратору"

2. ADMIN APPROVES ORDER
   ├─ Callback: approve_order_<id> (line 1409)
   ├─ Status changed: "pending" → "active" (line 1411)
   ├─ Calls b.notifyDrivers() (line 1426)
   └─ Driver should get notification
   
3. DRIVER NOTIFICATION CHECK
   Location: notifyDrivers() function (line 1751)
   
   LOGIC:
   a) Get drivers matching ROUTE (FromID, ToID)
   b) Get drivers matching TARIFF (TariffID)
   c) If driver has NO ROUTES → Send to them (default)
   d) If driver has ROUTE matching → Send
   e) If driver has route but DOESN'T MATCH → Skip
   
   ❌ PROBLEM FOUND!
```

---

## 🐛 BUG #1: Driver Tariff Toggle Logic

### Location: bot.go Line 1207-1209

```go
if strings.HasPrefix(data, "tgl_") {
    tariffID, _ := strconv.ParseInt(strings.TrimPrefix(data, "tgl_"), 10, 64)
    b.Stg.Tariff().Toggle(context.Background(), session.DBID, tariffID)
    return b.showDriverTariffs(c, false)
}
```

**Issue**: `session.DBID` used but session.DBID is database user ID.
**Check**: Is `session.DBID` the driver's database user ID? 
- **Yes**, it's correct! `session.DBID = user.ID` (line 276)

**Status**: ✅ **CORRECT** - tariff toggle should work

---

## 🐛 BUG #2: Driver Route Selection Logic

### Location: bot.go Lines 1183-1195 (Route From/To selection)

```go
if strings.HasPrefix(data, "dr_f_") {
    id, _ := strconv.ParseInt(strings.TrimPrefix(data, "dr_f_"), 10, 64)
    session.OrderData.FromLocationID = id  // ← USING OrderData (WRONG!)
    return b.handleAddRouteTo(c, session)
}

if strings.HasPrefix(data, "dr_t_") {
    toID, _ := strconv.ParseInt(strings.TrimPrefix(data, "dr_t_"), 10, 64)
    session.OrderData.ToLocationID = toID  // ← USING OrderData (WRONG!)
    if session.OrderData.FromLocationID == 0 {
        return c.Send("❌ Ошибка...")
    }
    return b.handleAddRouteComplete(c, session)
}
```

**Problem**: 
- Using `session.OrderData` (meant for ORDER data, not ROUTE data)
- OrderData.FromLocationID should be for ORDER, not for ROUTE
- This MIXUP causes confusion and potential bugs

**Impact**: Routes might work but logic is confusing. Should use separate fields like:
- `session.RouteFromID` and `session.RouteToID`
- OR `session.TempRouteFrom` and `session.TempRouteTo`

**Status**: ⚠️ **RISKY** - Works but design is flawed

---

## 🐛 BUG #3: Admin Approval → Driver Notification (CRITICAL)

### Location: bot.go Lines 1409-1430

```go
if strings.HasPrefix(data, "approve_order_") {
    id, _ := strconv.ParseInt(strings.TrimPrefix(data, "approve_order_"), 10, 64)
    order, _ := b.Stg.Order().GetByID(context.Background(), id)
    if order != nil {
        order.Status = "active"
        b.Stg.Order().Update(context.Background(), order)  // ✅ Update status
        
        // ✅ Get order details
        from, _ := b.Stg.Location().GetByID(context.Background(), order.FromLocationID)
        to, _ := b.Stg.Location().GetByID(context.Background(), order.ToLocationID)
        tariff, _ := b.Stg.Tariff().GetByID(context.Background(), order.TariffID)
        
        // ✅ Build notification message
        priceStr := fmt.Sprintf("%d %s", order.Price, order.Currency)
        routeStr := fmt.Sprintf("%s ➡️ %s", fromName, toName)
        notifMsg := fmt.Sprintf(messages["ru"]["notif_new"], order.ID, priceStr, routeStr)
        
        // 🔴 CALL NOTIFY DRIVERS
        b.notifyDrivers(order.ID, order.FromLocationID, order.ToLocationID, order.TariffID, notifMsg)
        
        // ✅ Notify client
        b.notifyUser(order.ClientID, "✅ Ваш заказ подтвержден администратором!...")
    }
    return c.Respond(&tele.CallbackResponse{Text: "Заказ одобрен"})
}
```

**Called Function**: `notifyDrivers()` (Line 1751)

---

## 🔍 notifyDrivers() Function Analysis

### Location: bot.go Lines 1751-1810

```go
func (b *Bot) notifyDrivers(orderID, fromID, toID, tariffID int64, text string) {
    // 1️⃣ GET TARGET BOT (Driver Bot)
    target := b
    if b.Type != BotTypeDriver {
        if p, ok := b.Peers[BotTypeDriver]; ok {
            target = p  // ✅ Get driver bot
        } else {
            b.Log.Error("Driver bot peer not found for notification")
            return  // ❌ EARLY RETURN - NO NOTIFICATION SENT!
        }
    }
    
    // 2️⃣ GET DRIVERS BY ROUTE
    routeDriversMap := make(map[int64]bool)
    routeDrivers, _ := b.Stg.Route().GetDriversByRoute(context.Background(), fromID, toID)
    for _, id := range routeDrivers {
        routeDriversMap[id] = true
    }
    
    // 3️⃣ ITERATE THROUGH ALL DRIVERS
    targetIDs := make(map[int64]bool)
    users, _ := b.Stg.User().GetAll(context.Background())
    
    for _, u := range users {
        // Check role and status
        if u.Role != "driver" || u.Status != "active" {
            continue  // ❌ SKIP non-active drivers
        }
        
        // Check tariff
        enabled, _ := b.Stg.Tariff().GetEnabled(context.Background(), u.ID)
        if !enabled[tariffID] {
            continue  // ❌ SKIP drivers who don't accept this tariff
        }
        
        // ROUTE LOGIC:
        // 1. If driver has matching route → notify
        if routeDriversMap[u.ID] {
            targetIDs[u.ID] = true
            continue
        }
        
        // 2. If driver has NO routes → notify (default)
        driverRoutes, _ := b.Stg.Route().GetDriverRoutes(context.Background(), u.ID)
        if len(driverRoutes) == 0 {
            targetIDs[u.ID] = true
        }
        // 3. If driver has routes but doesn't match → skip
    }
    
    // 4️⃣ SEND NOTIFICATIONS
    menu := &tele.ReplyMarkup{}
    menu.Inline(menu.Row(
        menu.Data("📥 Принять заказ", fmt.Sprintf("take_%d", orderID)),
        menu.Data("❌ Закрыть", "close_msg"),
    ))
    
    for id := range targetIDs {
        var teleID int64
        b.DB.QueryRow(context.Background(), "SELECT telegram_id FROM users WHERE id=$1", id).Scan(&teleID)
        if teleID != 0 {
            target.Bot.Send(&tele.User{ID: teleID}, text, menu, tele.ModeHTML)
        }
    }
}
```

---

## ⚠️ IDENTIFIED BUGS & ISSUES

### BUG #1: Driver Bot Peer Not Found ❌ **CRITICAL**

**Problem Line**: 1762-1767
```go
target := b
if b.Type != BotTypeDriver {
    if p, ok := b.Peers[BotTypeDriver]; ok {
        target = p
    } else {
        b.Log.Error("Driver bot peer not found for notification")
        return  // ❌ EXIT WITHOUT SENDING!
    }
}
```

**When This Fails**:
- If `notifyDrivers()` called from ADMIN BOT
- And driver bot peer NOT properly linked
- Function returns EARLY without sending any notifications!

**Check**: In main.go (Line 67-77), peers are linked:
```go
// Driver Peers
driverBot.Peers[BotTypeClient] = clientBot
driverBot.Peers[BotTypeAdmin] = adminBot

// Admin Peers
adminBot.Peers[BotTypeClient] = clientBot
adminBot.Peers[BotTypeDriver] = driverBot  // ✅ This should exist
```

**If this is NOT set correctly → NO notifications!**

---

### BUG #2: Status Check Too Strict ❌

**Problem Line**: 1779-1782
```go
if u.Role != "driver" || u.Status != "active" {
    continue
}
```

**Issue**: 
- Drivers with status "pending_signup" or "pending_review" are SKIPPED
- Only drivers with status "active" get notifications
- This is CORRECT for approved drivers
- But what if admin wants to test? Need better logging.

**Status**: ✅ **Correct Logic**

---

### BUG #3: Tariff Check Logic ❌ **CRITICAL**

**Problem Line**: 1784-1788
```go
enabled, _ := b.Stg.Tariff().GetEnabled(context.Background(), u.ID)
if !enabled[tariffID] {
    continue  // Skip driver
}
```

**Issue**: 
1. `GetEnabled()` returns map[int64]bool
2. If driver hasn't selected ANY tariffs → returns empty map
3. Empty map[tariffID] returns FALSE
4. Driver is SKIPPED

**But later (line 1800)**:
```go
// Check if driver has any routes
driverRoutes, _ := b.Stg.Route().GetDriverRoutes(context.Background(), u.ID)
if len(driverRoutes) == 0 {
    targetIDs[u.ID] = true  // ✅ Include if no routes
}
```

**MISMATCH**: 
- For routes: Include driver if no routes set (default)
- For tariffs: EXCLUDE driver if no tariffs set
- **INCONSISTENT LOGIC!**

**Impact**: 
- If driver doesn't select tariffs → won't get ANY orders!
- Driver must actively select tariffs to get notifications

**Status**: ❌ **BUG - Inconsistent with route logic**

---

### BUG #4: Database Query in Loop ⚠️

**Problem Line**: 1814-1818
```go
for id := range targetIDs {
    var teleID int64
    b.DB.QueryRow(context.Background(), "SELECT telegram_id FROM users WHERE id=$1", id).Scan(&teleID)
    if teleID != 0 {
        target.Bot.Send(&tele.User{ID: teleID}, text, menu, tele.ModeHTML)
    }
}
```

**Issue**:
- Direct DB query in loop (N queries)
- Already fetched users before (line 1775)
- Should use THAT data instead

**Status**: ⚠️ **Performance Issue** (not critical for small scale)

---

## 📋 ROOT CAUSE ANALYSIS

### Why Notifications Not Working:

**Most Likely Causes (In Order of Probability)**:

1. **Tariff Check Logic** (BUG #3)
   - Driver didn't select tariffs
   - `enabled[tariffID]` returns false
   - Driver SKIPPED
   - **FIX**: Change tariff logic to match route logic (default to all if none selected)

2. **Bot Peer Not Linked** (BUG #1)
   - `adminBot.Peers[BotTypeDriver]` not set
   - `notifyDrivers()` can't find driver bot
   - Returns early without error
   - **FIX**: Verify peers are linked in main.go

3. **Driver Status Not Active** (BUG #2)
   - Driver still "pending_review"
   - Notification check skips them
   - **FIX**: Approve driver first (via admin panel)

4. **Route Matching** (Design Issue)
   - Client order route doesn't match driver route
   - AND driver hasn't set default route (empty)
   - Driver skipped
   - **FIX**: Driver should set routes or select "All Routes" option

---

## 🔧 FIXES NEEDED

### Fix #1: Tariff Logic - Make Consistent with Routes

**File**: pkg/bot/bot.go, Line 1784-1790

**Change From**:
```go
enabled, _ := b.Stg.Tariff().GetEnabled(context.Background(), u.ID)
if !enabled[tariffID] {
    continue  // SKIP driver if not enabled
}
```

**Change To**:
```go
enabled, _ := b.Stg.Tariff().GetEnabled(context.Background(), u.ID)
// If driver has selected tariffs, check if this one is enabled
if len(enabled) > 0 && !enabled[tariffID] {
    continue  // Skip if tariff not enabled
}
// If driver hasn't selected any tariffs (empty), include by default
```

---

### Fix #2: Add Debug Logging

**File**: pkg/bot/bot.go, Line 1751

**Add After Line 1773**:
```go
b.Log.Info("notifyDrivers() called",
    logger.Int64("orderID", orderID),
    logger.Int64("fromID", fromID),
    logger.Int64("toID", toID),
    logger.Int64("tariffID", tariffID),
)
```

**Add Inside Driver Loop (Line 1777)**:
```go
b.Log.Info("Checking driver for notification",
    logger.Int64("driver_id", u.ID),
    logger.String("status", u.Status),
    logger.String("role", u.Role),
)
```

---

### Fix #3: Check Bot Peer Linking

**File**: cmd/main.go, Lines 67-77

**Verify**:
```go
// Driver Peers
driverBot.Peers[BotTypeClient] = clientBot
driverBot.Peers[BotTypeAdmin] = adminBot

// Admin Peers
adminBot.Peers[BotTypeClient] = clientBot
adminBot.Peers[BotTypeDriver] = driverBot  // ← MUST BE SET
```

---

### Fix #4: Use Pre-fetched Users Data

**File**: pkg/bot/bot.go, Line 1814

**Change From**:
```go
for id := range targetIDs {
    var teleID int64
    b.DB.QueryRow(...).Scan(&teleID)
    if teleID != 0 {
        target.Bot.Send(...)
    }
}
```

**Change To**:
```go
// Create map of user IDs to Telegram IDs (from already fetched users)
userMap := make(map[int64]int64)
for _, u := range users {
    userMap[u.ID] = u.TelegramID
}

// Use map instead of querying
for id := range targetIDs {
    if teleID, ok := userMap[id]; ok && teleID != 0 {
        target.Bot.Send(&tele.User{ID: teleID}, text, menu, tele.ModeHTML)
    }
}
```

---

## 🎯 TESTING CHECKLIST

To verify notifications work:

```
1. ✅ Driver Setup
   - [ ] Create driver account
   - [ ] Register car info
   - [ ] GET APPROVED by admin (status="active")
   - [ ] SELECT TARIFFS (e.g., "Economy", "Comfort")
   - [ ] SELECT ROUTES (e.g., "Fergona → Margilan")

2. ✅ Client Setup
   - [ ] Create client account
   - [ ] Make order:
     * From: Fergona
     * To: Margilan
     * Tariff: Economy (SAME as driver selected)
     * Time: Bugun 18:00

3. ✅ Admin Action
   - [ ] Go to "📦 Заказы на подтверждении"
   - [ ] Find the order
   - [ ] Click "✅ Одобрить"
   - [ ] Check bot logs for "notifyDrivers() called"

4. ✅ Check Driver
   - [ ] Driver should get notification
   - [ ] Message should show order details
   - [ ] [📥 Принять заказ] button should work
```

---

## 📊 SUMMARY

| Issue | Severity | Location | Fix |
|-------|----------|----------|-----|
| Tariff Logic Inconsistent | 🔴 HIGH | L1784 | Change to default-allow logic |
| Bot Peer Not Set | 🔴 HIGH | main.go L75 | Verify peers linked |
| Debug Logging Missing | 🟡 MEDIUM | L1751 | Add detailed logs |
| Performance (DB query loop) | 🟢 LOW | L1814 | Use pre-fetched data |

