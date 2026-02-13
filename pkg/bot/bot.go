package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"taxibot/config"
	"taxibot/pkg/logger"
	"taxibot/pkg/models"
	"taxibot/storage"

	"github.com/jackc/pgx/v5/pgxpool"
	tele "gopkg.in/telebot.v3"
)

type BotType string

const (
	BotTypeClient      BotType = "client"
	BotTypeDriverAdmin BotType = "driver_admin"
)

type UserSession struct {
	DBID       int64
	State      string
	OrderData  *models.Order
	TempString string
}

type Bot struct {
	Type     BotType
	Bot      *tele.Bot
	DB       *pgxpool.Pool
	Log      logger.ILogger
	Cfg      *config.Config
	Stg      storage.IStorage
	Sessions map[int64]*UserSession
	Peer     *Bot // Link for cross-bot notifications
}

const (
	StateIdle       = "idle"
	StateFrom       = "awaiting_from"
	StateTo         = "awaiting_to"
	StateTariff     = "awaiting_tariff"
	StatePassengers = "awaiting_passengers"
	StateDateTime   = "awaiting_datetime"
	StateConfirm    = "awaiting_confirm"

	StateTariffAdd    = "awaiting_tariff_name"
	StateDirectionAdd = "awaiting_direction_from"
	StateDirectionTo  = "awaiting_direction_to"
)

func New(botType BotType, cfg *config.Config, stg storage.IStorage, log logger.ILogger) (*Bot, error) {
	token := cfg.TelegramBotToken
	if botType == BotTypeDriverAdmin {
		token = cfg.AdminBotToken
	}

	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}
	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, err
	}
	bot := &Bot{
		Type:     botType,
		Bot:      b,
		DB:       stg.GetPool(),
		Log:      log,
		Cfg:      cfg,
		Stg:      stg,
		Sessions: make(map[int64]*UserSession),
	}
	bot.registerHandlers()
	return bot, nil
}

func (b *Bot) Start() {
	b.Log.Info(fmt.Sprintf("🤖 %s Bot Started...", b.Type))
	b.Bot.Start()
}

var messages = map[string]map[string]string{
	"uz": {
		"welcome":       "👋 Assalomu alaykum! Tizimga xush kelibsiz.",
		"contact_msg":   "Ro'yxatdan o'tish uchun telefon raqamingizni yuboring:",
		"share_contact": "📱 Raqamni Ulashish",
		"registered":    "🎉 Ro'yxatdan muvaffaqiyatli o'tdingiz!",
		"blocked":       "🚫 Sizning hisobingiz bloklangan.",
		"no_entry":      "🚫 Ushbu bot faqat haydovchilar va adminlar uchun.",
		"menu_client":   "👤 Mijoz menyusi:",
		"menu_driver":   "🚖 Haydovchi menyusi:",
		"menu_admin":    "🛠 Admin paneli:",
		"order_from":    "📍 Qayerdan olasiz? (Shahar/tuman nomi)",
		"order_to":      "🏁 Qayerga borasiz? (Shahar/tuman nomi)",
		"order_tariff":  "🚕 Tarifni tanlang:",
		"order_pass":    "👥 Yo'lovchilar sonini kiriting:",
		"order_time":    "📅 Ketish vaqti va sanasini kiriting (Masalan: Bugun 18:00):",
		"order_confirm": "💰 Buyurtma tafsilotlari:\nNarx: %d %s\n\nTasdiqlaysizmi?",
		"order_created": "✅ Buyurtmangiz qabul qilindi!",
		"no_orders":     "📭 Hozircha faol buyurtmalar yo'q.",
		"notif_new":     "🔔 YANGI ZAKAZ!\n🆔 #%d\n💰 Narx: %d %s\n📍 Yo'l: %s",
		"notif_taken":   "🚖 Buyurtmangiz haydovchi tomonidan qabul qilindi!\n🆔 ID: #%d\n🚗 Haydovchi: %s",
		"notif_done":    "🏁 Buyurtmangiz muvaffaqiyatli yakunlandi. Rahmat!",
		"notif_cancel":  "⚠️ Buyurtma #%d bekor qilindi.",
	},
}

