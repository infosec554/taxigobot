package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tele "gopkg.in/telebot.v3"

	"taxibot/config"
	"taxibot/pkg/logger"
	"taxibot/pkg/models"
	"taxibot/storage"
)

type BotType string

const (
	BotTypeClient BotType = "client"
	BotTypeDriver BotType = "driver"
	BotTypeAdmin  BotType = "admin"
)

type UserSession struct {
	DBID           int64
	State          string
	OrderData      *models.Order
	TempString     string
	LastActionTime time.Time
	DriverProfile  *models.DriverProfile
}

type Bot struct {
	Type     BotType
	Bot      *tele.Bot
	DB       *pgxpool.Pool
	Log      logger.ILogger
	Cfg      *config.Config
	Stg      storage.IStorage
	Sessions map[int64]*UserSession
	Peers    map[BotType]*Bot // Map of other bots to communicate with
}

const (
	StateIdle       = "idle"
	StateFrom       = "awaiting_from"
	StateTo         = "awaiting_to"
	StateTariff     = "awaiting_tariff"
	StatePassengers = "awaiting_passengers"
	StateDateTime   = "awaiting_datetime"
	StateConfirm    = "awaiting_confirm"

	StateTariffAdd   = "awaiting_tariff_name"
	StateLocationAdd = "awaiting_location_name"
	StateCarBrandAdd = "awaiting_car_brand_name"
	StateCarModelAdd = "awaiting_car_model_name"

	StateDriverRouteFrom = "awaiting_driver_route_from"
	StateDriverRouteTo   = "awaiting_driver_route_to"

	StateAdminLogin    = "awaiting_admin_login"
	StateAdminPassword = "awaiting_admin_password"

	StateCarBrand      = "awaiting_car_brand"
	StateCarModel      = "awaiting_car_model"
	StateCarModelOther = "awaiting_car_model_other"
	StateLicensePlate  = "awaiting_license_plate"

	StatePrice = "awaiting_price"
)

func (b *Bot) handleWebApp(c tele.Context) error {
	data := c.Message().WebAppData.Data
	// data is a JSON string like {"action":"take_order","order_id":123}
	// Simplified parsing for now:
	if strings.Contains(data, "take_order") {
		parts := strings.Split(data, ":")
		if len(parts) > 2 {
			idStr := strings.Trim(parts[2], "}\" ")
			id, _ := strconv.ParseInt(idStr, 10, 64)
			// Trigger take logic same as callback
			return b.handleTakeOrderWithID(c, id)
		}
	}
	return nil
}

func (b *Bot) handleTakeOrderWithID(c tele.Context, id int64) error {
	session := b.Sessions[c.Sender().ID]
	var dbID int64
	if session != nil {
		dbID = session.DBID
	} else {
		user := b.getCurrentUser(c)
		dbID = user.ID
	}

	// 1. Check if order is still active (not taken by someone else and approved)
	order, _ := b.Stg.Order().GetByID(context.Background(), id)
	if order == nil || order.Status != "active" {
		return c.Send("❌ Извините, этот заказ уже принят или отменен.")
	}

	// 2. Atomically request the order (active -> wait_confirm + driver_id)
	err := b.Stg.Order().RequestOrder(context.Background(), id, dbID)
	if err != nil {
		return c.Send("❌ Ошибка: " + err.Error())
	}

	c.Send("⏳ Ваш запрос отправлен администратору. Ожидайте подтверждения...")

	// 3. Notify Admin
	driver, _ := b.Stg.User().GetByID(context.Background(), dbID)
	if driver == nil {
		b.Log.Error("Driver not found for notification", logger.Int64("driver_id", dbID))
		return c.Send("❌ Информация о водителе не найдена.")
	}

	phone := "Неизвестно"
	if driver.Phone != nil {
		phone = *driver.Phone
	}

	msg := fmt.Sprintf("🔔 <b>ВОДИТЕЛЬ ХОЧЕТ ПРИНЯТЬ ЗАКАЗ</b>\n\n🆔 Заказ: #%d\n🚖 Водитель: <a href=\"tg://user?id=%d\">%s</a>\n📞 Тел: %s\n\n👤 Клиент: <a href=\"tg://user?id=%d\">%s</a>\n📞 Тел: %s",
		id, driver.TelegramID, driver.FullName, phone, order.ClientID, order.ClientUsername, order.ClientPhone)

	b.notifyAdmin(id, msg, "match") // "match" type allows us to send specific buttons

	return nil
}

func New(botType BotType, cfg *config.Config, stg storage.IStorage, log logger.ILogger) (*Bot, error) {
	token := cfg.TelegramBotToken
	if botType == BotTypeDriver {
		token = cfg.DriverBotToken
	} else if botType == BotTypeAdmin {
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
		Peers:    make(map[BotType]*Bot),
	}
	bot.registerHandlers()
	return bot, nil
}

func (b *Bot) Start() {
	b.Log.Info(fmt.Sprintf("🤖 %s Bot Started...", b.Type))
	b.Bot.Start()
}

var messages = map[string]map[string]string{
	"ru": {
		"welcome":       "👋 Здравствуйте! Добро пожаловать в систему.",
		"contact_msg":   "Для регистрации, пожалуйста, отправьте ваш номер телефона:",
		"share_contact": "📱 Поделиться номером",
		"registered":    "🎉 Вы успешно зарегистрированы!",
		"blocked":       "🚫 Ваш аккаунт заблокирован.",
		"no_entry":      "🚫 Этот бот только для водителей и админов.",
		"menu_client":   "👤 Меню клиента:",
		"menu_driver":   "🚖 Меню водителя:",
		"menu_admin":    "🛠 Панель администратора:",
		"order_from":    "📍 Откуда вас забрать? (Город/район)",
		"order_to":      "🏁 Куда вы едете? (Город/район)",
		"order_tariff":  "🚕 Выберите тариф:",
		"order_pass":    "👥 Введите количество пассажиров:",
		"order_time":    "📅 Введите дату и время (Например: Сегодня 18:00):",
		"order_confirm": "💰 Детали заказа:\nЦена: %d %s\n\nПодтверждаете?",
		"order_created": "✅ Ваш заказ принят!",
		"no_orders":     "📭 На данный момент активных заказов нет.",
		"notif_new":     "🔔 НОВЫЙ ЗАКАЗ!\n🆔 #%d\n💰 Цена: %s\n📍 Маршрут: %s",
		"notif_taken":   "🚖 Ваш заказ принят водителем!\n\n🆔 ID: #%d\n🚗 Водитель: %s\n📞 Тел: %s\n👤 Профиль: %s",
		"notif_done":    "🏁 Ваш заказ успешно завершен. Спасибо!",
		"notif_cancel":  "⚠️ Заказ #%d отменен.",
		"help_client":   "📖 <b>Помощь для клиентов:</b>\n\n➕ <b>Создать заказ</b> - Создание нового заказа. Выберите город, напишите пункт назначения и выберите тариф.\n📋 <b>Мои заказы</b> - Все ваши заказы и их статус.",
		"help_driver":   "📖 <b>Помощь для водителей:</b>\n\n📦 <b>Активные заказы</b> - Список всех свободных заказов на данный момент.\n📍 <b>Мои маршруты</b> - Города, по которым вы работаете. Уведомления приходят только по этим маршрутам.\n🚕 <b>Мои тарифы</b> - Тарифы, по которым вы работаете (Эконом, Комфорт и т.д.).\n📅 <b>Поиск по дате</b> - Просмотр заказов на определенную дату.\n📋 <b>Мои заказы</b> - Заказы, которые вы приняли и выполняете.",
		"help_admin":    "📖 <b>Помощь админ-панели:</b>\n\n👥 <b>Пользователи</b> - Роли и блокировка.\n📦 <b>Все заказы</b> - История заказов.\n⚙️ <b>Тарифы</b> / 🗺 <b>Города</b> - Добавить, удалить, ⬅️ Назад в меню.\n🚗 <b>Марки и модели</b> - Марки и модели авто для водителей.\n🚫 <b>Заблокированные</b> - Список заблокированных, кнопка «Разблокировать».\n📊 <b>Статистика</b> - Общая статистика.",
		// Admin action buttons — bitta joyda o‘zgartirish (universal)
		"admin_btn_approve":       "✅ Одобрить",
		"admin_btn_reject":        "❌ Отклонить",
		"admin_btn_confirm_order": "✅ Подтвердить",
		"admin_btn_reject_order":  "❌ Отклонить",
		"admin_btn_block":         "🚫 Заблокировать",
		"admin_btn_block_client":  "🚫 Заблокировать клиента",
	},
}

func (b *Bot) registerHandlers() {
	b.Bot.Handle("/start", b.handleStart)
	b.Bot.Handle("/help", b.handleHelp)

	// Client Handlers
	if b.Type == BotTypeClient {
		b.Bot.Handle(tele.OnContact, b.handleContact)
		b.Bot.Handle("➕ Создать заказ", b.handleOrderStart)
		b.Bot.Handle("📋 Мои заказы", b.handleMyOrders)
	}

	// Driver Handlers
	if b.Type == BotTypeDriver {
		b.Bot.Handle(tele.OnContact, b.handleContact)
		b.Bot.Handle("📦 Активные заказы", b.handleActiveOrders)
		b.Bot.Handle("📋 Мои заказы", b.handleMyOrdersDriver)
		b.Bot.Handle("📍 Мои маршруты", b.handleDriverRoutes)
		b.Bot.Handle("🚕 Мои тарифы", b.handleDriverTariffs)
		b.Bot.Handle("Поиск по дате", b.handleDriverCalendarSearch)
	}

	// Admin Handlers
	if b.Type == BotTypeAdmin {
		b.Bot.Handle(tele.OnContact, b.handleContact)
		b.Bot.Handle("👥 Пользователи", b.handleAdminUsers)
		b.Bot.Handle("📦 Все заказы", b.handleAdminOrders) // Keep for history/all
		b.Bot.Handle("⚙️ Тарифы", b.handleAdminTariffs)
		b.Bot.Handle("🗺 Города", b.handleAdminLocations)
		b.Bot.Handle("📊 Статистика", b.handleAdminStats)
		b.Bot.Handle("🚖 Водители на проверке", b.handleAdminPendingDrivers)
		b.Bot.Handle("🚕 Все водители", b.handleAdminActiveDrivers)
		b.Bot.Handle("📦 Заказы на подтверждении", b.handleAdminPendingOrders)

		b.Bot.Handle("➕ Добавить тариф", b.handleTariffAddStart)
		b.Bot.Handle("🗑 Удалить тариф", b.handleTariffDeleteStart)
		b.Bot.Handle("➕ Добавить город", b.handleLocationAddStart)
		b.Bot.Handle("🗑 Удалить город", b.handleLocationDeleteStart)
		b.Bot.Handle("🔍 Найти город", b.handleLocationGetStart)
		b.Bot.Handle("⬅️ Назад в меню", b.handleAdminBackToMenu)
		b.Bot.Handle("🚗 Марки и модели", b.handleAdminCars)
		b.Bot.Handle("🚫 Заблокированные", b.handleAdminBlocked)
		b.Bot.Handle("➕ Добавить марку", b.handleCarBrandAddStart)
		b.Bot.Handle("➕ Добавить модель", b.handleCarModelAddStart)
		b.Bot.Handle("🗑 Удалить марку", b.handleCarBrandDeleteStart)
		b.Bot.Handle("🗑 Удалить модель", b.handleCarModelDeleteStart)
	}

	b.Bot.Handle(tele.OnCallback, b.handleCallback)
	b.Bot.Handle(tele.OnText, b.handleText)
	b.Bot.Handle(tele.OnWebApp, b.handleWebApp)

	// Set Bot Commands for UI hint
	cmds := []tele.Command{
		{Text: "start", Description: "Запустить бота / Главное меню"},
		{Text: "help", Description: "Помощь и руководство"},
	}
	b.Bot.SetCommands(cmds)
}

func (b *Bot) handleStart(c tele.Context) error {
	b.Log.Info(fmt.Sprintf("Start command received from %d (%s)", c.Sender().ID, c.Sender().Username))

	ctx := context.Background()
	user, err := b.Stg.User().GetOrCreate(ctx, c.Sender().ID, c.Sender().Username, fmt.Sprintf("%s %s", c.Sender().FirstName, c.Sender().LastName))
	if err != nil {
		b.Log.Error("Failed to get or create user", logger.Error(err))
		return c.Send("❌ Ошибка системы. Попробуйте позже.")
	}

	// Admin bot: kirish login/parol yoki biriktirilgan AdminID orqali; telefon shart emas
	if b.Type == BotTypeAdmin {
		if b.Cfg.AdminID != 0 && c.Sender().ID == b.Cfg.AdminID {
			// Biriktirilgan admin — avtomatik admin
			if user.Role != "admin" {
				b.Stg.User().UpdateRole(ctx, c.Sender().ID, "admin")
				user, _ = b.Stg.User().Get(ctx, c.Sender().ID)
			}
		}
		// Boshqalar login/parol orqali kirishi mumkin (handleStart da StateAdminLogin)
	}

	// Check for blocked status
	if user.Status == "blocked" {
		return c.Send(messages["ru"]["blocked"])
	}

	if (b.Type == BotTypeDriver || b.Type == BotTypeAdmin) && user.Role == "client" && user.Status != "pending" {
		if b.Type != BotTypeAdmin {
			return c.Send("🚫 <b>Доступ запрещен!</b>\n\nВы зарегистрированы как клиент.\n\n👇 Пожалуйста, перейдите в бот для клиентов:\n@clienttaxigo_bot", tele.ModeHTML)
		}
	}

	if b.Type == BotTypeClient && user.Role == "driver" {
		return c.Send("🚫 <b>Доступ запрещен!</b>\n\nВы зарегистрированы как водитель.\n\n👇 Пожалуйста, перейдите в бот для водителей:\n@drivertaxisgo_bot", tele.ModeHTML)
	}

	// Always initialize/reset session on /start
	b.Sessions[c.Sender().ID] = &UserSession{
		DBID:      user.ID,
		State:     StateIdle,
		OrderData: &models.Order{ClientID: user.ID},
	}

	// Admin Login Flow: admin bo‘lmasa — faqat login/parol (telefon shart emas)
	if b.Type == BotTypeAdmin && user.Role != "admin" {
		b.Sessions[c.Sender().ID].State = StateAdminLogin
		return c.Send("🔐 <b>Админ-панель</b>\n\nВведите логин и пароль для входа. Номер телефона не требуется.\n\nПожалуйста, введите логин:", tele.ModeHTML)
	}

	if user.Status == "pending" && b.Type != BotTypeAdmin {
		menu := &tele.ReplyMarkup{ResizeKeyboard: true}
		menu.Reply(menu.Row(menu.Contact(messages["ru"]["share_contact"])))
		return c.Send(messages["ru"]["contact_msg"], menu)
	}

	// Handle driver registration flow states
	if b.Type == BotTypeDriver {
		switch user.Status {
		case "pending":
			return b.handleDriverRegistrationStart(c)
		case "pending_review":
			return c.Send("⏳ <b>Ваш профиль находится на проверке.</b>\n\nОжидайте уведомления от администратора.", tele.ModeHTML)
		case "rejected":
			return c.Send("❌ <b>Ваша заявка отклонена.</b>\n\nОбратитесь к администратору для уточнения деталей.", tele.ModeHTML)
		case "active":
			// OK — ko'rsatish
		default:
			// Noma'lum status — contact so'rash
			return c.Send("⚠️ Нажмите /start и пройдите регистрацию заново.")
		}
	}

	return b.showMenu(c, user)
}

