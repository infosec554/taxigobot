-- Up Migration
CREATE TYPE user_role AS ENUM ('admin', 'driver', 'client');
CREATE TYPE user_status AS ENUM ('active', 'blocked', 'pending');
CREATE TYPE order_status AS ENUM ('active', 'taken', 'completed', 'cancelled');

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT UNIQUE NOT NULL,
    username VARCHAR(255),
    full_name VARCHAR(255),
    phone VARCHAR(50),
    role user_role DEFAULT 'client',
    status user_status DEFAULT 'pending',
    language VARCHAR(10) DEFAULT 'uz',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tariffs (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS locations (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    client_id BIGINT REFERENCES users(id),
    driver_id BIGINT REFERENCES users(id),
    from_location_id BIGINT REFERENCES locations(id),
    to_location_id BIGINT REFERENCES locations(id),
    tariff_id BIGINT REFERENCES tariffs(id),
    price INTEGER DEFAULT 0,
    currency VARCHAR(10) DEFAULT 'RUB',
    passengers INTEGER DEFAULT 1,
    pickup_time TIMESTAMP WITH TIME ZONE,
    status order_status DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Default Tariffs
INSERT INTO tariffs (name) VALUES 
('✅ Эконом'), ('✅ Стандарт'), ('✅ Комфорт'), ('✅ Микровэн'), ('✅ Минивэн'), 
('✅ Комфорт+ / D класс'), ('✅ Бизнес / E класс'), ('✅ Микроавтобус'), ('✅ Автобус');

-- Default Locations (Major Russian Cities)
INSERT INTO locations (name) VALUES 
('Краснодар'), ('Красноярск'), ('Ставрополь'), ('Новосибирск'), ('Калуга'), ('Саратов'),
('Челябинск'), ('Ярославль'), ('Самара'), ('Волгоград'), ('Волжский'), ('Тверь'),
('Москва'), ('Воронеж'), ('Астрахань'), ('Казань'), ('Пермь'), ('Оренбург'),
('Нижний Н'), ('Адлер Сочи'), ('Омск'), ('Иркутск'), ('Дзержинск'), ('Ростов На Дону'),
('Кемерово'), ('Ульяновск'), ('Екатеринбург'), ('СПб'), ('Чебоксары'), ('Иваново'),
('Уфа'), ('Липецк'), ('Владимир'), ('Ижевск'), ('Тольятти'), ('Тюмень'),
('Томск'), ('Орел'), ('Тула'), ('Пенза'), ('Калининград'), ('Наб. Челны'),
('Череповец'), ('Брянск'), ('Тамбов'), ('Курск'), ('Майкоп'), ('Новороссийск'),
('Смоленск'), ('Барнаул'), ('Хабаровск'), ('Владикавказ'), ('Первоуральск');
