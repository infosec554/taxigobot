package bot

import (
	"context"
	"fmt"
	"strings"
	"taxibot/pkg/logger"
	"taxibot/pkg/models"

	tele "gopkg.in/telebot.v3"
)

// licensePlateRegex is removed to support diverse plate formats (e.g. temporary, transit, non-standard)

func normalizeLicensePlate(input string) string {
	mapping := map[rune]rune{
		'A': 'А', 'B': 'В', 'E': 'Е', 'K': 'К', 'M': 'М', 'H': 'Н', 'O': 'О', 'P': 'Р', 'C': 'С', 'T': 'Т', 'Y': 'У', 'X': 'Х',
	}
	var builder strings.Builder
	for _, r := range input {
		if cyr, ok := mapping[r]; ok {
			builder.WriteRune(cyr)
		} else {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func (b *Bot) handleDriverRegistrationStart(c tele.Context) error {
	// Step 1: Car Brand
	brands, err := b.Stg.Car().GetBrands(context.Background())
	if err != nil {
		b.Log.Error("Failed to get car brands", logger.Error(err))
		return c.Send("❌ Ошибка при загрузке списка автомобилей.")
	}

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	var currentRow []tele.Btn

	for i, brand := range brands {
		currentRow = append(currentRow, menu.Data(brand.Name, fmt.Sprintf("reg_brand_%d", brand.ID)))
		if (i+1)%3 == 0 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = []tele.Btn{}
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}

	// Add "Other" option if needed, but for now stick to DB
	menu.Inline(rows...)

	session := b.Sessions[c.Sender().ID]
	session.State = StateCarBrand

	return c.Send("🚗 <b>Выберите марку вашего автомобиля:</b>", menu, tele.ModeHTML)
}

func (b *Bot) handleCarBrandSelection(c tele.Context, brandID int64) error {
	user := b.getCurrentUser(c)
	session := b.Sessions[c.Sender().ID]
	if session.DriverProfile == nil {
		session.DriverProfile = &models.DriverProfile{UserID: user.ID}
	}

	// Fetch brand name for storage (simplification, storing name)
	// Ideally we store ID, but the task says "Save into driver_profiles table" which uses strings
	// based on my memory of migration. Let's check migration 0007 content again if unsure.
	// The migration has VARCHAR(255) for car_brand.
	// So fetching name is safer.
	// Actually, I can just get it from the button text? No, safer to fetch or use ID if possible.
	// But let's fetch models first.

	modelsList, err := b.Stg.Car().GetModels(context.Background(), brandID)
	if err != nil {
		return c.Send("❌ Ошибка при загрузке моделей.")
	}

	// Hack: I need the brand name.
	// I'll cheat and fetch all brands again or use a helper,
	// or assume the user selected from the list I just showed.
	// For now, let's fetch models.

	// Store Brand ID temporarily or Name?
	// Let's store Brand Name by fetching it relative to models? No.
	// Let's just fetch the brands list again filtered? No, `GetBrands` gets all.
	// Optimally I'd have GetBrandByID.
	// I'll skip storing the name *right now* and do it at the end or use a global cache?
	// Let's just update the profile with the Brand ID? No, table has VARCHAR.
	// Okay, I will add `GetBrand` to storage or just iterate.
	// Let's assume for now I can get it.
	// Actually, I can pass it in callback? No, data limit.

	// Let's iterate over brands to find name? Costly but safe.
	brands, _ := b.Stg.Car().GetBrands(context.Background())
	var brandName string
	for _, b := range brands {
		if b.ID == brandID {
			brandName = b.Name
			break
		}
	}
	session.DriverProfile.CarBrand = brandName

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	var currentRow []tele.Btn

	for i, m := range modelsList {
		currentRow = append(currentRow, menu.Data(m.Name, fmt.Sprintf("reg_model_%d", m.ID)))
		if (i+1)%3 == 0 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = []tele.Btn{}
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}

	// Add "Other"
	rows = append(rows, menu.Row(menu.Data("🖊 Другая", "reg_model_other")))

	menu.Inline(rows...)
	session.State = StateCarModel

	return c.Edit("🚗 <b>Выберите модель автомобиля:</b>", menu, tele.ModeHTML)
}

func (b *Bot) handleCarModelSelection(c tele.Context, modelID int64) error {
	session := b.Sessions[c.Sender().ID]

	// Need to find model name again
	// Current hack: I don't have GetModelByID easily accessible without brandID context usually.
	// But `driver_handlers.go` uses `GetModels` with brandID.
	// I stored brandName but not brandID in session.
	// I should probably store BrandID in session context or just TempString.
	// Let's assume I can't easily get it without another query.
	// I'll add `GetModel(id)` to storage?
	// Or just trust the button text if I could get it (not available in telebot callback context easily).

	// Simpler approach: Select * from car_models where id = modelID.
	// I haven't added `GetModelByID` to repo.
	// I will just ask for License Plate now.
	// Wait, I need to save the model name.
	// I will use `reg_model_<name>`? No, potentially long.
	// I will add `GetModelByID` to `car_repo.go` later?
	// Or I can use a raw query here for simplicity or add to repo.

	// Let's use a workaround: The callback data contains ID.
	// I'll implement `GetModelByID` in `car_repo`? No, I need to modify interface.
	// I'll just query standard QueryRow here? No, leakage.

	// Best way: Update `car_repo` to include `GetModelByID` or `GetBrandByID`.
	// For now, I will use `TempString` to store brandID?
	// Actually, let's just create a quick helper or modify the repo.
	// Or even simpler: usage of `GetModels` for the brand we selected?
	// But I only stored `CarBrand` string.

	// Let's use `TempString` to store BrandID in Step 1.

	// Re-think:
	// stored BrandName.
	// I need ModelName.
	// I can query all brands, find brand with that name, get its ID, get models, find model.
	// A bit convoluted but works without changing interface.

	// Optimized: Modify `car_repo` to have `GetModel(id)`.
	// I'll do that in a separate step if needed.
	// For now, let's assume I can get it.
	// I will execute a direct query via pool if allowed? No.

	// Let's use the convoluted way for safety:
	// Find brand ID by name (which I have).
	brands, _ := b.Stg.Car().GetBrands(context.Background())
	var brandID int64
	for _, br := range brands {
		if br.Name == session.DriverProfile.CarBrand {
			brandID = br.ID
			break
		}
	}

	modelsList, _ := b.Stg.Car().GetModels(context.Background(), brandID)
	for _, m := range modelsList {
		if m.ID == modelID {
			session.DriverProfile.CarModel = m.Name
			break
		}
	}

	session.State = StateLicensePlate
	return c.Edit("🔢 <b>Введите гос. номер автомобиля:</b>\n\nПример: <code>A123BC777</code>\n<i>(Можно использовать любой удобный формат)</i>", tele.ModeHTML)
}

func (b *Bot) handleCarModelOther(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	session.State = StateCarModelOther
	return c.Edit("🖊 <b>Введите модель автомобиля вручную:</b>", tele.ModeHTML)
}

func (b *Bot) handleLicensePlateInput(c tele.Context) error {
	plate := strings.ToUpper(strings.TrimSpace(c.Text()))
	plate = normalizeLicensePlate(plate)

	// Validation removed as requested to support all formats
	if len(plate) < 2 {
		return c.Send("❌ <b>Слишком короткий номер!</b>\nПожалуйста, введите корректный номер автомобиля.", tele.ModeHTML)
	}

	session := b.Sessions[c.Sender().ID]
	session.DriverProfile.LicensePlate = plate

	// Save Profile
	if err := b.Stg.User().CreateDriverProfile(context.Background(), session.DriverProfile); err != nil {
		b.Log.Error("Failed to create driver profile", logger.Error(err))
		return c.Send("❌ Ошибка при сохранении данных.")
	}

	// Move to Routes
	session.State = StateIdle // Reset state for route handler

	c.Send("✅ Данные автомобиля сохранены!")

	// Start Route setup
	return b.handleAddRouteStart(c, session)
}

func (b *Bot) handleRegistrationCheck(c tele.Context) error {
	// Check if user has car profile, routes and tariffs
	user := b.getCurrentUser(c)
	ctx := context.Background()

	// 1. Avtomobil profili tekshiruvi
	profile, _ := b.Stg.User().GetDriverProfile(ctx, user.ID)
	if profile == nil || profile.LicensePlate == "" {
		return c.Send("⚠️ <b>Необходимо заполнить данные автомобиля!</b>\n\nНажмите /start чтобы начать заново.", tele.ModeHTML)
	}

	// 2. Marshrut tekshiruvi
	routes, _ := b.Stg.Route().GetDriverRoutes(ctx, user.ID)
	if len(routes) == 0 {
		return c.Send("⚠️ <b>Необходимо добавить хотя бы один маршрут!</b>", tele.ModeHTML)
	}

	// 3. Tarif tekshiruvi
	enabledTariffs, _ := b.Stg.Tariff().GetEnabled(ctx, user.ID)
	hasTariff := false
	for _, v := range enabledTariffs {
		if v {
			hasTariff = true
			break
		}
	}
	if !hasTariff {
		return c.Send("⚠️ <b>Необходимо выбрать хотя бы один тариф!</b>", tele.ModeHTML)
	}

	// Check if already pending to avoid spam
	if user.Status == "pending_review" {
		return c.Send("⏳ <b>Ваш профиль уже находится на проверке.</b>\nПожалуйста, дождитесь решения администратора.", tele.ModeHTML)
	}

	// Submit for review
	b.Stg.User().UpdateStatusByID(ctx, user.ID, "pending_review")

	c.Send("🎉 <b>Регистрация завершена!</b>\n\nВаш профиль отправлен на проверку администратору. Ожидайте уведомления.", tele.ModeHTML)

	// Notify Admin (orderID parameter is repurposed for user.ID here)
	carDetails := fmt.Sprintf("🚗 %s %s (%s)", profile.CarBrand, profile.CarModel, profile.LicensePlate)

	tariffCount := 0
	for _, v := range enabledTariffs {
		if v {
			tariffCount++
		}
	}

	phone := "Не указан"
	if user.Phone != nil {
		phone = *user.Phone
	}

	msg := fmt.Sprintf("🔔 <b>НОВЫЙ ВОДИТЕЛЬ НА ПРОВЕРКЕ</b>\n\n👤 %s\n📞 %s\n%s\n\n📍 Маршрутов: %d\n🚕 Тарифov: %d",
		user.FullName, phone, carDetails, len(routes), tariffCount)

	// Notify Admin (orderID parameter is repurposed for user.ID here)
	b.notifyAdmin(user.ID, msg, "registration")

	return nil
}