func (b *Bot) handleContact(c tele.Context) error {
	// Log which bot instance handled the contact (helps diagnose duplicate sends)
	botInfo := "unknown"
	if b.Bot != nil && b.Bot.Me != nil {
		botInfo = b.Bot.Me.Username
	}
	b.Log.Info("Contact received",
		logger.Int64("sender_id", c.Sender().ID),
		logger.String("bot_type", string(b.Type)),
		logger.String("bot_username", botInfo),
	)

	// Deduplication guard: ignore duplicate contact events from same user
	// within a short window to prevent double-processing when multiple
	// bot instances or duplicate updates occur.
	session := b.Sessions[c.Sender().ID]
	ctx := context.Background()
	if session == nil {
		// DB dan to'g'ri ID ni olish kerak (TelegramID != DB ID)
		dbUser, _ := b.Stg.User().Get(ctx, c.Sender().ID)
		dbID := c.Sender().ID // fallback
		if dbUser != nil {
			dbID = dbUser.ID
		}
		session = &UserSession{DBID: dbID, State: StateIdle, OrderData: &models.Order{ClientID: dbID}}
		b.Sessions[c.Sender().ID] = session
	}
	if time.Since(session.LastActionTime) < 2*time.Second {
		b.Log.Info("Ignoring duplicate contact event", logger.Int64("sender_id", c.Sender().ID))
		return nil
	}
	session.LastActionTime = time.Now()
	if c.Message().Contact.UserID != c.Sender().ID {
		return c.Send("Пожалуйста, отправьте свой собственный номер.")
	}
	user, _ := b.Stg.User().Get(ctx, c.Sender().ID)
	if user.Status == "blocked" {
		return c.Send(messages["ru"]["blocked"])
	}

	if err := b.Stg.User().UpdatePhone(ctx, c.Sender().ID, c.Message().Contact.PhoneNumber); err != nil {
		b.Log.Error("Failed to update phone", logger.Error(err), logger.Int64("user_id", c.Sender().ID))
		return c.Send("❌ Ошибка при сохранении номера телефона.")
	}

	// If registering via Driver Bot, set role to driver
	if b.Type == BotTypeDriver {
		if err := b.Stg.User().UpdateRole(ctx, c.Sender().ID, "driver"); err != nil {
			b.Log.Error("Failed to update role to driver", logger.Error(err), logger.Int64("user_id", c.Sender().ID))
		}
		// Do not set active yet, wait for full registration
		if err := b.Stg.User().UpdateStatus(ctx, c.Sender().ID, "pending"); err != nil {
			b.Log.Error("Failed to update status to pending", logger.Error(err), logger.Int64("user_id", c.Sender().ID))
		}
	} else {
		if err := b.Stg.User().UpdateStatus(ctx, c.Sender().ID, "active"); err != nil {
			b.Log.Error("Failed to update status to active", logger.Error(err), logger.Int64("user_id", c.Sender().ID))
		}
	}
	user, _ = b.Stg.User().Get(ctx, c.Sender().ID)

	c.Send(messages["ru"]["registered"], tele.RemoveKeyboard)

	if b.Type == BotTypeDriver {
		// Dastlabki xabar adminga
		admins, _ := b.Stg.User().GetAll(ctx)
		adminMsg := fmt.Sprintf("🆕 <b>Новая регистрация водителя</b>\n\n👤 %s\n📞 %s\n\n<i>Ожидайте завершения ввода данных автомобиля и маршрутов...</i>",
			user.FullName, *user.Phone)

		for _, u := range admins {
			if u.Role == "admin" {
				b.Bot.Send(&tele.User{ID: u.TelegramID}, adminMsg, tele.ModeHTML)
			}
		}

		// Sessiyada to'g'ri DBID bo'lishini ta'minlaymiz
		s := b.Sessions[c.Sender().ID]
		if s == nil {
			b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		} else {
			s.DBID = user.ID
		}
		return b.handleDriverRegistrationStart(c)
	}

	return b.showMenu(c, user)
}

func (b *Bot) showMenu(c tele.Context, user *models.User) error {
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}

	if b.Type == BotTypeClient {
		menu.Reply(
			menu.Row(menu.Text("➕ Создать заказ")),
			menu.Row(menu.Text("📋 Мои заказы")),
		)
		return c.Send(messages["ru"]["menu_client"], &tele.SendOptions{ReplyMarkup: menu})
	}

	if user.Role == "admin" {
		menu.Reply(
			menu.Row(menu.Text("👥 Пользователи"), menu.Text("📊 Статистика")),
			menu.Row(menu.Text("🚖 Водители на проверке"), menu.Text("🚕 Все водители")),
			menu.Row(menu.Text("📦 Заказы на подтверждении")),
			menu.Row(menu.Text("📦 Все заказы")),
			menu.Row(menu.Text("⚙️ Тарифы"), menu.Text("🗺 Города")),
			menu.Row(menu.Text("🚗 Марки и модели"), menu.Text("🚫 Заблокированные")),
		)
		return c.Send(messages["ru"]["menu_admin"], &tele.SendOptions{ReplyMarkup: menu})
	}

	// Driver Menu
	menu.Reply(
		menu.Row(menu.Text("📦 Активные заказы")),
		menu.Row(menu.Text("📍 Мои маршруты"), menu.Text("🚕 Мои тарифы")),
		menu.Row(menu.Text("Поиск по дате")),
		menu.Row(menu.Text("📋 Мои заказы")),
	)
	return c.Send(messages["ru"]["menu_driver"], &tele.SendOptions{ReplyMarkup: menu})
}

func (b *Bot) handleHelp(c tele.Context) error {
	user := b.getCurrentUser(c)
	if user == nil {
		return c.Send("📖 Для использования системы нажмите /start.")
	}

	msgKey := "help_client"
	if user.Role == "admin" && b.Type == BotTypeAdmin {
		msgKey = "help_admin"
	} else if user.Role == "driver" || b.Type == BotTypeDriver {
		msgKey = "help_driver"
	}

	return c.Send(messages["ru"][msgKey], tele.ModeHTML)
}

func (b *Bot) handleOrderStart(c tele.Context) error {
	b.Log.Info("DEBUG: Handle Order Start", logger.Int64("user_id", c.Sender().ID))
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		if user == nil {
			return c.Send("❌ Ошибка: Информация о пользователе не найдена. Пожалуйста, нажмите /start еще раз.")
		}

		if user.Status != "active" {
			if user.Status == "pending" {
				return b.handleStart(c) // Redirect to registration
			}
			return c.Send(messages["ru"]["blocked"])
		}

		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]

		// Set initial time to avoid immediate block if just created (though LastActionTime is zero value, Since() > 1.5s)
	}

	// Debounce: Preventive double-click protection (1.5 seconds)
	if time.Since(session.LastActionTime) < 1500*time.Millisecond {
		return nil
	}
	session.LastActionTime = time.Now()

	session.State = StateFrom
	session.OrderData = &models.Order{ClientID: session.DBID}

	locations, _ := b.Stg.Location().GetAll(context.Background())
	menu := &tele.ReplyMarkup{}

	var rows []tele.Row
	var currentRow []tele.Btn
	for i, l := range locations {
		currentRow = append(currentRow, menu.Data(l.Name, fmt.Sprintf("cl_f_%d", l.ID)))
		if (i+1)%3 == 0 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = []tele.Btn{}
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}

	menu.Inline(rows...)
	menu.Inline(menu.Row(menu.Data("❌ Отменить", "cl_cancel")))
	return c.Send(messages["ru"]["order_from"], menu, tele.ModeHTML)
}

func (b *Bot) handleActiveOrders(c tele.Context) error {
	user := b.getCurrentUser(c)
	if user == nil {
		return c.Send("❌ Ошибка: Информация о пользователе не найдена. Пожалуйста, нажмите /start еще раз.")
	}
	if user.Status != "active" {
		return c.Send("🚫 <b>Доступ запрещен!</b>\n\nВаш профиль находится на проверке или заблокирован. Ожидайте подтверждения администратора.", tele.ModeHTML)
	}

	orders, _ := b.Stg.Order().GetActiveOrders(context.Background())
	if len(orders) == 0 {
		return c.Send(messages["ru"]["no_orders"])
	}

	for _, o := range orders {
		timeStr := "Неизвестно"
		if o.PickupTime != nil {
			loc := time.FixedZone("Europe/Moscow", 3*60*60)
			timeStr = o.PickupTime.In(loc).Format("02.01.2006 15:04")
		}

		txt := fmt.Sprintf("📦 <b>НОВЫЙ ЗАКАЗ #%d</b>\n\n📍 Маршрут: <b>%s ➡️ %s</b>\n💰 Цена: <b>%d %s</b>\n👥 Пассажиры: <b>%d</b>\n🕒 Время: <b>%s</b>\n\n👤 Клиент: <a href=\"tg://user?id=%d\">%s</a>\n📞 Тел: %s",
			o.ID, o.FromLocationName, o.ToLocationName, o.Price, o.Currency, o.Passengers, timeStr, o.ClientID, o.ClientUsername, o.ClientPhone)

		menu := &tele.ReplyMarkup{}
		menu.Inline(menu.Row(menu.Data("📥 Принять заказ", fmt.Sprintf("take_%d", o.ID))))
		c.Send(txt, menu, tele.ModeHTML)
	}
	return nil
}

func (b *Bot) handleMyOrdersDriver(c tele.Context) error {
	user := b.getCurrentUser(c)
	if user == nil {
		return c.Send("❌ Ошибка: Информация о пользователе не найдена. Пожалуйста, нажмите /start еще раз.")
	}
	if user.Status != "active" && user.Status != "pending_review" {
		return c.Send("🚫 <b>Доступ запрещен!</b>\n\nВаш профиль находится на проверке или заблокирован. Ожидайте подтверждения администратора.", tele.ModeHTML)
	}

	session := b.Sessions[c.Sender().ID]
	if session == nil {
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]
	}

	orders, _ := b.Stg.Order().GetDriverOrders(context.Background(), session.DBID)
	if len(orders) == 0 {
		return c.Send("У вас нет принятых заказов.")
	}

	for _, o := range orders {
		timeStr := "Неизвестно"
		if o.PickupTime != nil {
			loc := time.FixedZone("Europe/Moscow", 3*60*60)
			timeStr = o.PickupTime.In(loc).Format("02.01.2006 15:04")
		}

		txt := fmt.Sprintf("🚖 <b>ЗАКАЗ #%d</b>\n📍 %s ➡️ %s\n👥 Пассажиры: %d\n💰 Цена: %d %s\n📅 Время: %s\n📊 Статус: %s\n\n👤 Клиент: <a href=\"tg://user?id=%d\">%s</a>\n📞 Тел: %s",
			o.ID, o.FromLocationName, o.ToLocationName, o.Passengers, o.Price, o.Currency, timeStr, o.Status, o.ClientID, o.ClientUsername, o.ClientPhone)

		menu := &tele.ReplyMarkup{}
		if o.Status == "taken" {
			menu.Inline(
				menu.Row(menu.Data("🚗 Выехал", fmt.Sprintf("on_way_%d", o.ID))),
				menu.Row(menu.Data("↩️ Вернуть в пул", fmt.Sprintf("return_order_%d", o.ID))),
			)
		} else if o.Status == "on_way" {
			menu.Inline(menu.Row(menu.Data("📍 Прибыл", fmt.Sprintf("arrived_%d", o.ID))))
		} else if o.Status == "arrived" {
			menu.Inline(menu.Row(menu.Data("▶ Начал поездку", fmt.Sprintf("start_trip_%d", o.ID))))
		} else if o.Status == "in_progress" {
			menu.Inline(menu.Row(menu.Data("✅ Завершить", fmt.Sprintf("complete_%d", o.ID))))
		}
		c.Send(txt, menu, tele.ModeHTML)
	}
	return nil
}

func (b *Bot) handleAdminUsers(c tele.Context) error {
	return b.showUsersPage(c, 0)
}

func (b *Bot) showUsersPage(c tele.Context, page int) error {
	const limit = 5
	users, _ := b.Stg.User().GetAll(context.Background())
	totalPages := (len(users) + limit - 1) / limit

	if page < 0 {
		page = 0
	}
	if page >= totalPages && totalPages > 0 {
		page = totalPages - 1
	}

	start := page * limit
	end := start + limit
	if end > len(users) {
		end = len(users)
	}

	if len(users) == 0 {
		return c.Send("👥 Пользователи не найдены.")
	}

	total := len(users)
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("👥 <b>Пользователи</b>\n📊 Всего: <b>%d</b> | Страница <b>%d/%d</b>\n\n", total, page+1, totalPages))

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	for i := start; i < end; i++ {
		u := users[i]
		phone := "—"
		if u.Role != "admin" {
			if u.Phone != nil && *u.Phone != "" {
				phone = *u.Phone
			} else {
				phone = "Не указан"
			}
		}

		statusIcon := "✅"
		if u.Status == "blocked" {
			statusIcon = "🚫"
		} else if u.Status == "pending" || u.Status == "pending_review" {
			statusIcon = "⏳"
		}

		msg.WriteString(fmt.Sprintf("%s <b>%s</b> | <code>%d</code>\n📞 %s | Роль: <b>%s</b> | Статус: <b>%s</b>\n", statusIcon, u.FullName, u.TelegramID, phone, u.Role, u.Status))
		msg.WriteString("——————————————\n")

		// Block/Unblock button: show action opposite to current state
		var blockBtnLabel, blockBtnData string
		if u.Status == "blocked" {
			blockBtnLabel = "✅ Разблок"
			blockBtnData = fmt.Sprintf("adm_stat_%d_%d", u.TelegramID, page)
		} else {
			blockBtnLabel = "🚫 Блок"
			blockBtnData = fmt.Sprintf("adm_stat_%d_%d", u.TelegramID, page)
		}

		if u.Role == "admin" {
			rows = append(rows, menu.Row(menu.Data(fmt.Sprintf("🛡 %s (Администратор)", u.FullName), "noop")))
		} else {
			btnBlock := menu.Data(blockBtnLabel, blockBtnData)
			btnDel := menu.Data("🗑 Удалить", fmt.Sprintf("adm_del_user_%d_%d", u.TelegramID, page))
			rows = append(rows, menu.Row(btnBlock, btnDel))
		}
	}

	// Navigation
	var navRow []tele.Btn
	if page > 0 {
		navRow = append(navRow, menu.Data("⬅️ Предыдущая", fmt.Sprintf("users_page_%d", page-1)))
	}
	if page < totalPages-1 {
		navRow = append(navRow, menu.Data("Следующая ➡️", fmt.Sprintf("users_page_%d", page+1)))
	}
	// Always add Back button
	navRow = append(navRow, menu.Data("⬅️ Назад", "admin_back"))

	if len(navRow) > 0 {
		rows = append(rows, menu.Row(navRow...))
	}

	menu.Inline(rows...)

	if c.Callback() != nil {
		return c.Edit(msg.String(), menu, tele.ModeHTML)
	}
	return c.Send(msg.String(), menu, tele.ModeHTML)
}

