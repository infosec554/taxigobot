# TaxiBot - Detailed Functionality & Flow Analysis

## 📋 System Overview

**TaxiBot** is a **3-Bot Telegram Ecosystem** for a taxi ordering system. It's designed with separate bot interfaces for different user types, all connected to a shared PostgreSQL database.

### Three Bot Types:
1. **Client Bot** - Customers create and track orders
2. **Driver Bot** - Drivers accept orders and manage their work
3. **Admin Bot** - Administrators manage system (users, tariffs, cities, orders)

---

## 🏗️ Architecture

### Bot Initialization Flow (main.go)

```
1. Load Configuration (.env file)
   ↓
2. Initialize Logger
   ↓
3. Connect to PostgreSQL (Shared Storage)
   ↓
4. Create 3 Bots:
   - Client Bot (BotTypeClient)
   - Driver Bot (BotTypeDriver)
   - Admin Bot (BotTypeAdmin)
   ↓
5. Link Bots as Peers (for inter-bot communication):
   - Client Bot knows about Driver & Admin bots
   - Driver Bot knows about Client & Admin bots
   - Admin Bot knows about Client & Driver bots
   ↓
6. Start Web Server (API for mini apps)
   ↓
7. Run All 3 Bots in Parallel (goroutines)
   ↓
8. Listen for Shutdown Signals (graceful shutdown)
```

---

## 🔄 User Flow by Role

### CLIENT FLOW (Order Creation & Tracking)

```
User Sends /start
    ↓
Bot Checks: Is user in database?
    ├─ NO: Create user record
    └─ YES: Fetch existing user
    ↓
Check User Status:
    ├─ blocked → Send "Account blocked" message
    └─ pending → Ask for phone number sharing
    ↓
Phone Number Shared
    ↓
Set Status to "active"
    ↓
Display Client Menu:
    ├─ ➕ Create Order
    └─ 📋 My Orders
```

#### **Order Creation Workflow (StateFlow)**

```
Client clicks "➕ Create Order"
    ↓
State: awaiting_from
    → Bot: "Where should we pick you up?"
    → Client: Sends location name (e.g., "Fergona Railway Station")
    ↓
State: awaiting_to
    → Bot: "Where are you going?"
    → Client: Sends destination (e.g., "Margilan City Center")
    ↓
State: awaiting_tariff
    → Bot: Shows available tariffs with prices
    → Client: Selects tariff (Economy, Comfort, Premium, etc.)
    ↓
State: awaiting_passengers
    → Bot: "How many passengers?"
    → Client: Enters number (1-8)
    ↓
State: awaiting_datetime
    → Bot: "When do you need pickup? (e.g., 'Today 18:00')"
    → Client: Sends date/time
    ↓
State: awaiting_confirm
    → Bot: Shows order summary with calculated price
    → Client: Confirms or cancels
    ↓
Order Created in Database
    ↓
Database Saves:
    ├─ Order ID, Client ID, Driver ID (null initially)
    ├─ From/To Locations (by ID)
    ├─ Tariff ID, Price, Passengers
    ├─ Pickup Time
    ├─ Status: "active" (waiting for driver)
    └─ Created Timestamp
    ↓
Bot Notifies Admin & Available Drivers:
    "🔔 NEW ORDER! #123 | Price: 50,000 som | Route: Fergona → Margilan"
    ↓
Client Waits for Driver
    ↓
Client can check "📋 My Orders" to see status:
    ├─ active (waiting for driver)
    ├─ accepted (driver found)
    ├─ on_way (driver coming)
    ├─ arrived (driver at location)
    ├─ in_progress (trip started)
    ├─ completed (trip done)
    └─ cancelled
```

---

### DRIVER FLOW (Order Acceptance & Delivery)

```
Driver Sends /start
    ↓
Check Status:
    ├─ pending → Ask for phone number
    ├─ pending_signup → Start Registration Form
    ├─ pending_review → "Your profile is under review"
    └─ active → Show driver menu
    ↓
Driver Menu Shown:
    ├─ 📦 Active Orders (browse available orders)
    ├─ 📍 My Routes (cities where they work)
    ├─ 🚕 My Tariffs (which tariff types they accept)
    ├─ Search by Date
    └─ 📋 My Orders (orders they've accepted)
```

