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
    role user_role DEFAULT 'client', -- MD bo'yicha default 'client'
    status user_status DEFAULT 'pending', -- MD bo'yicha default 'pending'
    language VARCHAR(10) DEFAULT 'uz',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tariffs (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS directions (
    id BIGSERIAL PRIMARY KEY,
    from_location VARCHAR(255) NOT NULL,
    to_location VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    client_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    driver_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    direction_id BIGINT REFERENCES directions(id),
    tariff_id BIGINT REFERENCES tariffs(id),
    price INTEGER DEFAULT 0,
    currency VARCHAR(10) DEFAULT 'UZS',
    passengers INTEGER DEFAULT 1,
    pickup_time TIMESTAMP WITH TIME ZONE,
    status order_status DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