func (b *Bot) handleAdminOrders(c tele.Context) error {
	return b.showOrdersPage(c, 0)
}

func (b *Bot) showOrdersPage(c tele.Context, page int) error {
	const limit = 5
	orders, _ := b.Stg.Order().GetAll(context.Background())
	// In real app, use DB offset/limit. Here purely slicing.
	// Sort by ID desc (newest first)
	// Sort by ID desc (newest first) - already sorted by DB query

	totalPages := (len(orders) + limit - 1) / limit
	if page < 0 {
		page = 0
	}
	if page >= totalPages && totalPages > 0 {
		page = totalPages - 1
	}

	start := page * limit
	end := start + limit
	if end > len(orders) {
		end = len(orders)
	}

	if len(orders) == 0 {
		return c.Send("📦 Заказы не найдены.")
	}

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("📦 <b>Все заказы (%d/%d):</b>\n\n", page+1, totalPages))

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	for i := start; i < end; i++ {
		o := orders[i]

		total, completed, cancelled, _ := b.Stg.Order().GetClientStats(context.Background(), o.ClientID)

		msg.WriteString(fmt.Sprintf("🔹 <b>#%d</b> | %s\n📍 %s -> %s\n💰 %d %s\n📊 История: Всего %d | ✅ %d | ❌ %d\n\n",
			o.ID, o.Status, o.FromLocationName, o.ToLocationName, o.Price, o.Currency, total, completed, cancelled))

		if o.Status != "completed" && o.Status != "cancelled" {
			rows = append(rows, menu.Row(menu.Data(fmt.Sprintf("❌ Отменить #%d", o.ID), fmt.Sprintf("adm_cancel_%d_%d", o.ID, page))))
		}
	}

	var navRow []tele.Btn
	if page > 0 {
		navRow = append(navRow, menu.Data("⬅️ Предыдущая", fmt.Sprintf("orders_page_%d", page-1)))
	}
	if page < totalPages-1 {
		navRow = append(navRow, menu.Data("Следующая ➡️", fmt.Sprintf("orders_page_%d", page+1)))
	}
	// Always add Back button
	navRow = append(navRow, menu.Data("⬅️ Назад", "admin_back"))

	if len(navRow) > 0 {
		rows = append(rows, menu.Row(navRow...))
	}

	menu.Inline(rows...)
	if c.Callback() != nil {
		return c.Edit(msg.String(), menu, tele.ModeHTML)
	}
	return c.Send(msg.String(), menu, tele.ModeHTML)
}

func (b *Bot) handleAdminTariffs(c tele.Context) error {
	// Prevent duplicate messages (dedupe guard for slow network/double clicks)
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		if user == nil {
			return c.Send("❌ Ошибка: Информация о пользователе не найдена. Пожалуйста, нажмите /start еще раз.")
		}
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]
	}

	// Skip if action was just done (within 1.5 seconds)
	if time.Since(session.LastActionTime) < 1500*time.Millisecond {
		return nil
	}
	session.LastActionTime = time.Now()

	tariffs, _ := b.Stg.Tariff().GetAll(context.Background())
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	menu.Reply(
		menu.Row(menu.Text("➕ Добавить тариф"), menu.Text("🗑 Удалить тариф")),
		menu.Row(menu.Text("⬅️ Назад в меню")),
	)

	var msg strings.Builder
	b.Log.Info("Handling Admin Tariffs Display")
	msg.WriteString("⚙️ <b>Доступные тарифы:</b>\n\n")
	for i, t := range tariffs {
		msg.WriteString(fmt.Sprintf("%d. ⚙️ <b>%s</b> (ID: %d)\n", i+1, t.Name, t.ID))
	}

	return c.Send(msg.String(), menu, tele.ModeHTML)
}

func (b *Bot) handleTariffDeleteStart(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		if user == nil {
			return c.Send("❌ Ошибка: Информация о пользователе не найдена. Пожалуйста, нажмите /start еще раз.")
		}
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]
	}

	session.State = "awaiting_tariff_delete_id"

	return c.Send("🗑 <b>Удаление тарифа</b>\n\nВведите ID тарифа, который хотите удалить:", tele.ModeHTML)
}

func (b *Bot) handleLocationDeleteStart(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		if user == nil {
			return c.Send("❌ Ошибка: Информация о пользователе не найдена. Пожалуйста, нажмите /start еще раз.")
		}
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]
	}

	session.State = "awaiting_location_delete_id"

	return c.Send("🗑 <b>Удаление города</b>\n\nВведите ID города, который хотите удалить:", tele.ModeHTML)
}

func (b *Bot) handleLocationGetStart(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		if user == nil {
			return c.Send("❌ Ошибка: Информация о пользователе не найдена. Пожалуйста, нажмите /start еще раз.")
		}
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]
	}

	session.State = "awaiting_location_get_id"

	return c.Send("🔍 <b>Получить город</b>\n\nВведите ID города для поиска:", tele.ModeHTML)
}

func (b *Bot) handleAdminLocations(c tele.Context) error {
	// Prevent duplicate messages (dedupe guard for slow network/double clicks)
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		if user == nil {
			return c.Send("❌ Ошибка: Информация о пользователе не найдена. Пожалуйста, нажмите /start еще раз.")
		}
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]
	}

	// Skip if action was just done (within 1.5 seconds)
	if time.Since(session.LastActionTime) < 1500*time.Millisecond {
		return nil
	}
	session.LastActionTime = time.Now()

	locations, _ := b.Stg.Location().GetAll(context.Background())
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	menu.Reply(
		menu.Row(menu.Text("➕ Добавить город"), menu.Text("🗑 Удалить город")),
		menu.Row(menu.Text("🔍 Найти город")),
		menu.Row(menu.Text("⬅️ Назад в меню")),
	)

	var msg strings.Builder
	msg.WriteString("🗺 <b>Доступные города:</b>\n\n")
	msg.WriteString("┌─────┬──────────────────────┐\n")
	msg.WriteString("│  ID │       Название       │\n")
	msg.WriteString("├─────┼──────────────────────┤\n")

	for _, l := range locations {
		msg.WriteString(fmt.Sprintf("│ %4d │ %-20s │\n", l.ID, l.Name))
	}

	msg.WriteString("└─────┴──────────────────────┘\n")

	return c.Send(msg.String(), menu, tele.ModeHTML)
}

func (b *Bot) handleText(c tele.Context) error {
	b.Log.Info("Handle Text", logger.String("text", c.Text()))

	session, ok := b.Sessions[c.Sender().ID]
	if !ok || session.State == StateIdle {
		return nil
	}

	// Guard: If it's a menu button text, don't process it as input for states
	txt := c.Text()
	isMenu := txt == "➕ Создать заказ" || txt == "📋 Мои заказы" ||
		txt == "📦 Активные заказы" || txt == "📍 Мои маршруты" || txt == "🚕 Мои тарифы" ||
		txt == "Поиск по дате" || txt == "👥 Пользователи" || txt == "📦 Все заказы" ||
		txt == "⚙️ Тарифы" || txt == "🗺 Города" || txt == "📊 Статистика" ||
		txt == "➕ Добавить тариф" || txt == "🗑 Удалить тариф" ||
		txt == "➕ Добавить город" || txt == "🗑 Удалить город" || txt == "🔍 Найти город" ||
		txt == "⬅️ Назад в меню" || txt == "🚗 Марки и модели" || txt == "🚫 Заблокированные" ||
		txt == "➕ Добавить марку" || txt == "➕ Добавить модель" ||
		txt == "🗑 Удалить марку" || txt == "🗑 Удалить модель"

	if isMenu {
		// Senior Fix: Reset state when switching between main menus to avoid state conflict
		session.State = StateIdle
		return nil
	}

	b.Log.Info("Processing Text State", logger.String("state", session.State))

	switch session.State {
	case StateFrom:
		session.TempString = c.Text()
		session.State = StateTo
		return c.Send(messages["ru"]["order_to"])
	case StateTo:
		session.TempString = session.TempString + " ➡️ " + c.Text()
		session.State = StateTariff
		tariffs, _ := b.Stg.Tariff().GetAll(context.Background())
		menu := &tele.ReplyMarkup{}
		var rows []tele.Row
		var currentRow []tele.Btn
		for i, t := range tariffs {
			currentRow = append(currentRow, menu.Data(t.Name, fmt.Sprintf("tf_%d", t.ID)))
			if (i+1)%2 == 0 {
				rows = append(rows, menu.Row(currentRow...))
				currentRow = []tele.Btn{}
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, menu.Row(currentRow...))
		}
		menu.Inline(rows...)
		return c.Send(messages["ru"]["order_tariff"], menu)
	case StatePassengers:
		count, err := strconv.Atoi(c.Text())
		if err != nil || count < 1 {
			return c.Send("❌ Пожалуйста, введите корректное число пассажиров (например: 2).")
		}
		session.OrderData.Passengers = count
		session.State = StatePrice
		menu := &tele.ReplyMarkup{}
		menu.Inline(menu.Row(menu.Data("❌ Отменить", "cl_cancel")))
		return c.Send("💰 <b>Укажите сумму за поездку (RUB):</b>\n\nНапример: <code>1500</code>", menu, tele.ModeHTML)
	case StatePrice:
		priceStr := strings.TrimSpace(c.Text())
		price, err := strconv.Atoi(priceStr)
		if err != nil || price <= 0 {
			return c.Send("❌ Неверный формат. Введите сумму числом (например: <code>1500</code>).", tele.ModeHTML)
		}
		session.OrderData.Price = price
		session.State = StateConfirm

		from, _ := b.Stg.Location().GetByID(context.Background(), session.OrderData.FromLocationID)
		to, _ := b.Stg.Location().GetByID(context.Background(), session.OrderData.ToLocationID)
		tariff, _ := b.Stg.Tariff().GetByID(context.Background(), session.OrderData.TariffID)

		fromName, toName, tariffName := "Неизвестно", "Неизвестно", "Неизвестно"
		if from != nil {
			fromName = from.Name
		}
		if to != nil {
			toName = to.Name
		}
		if tariff != nil {
			tariffName = tariff.Name
		}

		timeStr := "Неизвестно"
		if session.OrderData.PickupTime != nil {
			loc := time.FixedZone("Europe/Moscow", 3*60*60)
			timeStr = session.OrderData.PickupTime.In(loc).Format("02.01.2006 15:04")
		}

		msg := fmt.Sprintf(
			"<b>✅ Подтверждение заказа</b>\n\n📍 <b>%s ➡️ %s</b>\n🚕 Тариф: <b>%s</b>\n👥 Пассажиры: <b>%d</b>\n💰 Сумма: <b>%d RUB</b>\n📅 Время: <b>%s</b>\n\nПодтверждаете?",
			fromName, toName, tariffName, session.OrderData.Passengers, price, timeStr,
		)
		menu := &tele.ReplyMarkup{}
		menu.Inline(menu.Row(
			menu.Data("✅ Подтвердить", "confirm_yes"),
			menu.Data("❌ Отменить", "confirm_no"),
		))
		return c.Send(msg, menu, tele.ModeHTML)
	case StateLicensePlate:
		return b.handleLicensePlateInput(c)
	case StateCarModelOther:
		if session.DriverProfile == nil {
			user := b.getCurrentUser(c)
			if user == nil {
				return c.Send("❌ Ошибка: Информация о пользователе не найдена. Пожалуйста, нажмите /start еще раз.")
			}
			session.DriverProfile = &models.DriverProfile{UserID: user.ID}
		}
		session.DriverProfile.CarModel = c.Text()
		session.State = StateLicensePlate
		return c.Send("🔢 <b>Введите гос. номер автомобиля:</b>\n\nПример: <code>A123BC777</code> (русские буквы)", tele.ModeHTML)
	case StateDriverRouteFrom:
		// Fallback if text entered instead of button, or search logic
		return c.Send("📍 Пожалуйста, выберите город из списка.")
	case StateTariffAdd:
		name := strings.TrimSpace(c.Text())
		if name == "" {
			return c.Send("❌ Введите непустое название тарифа.")
		}
		if err := b.Stg.Tariff().Create(context.Background(), name); err != nil {
			return c.Send("❌ Ошибка: " + err.Error())
		}
		session.State = StateIdle
		_ = c.Send("✅ Тариф добавлен!")
		return b.handleAdminTariffs(c)
	case StateLocationAdd:
		name := strings.TrimSpace(c.Text())
		if name == "" {
			return c.Send("❌ Введите непустое название города.")
		}
		if err := b.Stg.Location().Create(context.Background(), name); err != nil {
			return c.Send("❌ Ошибка: " + err.Error())
		}
		session.State = StateIdle
		_ = c.Send("✅ Город добавлен!")
		return b.handleAdminLocations(c)
	case StateCarBrandAdd:
		name := strings.TrimSpace(c.Text())
		if name == "" {
			return c.Send("❌ Введите непустое название марки.")
		}
		if err := b.Stg.Car().CreateBrand(context.Background(), name); err != nil {
			return c.Send("❌ Ошибка: " + err.Error())
		}
		session.State = StateIdle
		_ = c.Send("✅ Марка добавлена!")
		return b.handleAdminCars(c)
	case StateCarModelAdd:
		name := strings.TrimSpace(c.Text())
		if name == "" {
			return c.Send("❌ Введите непустое название модели.")
		}
		brandID, _ := strconv.ParseInt(session.TempString, 10, 64)
		if brandID == 0 {
			session.State = StateIdle
			return c.Send("❌ Сессия сброшена. Выберите марку снова.")
		}
		if err := b.Stg.Car().CreateModel(context.Background(), brandID, name); err != nil {
			return c.Send("❌ Ошибка: " + err.Error())
		}
		session.State = StateIdle
		session.TempString = ""
		_ = c.Send("✅ Модель добавлена!")
		return b.handleAdminCars(c)
	case "awaiting_tariff_delete_id":
		id, err := strconv.ParseInt(c.Text(), 10, 64)
		if err != nil {
			return c.Send("❌ Неверный ID! Пожалуйста, введите число.")
		}
		err = b.Stg.Tariff().Delete(context.Background(), id)
		if err != nil {
			return c.Send("❌ Ошибка: " + err.Error())
		}
		session.State = StateIdle
		_ = c.Send("✅ Тариф успешно удален!")
		return b.handleAdminTariffs(c)
	case "awaiting_location_delete_id":
		id, err := strconv.ParseInt(c.Text(), 10, 64)
		if err != nil {
			return c.Send("❌ Неверный ID! Пожалуйста, введите число.")
		}
		err = b.Stg.Location().Delete(context.Background(), id)
		if err != nil {
			return c.Send("❌ Ошибка: " + err.Error())
		}
		session.State = StateIdle
		_ = c.Send("✅ Город успешно удален!")
		return b.handleAdminLocations(c)
	case "awaiting_location_get_id":
		id, err := strconv.ParseInt(c.Text(), 10, 64)
		if err != nil {
			return c.Send("❌ Неверный ID! Пожалуйста, введите число.")
		}
		location, err := b.Stg.Location().GetByID(context.Background(), id)
		if err != nil {
			return c.Send("❌ Город не найден!")
		}
		session.State = StateIdle
		return c.Send(fmt.Sprintf("🔍 <b>Информация о городе:</b>\n\n🆔 ID: %d\n📍 Название: %s", location.ID, location.Name), tele.ModeHTML)
	case StateAdminLogin:
		// Admin login: check username
		if c.Text() == b.Cfg.AdminLogin {
			session.State = StateAdminPassword
			return c.Send("🔐 <b>Пароль:</b>")
		}
		return c.Send("❌ Логин неверный. Попробуйте еще раз:")
	case StateAdminPassword:
		// Admin password: check and grant access if correct
		if c.Text() == b.Cfg.AdminPassword {
			// Mark as logged in (update role to admin)
			b.Stg.User().UpdateRole(context.Background(), c.Sender().ID, "admin")
			session.State = StateIdle
			user, _ := b.Stg.User().Get(context.Background(), c.Sender().ID)
			if user == nil {
				return c.Send("❌ Ошибка: Пользователь не найден. Пожалуйста, нажмите /start еще раз.")
			}
			return b.showMenu(c, user)
		}
		return c.Send("❌ Пароль неверный. Попробуйте еще раз:")
	}
	return nil
}

