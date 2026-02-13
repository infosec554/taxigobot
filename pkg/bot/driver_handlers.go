package bot

import (
	"context"
	"fmt"
	"taxibot/pkg/models"
	"time"

	tele "gopkg.in/telebot.v3"
)

func (b *Bot) handleDriverTariffs(c tele.Context) error {
	user := b.getCurrentUser(c)
	tariffs, _ := b.Stg.Tariff().GetAll(context.Background())
	enabled, _ := b.Stg.Tariff().GetEnabled(context.Background(), user.ID)

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	var currentRow []tele.Btn
	for i, t := range tariffs {
		icon := "🔴"
		if enabled[t.ID] {
			icon = "✅"
		}
		currentRow = append(currentRow, menu.Data(fmt.Sprintf("%s %s", icon, t.Name), fmt.Sprintf("tgl_%d", t.ID)))
		if (i+1)%2 == 0 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = []tele.Btn{}
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}
	menu.Inline(rows...)
	return c.Send("<b>🚕 Tariflarim</b>\n\nQaysi tariflardan buyurtma olmoqchisiz? Tanlang:", menu, tele.ModeHTML)
}

func (b *Bot) handleDriverRoutes(c tele.Context) error {
	user := b.getCurrentUser(c)
	routes, _ := b.Stg.Route().GetDriverRoutes(context.Background(), user.ID)

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	txt := "<b>📍 Yo'nalishlarim</b>\n\n"
	if len(routes) == 0 {
		txt += "Hozircha hech qanday yo'nalish qo'shmagansiz."
	} else {
		for i, r := range routes {
			from, _ := b.Stg.Location().GetByID(context.Background(), r[0])
			to, _ := b.Stg.Location().GetByID(context.Background(), r[1])
			fromName, toName := "Noma'lum", "Noma'lum"
			if from != nil {
				fromName = from.Name
			}
			if to != nil {
				toName = to.Name
			}
			txt += fmt.Sprintf("%d. %s ➡️ %s\n", i+1, fromName, toName)
		}
	}

	rows = append(rows, menu.Row(menu.Data("➕ Yangi qo'shish", "add_route")))
	if len(routes) > 0 {
		rows = append(rows, menu.Row(menu.Data("🗑 Tozalash", "clear_routes")))
	}

	menu.Inline(rows...)
	return c.Send(txt, menu, tele.ModeHTML)
}

func (b *Bot) handleAddRouteStart(c tele.Context, session *UserSession) error {
	session.State = StateDriverRouteFrom
	locations, _ := b.Stg.Location().GetAll(context.Background())

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	var currentRow []tele.Btn
	for i, l := range locations {
		currentRow = append(currentRow, menu.Data(l.Name, fmt.Sprintf("dr_f_%d", l.ID)))
		if (i+1)%3 == 0 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = []tele.Btn{}
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}
	menu.Inline(rows...)

	msg := "<b>📍 Qayerdan ketasiz?</b>\nShaharni tanlang:"
	if c.Callback() != nil {
		return c.Edit(msg, menu, tele.ModeHTML)
	}
	return c.Send(msg, menu, tele.ModeHTML)
}

func (b *Bot) handleDriverCalendarSearch(c tele.Context) error {
	now := time.Now()
	return b.generateCalendarWithPrefix(c, now.Year(), int(now.Month()), "sc_cal_")
}

func (b *Bot) handleDriverAgenda(c tele.Context) error {
	orders, _ := b.Stg.Order().GetActiveOrders(context.Background())
	if len(orders) == 0 {
		return c.Send("Hozircha faol zakazlar yo'q.")
	}

	// Group by date
	groups := make(map[string][]models.Order)
	var dates []string
	for _, o := range orders {
		if o.PickupTime == nil {
			continue
		}
		d := o.PickupTime.Format("2006-01-02")
		if _, ok := groups[d]; !ok {
			dates = append(dates, d)
		}
		groups[d] = append(groups[d], *o)
	}

	txt := "<b>📋 Haftalik zakazlar ro'yxat:</b>\n\n"
	for _, d := range dates {
		parsedDate, _ := time.Parse("2006-01-02", d)
		txt += fmt.Sprintf("📅 <b>%s</b>\n", parsedDate.Format("02.01.2006"))
		for _, o := range groups[d] {
			txt += fmt.Sprintf("▫️ %s: <b>%s ➞ %s</b> (#%d)\n",
				o.PickupTime.Format("15:04"), o.FromLocationName, o.ToLocationName, o.ID)
		}
		txt += "\n"
	}

	txt += "<i>Batafsil ma'lumot uchun zakaz ID raqamini ko'ring.</i>"
	return c.Send(txt, tele.ModeHTML)
}

func (b *Bot) handleDriverDateSearch(c tele.Context, dateStr string) error {
	return b.driverDateSearchLogic(c, dateStr, false)
}

func (b *Bot) handleDriverDateSearchAll(c tele.Context, dateStr string) error {
	return b.driverDateSearchLogic(c, dateStr, true)
}

func (b *Bot) driverDateSearchLogic(c tele.Context, dateStr string, showAll bool) error {
	user := b.getCurrentUser(c)
	orders, _ := b.Stg.Order().GetActiveOrders(context.Background())
	driverRoutes, _ := b.Stg.Route().GetDriverRoutes(context.Background(), user.ID)

	// Convert routes to map for easy lookup: strings "FromID-ToID"
	routeMap := make(map[string]bool)
	for _, r := range driverRoutes {
		routeMap[fmt.Sprintf("%d-%d", r[0], r[1])] = true
	}
	hasRoutes := len(driverRoutes) > 0

	foundAnyDate := false
	foundForRoute := false

	for _, o := range orders {
		loc := time.FixedZone("Europe/Moscow", 3*60*60)
		if o.PickupTime != nil && o.PickupTime.In(loc).Format("2006-01-02") == dateStr {
			foundAnyDate = true

			// Filter by route if driver has routes and NOT showing all
			if hasRoutes && !showAll {
				key := fmt.Sprintf("%d-%d", o.FromLocationID, o.ToLocationID)
				if !routeMap[key] {
					continue
				}
			}
			foundForRoute = true

			txt := fmt.Sprintf("📦 <b>ZAKAZ #%d</b>\n📍 %s ➡️ %s\n👥 %d kishi\n🕒 %s\n💰 %d %s",
				o.ID, o.FromLocationName, o.ToLocationName, o.Passengers, o.PickupTime.In(loc).Format("15:04"), o.Price, o.Currency)

			btnMenu := &tele.ReplyMarkup{}
			btnMenu.Inline(btnMenu.Row(
				btnMenu.Data("📥 Zakazni olish", fmt.Sprintf("take_%d", o.ID)),
				btnMenu.Data("❌ Yopish", "close_msg"),
			))
			c.Send(txt, btnMenu, tele.ModeHTML)
		}
	}

	if !foundAnyDate {
		return c.Send(fmt.Sprintf("📅 <b>%s</b> sanasida hech qanday faol zakaz topilmadi.", dateStr), tele.ModeHTML)
	}

	if hasRoutes && !foundForRoute && !showAll {
		// Found orders but not for this route. Offer to show all.
		menu := &tele.ReplyMarkup{}
		menu.Inline(menu.Row(menu.Data("🔄 Barchasini ko'rsatish", fmt.Sprintf("sc_cal_all_%s", dateStr))))
		return c.Send(fmt.Sprintf("📅 <b>%s</b> sanasida sizning yo'nalishingiz bo'yicha zakazlar topilmadi.", dateStr), menu, tele.ModeHTML)
	}

	return nil
}