func (b *Bot) registerHandlers() {
	b.Bot.Handle("/start", b.handleStart)

	if b.Type == BotTypeClient {
		b.Bot.Handle(tele.OnContact, b.handleContact)
		b.Bot.Handle("➕ Zakaz berish", b.handleOrderStart)
		b.Bot.Handle("📋 Mening zakazlarim", b.handleMyOrders)
	} else {
		b.Bot.Handle("📦 Faol zakazlar", b.handleActiveOrders)
		b.Bot.Handle("📋 Mening zakazlarim", b.handleMyOrdersDriver)
		b.Bot.Handle("👥 Userlar", b.handleAdminUsers)
		b.Bot.Handle("📦 Jami zakazlar", b.handleAdminOrders)
		b.Bot.Handle("⚙️ Tariflar", b.handleAdminTariffs)
		b.Bot.Handle("🗺 Yo'nalishlar", b.handleAdminDirections)
		b.Bot.Handle("📊 Statistika", b.handleAdminStats)
		b.Bot.Handle("➕ Tarif qo'shish", b.handleTariffAddStart)
		b.Bot.Handle("➕ Yo'nalish qo'shish", b.handleDirectionAddStart)
		b.Bot.Handle("🏠 Asosiy menyu", b.handleStart)
		b.Bot.Handle("🏠 Menyuga qaytish", b.handleStart)
	}

	b.Bot.Handle(tele.OnCallback, b.handleCallback)
	b.Bot.Handle(tele.OnText, b.handleText)
}

func (b *Bot) handleStart(c tele.Context) error {
	ctx := context.Background()
	user, _ := b.Stg.User().GetOrCreate(ctx, c.Sender().ID, c.Sender().Username, fmt.Sprintf("%s %s", c.Sender().FirstName, c.Sender().LastName))

	// 🛠 ADMIN INITIALIZATION (ID or Username)
	isAdmin := (b.Cfg.AdminID != 0 && c.Sender().ID == b.Cfg.AdminID) ||
		(b.Cfg.AdminUsername != "" && c.Sender().Username == b.Cfg.AdminUsername)

	if isAdmin && user.Role != "admin" {
		b.Stg.User().UpdateRole(ctx, c.Sender().ID, "admin")
		b.Stg.User().UpdateStatus(ctx, c.Sender().ID, "active")
		user, _ = b.Stg.User().Get(ctx, c.Sender().ID)
	}

	b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}

	if user.Status == "blocked" {
		return c.Send(messages["uz"]["blocked"])
	}

	if b.Type == BotTypeDriverAdmin {
		if user.Role != "admin" && user.Role != "driver" {
			return c.Send(messages["uz"]["no_entry"])
		}
	}

	if user.Status == "pending" && b.Type == BotTypeClient {
		menu := &tele.ReplyMarkup{ResizeKeyboard: true}
		menu.Reply(menu.Row(menu.Contact(messages["uz"]["share_contact"])))
		return c.Send(messages["uz"]["contact_msg"], menu)
	}

	return b.showMenu(c, user)
}

func (b *Bot) handleContact(c tele.Context) error {
	if b.Type != BotTypeClient {
		return nil
	}
	if c.Message().Contact.UserID != c.Sender().ID {
		return c.Send("O'zingizni raqamingizni yuboring.")
	}
	ctx := context.Background()
	b.Stg.User().UpdatePhone(ctx, c.Sender().ID, c.Message().Contact.PhoneNumber)
	b.Stg.User().UpdateStatus(ctx, c.Sender().ID, "active")
	user, _ := b.Stg.User().Get(ctx, c.Sender().ID)
	c.Send(messages["uz"]["registered"], tele.RemoveKeyboard)
	return b.showMenu(c, user)
}

func (b *Bot) showMenu(c tele.Context, user *models.User) error {
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}

	if b.Type == BotTypeClient {
		menu.Reply(menu.Row(menu.Text("➕ Zakaz berish")), menu.Row(menu.Text("📋 Mening zakazlarim")))
		return c.Send(messages["uz"]["menu_client"], menu)
	}

	if user.Role == "admin" {
		menu.Reply(
			menu.Row(menu.Text("👥 Userlar"), menu.Text("📦 Jami zakazlar")),
			menu.Row(menu.Text("⚙️ Tariflar"), menu.Text("🗺 Yo'nalishlar")),
			menu.Row(menu.Text("📊 Statistika")),
			menu.Row(menu.Text("📦 Faol zakazlar"), menu.Text("📋 Mening zakazlarim")),
		)
		return c.Send(messages["uz"]["menu_admin"], menu)
	}

	menu.Reply(menu.Row(menu.Text("📦 Faol zakazlar")), menu.Row(menu.Text("📋 Mening zakazlarim")))
	return c.Send(messages["uz"]["menu_driver"], menu)
}