func (b *Bot) handleCallback(c tele.Context) error {
	data := strings.TrimSpace(c.Callback().Data)

	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		if user == nil {
			return nil
		}
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle, OrderData: &models.Order{ClientID: user.ID}}
		session = b.Sessions[c.Sender().ID]
	}
	if session.OrderData == nil {
		session.OrderData = &models.Order{ClientID: session.DBID}
	}

	b.Log.Info("Handle Callback",
		logger.String("data", data),
		logger.Int64("from_id", session.OrderData.FromLocationID),
		logger.Int64("to_id", session.OrderData.ToLocationID),
	)

	// Guard: Check for session loss during order flow (Client only) — bitta xabar, chakashmaslik
	if b.Type == BotTypeClient {
		isOrderFlowCallback := strings.HasPrefix(data, "cl_t_") ||
			strings.HasPrefix(data, "tf_") ||
			strings.HasPrefix(data, "cal_") ||
			strings.HasPrefix(data, "time_") ||
			strings.HasPrefix(data, "confirm_")

		b.Log.Info("DEBUG: Handle Callback",
			logger.String("data", data),
			logger.Bool("is_order_flow", isOrderFlowCallback),
			logger.Int64("from_id", session.OrderData.FromLocationID),
			logger.Int64("to_id", session.OrderData.ToLocationID),
			logger.Int64("tariff_id", session.OrderData.TariffID),
		)

		sessionLost := false
		if isOrderFlowCallback && session.OrderData.FromLocationID == 0 {
			sessionLost = true
		}
		if (strings.HasPrefix(data, "tf_") || strings.HasPrefix(data, "cal_") ||
			strings.HasPrefix(data, "time_") || strings.HasPrefix(data, "confirm_")) && session.OrderData.ToLocationID == 0 {
			sessionLost = true
		}
		if (strings.HasPrefix(data, "cal_") || strings.HasPrefix(data, "time_") ||
			strings.HasPrefix(data, "confirm_")) && session.OrderData.TariffID == 0 {
			sessionLost = true
		}
		if sessionLost {
			c.Delete()
			return c.Send("⚠️ <b>Сессия обновлена.</b>\n\nИз-за перезапуска бота, пожалуйста, оформите заказ заново:\n/start", tele.ModeHTML)
		}
	}

	if b.Type == BotTypeClient && strings.HasPrefix(data, "cl_f_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "cl_f_"), 10, 64)
		session.OrderData.FromLocationID = id
		session.State = StateTo

		locations, _ := b.Stg.Location().GetAll(context.Background())
		menu := &tele.ReplyMarkup{}
		var rows []tele.Row
		var currentRow []tele.Btn
		count := 0
		for _, l := range locations {
			if l.ID != id {
				// include fromID in callback so we can recover session if lost
				currentRow = append(currentRow, menu.Data(l.Name, fmt.Sprintf("cl_t_%d_%d", id, l.ID)))
				count++
				if count%3 == 0 {
					rows = append(rows, menu.Row(currentRow...))
					currentRow = []tele.Btn{}
				}
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, menu.Row(currentRow...))
		}
		menu.Inline(rows...)
		menu.Inline(menu.Row(menu.Data("❌ Отменить", "cl_cancel")))
		c.Respond(&tele.CallbackResponse{})
		return c.Edit(messages["ru"]["order_to"], menu, tele.ModeHTML)
	}

	if b.Type == BotTypeClient && strings.HasPrefix(data, "cl_t_") {
		s := strings.TrimPrefix(data, "cl_t_")
		var fromID, toID int64
		if strings.Contains(s, "_") {
			parts := strings.SplitN(s, "_", 2)
			fromID, _ = strconv.ParseInt(parts[0], 10, 64)
			toID, _ = strconv.ParseInt(parts[1], 10, 64)
			// if session lost, recover FromLocationID from callback
			if session.OrderData.FromLocationID == 0 {
				session.OrderData.FromLocationID = fromID
			}
		} else {
			toID, _ = strconv.ParseInt(s, 10, 64)
		}
		session.OrderData.ToLocationID = toID
		session.State = StateTariff

		tariffs, _ := b.Stg.Tariff().GetAll(context.Background())
		menu := &tele.ReplyMarkup{}
		var rows []tele.Row
		var currentRow []tele.Btn
		for i, t := range tariffs {
			// include from and to IDs in tariff callback so we can recover session if lost
			fromID := session.OrderData.FromLocationID
			toID := session.OrderData.ToLocationID
			currentRow = append(currentRow, menu.Data(t.Name, fmt.Sprintf("tf_%d_%d_%d", fromID, toID, t.ID)))
			if (i+1)%2 == 0 {
				rows = append(rows, menu.Row(currentRow...))
				currentRow = []tele.Btn{}
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, menu.Row(currentRow...))
		}
		menu.Inline(rows...)
		menu.Inline(menu.Row(menu.Data("❌ Отменить", "cl_cancel")))
		return c.Edit(messages["ru"]["order_tariff"], menu)
	}

	if b.Type == BotTypeClient && strings.HasPrefix(data, "tf_") {
		s := strings.TrimPrefix(data, "tf_")
		var fromID, toID, tariffID int64
		if strings.Contains(s, "_") {
			parts := strings.SplitN(s, "_", 3)
			if len(parts) >= 3 {
				fromID, _ = strconv.ParseInt(parts[0], 10, 64)
				toID, _ = strconv.ParseInt(parts[1], 10, 64)
				tariffID, _ = strconv.ParseInt(parts[2], 10, 64)
			}
			// recover session data if missing
			if session.OrderData.FromLocationID == 0 {
				session.OrderData.FromLocationID = fromID
			}
			if session.OrderData.ToLocationID == 0 {
				session.OrderData.ToLocationID = toID
			}
		} else {
			tariffID, _ = strconv.ParseInt(s, 10, 64)
		}
		session.OrderData.TariffID = tariffID
		session.OrderData.Passengers = 1 // Default to 1 passenger
		session.State = StateDateTime

		// Show calendar for current month
		now := time.Now()
		return b.generateCalendar(c, now.Year(), int(now.Month()))
	}

	// Passenger selection removed - now defaults to 1

	if strings.HasPrefix(data, "nav_") {
		// Calendar navigation: nav_YYYY_M
		parts := strings.Split(data, "_")
		year, _ := strconv.Atoi(parts[1])
		month, _ := strconv.Atoi(parts[2])
		return b.generateCalendar(c, year, month)
	}

	if b.Type == BotTypeClient && strings.HasPrefix(data, "cal_") {
		// Date selected: cal_2024-02-15
		dateStr := strings.TrimPrefix(data, "cal_")
		session.TempString = dateStr

		// Show time selection
		menu := &tele.ReplyMarkup{}
		var rows []tele.Row
		var currentRow []tele.Btn

		// Use Moscow time (UTC+3)
		loc := time.FixedZone("Europe/Moscow", 3*60*60)
		now := time.Now().In(loc)

		currentHour := -1
		if dateStr == now.Format("2006-01-02") {
			currentHour = now.Hour()
		}

		for h := 0; h <= 23; h++ {
			timeStr := fmt.Sprintf("%02d:00", h)
			var btn tele.Btn
			if h <= currentHour {
				btn = menu.Data("🔒 "+timeStr, "ignore")
			} else {
				btn = menu.Data(timeStr, fmt.Sprintf("time_%s", timeStr))
			}
			currentRow = append(currentRow, btn)
			if len(currentRow) == 4 {
				rows = append(rows, menu.Row(currentRow...))
				currentRow = []tele.Btn{}
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, menu.Row(currentRow...))
		}
		menu.Inline(rows...)
		menu.Inline(menu.Row(menu.Data("❌ Отменить", "cl_cancel")))
		c.Respond(&tele.CallbackResponse{})
		return c.Edit("🕒 Выберите время:", menu)
	}

	if strings.HasPrefix(data, "take_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "take_"), 10, 64)
		b.Bot.Edit(c.Callback().Message, "✅ Заказ принят!")
		return b.handleTakeOrderWithID(c, id)
	}

	if strings.HasPrefix(data, "complete_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "complete_"), 10, 64)
		b.Stg.Order().CompleteOrder(context.Background(), id)
		b.Bot.Edit(c.Callback().Message, "🏁 Заказ завершен!")
		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		if order != nil {
			b.notifyUser(order.ClientID, messages["ru"]["notif_done"])
		}
		return c.Respond()
	}

	if strings.HasPrefix(data, "cancel_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "cancel_"), 10, 64)
		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		oldStatus := ""
		if order != nil {
			oldStatus = order.Status
		}

		rows, err := b.Stg.Order().CancelOrder(context.Background(), id)
		if err != nil || rows == 0 {
			return c.Respond(&tele.CallbackResponse{Text: "❌ Невозможно отменить. Возможно, заказ уже принят."})
		}

		// Notify Admin if it was still in pending/active
		if oldStatus == "pending" || oldStatus == "active" {
			b.notifyAdmin(id, fmt.Sprintf("⚠️ <b>Заказ #%d отменен клиентом.</b>", id))
		}
		// Notify Driver if it was already wait_confirm or taken
		if (oldStatus == "wait_confirm" || oldStatus == "taken") && order.DriverID != nil {
			b.notifyUser(*order.DriverID, fmt.Sprintf("❌ <b>Заказ #%d, который вы выбрали, отменен клиентом.</b>", id))
		}

		c.Respond(&tele.CallbackResponse{Text: "Заказ отменен"})
		return c.Edit("❌ <b>Заказ отменен.</b>", tele.ModeHTML)
	}

	if data == "cl_cancel" {
		return b.resetOrderFlow(c)
	}

	if strings.HasPrefix(data, "return_order_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "return_order_"), 10, 64)
		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		if order == nil || order.Status != "taken" {
			return c.Respond(&tele.CallbackResponse{Text: "Ошибка: Заказ уже в пути или завершен."})
		}

		// Reset status to active and remove driver
		if _, err := b.DB.Exec(context.Background(), "UPDATE orders SET status='active', driver_id=NULL WHERE id=$1", id); err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "Ошибка базы данных"})
		}

		b.Bot.Edit(c.Callback().Message, "✅ Заказ возвращен в пул. Теперь его могут увидеть другие водители.")

		// Senior Logic: Re-notify other drivers
		from, _ := b.Stg.Location().GetByID(context.Background(), order.FromLocationID)
		to, _ := b.Stg.Location().GetByID(context.Background(), order.ToLocationID)
		tariff, _ := b.Stg.Tariff().GetByID(context.Background(), order.TariffID)
		fromName, toName, tariffName := "Неизвестно", "Неизвестно", "Неизвестно"
		if from != nil {
			fromName = from.Name
		}
		if to != nil {
			toName = to.Name
		}
		if tariff != nil {
			tariffName = tariff.Name
		}

		priceStr := fmt.Sprintf("%d %s", order.Price, order.Currency)
		routeStr := fmt.Sprintf("%s ➡️ %s", fromName, toName)
		notifMsg := fmt.Sprintf("♻️ <b>ЗАКАЗ СНОВА ДОСТУПЕН (Вернул водитель)</b>\n\n🆔 #%d\n📍 %s\n💰 Цена: <b>%s</b>\n🚕 Тариф: <b>%s</b>", id, routeStr, priceStr, tariffName)

		b.notifyDrivers(order.ID, order.FromLocationID, order.ToLocationID, order.TariffID, notifMsg)

		// Notify Client
		b.notifyUser(order.ClientID, fmt.Sprintf("⚠️ <b>Водитель отменил принятие заказа #%d.</b>\n\nМы снова ищем вам машину. Пожалуйста, подождите.", id))

		return c.Respond()
	}

	if data == "agenda_view" {
		return b.handleDriverAgenda(c)
	}
	if strings.HasPrefix(data, "on_way_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "on_way_"), 10, 64)
		return b.handleDriverOnWay(c, id)
	}

	if strings.HasPrefix(data, "arrived_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "arrived_"), 10, 64)
		return b.handleDriverArrived(c, id)
	}

	if strings.HasPrefix(data, "start_trip_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "start_trip_"), 10, 64)
		return b.handleDriverStartTrip(c, id)
	}

	if data == "native_cal" {
		now := time.Now()
		return b.generateCalendarWithPrefix(c, now.Year(), int(now.Month()), "sc_cal_")
	}

	if strings.HasPrefix(data, "sc_nav_") {
		parts := strings.Split(data, "_")
		if len(parts) >= 4 {
			year, _ := strconv.Atoi(parts[2])
			month, _ := strconv.Atoi(parts[3])
			return b.generateCalendarWithPrefix(c, year, month, "sc_cal_")
		}
		return nil
	}

	if strings.HasPrefix(data, "sc_cal_all_") {
		dateStr := strings.TrimPrefix(data, "sc_cal_all_")
		return b.handleDriverDateSearchAll(c, dateStr)
	}

	if strings.HasPrefix(data, "sc_cal_") {
		dateStr := strings.TrimPrefix(data, "sc_cal_")
		return b.handleDriverDateSearch(c, dateStr)
	}

	// Route Admin Moderation and Pagination callbacks
	isAdminCallback := strings.HasPrefix(data, "user_blk_") ||
		strings.HasPrefix(data, "user_act_") ||
		strings.HasPrefix(data, "set_role_") ||
		strings.HasPrefix(data, "approve_driver_") ||
		strings.HasPrefix(data, "reject_driver_") ||
		strings.HasPrefix(data, "block_driver_") ||
		strings.HasPrefix(data, "approve_order_") ||
		strings.HasPrefix(data, "reject_order_") ||
		strings.HasPrefix(data, "block_user_") ||
		strings.HasPrefix(data, "users_page_") ||
		strings.HasPrefix(data, "orders_page_") ||
		strings.HasPrefix(data, "adm_role_") ||
		strings.HasPrefix(data, "adm_stat_") ||
		strings.HasPrefix(data, "adm_cancel_") ||
		strings.HasPrefix(data, "adm_approve_") ||
		strings.HasPrefix(data, "adm_reject_") ||
		strings.HasPrefix(data, "approve_match_") ||
		strings.HasPrefix(data, "reject_match_") ||
		strings.HasPrefix(data, "car_addmodel_") ||
		strings.HasPrefix(data, "unblock_")

	if isAdminCallback {
		return b.handleAdminCallbacks(c, data)
	}

	if strings.HasPrefix(data, "reg_brand_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "reg_brand_"), 10, 64)
		return b.handleCarBrandSelection(c, id)
	}

	if strings.HasPrefix(data, "reg_model_") {
		if data == "reg_model_other" {
			return b.handleCarModelOther(c)
		}
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "reg_model_"), 10, 64)
		return b.handleCarModelSelection(c, id)
	}

	// Driver Callbacks
	if data == "add_route" {
		return b.handleAddRouteStart(c, session)
	}

	if data == "clear_routes" {
		b.Stg.Route().ClearRoutes(context.Background(), session.DBID)
		return b.handleDriverRoutes(c)
	}

	if strings.HasPrefix(data, "dr_f_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "dr_f_"), 10, 64)
		// Store route from location temporarily (not in OrderData)
		session.TempString = fmt.Sprintf("route_from:%d", id)
		session.OrderData.FromLocationID = id // Also store in OrderData for now
		return b.handleAddRouteTo(c, session)
	}

	if strings.HasPrefix(data, "dr_t_") {
		toID, _ := strconv.ParseInt(strings.TrimPrefix(data, "dr_t_"), 10, 64)
		session.OrderData.ToLocationID = toID
		if session.OrderData.FromLocationID == 0 {
			return c.Send("❌ Ошибка: Не выбран город отправления. Пожалуйста, начните заново.")
		}
		b.Log.Info("Driver route data ready",
			logger.Int64("from_id", session.OrderData.FromLocationID),
			logger.Int64("to_id", session.OrderData.ToLocationID),
			logger.String("temp_string", session.TempString),
		)
		return b.handleAddRouteComplete(c, session)
	}

	if strings.HasPrefix(data, "tgl_") {
		tariffID, _ := strconv.ParseInt(strings.TrimPrefix(data, "tgl_"), 10, 64)
		b.Stg.Tariff().Toggle(context.Background(), session.DBID, tariffID)
		return b.showDriverTariffs(c, false)
	}

	if data == "tf_del_mode" {
		return b.showDriverTariffs(c, true)
	}

	if data == "tf_back" {
		return b.showDriverTariffs(c, false)
	}

	if data == "tf_done" {
		// Faqat ro'yxatdan o'tish jarayonida registration check ishga tushsin
		user := b.getCurrentUser(c)
		c.Respond(&tele.CallbackResponse{})
		if user != nil && (user.Status == "pending" || user.Status == "pending_review") {
			return b.handleRegistrationCheck(c)
		}
		// Ro'yxatdan o'tgan driver uchun inline xabarni o'chirib asosiy menuga qaytish
		c.Delete()
		return b.showMenu(c, user)
	}

	if data == "routes_done" {
		// Faqat ro'yxatdan o'tish jarayonida tarif sahifasiga o'tish
		user := b.getCurrentUser(c)
		c.Respond(&tele.CallbackResponse{})
		if user != nil && (user.Status == "pending" || user.Status == "pending_review") {
			return b.handleDriverTariffs(c)
		}
		// Ro'yxatdan o'tgan driver uchun inline xabarni o'chirib asosiy menuga qaytish
		c.Delete()
		return b.showMenu(c, user)
	}

	if strings.HasPrefix(data, "del_tf_") {
		tariffID, _ := strconv.ParseInt(strings.TrimPrefix(data, "del_tf_"), 10, 64)
		// Unsubscribe driver from tariff (Toggle removes if already enabled)
		enabled, _ := b.Stg.Tariff().GetEnabled(context.Background(), session.DBID)
		if enabled[tariffID] {
			b.Stg.Tariff().Toggle(context.Background(), session.DBID, tariffID)
		}
		return b.showDriverTariffs(c, true)
	}

	if b.Type == BotTypeClient {
		switch data {
		case "confirm_yes":
			if session.OrderData == nil || session.OrderData.FromLocationID == 0 || session.OrderData.ToLocationID == 0 || session.OrderData.TariffID == 0 {
				b.Log.Warning("Invalid order data in session for confirm_yes", logger.Int64("user_id", c.Sender().ID))
				return c.Send("⚠️ <b>Ошибка:</b> Данные заказа не найдены. Пожалуйста, нажмите /start и оформите заказ заново.", tele.ModeHTML)
			}
			if session.State != StateConfirm {
				return c.Respond(&tele.CallbackResponse{Text: "❌ Сессия устарела."})
			}

			// Get client info for order
			client, _ := b.Stg.User().GetByID(context.Background(), session.DBID)
			if client != nil {
				if client.Username != "" {
					session.OrderData.ClientUsername = client.Username
				} else {
					session.OrderData.ClientUsername = "Неизвестно"
				}
				if client.Phone != nil {
					session.OrderData.ClientPhone = *client.Phone
				} else {
					session.OrderData.ClientPhone = "Неизвестно"
				}
			} else {
				session.OrderData.ClientUsername = "Неизвестно"
				session.OrderData.ClientPhone = "Неизвестно"
			}

			session.OrderData.Status = "pending"
			order, err := b.Stg.Order().Create(context.Background(), session.OrderData)
			if err == nil {
				c.Send(messages["ru"]["order_created"])
				// Reconstructing strictly for Admin message:
				from, _ := b.Stg.Location().GetByID(context.Background(), session.OrderData.FromLocationID)
				to, _ := b.Stg.Location().GetByID(context.Background(), session.OrderData.ToLocationID)
				fromName, toName := "Неизвестно", "Неизвестно"
				if from != nil {
					fromName = from.Name
				}
				if to != nil {
					toName = to.Name
				}
				timeStr := "Сейчас"
				if session.OrderData.PickupTime != nil {
					loc := time.FixedZone("Europe/Moscow", 3*60*60)
					timeStr = session.OrderData.PickupTime.In(loc).Format("02.01.2006 15:04")
				}

				clientName := "Неизвестно"
				clientTeleID := int64(0)
				if client != nil {
					clientName = client.FullName
					clientTeleID = client.TelegramID
				}

				adminMsg := fmt.Sprintf("🔔 <b>НОВЫЙ ЗАКАЗ (На утверждение)</b>\n\n🆔 #%d\n📍 %s ➡️ %s\n💰 Цена: %d %s\n👥 Пассажиры: %d\n📅 Время: %s\n\n👤 Клиент: <a href=\"tg://user?id=%d\">%s</a>\n📞 Тел: %s",
					order.ID, fromName, toName, order.Price, order.Currency, order.Passengers, timeStr, clientTeleID, clientName, order.ClientPhone)

				b.notifyAdmin(order.ID, adminMsg)
				c.Send("⏳ Ваш заказ отправлен администратору. Ожидайте подтверждения.")
			} else {
				b.Log.Error("Order creation failed", logger.Error(err))
				c.Send("❌ Произошла ошибка при создании заказа.")
			}
			session.State = StateIdle
			return b.showMenu(c, b.getCurrentUser(c))
		case "confirm_no":
			return b.resetOrderFlow(c)
		}
	}

	if data == "ignore" {
		return c.Respond(&tele.CallbackResponse{Text: ""})
	}

	if b.Type == BotTypeClient && strings.HasPrefix(data, "time_") {
		timeStr := strings.TrimPrefix(data, "time_") // "14:00"
		if session.TempString == "" {
			c.Delete()
			return c.Send("⚠️ <b>Ошибка:</b> Дата не выбрана. Пожалуйста, нажмите /start и оформите заказ заново.", tele.ModeHTML)
		}

		fullTimeStr := fmt.Sprintf("%s %s", session.TempString, timeStr) // "2023-10-27 14:00"
		loc := time.FixedZone("Europe/Moscow", 3*60*60)
		parsedTime, err := time.ParseInLocation("2006-01-02 15:04", fullTimeStr, loc)
		if err != nil {
			b.Log.Error("Failed to parse time", logger.Error(err), logger.String("fullTimeStr", fullTimeStr))
			c.Delete()
			return c.Send("⚠️ <b>Ошибка:</b> Неверный формат времени. Пожалуйста, нажмите /start и оформите заказ заново.", tele.ModeHTML)
		}

		// Convert to UTC for storage
		utcTime := parsedTime.UTC()

		session.OrderData.PickupTime = &utcTime
		session.OrderData.Price = 0
		session.OrderData.Currency = "RUB"

		session.State = StatePassengers

		// Show passenger count selection
		menu := &tele.ReplyMarkup{}
		menu.Inline(
			menu.Row(menu.Data("1", "pass_1"), menu.Data("2", "pass_2"), menu.Data("3", "pass_3"), menu.Data("4", "pass_4")),
			menu.Row(menu.Data("5", "pass_5"), menu.Data("6", "pass_6"), menu.Data("7", "pass_7"), menu.Data("8", "pass_8")),
		)

		c.Respond(&tele.CallbackResponse{})
		return c.Edit("👥 <b>Количество пассажиров?</b>\n\nВыберите из списка или напишите число:", menu, tele.ModeHTML)
	}

	if b.Type == BotTypeClient && strings.HasPrefix(data, "pass_") {
		count, _ := strconv.Atoi(strings.TrimPrefix(data, "pass_"))
		session.OrderData.Passengers = count
		session.State = StatePrice
		c.Respond(&tele.CallbackResponse{})
		return c.Edit("💰 <b>Укажите сумму за поездку (RUB):</b>\n\nНапример: <code>1500</code>", tele.ModeHTML)
	}

	return nil
}

