package bot

import (
	"context"
	"fmt"
	"time"

	tele "gopkg.in/telebot.v3"
)

func (b *Bot) handleDriverTariffs(c tele.Context) error {
	user := b.getCurrentUser(c)
	tariffs, _ := b.Stg.Tariff().GetAll(context.Background())
	enabled, _ := b.Stg.Tariff().GetEnabled(context.Background(), user.ID)

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, t := range tariffs {
		icon := "🔴"
		if enabled[t.ID] {
			icon = "✅"
		}
		rows = append(rows, menu.Row(menu.Data(fmt.Sprintf("%s %s", icon, t.Name), fmt.Sprintf("tgl_%d", t.ID))))
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

	return c.Edit("<b>📍 Qayerdan ketasiz?</b>\nShaharni tanlang:", menu, tele.ModeHTML)
}

func (b *Bot) handleDriverCalendarSearch(c tele.Context) error {
	now := time.Now()
	// Using the custom prefix sc_cal_ to distinguish from order creation
	return b.generateCalendarWithPrefix(c, now.Year(), int(now.Month()), "sc_cal_")
}

func (b *Bot) handleDriverDateSearch(c tele.Context, dateStr string) error {
	orders, _ := b.Stg.Order().GetActiveOrders(context.Background())

	found := false
	for _, o := range orders {
		if o.PickupTime != nil && o.PickupTime.Format("2006-01-02") == dateStr {
			found = true
			txt := fmt.Sprintf("📦 <b>ZAKAZ #%d</b>\n📍 %s ➡️ %s\n👥 %d kishi\n🕒 %s",
				o.ID, o.FromLocationName, o.ToLocationName, o.Passengers, o.PickupTime.Format("15:04"))

			btnMenu := &tele.ReplyMarkup{}
			btnMenu.Inline(btnMenu.Row(btnMenu.Data("📥 Zakazni olish", fmt.Sprintf("take_%d", o.ID))))
			c.Send(txt, btnMenu, tele.ModeHTML)
		}
	}

	if !found {
		return c.Send(fmt.Sprintf("📅 <b>%s</b> sanasida hech qanday faol zakaz topilmadi.", dateStr), tele.ModeHTML)
	}

	return nil
}