func (b *Bot) handleOrderStart(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	session.State = StateFrom
	session.OrderData = &models.Order{ClientID: session.DBID}
	return c.Send(messages["uz"]["order_from"], tele.RemoveKeyboard)
}

func (b *Bot) handleActiveOrders(c tele.Context) error {
	orders, _ := b.Stg.Order().GetActiveOrders(context.Background())
	if len(orders) == 0 {
		return c.Send(messages["uz"]["no_orders"])
	}

	for _, o := range orders {
		txt := fmt.Sprintf("📦 ZAKAZ #%d\n💰 Narx: %d %s\n👥 Yo'lovchilar: %d", o.ID, o.Price, o.Currency, o.Passengers)
		menu := &tele.ReplyMarkup{}
		menu.Inline(menu.Row(menu.Data("📥 Zakazni olish", fmt.Sprintf("take_%d", o.ID))))
		c.Send(txt, menu)
	}
	return nil
}

func (b *Bot) handleMyOrdersDriver(c tele.Context) error {
	orders, _ := b.Stg.Order().GetDriverOrders(context.Background(), b.Sessions[c.Sender().ID].DBID)
	if len(orders) == 0 {
		return c.Send("Sizda olingan zakazlar yo'q.")
	}

	for _, o := range orders {
		txt := fmt.Sprintf("🚖 ZAKAZ #%d\n💰 Narx: %d %s\n📊 Status: %s", o.ID, o.Price, o.Currency, o.Status)
		menu := &tele.ReplyMarkup{}
		if o.Status == "taken" {
			menu.Inline(menu.Row(menu.Data("✅ Yakunlash", fmt.Sprintf("complete_%d", o.ID))))
		}
		c.Send(txt, menu)
	}
	return nil
}

func (b *Bot) handleAdminUsers(c tele.Context) error {
	users, _ := b.Stg.User().GetAll(context.Background())
	for _, u := range users {
		txt := fmt.Sprintf("👤 %s\n📞 %s\nRole: %s\nStatus: %s", u.FullName, u.Phone, u.Role, u.Status)
		menu := &tele.ReplyMarkup{}
		btnRole := menu.Data("🚕 Driver qilish", fmt.Sprintf("set_role_driver_%d", u.TelegramID))
		if u.Role == "driver" {
			btnRole = menu.Data("👤 Client qilish", fmt.Sprintf("set_role_client_%d", u.TelegramID))
		}
		btnStatus := menu.Data("🚫 Block", fmt.Sprintf("user_blk_%d", u.TelegramID))
		if u.Status == "blocked" {
			btnStatus = menu.Data("✅ Activate", fmt.Sprintf("user_act_%d", u.TelegramID))
		}
		menu.Inline(menu.Row(btnRole, btnStatus))
		c.Send(txt, menu)
	}
	return nil
}

func (b *Bot) handleAdminOrders(c tele.Context) error {
	orders, _ := b.Stg.Order().GetAll(context.Background())
	for _, o := range orders {
		txt := fmt.Sprintf("📦 #%d | Status: %s\n💰 Narx: %d", o.ID, o.Status, o.Price)
		menu := &tele.ReplyMarkup{}
		if o.Status != "completed" && o.Status != "cancelled" {
			menu.Inline(menu.Row(menu.Data("❌ Bekor qilish (Admin)", fmt.Sprintf("cancel_%d", o.ID))))
			c.Send(txt, menu)
		} else {
			c.Send(txt)
		}
	}
	return nil
}

func (b *Bot) handleAdminTariffs(c tele.Context) error {
	tariffs, _ := b.Stg.Tariff().GetAll(context.Background())
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	menu.Reply(menu.Row(menu.Text("➕ Tarif qo'shish")), menu.Row(menu.Text("🏠 Asosiy menyu")))
	c.Send("⚙️ Tariflar boshqaruvi:", menu)
	for _, t := range tariffs {
		c.Send(fmt.Sprintf("🚕 %s", t.Name))
	}
	return nil
}