func (b *Bot) handleAdminCallbacks(c tele.Context, data string) error {
	if b.Type != BotTypeAdmin {
		return nil
	}
	// Ruxsat: DB da roli "admin" bo‘lgan foydalanuvchi (login/parol yoki AdminID orqali)
	adm, _ := b.Stg.User().Get(context.Background(), c.Sender().ID)
	if adm == nil || adm.Role != "admin" {
		return nil
	}

	// Марка/модель: tanlashdan keyin model nomi so‘raladi
	if strings.HasPrefix(data, "car_addmodel_") {
		if data == "car_addmodel_cancel" {
			return c.Respond(&tele.CallbackResponse{Text: "Отменено"}) // Message stays, user can press Назад в меню
		}
		brandID, _ := strconv.ParseInt(strings.TrimPrefix(data, "car_addmodel_"), 10, 64)
		if brandID == 0 {
			return c.Respond(&tele.CallbackResponse{Text: "Ошибка"})
		}
		session := b.Sessions[c.Sender().ID]
		if session == nil {
			session = &UserSession{State: StateIdle}
			b.Sessions[c.Sender().ID] = session
		}
		session.State = StateCarModelAdd
		session.TempString = strconv.FormatInt(brandID, 10)
		_ = c.Respond(&tele.CallbackResponse{Text: "Введите название модели"})
		return c.Send("✏️ Введите название модели автомобиля:")
	}

	// Admin: delete car brand
	if strings.HasPrefix(data, "adm_del_brand_") {
		brandID, _ := strconv.ParseInt(strings.TrimPrefix(data, "adm_del_brand_"), 10, 64)
		if err := b.Stg.Car().DeleteBrand(context.Background(), brandID); err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "❌ Ошибка. Сначала удалите все модели этой марки."})
		}
		c.Respond(&tele.CallbackResponse{Text: "✅ Марка удалена"})
		return c.Edit("✅ <b>Марка удалена.</b>", tele.ModeHTML)
	}

	// Admin: select brand to delete its model
	if strings.HasPrefix(data, "adm_sel_brand_del_") {
		brandID, _ := strconv.ParseInt(strings.TrimPrefix(data, "adm_sel_brand_del_"), 10, 64)
		models, _ := b.Stg.Car().GetModels(context.Background(), brandID)
		if len(models) == 0 {
			return c.Respond(&tele.CallbackResponse{Text: "Нет моделей для этой марки"})
		}
		menu := &tele.ReplyMarkup{}
		var rows []tele.Row
		for _, m := range models {
			rows = append(rows, menu.Row(menu.Data(m.Name, fmt.Sprintf("adm_del_model_%d", m.ID))))
		}
		rows = append(rows, menu.Row(menu.Data("⬅️ Назад", "adm_del_model_cancel")))
		menu.Inline(rows...)
		c.Respond(&tele.CallbackResponse{})
		return c.Edit("🗑 <b>Выберите модель для удаления:</b>", menu, tele.ModeHTML)
	}

	// Admin: delete car model
	if strings.HasPrefix(data, "adm_del_model_") {
		if data == "adm_del_model_cancel" {
			return c.Respond(&tele.CallbackResponse{Text: "Отменено"})
		}
		modelID, _ := strconv.ParseInt(strings.TrimPrefix(data, "adm_del_model_"), 10, 64)
		if err := b.Stg.Car().DeleteModel(context.Background(), modelID); err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "❌ Ошибка удаления"})
		}
		c.Respond(&tele.CallbackResponse{Text: "✅ Модель удалена"})
		return c.Edit("✅ <b>Модель удалена.</b>", tele.ModeHTML)
	}

	// Разблокировать пользователя
	if strings.HasPrefix(data, "unblock_") {
		userDBID, _ := strconv.ParseInt(strings.TrimPrefix(data, "unblock_"), 10, 64)
		b.Stg.User().UpdateStatusByID(context.Background(), userDBID, "active")
		c.Edit(c.Callback().Message, c.Callback().Message.Text+"\n\n✅ <b>Разблокирован</b>", tele.ModeHTML)
		return c.Respond(&tele.CallbackResponse{Text: "Разблокировано"})
	}

	if strings.HasPrefix(data, "set_role_") {
		// Format: set_role_{role}_{telegramID}
		// Rol nomida '_' bo'lishi mumkin, shuning uchun oxiridan ID ajratamiz
		trimmed := strings.TrimPrefix(data, "set_role_")
		lastUnderscore := strings.LastIndex(trimmed, "_")
		if lastUnderscore < 0 {
			return c.Respond(&tele.CallbackResponse{Text: "❌ Неверный формат"})
		}
		role := trimmed[:lastUnderscore]
		id, err := strconv.ParseInt(trimmed[lastUnderscore+1:], 10, 64)
		if err != nil || role == "" {
			return c.Respond(&tele.CallbackResponse{Text: "❌ Неверный формат"})
		}
		b.Stg.User().UpdateRole(context.Background(), id, role)
		return c.Respond(&tele.CallbackResponse{Text: "OK"})
	}
	if strings.HasPrefix(data, "user_blk_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "user_blk_"), 10, 64)
		b.Stg.User().UpdateStatus(context.Background(), id, "blocked")
		return c.Respond(&tele.CallbackResponse{Text: "Заблокировано"})
	}
	if strings.HasPrefix(data, "user_act_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "user_act_"), 10, 64)
		b.Stg.User().UpdateStatus(context.Background(), id, "active")
		return c.Respond(&tele.CallbackResponse{Text: "Активировано"})
	}

	// Driver Moderation
	if strings.HasPrefix(data, "approve_driver_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "approve_driver_"), 10, 64)
		// Protect admin role
		user, _ := b.Stg.User().GetByID(context.Background(), id)
		newRole := "driver"
		if user != nil && user.Role == "admin" {
			newRole = "admin"
		}
		b.Stg.User().UpdateStatusByID(context.Background(), id, "active")
		b.Stg.User().UpdateRoleByID(context.Background(), id, newRole)
		b.notifyDriverSpecific(id, "✅ Ваш аккаунт водителя подтвержден! Теперь вы можете принимать заказы.")
		c.Edit(c.Callback().Message, fmt.Sprintf("%s\n\n✅ <b>Одобрено</b>", c.Callback().Message.Text), tele.ModeHTML)
		return c.Respond(&tele.CallbackResponse{Text: "Водитель одобрен"})
	}
	if strings.HasPrefix(data, "reject_driver_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "reject_driver_"), 10, 64)
		b.Stg.User().UpdateStatusByID(context.Background(), id, "rejected")
		b.notifyUser(id, "❌ Ваша заявка на водителя отклонена.")
		c.Edit(c.Callback().Message, fmt.Sprintf("%s\n\n❌ <b>Отклонено</b>", c.Callback().Message.Text), tele.ModeHTML)
		return c.Respond(&tele.CallbackResponse{Text: "Водитель отклонен"})
	}
	if strings.HasPrefix(data, "block_driver_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "block_driver_"), 10, 64)
		b.Stg.User().UpdateStatusByID(context.Background(), id, "blocked")
		b.notifyUser(id, "🚫 Ваш аккаунт заблокирован.")
		c.Edit(c.Callback().Message, fmt.Sprintf("%s\n\n🚫 <b>Заблокирован</b>", c.Callback().Message.Text), tele.ModeHTML)
		return c.Respond(&tele.CallbackResponse{Text: "Водитель заблокирован"})
	}

	// Order Moderation (From Notifications)
	if strings.HasPrefix(data, "approve_order_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "approve_order_"), 10, 64)
		b.Log.Info("Admin approving order",
			logger.Int64("admin_id", c.Sender().ID),
			logger.Int64("order_id", id),
		)
		return b.approveOrderByAdmin(c, id, "")
	}
	if strings.HasPrefix(data, "reject_order_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "reject_order_"), 10, 64)
		b.Log.Info("Admin rejecting order",
			logger.Int64("admin_id", c.Sender().ID),
			logger.Int64("order_id", id),
		)
		// Use the new granular status for admin rejections
		b.Stg.Order().UpdateStatus(context.Background(), id, "cancelled_by_admin")
		b.Log.Info("Order rejected successfully",
			logger.Int64("order_id", id),
			logger.String("new_status", "cancelled_by_admin"),
		)

		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		if order != nil {
			b.notifyUser(order.ClientID, "❌ Ваш заказ отклонен администратором.")
		}
		c.Edit(c.Callback().Message, fmt.Sprintf("%s\n\n❌ <b>Отклонено</b>", c.Callback().Message.Text), tele.ModeHTML)
		return c.Respond(&tele.CallbackResponse{Text: "Заказ отклонен"})
	}
	if strings.HasPrefix(data, "block_user_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "block_user_"), 10, 64)
		b.Stg.User().UpdateStatus(context.Background(), id, "blocked")
		b.notifyUser(id, "🚫 Ваш аккаунт заблокирован.")
		c.Edit(c.Callback().Message, fmt.Sprintf("%s\n\n🚫 <b>Клиент заблокирован</b>", c.Callback().Message.Text), tele.ModeHTML)
		return c.Respond(&tele.CallbackResponse{Text: "Клиент заблокирован"})
	}

	// Pagination Handlers
	if strings.HasPrefix(data, "users_page_") {
		page, _ := strconv.Atoi(strings.TrimPrefix(data, "users_page_"))
		return b.showUsersPage(c, page)
	}
	if strings.HasPrefix(data, "orders_page_") {
		page, _ := strconv.Atoi(strings.TrimPrefix(data, "orders_page_"))
		return b.showOrdersPage(c, page)
	}

	// Admin Actions with Pagination Return
	if strings.HasPrefix(data, "adm_role_") {
		parts := strings.Split(strings.TrimPrefix(data, "adm_role_"), "_") // ID_PAGE
		teleID, _ := strconv.ParseInt(parts[0], 10, 64)
		page, _ := strconv.Atoi(parts[1])

		user, _ := b.Stg.User().Get(context.Background(), teleID)
		if user != nil {
			if user.Role == "admin" {
				return c.Respond(&tele.CallbackResponse{Text: "❌ Admin rolini o'zgartirib bo'lmaydi"})
			}
			newRole := "driver"
			if user.Role == "driver" {
				newRole = "client"
			}
			b.Stg.User().UpdateRole(context.Background(), teleID, newRole)
		}
		return b.showUsersPage(c, page)
	}

	if strings.HasPrefix(data, "adm_stat_") {
		parts := strings.Split(strings.TrimPrefix(data, "adm_stat_"), "_") // ID_PAGE
		teleID, _ := strconv.ParseInt(parts[0], 10, 64)
		page, _ := strconv.Atoi(parts[1])

		user, _ := b.Stg.User().Get(context.Background(), teleID)
		if user != nil {
			newStatus := "blocked"
			if user.Status == "blocked" {
				newStatus = "active"
			}
			b.Stg.User().UpdateStatus(context.Background(), teleID, newStatus)
		}
		return b.showUsersPage(c, page)
	}

	if strings.HasPrefix(data, "adm_del_user_") {
		parts := strings.Split(strings.TrimPrefix(data, "adm_del_user_"), "_") // teleID_page
		teleID, _ := strconv.ParseInt(parts[0], 10, 64)
		page, _ := strconv.Atoi(parts[1])
		b.Stg.User().DeleteUser(context.Background(), teleID)
		c.Respond(&tele.CallbackResponse{Text: "✅ Пользователь удалён"})
		return b.showUsersPage(c, page)
	}

	if data == "noop" {
		return c.Respond(&tele.CallbackResponse{})
	}

	if data == "admin_back" {
		return c.Delete()
	}

	if strings.HasPrefix(data, "adm_cancel_") {
		parts := strings.Split(strings.TrimPrefix(data, "adm_cancel_"), "_") // ID_PAGE
		orderID, _ := strconv.ParseInt(parts[0], 10, 64)
		page, _ := strconv.Atoi(parts[1])

		order, _ := b.Stg.Order().GetByID(context.Background(), orderID)
		if order == nil {
			return c.Respond(&tele.CallbackResponse{Text: "Заказ не найден"})
		}

		rows, _ := b.Stg.Order().CancelOrder(context.Background(), orderID)
		if rows > 0 {
			// Notify Client
			b.notifyUser(order.ClientID, fmt.Sprintf("❌ <b>Ваш заказ #%d отменен модератором.</b>", orderID))
			// Notify Driver if any
			if order.DriverID != nil {
				b.notifyUser(*order.DriverID, fmt.Sprintf("❌ <b>Заказ #%d отменен модератором.</b>", orderID))
			}
			c.Respond(&tele.CallbackResponse{Text: "Заказ отменен"})
		} else {
			c.Respond(&tele.CallbackResponse{Text: "Не удалось отменить (уже завершен?)"})
		}
		return b.showOrdersPage(c, page)
	}

	if strings.HasPrefix(data, "adm_approve_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "adm_approve_"), 10, 64)
		b.Log.Info("Admin approving order", logger.Int64("order_id", id))
		return b.approveOrderByAdmin(c, id, "✅ Подтверждено и отправлено водителям.")
	}

	if strings.HasPrefix(data, "adm_reject_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "adm_reject_"), 10, 64)
		b.Log.Info("Admin rejecting order", logger.Int64("order_id", id))
		b.Stg.Order().UpdateStatus(context.Background(), id, "cancelled_by_admin")
		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		if order != nil {
			b.notifyUser(order.ClientID, "❌ Ваш заказ отменен администратором.")
		}
		return c.Edit("❌ Отклонено.")
	}

	// Match Approval (Driver <-> Client)
	if strings.HasPrefix(data, "approve_match_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "approve_match_"), 10, 64)

		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		if order == nil {
			return c.Edit("❌ Заказ не найден.")
		}
		if order.Status != "wait_confirm" || order.DriverID == nil {
			return c.Edit("❌ Этот заказ не находится в статусе ожидания подтверждения.")
		}

		// 1. Finalize Order (wait_confirm -> taken)
		if _, err := b.DB.Exec(context.Background(), "UPDATE orders SET status='taken' WHERE id=$1 AND status='wait_confirm'", id); err != nil {
			return c.Edit("❌ Произошла ошибка.")
		}

		// 2. Notify Client (with Driver details)
		driver, _ := b.Stg.User().GetByID(context.Background(), *order.DriverID)
		phone := "Неизвестно"
		if driver != nil {
			if driver.Phone != nil {
				phone = *driver.Phone
			}
			profile := fmt.Sprintf("<a href=\"tg://user?id=%d\">%s</a>", driver.TelegramID, driver.FullName)
			msg := fmt.Sprintf(messages["ru"]["notif_taken"], id, driver.FullName, phone, profile)
			b.notifyUser(order.ClientID, msg)
		}

		// 3. Notify Driver
		client, _ := b.Stg.User().GetByID(context.Background(), order.ClientID)
		clientInfo := "Данные клиента недоступны"
		if client != nil {
			clientPhone := "Неизвестно"
			if client.Phone != nil {
				clientPhone = *client.Phone
			}
			clientInfo = fmt.Sprintf("👤 Клиент: <a href=\"tg://user?id=%d\">%s</a>\n📞 Тел: %s", client.TelegramID, client.FullName, clientPhone)
		}
		b.notifyDriverSpecific(*order.DriverID, fmt.Sprintf("✅ Админ подтвердил заказ! (#%d)\n\n%s\n\nСвяжитесь с клиентом.", id, clientInfo))

		return c.Edit("✅ Успешно прикреплено.")
	}

	if strings.HasPrefix(data, "reject_match_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "reject_match_"), 10, 64)

		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		if order == nil {
			return c.Edit("❌ Заказ не найден.")
		}
		var requestedDriverID *int64
		if order.DriverID != nil {
			tmp := *order.DriverID
			requestedDriverID = &tmp
		}

		// 1. Reset Status to Active only if still waiting confirm
		if _, err := b.DB.Exec(context.Background(), "UPDATE orders SET status='active', driver_id=NULL WHERE id=$1 AND status='wait_confirm'", id); err != nil {
			return c.Edit("❌ Произошла ошибка.")
		}

		// 2. Notify rejected driver
		if requestedDriverID != nil {
			b.notifyDriverSpecific(*requestedDriverID, fmt.Sprintf("❌ Админ отклонил ваш запрос на заказ. (#%d)", id))
		}

		// 3. Senior Fix: Recycler Logic - Re-notify other drivers that order is back in pool
		from, _ := b.Stg.Location().GetByID(context.Background(), order.FromLocationID)
		to, _ := b.Stg.Location().GetByID(context.Background(), order.ToLocationID)
		tariff, _ := b.Stg.Tariff().GetByID(context.Background(), order.TariffID)
		fromName, toName, tariffName := "Неизвестно", "Неизвестно", "Неизвестно"
		if from != nil {
			fromName = from.Name
		}
		if to != nil {
			toName = to.Name
		}
		if tariff != nil {
			tariffName = tariff.Name
		}

		priceStr := fmt.Sprintf("%d %s", order.Price, order.Currency)
		routeStr := fmt.Sprintf("%s ➡️ %s", fromName, toName)
		notifMsg := fmt.Sprintf("♻️ <b>ЗАКАЗ СНОВА ДОСТУПЕН</b>\n\n🆔 #%d\n📍 %s\n💰 Цена: <b>%s</b>\n🚕 Тариф: <b>%s</b>", id, routeStr, priceStr, tariffName)

		b.notifyDrivers(order.ID, order.FromLocationID, order.ToLocationID, order.TariffID, notifMsg)

		return c.Edit("❌ Отклонено. Заказ снова активирован и разослан водителям.")
	}

	return nil
}