#### **Driver Registration Flow (pending_signup)**

```
Driver Completes Phone Verification
    ↓
Status: pending_signup
    ↓
Bot: "Enter car brand" → State: awaiting_car_brand
    ↓
Driver sends car brand (e.g., "Toyota", "Chevrolet")
    ↓
Bot: "Enter car model" → State: awaiting_car_model
    ↓
Driver sends model (e.g., "Camry", "Nexia")
    ↓
Option to enter custom model → State: awaiting_car_model_other
    ↓
Bot: "Enter license plate number" → State: awaiting_license_plate
    ↓
Driver sends plate (e.g., "10A123AA")
    ↓
All data saved to DriverProfile table:
    ├─ car_brand
    ├─ car_model
    ├─ license_plate
    └─ owner_id (driver's user ID)
    ↓
Status changed to "pending_review"
    ↓
Admin is notified: "🚖 Driver pending approval"
    ↓
Admin reviews & approves/rejects
    ↓
If approved:
    └─ Status → "active"
    └─ Driver can now accept orders
    ↓
If rejected:
    └─ Stays pending_review (awaits resubmission)
```

#### **Order Acceptance Flow**

```
Driver clicks "📦 Active Orders"
    ↓
Bot fetches orders where:
    ├─ Status = "active" (no driver accepted yet)
    ├─ Tariff ID matches driver's selected tariffs
    └─ Route matches driver's working cities
    ↓
Bot displays 10 orders per page with:
    ├─ Order ID
    ├─ Pickup location
    ├─ Destination
    ├─ Price
    ├─ Passengers
    ├─ Pickup time
    └─ [Accept] button
    ↓
Driver clicks "Accept Order"
    ↓
Bot checks if order still available:
    ├─ YES: Proceed with acceptance
    └─ NO: "❌ Order already taken"
    ↓
Database updates:
    ├─ Order.driver_id = driver's user ID
    ├─ Order.status = "accepted"
    └─ Accept timestamp
    ↓
Notifications sent:
    ├─ Client: "🚖 Driver accepted your order! Name: Ahmed, Phone: +998..."
    ├─ Driver: "✅ Order #123 accepted"
    └─ Admin: Order status changed
```

#### **Trip Progress Updates**

```
Order Status: accepted
    ↓
Driver clicks "📋 My Orders"
    ↓
Shows driver's accepted orders with action buttons:
    ├─ ➡️ On Way (driver leaving to pick up)
    ├─ ✅ Arrived (driver at pickup location)
    ├─ ▶️ Start Trip (passenger in car, trip begins)
    └─ 🏁 Complete (trip finished)

Status Transitions:
    
    accepted → on_way
    (Driver updates, client notified: "🚖 Driver is coming!")
    
    on_way → arrived
    (Driver arrives, client notified: "🚖 Driver has arrived!")
    
    arrived → in_progress
    (Trip started, client notified: "▶️ Trip started!")
    
    in_progress → completed
    (Trip finished, client notified: "🏁 Order completed!")
    ↓
Order removed from active lists
    ↓
Added to order history
```

---

### ADMIN FLOW (System Management)

```
Admin Sends /start
    ↓
Bot Checks: Admin ID from config file
    ├─ Matches: Set role to "admin"
    └─ Doesn't match: Access denied
    ↓
Admin Menu Shown:
    ├─ 👥 Users (manage user roles, block/unblock)
    ├─ 📦 All Orders (view complete order history)
    ├─ 📊 Statistics (system stats)
    ├─ 🚖 Pending Drivers (review driver registrations)
    ├─ 📦 Pending Orders (approve orders before dispatching)
    ├─ ⚙️ Tariffs (add/edit/delete taxi tariffs)
    └─ 🗺 Cities (add/edit/delete working cities)
```

#### **Tariff Management**

```
Admin clicks "⚙️ Tariffs"
    ↓
Shows options:
    ├─ ➕ Add Tariff
    ├─ 🗑 Delete Tariff
    └─ View all tariffs (with prices)
    ↓
Add Tariff Flow:
    State: awaiting_tariff_name
        → Admin: "Enter tariff name" (e.g., "Economy")
        → Admin: "Enter base price" (e.g., "15000")
        → Admin: "Enter price per km" (e.g., "1500")
    ↓
Tariff saved to database
    ↓
Delete Tariff Flow:
    → Shows list of all tariffs
    → Admin selects tariff to delete
    → Confirmation and removal
```