func (b *Bot) handleAdminDirections(c tele.Context) error {
	directions, _ := b.Stg.Direction().GetAll(context.Background())
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	menu.Reply(menu.Row(menu.Text("➕ Yo'nalish qo'shish")), menu.Row(menu.Text("🏠 Asosiy menyu")))
	c.Send("🗺 Yo'nalishlar boshqaruvi:", menu)
	for _, d := range directions {
		c.Send(fmt.Sprintf("📍 %s ➡️ %s", d.FromLocation, d.ToLocation))
	}
	return nil
}

func (b *Bot) handleAdminStats(c tele.Context) error {
	orders, _ := b.Stg.Order().GetAll(context.Background())
	users, _ := b.Stg.User().GetAll(context.Background())
	active := 0
	for _, o := range orders {
		if o.Status == "active" || o.Status == "taken" {
			active++
		}
	}
	return c.Send(fmt.Sprintf("📊 STATISTICS\n\nJami foydalanuvchilar: %d\nFaol zakazlar: %d\nJami zakazlar: %d", len(users), active, len(orders)))
}

func (b *Bot) handleText(c tele.Context) error {
	session, ok := b.Sessions[c.Sender().ID]
	if !ok || session.State == StateIdle {
		return nil
	}

	switch session.State {
	case StateFrom:
		session.TempString = c.Text()
		session.State = StateTo
		return c.Send(messages["uz"]["order_to"])
	case StateTo:
		session.TempString = session.TempString + " ➡️ " + c.Text()
		session.State = StateTariff
		tariffs, _ := b.Stg.Tariff().GetAll(context.Background())
		menu := &tele.ReplyMarkup{}
		var rows []tele.Row
		for _, t := range tariffs {
			rows = append(rows, menu.Row(menu.Data(t.Name, fmt.Sprintf("tf_%d", t.ID))))
		}
		menu.Inline(rows...)
		return c.Send(messages["uz"]["order_tariff"], menu)
	case StatePassengers:
		num, _ := strconv.Atoi(c.Text())
		session.OrderData.Passengers = num
		session.State = StateDateTime
		return c.Send(messages["uz"]["order_time"])
	case StateDateTime:
		now := time.Now()
		session.OrderData.PickupTime = &now
		session.State = StateConfirm
		session.OrderData.Price = 50000
		session.OrderData.Currency = "UZS"
		menu := &tele.ReplyMarkup{}
		menu.Inline(menu.Row(menu.Data("✅ Tasdiqlash", "confirm_yes"), menu.Data("❌ Bekor qilish", "confirm_no")))
		return c.Send(fmt.Sprintf(messages["uz"]["order_confirm"], session.OrderData.Price, session.OrderData.Currency), menu)
	case StateTariffAdd:
		b.Stg.Tariff().Create(context.Background(), c.Text())
		session.State = StateIdle
		return b.showMenu(c, b.getCurrentUser(c))
	case StateDirectionAdd:
		session.TempString = c.Text()
		session.State = StateDirectionTo
		return c.Send("🏁 Qayerga?")
	case StateDirectionTo:
		b.Stg.Direction().Create(context.Background(), session.TempString, c.Text())
		session.State = StateIdle
		return b.showMenu(c, b.getCurrentUser(c))
	}
	return nil
}

func (b *Bot) handleCallback(c tele.Context) error {
	data := c.Callback().Data
	session := b.Sessions[c.Sender().ID]

	if strings.HasPrefix(data, "tf_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "tf_"), 10, 64)
		session.OrderData.TariffID = id
		session.State = StatePassengers
		b.Bot.Edit(c.Callback().Message, messages["uz"]["order_tariff"])
		return c.Send(messages["uz"]["order_pass"])
	}

	if strings.HasPrefix(data, "take_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "take_"), 10, 64)
		err := b.Stg.Order().TakeOrder(context.Background(), id, session.DBID)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "Xatolik: " + err.Error(), ShowAlert: true})
		}
		b.Bot.Edit(c.Callback().Message, "✅ Buyurtma qabul qilindi!")
		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		driver, _ := b.Stg.User().Get(context.Background(), c.Sender().ID)
		b.notifyUser(order.ClientID, fmt.Sprintf(messages["uz"]["notif_taken"], id, driver.FullName))
		return c.Respond()
	}

	if strings.HasPrefix(data, "complete_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "complete_"), 10, 64)
		b.Stg.Order().CompleteOrder(context.Background(), id)
		b.Bot.Edit(c.Callback().Message, "🏁 Buyurtma yakunlandi!")
		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		b.notifyUser(order.ClientID, messages["uz"]["notif_done"])
		return c.Respond()
	}

	if strings.HasPrefix(data, "cancel_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "cancel_"), 10, 64)
		b.Stg.Order().CancelOrder(context.Background(), id)
		b.Bot.Edit(c.Callback().Message, "❌ Bekor qilindi.")
		return c.Respond()
	}

	if strings.HasPrefix(data, "user_blk_") || strings.HasPrefix(data, "user_act_") || strings.HasPrefix(data, "set_role_") {
		return b.handleAdminCallbacks(c, data)
	}

	switch data {
	case "confirm_yes":
		session.OrderData.Status = "active"
		order, err := b.Stg.Order().Create(context.Background(), session.OrderData)
		if err == nil {
			c.Send(messages["uz"]["order_created"])
			b.notifyDrivers(fmt.Sprintf(messages["uz"]["notif_new"], order.ID, order.Price, order.Currency, session.TempString))
		}
		session.State = StateIdle
		return b.showMenu(c, b.getCurrentUser(c))
	case "confirm_no":
		session.State = StateIdle
		c.Send("❌ Bekor qilindi.")
		return b.showMenu(c, b.getCurrentUser(c))
	}
	return nil
}