// approveOrderByAdmin — umumiy order tasdiqlash logikasi.
// successMsg bo'sh bo'lsa, xabarga "✅ Подтверждено" qo'shiladi.
func (b *Bot) approveOrderByAdmin(c tele.Context, orderID int64, successMsg string) error {
	order, _ := b.Stg.Order().GetByID(context.Background(), orderID)
	if order == nil {
		c.Edit("❌ Заказ не найден.")
		return c.Respond(&tele.CallbackResponse{Text: "Заказ не найден"})
	}

	if order.Status != "pending" {
		c.Edit(fmt.Sprintf("❌ Невозможно подтвердить. Текущий статус: %s", order.Status))
		return c.Respond(&tele.CallbackResponse{Text: "Заказ уже не в ожидании"})
	}

	order.Status = "active"
	b.Stg.Order().Update(context.Background(), order)
	b.Log.Info("Order approved", logger.Int64("order_id", orderID), logger.String("status", "active"))

	from, _ := b.Stg.Location().GetByID(context.Background(), order.FromLocationID)
	to, _ := b.Stg.Location().GetByID(context.Background(), order.ToLocationID)
	tariff, _ := b.Stg.Tariff().GetByID(context.Background(), order.TariffID)

	fromName, toName, tariffName := "Неизвестно", "Неизвестно", "Неизвестно"
	if from != nil {
		fromName = from.Name
	}
	if to != nil {
		toName = to.Name
	}
	if tariff != nil {
		tariffName = tariff.Name
	}

	priceStr := fmt.Sprintf("%d %s", order.Price, order.Currency)
	routeStr := fmt.Sprintf("%s ➡️ %s", fromName, toName)
	notifMsg := fmt.Sprintf(messages["ru"]["notif_new"], order.ID, priceStr, routeStr)
	notifMsg += fmt.Sprintf("\n🚕 Тариф: <b>%s</b>\n👥 Пассажиров: <b>%d</b>", tariffName, order.Passengers)

	b.notifyDrivers(order.ID, order.FromLocationID, order.ToLocationID, order.TariffID, notifMsg)
	clientNotif := fmt.Sprintf("✅ <b>Ваш заказ #%d подтвержден!</b>\n\n📍 %s\n💰 Цена: <b>%d %s</b>\n\nИщем водителя...", order.ID, routeStr, order.Price, order.Currency)
	b.notifyUser(order.ClientID, clientNotif)

	if successMsg != "" {
		c.Edit(successMsg)
	} else {
		c.Edit(c.Callback().Message, fmt.Sprintf("%s\n\n✅ <b>Подтверждено</b>", c.Callback().Message.Text), tele.ModeHTML)
	}
	return c.Respond(&tele.CallbackResponse{Text: "Заказ одобрен"})
}

