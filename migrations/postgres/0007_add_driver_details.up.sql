-- Up Migration
CREATE TABLE IF NOT EXISTS car_brands (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS car_models (
    id BIGSERIAL PRIMARY KEY,
    brand_id BIGINT REFERENCES car_brands(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS driver_profiles (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    car_brand VARCHAR(255),
    car_model VARCHAR(255),
    license_plate VARCHAR(20)
);

ALTER TABLE orders ADD COLUMN IF NOT EXISTS accepted_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS on_way_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS arrived_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS started_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS completed_at TIMESTAMP WITH TIME ZONE;

ALTER TYPE order_status ADD VALUE IF NOT EXISTS 'on_way' AFTER 'taken';
ALTER TYPE order_status ADD VALUE IF NOT EXISTS 'arrived' AFTER 'on_way';
ALTER TYPE order_status ADD VALUE IF NOT EXISTS 'in_progress' AFTER 'arrived';

-- Seed Brands
INSERT INTO car_brands (name) VALUES 
('LADA'), ('Hyundai'), ('Kia'), ('Toyota'), ('Volkswagen'), ('Mercedes'), ('BMW'), ('Chevrolet'), ('Daewoo');

-- Seed Models (Sample)
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='LADA'), 'Vesta'),
((SELECT id FROM car_brands WHERE name='LADA'), 'Granta'),
((SELECT id FROM car_brands WHERE name='LADA'), 'Largus'),
((SELECT id FROM car_brands WHERE name='Hyundai'), 'Solaris'),
((SELECT id FROM car_brands WHERE name='Hyundai'), 'Creta'),
((SELECT id FROM car_brands WHERE name='Kia'), 'Rio'),
((SELECT id FROM car_brands WHERE name='Kia'), 'K5'),
((SELECT id FROM car_brands WHERE name='Toyota'), 'Camry'),
((SELECT id FROM car_brands WHERE name='Toyota'), 'Corolla'),
((SELECT id FROM car_brands WHERE name='Chevrolet'), 'Cobalt'),
((SELECT id FROM car_brands WHERE name='Chevrolet'), 'Lacetti'),
((SELECT id FROM car_brands WHERE name='Daewoo'), 'Nexia'),
((SELECT id FROM car_brands WHERE name='Daewoo'), 'Gentra');