func (b *Bot) handleAdminCallbacks(c tele.Context, data string) error {
	if b.Type != BotTypeDriverAdmin {
		return nil
	}
	if strings.HasPrefix(data, "set_role_") {
		parts := strings.Split(data, "_")
		id, _ := strconv.ParseInt(parts[3], 10, 64)
		b.Stg.User().UpdateRole(context.Background(), id, parts[2])
		return c.Respond(&tele.CallbackResponse{Text: "OK"})
	}
	if strings.HasPrefix(data, "user_blk_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "user_blk_"), 10, 64)
		b.Stg.User().UpdateStatus(context.Background(), id, "blocked")
		return c.Respond(&tele.CallbackResponse{Text: "Bloklandi"})
	}
	if strings.HasPrefix(data, "user_act_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "user_act_"), 10, 64)
		b.Stg.User().UpdateStatus(context.Background(), id, "active")
		return c.Respond(&tele.CallbackResponse{Text: "Aktiv qilindi"})
	}
	return nil
}

func (b *Bot) notifyUser(dbID int64, text string) {
	target := b
	if b.Type != BotTypeClient && b.Peer != nil {
		target = b.Peer
	}
	var teleID int64
	b.DB.QueryRow(context.Background(), "SELECT telegram_id FROM users WHERE id=$1", dbID).Scan(&teleID)
	if teleID != 0 {
		target.Bot.Send(&tele.User{ID: teleID}, text)
	}
}

func (b *Bot) notifyDrivers(text string) {
	target := b
	if b.Type != BotTypeDriverAdmin && b.Peer != nil {
		target = b.Peer
	}
	users, _ := b.Stg.User().GetAll(context.Background())
	for _, u := range users {
		if u.Role == "driver" || u.Role == "admin" {
			target.Bot.Send(&tele.User{ID: u.TelegramID}, text)
		}
	}
}

func (b *Bot) handleMyOrders(c tele.Context) error {
	orders, _ := b.Stg.Order().GetClientOrders(context.Background(), b.Sessions[c.Sender().ID].DBID)
	if len(orders) == 0 {
		return c.Send("Sizda zakazlar yo'q.")
	}
	for _, o := range orders {
		txt := fmt.Sprintf("📦 #%d: %s", o.ID, o.Status)
		menu := &tele.ReplyMarkup{}
		if o.Status == "active" {
			menu.Inline(menu.Row(menu.Data("❌ Bekor qilish", fmt.Sprintf("cancel_%d", o.ID))))
		}
		c.Send(txt, menu)
	}
	return nil
}

func (b *Bot) handleTariffAddStart(c tele.Context) error {
	b.Sessions[c.Sender().ID].State = StateTariffAdd
	return c.Send("Tarif nomini yozing:")
}

func (b *Bot) handleDirectionAddStart(c tele.Context) error {
	b.Sessions[c.Sender().ID].State = StateDirectionAdd
	return c.Send("Qayerdan?")
}

func (b *Bot) getCurrentUser(c tele.Context) *models.User {
	u, _ := b.Stg.User().Get(context.Background(), c.Sender().ID)
	return u
}