func (b *Bot) notifyDriverSpecific(driverID int64, text string) {
	target := b
	if b.Type != BotTypeDriver {
		if p, ok := b.Peers[BotTypeDriver]; ok {
			target = p
		} else {
			return
		}
	}
	var teleID int64
	b.DB.QueryRow(context.Background(), "SELECT telegram_id FROM users WHERE id=$1", driverID).Scan(&teleID)
	if teleID != 0 {
		// Include driver menu in the activation message
		menu := &tele.ReplyMarkup{ResizeKeyboard: true}
		menu.Reply(
			menu.Row(menu.Text("📦 Активные заказы")),
			menu.Row(menu.Text("📍 Мои маршруты"), menu.Text("🚕 Мои тарифы")),
			menu.Row(menu.Text("Поиск по дате")),
			menu.Row(menu.Text("📋 Мои заказы")),
		)
		target.Bot.Send(&tele.User{ID: teleID}, text, &tele.SendOptions{ReplyMarkup: menu, ParseMode: tele.ModeHTML})

		// Reset session state in the driver bot
		if target.Sessions[teleID] != nil {
			target.Sessions[teleID].State = StateIdle
		} else {
			target.Sessions[teleID] = &UserSession{DBID: driverID, State: StateIdle}
		}
	}
}

func (b *Bot) resetOrderFlow(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	if session != nil {
		session.State = StateIdle
		session.OrderData = &models.Order{ClientID: session.DBID}
		session.TempString = ""
	}
	c.Respond(&tele.CallbackResponse{Text: "Отменено"})
	c.Edit("❌ <b>Заказ отменен.</b>", tele.ModeHTML)
	user := b.getCurrentUser(c)
	return b.showMenu(c, user)
}

func (b *Bot) notifyUser(dbID int64, text string) {
	target := b
	if b.Type != BotTypeClient {
		if p, ok := b.Peers[BotTypeClient]; ok {
			target = p
		} else {
			b.Log.Error("Client bot peer not found for notification")
			return
		}
	}
	var teleID int64
	b.DB.QueryRow(context.Background(), "SELECT telegram_id FROM users WHERE id=$1", dbID).Scan(&teleID)
	if teleID != 0 {
		target.Bot.Send(&tele.User{ID: teleID}, text, tele.ModeHTML)
	}
}

func (b *Bot) notifyAdmin(orderID int64, text string, msgType ...string) {
	target := b
	if b.Type != BotTypeAdmin {
		if p, ok := b.Peers[BotTypeAdmin]; ok {
			target = p
			b.Log.Info("Using Admin Bot peer for notification", logger.Int64("order_id", orderID))
		} else {
			b.Log.Error("Admin bot peer not found for notification")
			return
		}
	}

	// Create Keyboard based on type
	menu := &tele.ReplyMarkup{}

	t := ""
	if len(msgType) > 0 {
		t = msgType[0]
	}

	switch t {
	case "match":
		menu.Inline(menu.Row(
			menu.Data("✅ Подтвердить", fmt.Sprintf("approve_match_%d", orderID)),
			menu.Data("❌ Отклонить", fmt.Sprintf("reject_match_%d", orderID)),
		))
	case "registration":
		// For driver registration, orderID is actually userID
		menu.Inline(menu.Row(
			menu.Data(messages["ru"]["admin_btn_approve"], fmt.Sprintf("approve_driver_%d", orderID)),
			menu.Data(messages["ru"]["admin_btn_reject"], fmt.Sprintf("reject_driver_%d", orderID)),
		))
	default:
		menu.Inline(menu.Row(
			menu.Data("✅ Подтвердить", fmt.Sprintf("adm_approve_%d", orderID)),
			menu.Data("❌ Отменить", fmt.Sprintf("adm_reject_%d", orderID)),
		))
	}

	// Send to all admins
	admins, _ := b.Stg.User().GetAll(context.Background())
	sentCount := 0
	for _, u := range admins {
		if u.Role == "admin" {
			_, err := target.Bot.Send(&tele.User{ID: u.TelegramID}, text, menu, tele.ModeHTML)
			if err != nil {
				b.Log.Error("Failed to notify admin", logger.Error(err), logger.Int64("admin_id", u.TelegramID))
			} else {
				sentCount++
			}
		}
	}
	b.Log.Info("Admin notifications processed", logger.Int("sent_count", sentCount), logger.String("type", t))
}

func (b *Bot) notifyDrivers(orderID, fromID, toID, tariffID int64, text string) {
	target := b
	if b.Type != BotTypeDriver {
		if p, ok := b.Peers[BotTypeDriver]; ok {
			target = p
		} else {
			b.Log.Error("Driver bot peer not found for notification")
			return
		}
	}

	// Get drivers explicitly matching the route
	routeDriversMap := make(map[int64]bool)
	routeDrivers, _ := b.Stg.Route().GetDriversByRoute(context.Background(), fromID, toID)
	for _, id := range routeDrivers {
		routeDriversMap[id] = true
	}

	targetIDs := make(map[int64]bool)
	users, _ := b.Stg.User().GetAll(context.Background())

	b.Log.Info("notifyDrivers: Starting driver notification",
		logger.Int64("orderID", orderID),
		logger.Int64("fromID", fromID),
		logger.Int64("toID", toID),
		logger.Int64("tariffID", tariffID),
	)

	for _, u := range users {
		if u.Role != "driver" || u.Status != "active" {
			b.Log.Info("notifyDrivers: Skipping non-active or non-driver user",
				logger.Int64("user_id", u.ID),
				logger.String("role", u.Role),
				logger.String("status", u.Status),
			)
			continue
		}

		// Check tariff first
		// FIXED: If driver has selected tariffs, check if this one is enabled
		// If driver has NO tariffs selected (empty), include by default
		enabled, _ := b.Stg.Tariff().GetEnabled(context.Background(), u.ID)
		if len(enabled) > 0 && !enabled[tariffID] {
			b.Log.Info("notifyDrivers: Driver tariff not enabled",
				logger.Int64("driver_id", u.ID),
				logger.Int64("tariffID", tariffID),
			)
			continue
		}

		// Route Logic:
		// 1. If driver matches route -> Notify
		// 2. If driver has NO routes at all -> Notify (Default)
		// 3. If driver has routes but doesn't match -> Skip

		if routeDriversMap[u.ID] {
			b.Log.Info("notifyDrivers: Driver matches route",
				logger.Int64("driver_id", u.ID),
				logger.Int64("fromID", fromID),
				logger.Int64("toID", toID),
			)
			targetIDs[u.ID] = true
			continue
		}

		// Check if driver has any routes
		driverRoutes, _ := b.Stg.Route().GetDriverRoutes(context.Background(), u.ID)
		if len(driverRoutes) == 0 {
			b.Log.Info("notifyDrivers: Driver has no routes set (default include)",
				logger.Int64("driver_id", u.ID),
			)
			targetIDs[u.ID] = true
		} else {
			b.Log.Info("notifyDrivers: Driver route doesn't match",
				logger.Int64("driver_id", u.ID),
				logger.Int64("driver_routes_count", int64(len(driverRoutes))),
			)
		}
	}

	b.Log.Info("notifyDrivers: Target drivers count",
		logger.Int64("count", int64(len(targetIDs))),
	)

	menu := &tele.ReplyMarkup{}
	menu.Inline(menu.Row(
		menu.Data("📥 Принять заказ", fmt.Sprintf("take_%d", orderID)),
		menu.Data("❌ Закрыть", "close_msg"),
	))

	// Build user map from already fetched users data
	userMap := make(map[int64]int64)
	for _, u := range users {
		userMap[u.ID] = u.TelegramID
	}

	// Send notifications using pre-fetched data instead of DB query in loop
	sentCount := 0
	for id := range targetIDs {
		if teleID, ok := userMap[id]; ok && teleID != 0 {
			_, err := target.Bot.Send(&tele.User{ID: teleID}, text, menu, tele.ModeHTML)
			if err != nil {
				b.Log.Error("Failed to send notification to driver",
					logger.Int64("driver_id", id),
					logger.Int64("telegram_id", teleID),
					logger.Error(err),
				)
			} else {
				sentCount++
				b.Log.Info("Notification sent to driver",
					logger.Int64("driver_id", id),
					logger.Int64("telegram_id", teleID),
				)
			}
		}
	}
	b.Log.Info("notifyDrivers: Notifications sent",
		logger.Int64("orderID", orderID),
		logger.Int64("count", int64(sentCount)),
	)
}

func (b *Bot) handleMyOrders(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]
	}

	orders, _ := b.Stg.Order().GetClientOrders(context.Background(), session.DBID)
	if len(orders) == 0 {
		return c.Send("У вас нет заказов.")
	}
	for _, o := range orders {
		timeStr := "Неизвестно"
		if o.PickupTime != nil {
			loc := time.FixedZone("Europe/Moscow", 3*60*60)
			timeStr = o.PickupTime.In(loc).Format("02.01.2006 15:04")
		}

		txt := fmt.Sprintf("📦 <b>Заказ #%d</b>\n📍 %s ➡️ %s\n👥 Пассажиры: %d\n📅 Время: %s\n📊 Статус: %s",
			o.ID, o.FromLocationName, o.ToLocationName, o.Passengers, timeStr, o.Status)

		menu := &tele.ReplyMarkup{}
		if o.Status == "active" || o.Status == "pending" || o.Status == "wait_confirm" || o.Status == "taken" || o.Status == "on_way" {
			menu.Inline(menu.Row(menu.Data("❌ Отменить", fmt.Sprintf("cancel_%d", o.ID))))
		}
		c.Send(txt, menu, tele.ModeHTML)
	}
	return nil
}

func (b *Bot) handleAdminBackToMenu(c tele.Context) error {
	if b.Type != BotTypeAdmin {
		return nil
	}
	session := b.Sessions[c.Sender().ID]
	if session != nil {
		session.State = StateIdle
	}
	user := b.getCurrentUser(c)
	if user == nil {
		return c.Send("❌ Нажмите /start")
	}
	return b.showMenu(c, user)
}

func (b *Bot) handleTariffAddStart(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		if user == nil {
			return c.Send("❌ Нажмите /start")
		}
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]
	}
	session.State = StateTariffAdd
	return c.Send("✏️ Введите название тарифа (или нажмите <b>⬅️ Назад в меню</b> для отмены):", tele.ModeHTML)
}

func (b *Bot) handleLocationAddStart(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		if user == nil {
			return c.Send("❌ Нажмите /start")
		}
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]
	}
	session.State = StateLocationAdd
	return c.Send("✏️ Введите название города/района (или нажмите <b>⬅️ Назад в меню</b> для отмены):", tele.ModeHTML)
}

func (b *Bot) handleAdminCars(c tele.Context) error {
	if b.Type != BotTypeAdmin {
		return nil
	}
	ctx := context.Background()
	adm, _ := b.Stg.User().Get(ctx, c.Sender().ID)
	if adm == nil || adm.Role != "admin" {
		return nil
	}
	brands, _ := b.Stg.Car().GetBrands(ctx)
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	menu.Reply(
		menu.Row(menu.Text("➕ Добавить марку"), menu.Text("➕ Добавить модель")),
		menu.Row(menu.Text("🗑 Удалить марку"), menu.Text("🗑 Удалить модель")),
		menu.Row(menu.Text("⬅️ Назад в меню")),
	)
	var msg strings.Builder
	msg.WriteString("🚗 <b>Марки и модели автомобилей</b>\n\n")
	for _, br := range brands {
		models, _ := b.Stg.Car().GetModels(ctx, br.ID)
		msg.WriteString(fmt.Sprintf("• <b>%s</b> (ID: %d): ", br.Name, br.ID))
		for i, m := range models {
			if i > 0 {
				msg.WriteString(", ")
			}
			msg.WriteString(m.Name)
		}
		msg.WriteString("\n")
	}
	return c.Send(msg.String(), menu, tele.ModeHTML)
}

func (b *Bot) handleCarBrandAddStart(c tele.Context) error {
	if b.Type != BotTypeAdmin {
		return nil
	}
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		if user == nil {
			return c.Send("❌ Нажмите /start")
		}
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]
	}
	session.State = StateCarBrandAdd
	return c.Send("✏️ Введите название марки автомобиля (или <b>⬅️ Назад в меню</b> для отмены):", tele.ModeHTML)
}

func (b *Bot) handleCarModelAddStart(c tele.Context) error {
	if b.Type != BotTypeAdmin {
		return nil
	}
	ctx := context.Background()
	brands, err := b.Stg.Car().GetBrands(ctx)
	if err != nil || len(brands) == 0 {
		return c.Send("❌ Сначала добавьте хотя бы одну марку.")
	}
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, br := range brands {
		rows = append(rows, menu.Row(menu.Data(br.Name, fmt.Sprintf("car_addmodel_%d", br.ID))))
	}
	rows = append(rows, menu.Row(menu.Data("⬅️ Отмена", "car_addmodel_cancel")))
	menu.Inline(rows...)
	return c.Send("Выберите марку для новой модели:", menu)
}

func (b *Bot) handleCarBrandDeleteStart(c tele.Context) error {
	if b.Type != BotTypeAdmin {
		return nil
	}
	ctx := context.Background()
	brands, err := b.Stg.Car().GetBrands(ctx)
	if err != nil || len(brands) == 0 {
		return c.Send("❌ Нет марок для удаления.")
	}
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, br := range brands {
		rows = append(rows, menu.Row(menu.Data(fmt.Sprintf("🗑 %s", br.Name), fmt.Sprintf("adm_del_brand_%d", br.ID))))
	}
	menu.Inline(rows...)
	return c.Send("🗑 <b>Выберите марку для удаления:</b>\n<i>Все модели марки тоже будут удалены.</i>", menu, tele.ModeHTML)
}

func (b *Bot) handleCarModelDeleteStart(c tele.Context) error {
	if b.Type != BotTypeAdmin {
		return nil
	}
	ctx := context.Background()
	brands, err := b.Stg.Car().GetBrands(ctx)
	if err != nil || len(brands) == 0 {
		return c.Send("❌ Нет марок.")
	}
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, br := range brands {
		rows = append(rows, menu.Row(menu.Data(br.Name, fmt.Sprintf("adm_sel_brand_del_%d", br.ID))))
	}
	menu.Inline(rows...)
	return c.Send("🗑 <b>Выберите марку, из которой удалить модель:</b>", menu, tele.ModeHTML)
}