#### **City/Location Management**

```
Admin clicks "🗺 Cities"
    ↓
Options:
    ├─ ➕ Add City
    ├─ 🗑 Delete City
    └─ 🔍 Search City
    ↓
Add City Flow:
    State: awaiting_location_name
        → Admin: "Enter city name"
        → Location saved with unique ID
    ↓
Delete City Flow:
    → Shows all cities with map
    → Admin selects city
    → Removed from system
```

#### **Driver Review & Approval**

```
Admin clicks "🚖 Pending Drivers"
    ↓
Shows drivers with status = "pending_review":
    ├─ Driver name
    ├─ Phone number
    ├─ Car info (brand, model, plate)
    ├─ [Approve] button
    └─ [Reject] button
    ↓
Admin clicks [Approve]:
    ├─ Driver status → "active"
    ├─ Driver notified: "✅ Your profile approved!"
    └─ Driver can now accept orders
    ↓
Admin clicks [Reject]:
    ├─ Driver status stays "pending_review"
    ├─ Driver can resubmit
    └─ Admin can provide reason
```

#### **Order Confirmation**

```
Admin clicks "📦 Pending Orders"
    ↓
Shows orders with status = "pending":
    ├─ Order details (from, to, driver, client)
    ├─ [Approve] button
    └─ [Reject] button
    ↓
Admin approves/rejects order
    ↓
Status updates in database
```

---

## 💾 Database Models

### Order Model
```go
type Order struct {
    ID              int64      // Unique order ID
    ClientID        int64      // User who created order
    DriverID        *int64     // Driver who accepted (null if no driver yet)
    FromLocationID  int64      // Pickup location ID
    ToLocationID    int64      // Destination location ID
    TariffID        int64      // Taxi type (Economy, Comfort, etc.)
    Price           int        // Order price (in currency units)
    Currency        string     // "som", "usd", etc.
    Passengers      int        // Number of passengers
    PickupTime      *time.Time // Requested pickup time
    Status          string     // active, accepted, on_way, arrived, in_progress, completed, cancelled, pending
    CreatedAt       time.Time  // When order was created
    
    // Joined info (from other tables)
    ClientUsername  string
    ClientPhone     string
    FromLocationName string
    ToLocationName  string
}
```

### User Model
```go
type User struct {
    ID              int64
    TelegramID      int64     // Telegram user ID
    Username        string    // Telegram @username
    FirstName       string
    LastName        string
    PhoneNumber     string
    Role            string    // "client", "driver", "admin"
    Status          string    // "pending", "active", "pending_signup", "pending_review", "blocked"
    CreatedAt       time.Time
}
```

### DriverProfile Model
```go
type DriverProfile struct {
    ID              int64
    OwnerID         int64     // User ID of driver
    CarBrand        string    // "Toyota", "Chevrolet", etc.
    CarModel        string    // "Camry", "Nexia", etc.
    LicensePlate    string    // Vehicle license plate
    VerificationStatus string // "pending", "approved", "rejected"
}
```

---

## 🔔 Notification System

### Key Notifications

**When Order Created:**
1. **Admin**: "🔔 NEW ORDER! #123 | Price: 50,000 som | Route: Fergona → Margilan"
2. **Relevant Drivers** (matching tariff + route): Same notification

**When Driver Accepts:**
1. **Client**: "🚖 Driver accepted! Name: Ahmed | Phone: +998-91-123-45-67"
2. **Admin**: Order status changed to "accepted"

**When Driver Updates Status:**
- Client gets real-time notifications:
  - "🚖 Driver is coming!"
  - "🚖 Driver has arrived!"
  - "▶️ Trip started!"
  - "🏁 Order completed!"

**When Driver Registration Changes:**
1. **Admin**: "🚖 Driver pending approval" (on pending_review)
2. **Driver**: Notification when approved or rejected

---

## 🛠️ Handler Functions (Key Operations)

### Client Handlers
| Handler | Function |
|---------|----------|
| `handleStart` | Initialize user session |
| `handleContact` | Process phone verification |
| `handleOrderStart` | Begin order creation |
| `handleMyOrders` | Show client's orders |
| `handleTakeOrderWithID` | (For web app) Accept order |