func (b *Bot) handleAdminBlocked(c tele.Context) error {
	if b.Type != BotTypeAdmin {
		return nil
	}
	ctx := context.Background()
	adm, _ := b.Stg.User().Get(ctx, c.Sender().ID)
	if adm == nil || adm.Role != "admin" {
		return nil
	}
	users, err := b.Stg.User().GetBlockedUsers(ctx)
	if err != nil {
		return c.Send("❌ Ошибка при получении списка.")
	}
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	menu.Reply(menu.Row(menu.Text("⬅️ Назад в меню")))

	if len(users) == 0 {
		return c.Send("🚫 <b>Заблокированные пользователи</b>\n\nНет заблокированных. Блокировка: в разделе «Пользователи» нажмите 🚫/✅ для пользователя.", menu, tele.ModeHTML)
	}
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("🚫 <b>Заблокированные</b> (всего %d)\n\n", len(users)))
	for _, u := range users {
		phone := "—"
		if u.Phone != nil && *u.Phone != "" {
			phone = *u.Phone
		}
		msg.WriteString(fmt.Sprintf("🆔 %d | %s\n📞 %s | Роль: %s\n", u.TelegramID, u.FullName, phone, u.Role))
		msg.WriteString("------------------------------\n")
	}
	// Inline: Разблокировать for each (by DB id for UpdateStatusByID)
	inline := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, u := range users {
		rows = append(rows, inline.Row(inline.Data("✅ Разблокировать "+u.FullName, fmt.Sprintf("unblock_%d", u.ID))))
	}
	rows = append(rows, inline.Row(inline.Data("⬅️ Назад", "admin_back")))
	inline.Inline(rows...)
	return c.Send(msg.String(), inline, tele.ModeHTML)
}

func (b *Bot) generateCalendar(c tele.Context, year, month int) error {
	return b.generateCalendarWithPrefix(c, year, month, "cal_")
}

func (b *Bot) generateCalendarWithPrefix(c tele.Context, year, month int, prefix string) error {
	// Month names in Russian
	monthNames := []string{"", "Январь", "Февраль", "Март", "Апрель", "Май", "Июнь",
		"Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь"}

	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	// Header with month and year
	header := fmt.Sprintf("📅 %s %d", monthNames[month], year)

	// Week day names
	rows = append(rows, menu.Row(
		menu.Data("Пн", "ignore"), menu.Data("Вт", "ignore"), menu.Data("Ср", "ignore"),
		menu.Data("Чт", "ignore"), menu.Data("Пт", "ignore"), menu.Data("Сб", "ignore"), menu.Data("Вс", "ignore"),
	))

	// Get first day of week (0 = Sunday, 1 = Monday, etc.)
	firstWeekday := int(firstDay.Weekday())
	if firstWeekday == 0 {
		firstWeekday = 7 // Sunday becomes 7
	}
	firstWeekday-- // Adjust to Monday = 0

	var currentRow []tele.Btn

	// Add empty cells before first day
	for i := 0; i < firstWeekday; i++ {
		currentRow = append(currentRow, menu.Data(" ", "ignore"))
	}

	// Use fixed zone for Moscow (UTC+3)
	loc := time.FixedZone("Europe/Moscow", 3*60*60)
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Add all days of the month
	for day := 1; day <= lastDay.Day(); day++ {
		dayDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

		// Disable past dates
		// If dayDate is BEFORE today, it's past.
		// If dayDate EQUALS today, it's active.
		btnText := fmt.Sprintf("%d", day)
		var btn tele.Btn

		if dayDate.Before(today) {
			btn = menu.Data(btnText, "ignore")
		} else {
			btn = menu.Data(btnText, fmt.Sprintf("%s%s", prefix, dayDate.Format("2006-01-02")))
		}
		currentRow = append(currentRow, btn)

		if len(currentRow) == 7 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = []tele.Btn{}
		}
	}

	// Fill remaining cells
	if len(currentRow) > 0 {
		for len(currentRow) < 7 {
			currentRow = append(currentRow, menu.Data(" ", "ignore"))
		}
		rows = append(rows, menu.Row(currentRow...))
	}

	// Navigation buttons
	prevMonth := month - 1
	prevYear := year
	if prevMonth < 1 {
		prevMonth = 12
		prevYear--
	}

	nextMonth := month + 1
	nextYear := year
	if nextMonth > 12 {
		nextMonth = 1
		nextYear++
	}

	// Only show navigation if not too far in past/future
	var navRow []tele.Btn
	navPrefix := "nav_"
	if prefix == "sc_cal_" {
		navPrefix = "sc_nav_"
	}

	if prevYear > time.Now().Year()-1 || (prevYear == time.Now().Year()-1 && prevMonth >= int(time.Now().Month())) {
		navRow = append(navRow, menu.Data(fmt.Sprintf("⬅️ %s", monthNames[prevMonth]), fmt.Sprintf("%s%d_%d", navPrefix, prevYear, prevMonth)))
	}
	if nextYear < time.Now().Year()+2 {
		navRow = append(navRow, menu.Data(fmt.Sprintf("%s ➡️", monthNames[nextMonth]), fmt.Sprintf("%s%d_%d", navPrefix, nextYear, nextMonth)))
	}

	if len(navRow) > 0 {
		rows = append(rows, menu.Row(navRow...))
	}

	menu.Inline(rows...)
	if c.Callback() != nil {
		c.Respond(&tele.CallbackResponse{})
		return c.Edit(header, menu)
	}
	return c.Send(header, menu)
}

func (b *Bot) getCurrentUser(c tele.Context) *models.User {
	u, err := b.Stg.User().Get(context.Background(), c.Sender().ID)
	if err == nil {
		return u
	}
	// If user not found, create (recover from DB wipe or new joiner clicking old button)
	sender := c.Sender()
	u, err = b.Stg.User().GetOrCreate(context.Background(), sender.ID, sender.Username, fmt.Sprintf("%s %s", sender.FirstName, sender.LastName))
	if err != nil {
		b.Log.Error("Failed to get or create user", logger.Error(err))
		return nil
	}
	return u
}

func (b *Bot) handleAdminPendingDrivers(c tele.Context) error {
	ctx := context.Background()
	adm, _ := b.Stg.User().Get(ctx, c.Sender().ID)
	if adm == nil || adm.Role != "admin" {
		return nil
	}
	drivers, err := b.Stg.User().GetPendingDrivers(ctx)
	if err != nil {
		b.Log.Error("Failed to get pending drivers", logger.Error(err))
		return c.Send("❌ Ошибка при получении списка водителей.")
	}

	if len(drivers) == 0 {
		return c.Send("📭 Нет водителей, ожидающих проверки.")
	}

	ru := messages["ru"]
	for _, d := range drivers {
		profile, _ := b.Stg.User().GetDriverProfile(ctx, d.ID)
		carInfo := "Нет данных"
		if profile != nil {
			carInfo = fmt.Sprintf("🚗 %s %s (%s)", profile.CarBrand, profile.CarModel, profile.LicensePlate)
		}

		routes, _ := b.Stg.Route().GetDriverRoutes(ctx, d.ID)
		routesStr := ""
		for i, r := range routes {
			from, _ := b.Stg.Location().GetByID(ctx, r[0])
			to, _ := b.Stg.Location().GetByID(ctx, r[1])
			fromName, toName := "?", "?"
			if from != nil {
				fromName = from.Name
			}
			if to != nil {
				toName = to.Name
			}
			routesStr += fmt.Sprintf("\n📍 %d. %s ➡️ %s", i+1, fromName, toName)
		}

		enabledTariffs, _ := b.Stg.Tariff().GetEnabled(ctx, d.ID)
		tariffsStr := ""
		allTariffs, _ := b.Stg.Tariff().GetAll(ctx)
		for _, t := range allTariffs {
			if enabledTariffs[t.ID] {
				tariffsStr += fmt.Sprintf("%s, ", t.Name)
			}
		}
		if len(tariffsStr) > 2 {
			tariffsStr = tariffsStr[:len(tariffsStr)-2]
		}

		moscowLoc := time.FixedZone("Europe/Moscow", 3*60*60)
		msg := fmt.Sprintf("👤 <b>Водитель:</b> %s\n📞 Телефон: %s\n🆔 Telegram ID: %d\n📅 Дата регистрации: %s\n\n%s\n\n🛣 <b>Маршруты:</b>%s\n\n💰 <b>Тарифы:</b> %s",
			d.FullName, *d.Phone, d.TelegramID, d.CreatedAt.In(moscowLoc).Format("02.01.2006 15:04"), carInfo, routesStr, tariffsStr)

		menu := &tele.ReplyMarkup{}
		menu.Inline(
			menu.Row(
				menu.Data(ru["admin_btn_approve"], fmt.Sprintf("approve_driver_%d", d.ID)),
				menu.Data(ru["admin_btn_reject"], fmt.Sprintf("reject_driver_%d", d.ID)),
			),
			menu.Row(menu.Data(ru["admin_btn_block"], fmt.Sprintf("block_driver_%d", d.ID))),
		)
		c.Send(msg, menu, tele.ModeHTML)
	}
	return nil
}

func (b *Bot) handleAdminActiveDrivers(c tele.Context) error {
	ctx := context.Background()
	adm, _ := b.Stg.User().Get(ctx, c.Sender().ID)
	if adm == nil || adm.Role != "admin" {
		return nil
	}
	drivers, err := b.Stg.User().GetActiveDrivers(ctx)
	if err != nil {
		b.Log.Error("Failed to get active drivers", logger.Error(err))
		return c.Send("❌ Ошибка при получении списка водителей.")
	}

	if len(drivers) == 0 {
		return c.Send("📭 Активных водителей пока нет.")
	}

	ru := messages["ru"]
	for _, d := range drivers {
		profile, _ := b.Stg.User().GetDriverProfile(ctx, d.ID)
		carInfo := "Нет данных"
		if profile != nil {
			carInfo = fmt.Sprintf("🚗 %s %s (%s)", profile.CarBrand, profile.CarModel, profile.LicensePlate)
		}

		routes, _ := b.Stg.Route().GetDriverRoutes(ctx, d.ID)
		routesStr := ""
		for i, r := range routes {
			from, _ := b.Stg.Location().GetByID(ctx, r[0])
			to, _ := b.Stg.Location().GetByID(ctx, r[1])
			fromName, toName := "?", "?"
			if from != nil {
				fromName = from.Name
			}
			if to != nil {
				toName = to.Name
			}
			routesStr += fmt.Sprintf("\n📍 %d. %s ➡️ %s", i+1, fromName, toName)
		}

		enabledTariffs, _ := b.Stg.Tariff().GetEnabled(ctx, d.ID)
		tariffsStr := ""
		allTariffs, _ := b.Stg.Tariff().GetAll(ctx)
		for _, t := range allTariffs {
			if enabledTariffs[t.ID] {
				tariffsStr += fmt.Sprintf("%s, ", t.Name)
			}
		}
		if len(tariffsStr) > 2 {
			tariffsStr = tariffsStr[:len(tariffsStr)-2]
		}

		moscowLoc := time.FixedZone("Europe/Moscow", 3*60*60)
		msg := fmt.Sprintf("👤 <b>Водитель:</b> %s\n📞 Телефон: %s\n🆔 Telegram ID: %d\n📅 Дата регистрации: %s\n\n%s\n\n🛣 <b>Маршруты:</b>%s\n\n💰 <b>Тарифы:</b> %s",
			d.FullName, *d.Phone, d.TelegramID, d.CreatedAt.In(moscowLoc).Format("02.01.2006 15:04"), carInfo, routesStr, tariffsStr)

		menu := &tele.ReplyMarkup{}
		menu.Inline(
			menu.Row(
				menu.Data(ru["admin_btn_block"], fmt.Sprintf("block_driver_%d", d.ID)),
			),
		)
		c.Send(msg, menu, tele.ModeHTML)
	}
	return nil
}

func (b *Bot) handleAdminPendingOrders(c tele.Context) error {
	ctx := context.Background()
	adm, _ := b.Stg.User().Get(ctx, c.Sender().ID)
	if adm == nil || adm.Role != "admin" {
		return nil
	}
	orders, err := b.Stg.Order().GetPendingOrders(ctx)
	if err != nil {
		b.Log.Error("Failed to get pending orders", logger.Error(err))
		return c.Send("❌ Ошибка при получении списка заказов.")
	}

	if len(orders) == 0 {
		return c.Send("📭 Нет заказов, ожидающих подтверждения.")
	}

	ru := messages["ru"]
	for _, o := range orders {
		total, completed, cancelled, _ := b.Stg.Order().GetClientStats(ctx, o.ClientID)
		tariff, _ := b.Stg.Tariff().GetByID(ctx, o.TariffID)
		tariffName := "Неизвестно"
		if tariff != nil {
			tariffName = tariff.Name
		}
		pickupTimeStr := "Не указано"
		if o.PickupTime != nil {
			pickupTimeStr = o.PickupTime.In(time.FixedZone("Europe/Moscow", 3*60*60)).Format("02.01.2006 15:04")
		}

		clientDisplay := o.ClientUsername
		if clientDisplay == "" {
			clientDisplay = "Неизвестно"
		}
		msg := fmt.Sprintf("📦 <b>Заказ #%d</b>\n\n👤 Клиент: @%s\n📞 Телефон: %s\n📊 История: Всего %d | ✅ %d | ❌ %d\n\n📍 Маршрут: %s ➡️ %s\n🚕 Тариф: %s\n👥 Пассажиры: %d\n💰 Цена: %d %s\n📅 Время: %s",
			o.ID, clientDisplay, o.ClientPhone, total, completed, cancelled,
			o.FromLocationName, o.ToLocationName, tariffName, o.Passengers, o.Price, o.Currency, pickupTimeStr)

		menu := &tele.ReplyMarkup{}
		menu.Inline(
			menu.Row(
				menu.Data(ru["admin_btn_confirm_order"], fmt.Sprintf("approve_order_%d", o.ID)),
				menu.Data(ru["admin_btn_reject_order"], fmt.Sprintf("reject_order_%d", o.ID)),
			),
			menu.Row(menu.Data(ru["admin_btn_block_client"], fmt.Sprintf("block_user_%d", o.ClientID))),
		)
		c.Send(msg, menu, tele.ModeHTML)
	}
	return nil
}

func (b *Bot) handleAdminStats(c tele.Context) error {
	ctx := context.Background()
	adm, _ := b.Stg.User().Get(ctx, c.Sender().ID)
	if adm == nil || adm.Role != "admin" {
		return nil
	}
	totalUsers, _ := b.Stg.User().GetTotalUsers(ctx)
	totalDrivers, _ := b.Stg.User().GetTotalDrivers(ctx)
	activeOrders, _ := b.Stg.Order().GetActiveOrdersCount(ctx)
	totalOrders, _ := b.Stg.Order().GetTotalOrdersCount(ctx)
	dailyOrders, _ := b.Stg.Order().GetDailyOrderCount(ctx)
	cancelRate, _ := b.Stg.Order().GetGlobalCancelRate(ctx)

	msg := fmt.Sprintf("📊 <b>Статистика сервиса</b>\n\n👤 Всего пользователей: <b>%d</b>\n🚖 Водителей: <b>%d</b>\n\n📦 Активных заказов: <b>%d</b>\n📦 Всего заказов: <b>%d</b>\n📅 Заказов сегодня: <b>%d</b>\n📉 Процент отмен: <b>%.2f%%</b>",
		totalUsers, totalDrivers, activeOrders, totalOrders, dailyOrders, cancelRate)

	return c.Send(msg, tele.ModeHTML)
}