### Driver Handlers
| Handler | Function |
|---------|----------|
| `handleActiveOrders` | Show available orders |
| `handleMyOrdersDriver` | Show accepted orders |
| `handleDriverRoutes` | Manage working cities |
| `handleDriverTariffs` | Select tariff types |
| `handleDriverOnWay` | Update status to "on_way" |
| `handleDriverArrived` | Update status to "arrived" |
| `handleDriverStartTrip` | Update status to "in_progress" |

### Admin Handlers
| Handler | Function |
|---------|----------|
| `handleAdminUsers` | Manage users |
| `handleAdminOrders` | View all orders history |
| `handleAdminTariffs` | Manage tariffs |
| `handleAdminLocations` | Manage cities |
| `handleAdminStats` | System statistics |
| `handleAdminPendingDrivers` | Review drivers |
| `handleAdminPendingOrders` | Approve orders |

---

## 📊 Order Status Flow (State Machine)

```
     ┌─────────────┐
     │   pending   │ (awaiting approval)
     └──────┬──────┘
            ↓
     ┌─────────────┐
     │   active    │ (waiting for driver)
     └──────┬──────┘
            ↓
     ┌─────────────┐
     │  accepted   │ (driver found)
     └──────┬──────┘
            ↓
     ┌─────────────┐
     │   on_way    │ (driver coming)
     └──────┬──────┘
            ↓
     ┌─────────────┐
     │   arrived   │ (driver at location)
     └──────┬──────┘
            ↓
     ┌─────────────┐
     │ in_progress │ (trip ongoing)
     └──────┬──────┘
            ↓
     ┌─────────────┐
     │  completed  │ (trip finished)
     └─────────────┘

     (At any point)
            ↓
     ┌─────────────┐
     │  cancelled  │
     └─────────────┘
```

---

## 🌐 Inter-Bot Communication (Peer System)

Bots can notify each other through the `Peers` map:

```go
Peers map[BotType]*Bot {
    BotTypeDriver: driverBotInstance,
    BotTypeClient: clientBotInstance,
    BotTypeAdmin:  adminBotInstance,
}
```

**Example Use Cases:**
- Client orders → Notify drivers in `driverBot.notifyDrivers()`
- Driver accepts → Notify client in `b.notifyUser(clientID, message)`
- Admin approves driver → Notify driver in driver bot

---

## 📝 Session Management

Each user has a `UserSession` stored in memory:

```go
type UserSession struct {
    DBID            int64          // Database user ID
    State           string         // Current state (awaiting_from, awaiting_to, etc.)
    OrderData       *models.Order  // Temporary order data during creation
    TempString      string         // Temporary string storage
    LastActionTime  time.Time      // Last user action timestamp
    DriverProfile   *models.DriverProfile
}
```

**Flow:**
1. User sends message → Bot loads their session
2. Bot checks current `State` → Determines what to do with message
3. Bot updates `State` → Moves to next step
4. On completion → Save to database, reset session state

---

## 🔐 Security & Validation

### Admin Access Control
```go
if b.Type == BotTypeAdmin {
    if c.Sender().ID != b.Cfg.AdminID {
        return c.Send("Access denied")
    }
}
```

### User Role Validation
```go
if b.Type == BotTypeDriver && user.Role == "client" {
    return c.Send("You must register as a driver")
}
```

### Status Checks
- Blocked users → Can't interact
- Pending users → Must share phone number
- Pending_review → Can't accept orders yet

---

## 📦 Data Flow Summary

```
Telegram User
    ↓
Telegram Bot API
    ↓
Bot Handler (processes command/message)
    ↓
Session Management (check state)
    ↓
Database Storage (PostgreSQL)
    ↓
Response to User
    ↓
Notifications to Other Users/Bots
```

---

## 🚀 Key Features

✅ **Multi-role system** (Client, Driver, Admin)
✅ **Real-time order matching** (drivers see relevant orders)
✅ **Order status tracking** (7-step journey)
✅ **Driver verification** (profile review system)
✅ **Tariff management** (flexible pricing)
✅ **Location management** (city-based filtering)
✅ **Session-based flow** (state machine)
✅ **Inter-bot communication** (peer notifications)
✅ **Shared database** (PostgreSQL)
✅ **Web API** (for mini-apps)

