-- Add more popular Russian and CIS car models

-- Add more Brands
INSERT INTO car_brands (name) VALUES 
('Gazelle'),
('UAZ'),
('Volga'),
('Kamaz'),
('IVECO'),
('Ford'),
('Skoda'),
('Renault'),
('Mitsubishi'),
('Honda'),
('Nissan'),
('Mazda'),
('Suzuki'),
('Geely'),
('Chery'),
('BYD'),
('Opel'),
('Audi'),
('Volkswagen Kombi')
ON CONFLICT (name) DO NOTHING;

-- Add more LADA models (most popular in Russia/Central Asia)
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='LADA'), 'Niva'),
((SELECT id FROM car_brands WHERE name='LADA'), '2110'),
((SELECT id FROM car_brands WHERE name='LADA'), '2111'),
((SELECT id FROM car_brands WHERE name='LADA'), '2112'),
((SELECT id FROM car_brands WHERE name='LADA'), 'Kalina'),
((SELECT id FROM car_brands WHERE name='LADA'), 'Priora'),
((SELECT id FROM car_brands WHERE name='LADA'), 'Xray')
ON CONFLICT DO NOTHING;

-- Add Gazelle models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Gazelle'), 'Gazelle 2700'),
((SELECT id FROM car_brands WHERE name='Gazelle'), 'Gazelle 3221'),
((SELECT id FROM car_brands WHERE name='Gazelle'), 'Gazelle NEXT'),
((SELECT id FROM car_brands WHERE name='Gazelle'), 'Gazelle BUSINESS')
ON CONFLICT DO NOTHING;

-- Add UAZ models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='UAZ'), 'Patriot'),
((SELECT id FROM car_brands WHERE name='UAZ'), 'Hunter'),
((SELECT id FROM car_brands WHERE name='UAZ'), 'Bukhanka'),
((SELECT id FROM car_brands WHERE name='UAZ'), '469')
ON CONFLICT DO NOTHING;

-- Add Hyundai models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Hyundai'), 'Accent'),
((SELECT id FROM car_brands WHERE name='Hyundai'), 'i30'),
((SELECT id FROM car_brands WHERE name='Hyundai'), 'Elantra'),
((SELECT id FROM car_brands WHERE name='Hyundai'), 'Sonata'),
((SELECT id FROM car_brands WHERE name='Hyundai'), 'Tucson')
ON CONFLICT DO NOTHING;

-- Add Kia models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Kia'), 'Picanto'),
((SELECT id FROM car_brands WHERE name='Kia'), 'Cerato'),
((SELECT id FROM car_brands WHERE name='Kia'), 'Optima'),
((SELECT id FROM car_brands WHERE name='Kia'), 'Sorento'),
((SELECT id FROM car_brands WHERE name='Kia'), 'Sportage')
ON CONFLICT DO NOTHING;

-- Add Toyota models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Toyota'), 'Yaris'),
((SELECT id FROM car_brands WHERE name='Toyota'), 'Auris'),
((SELECT id FROM car_brands WHERE name='Toyota'), 'Avensis'),
((SELECT id FROM car_brands WHERE name='Toyota'), 'RAV4'),
((SELECT id FROM car_brands WHERE name='Toyota'), 'Highlander'),
((SELECT id FROM car_brands WHERE name='Toyota'), 'Prius'),
((SELECT id FROM car_brands WHERE name='Toyota'), 'Land Cruiser')
ON CONFLICT DO NOTHING;

-- Add Chevrolet models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Chevrolet'), 'Aveo'),
((SELECT id FROM car_brands WHERE name='Chevrolet'), 'Cruze'),
((SELECT id FROM car_brands WHERE name='Chevrolet'), 'Malibu'),
((SELECT id FROM car_brands WHERE name='Chevrolet'), 'Orlando'),
((SELECT id FROM car_brands WHERE name='Chevrolet'), 'Captiva')
ON CONFLICT DO NOTHING;

-- Add Daewoo models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Daewoo'), 'Matiz'),
((SELECT id FROM car_brands WHERE name='Daewoo'), 'Kalos'),
((SELECT id FROM car_brands WHERE name='Daewoo'), 'Evanda')
ON CONFLICT DO NOTHING;

-- Add Ford models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Ford'), 'Fiesta'),
((SELECT id FROM car_brands WHERE name='Ford'), 'Focus'),
((SELECT id FROM car_brands WHERE name='Ford'), 'Mondeo'),
((SELECT id FROM car_brands WHERE name='Ford'), 'Fusion'),
((SELECT id FROM car_brands WHERE name='Ford'), 'Kuga')
ON CONFLICT DO NOTHING;

-- Add Renault models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Renault'), 'Logan'),
((SELECT id FROM car_brands WHERE name='Renault'), 'Sandero'),
((SELECT id FROM car_brands WHERE name='Renault'), 'Duster'),
((SELECT id FROM car_brands WHERE name='Renault'), 'Megane'),
((SELECT id FROM car_brands WHERE name='Renault'), 'Laguna'),
((SELECT id FROM car_brands WHERE name='Renault'), 'Scenic')
ON CONFLICT DO NOTHING;

-- Add Skoda models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Skoda'), 'Octavia'),
((SELECT id FROM car_brands WHERE name='Skoda'), 'Fabia'),
((SELECT id FROM car_brands WHERE name='Skoda'), 'Superb'),
((SELECT id FROM car_brands WHERE name='Skoda'), 'Yeti')
ON CONFLICT DO NOTHING;

-- Add Nissan models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Nissan'), 'Almera'),
((SELECT id FROM car_brands WHERE name='Nissan'), 'Teana'),
((SELECT id FROM car_brands WHERE name='Nissan'), 'Qashqai'),
((SELECT id FROM car_brands WHERE name='Nissan'), 'X-Trail'),
((SELECT id FROM car_brands WHERE name='Nissan'), 'Patrol')
ON CONFLICT DO NOTHING;

-- Add Mitsubishi models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Mitsubishi'), 'Lancer'),
((SELECT id FROM car_brands WHERE name='Mitsubishi'), 'Outlander'),
((SELECT id FROM car_brands WHERE name='Mitsubishi'), 'Pajero'),
((SELECT id FROM car_brands WHERE name='Mitsubishi'), 'ASX')
ON CONFLICT DO NOTHING;

-- Add Honda models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Honda'), 'Jazz'),
((SELECT id FROM car_brands WHERE name='Honda'), 'Civic'),
((SELECT id FROM car_brands WHERE name='Honda'), 'Accord'),
((SELECT id FROM car_brands WHERE name='Honda'), 'CR-V'),
((SELECT id FROM car_brands WHERE name='Honda'), 'Odyssey')
ON CONFLICT DO NOTHING;

-- Add Mazda models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Mazda'), 'Mazda2'),
((SELECT id FROM car_brands WHERE name='Mazda'), 'Mazda3'),
((SELECT id FROM car_brands WHERE name='Mazda'), 'Mazda5'),
((SELECT id FROM car_brands WHERE name='Mazda'), 'Mazda6'),
((SELECT id FROM car_brands WHERE name='Mazda'), 'CX-5'),
((SELECT id FROM car_brands WHERE name='Mazda'), 'CX-7')
ON CONFLICT DO NOTHING;

-- Add Suzuki models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Suzuki'), 'Alto'),
((SELECT id FROM car_brands WHERE name='Suzuki'), 'Swift'),
((SELECT id FROM car_brands WHERE name='Suzuki'), 'SX4'),
((SELECT id FROM car_brands WHERE name='Suzuki'), 'Vitara'),
((SELECT id FROM car_brands WHERE name='Suzuki'), 'Jimny')
ON CONFLICT DO NOTHING;

-- Add Geely models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Geely'), 'MK'),
((SELECT id FROM car_brands WHERE name='Geely'), 'MK2'),
((SELECT id FROM car_brands WHERE name='Geely'), 'Emgrand'),
((SELECT id FROM car_brands WHERE name='Geely'), 'Otaka')
ON CONFLICT DO NOTHING;

-- Add Chery models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Chery'), 'QQ'),
((SELECT id FROM car_brands WHERE name='Chery'), 'Tiggo'),
((SELECT id FROM car_brands WHERE name='Chery'), 'Amulet'),
((SELECT id FROM car_brands WHERE name='Chery'), 'Jaggi')
ON CONFLICT DO NOTHING;

-- Add BYD models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='BYD'), 'Seagull'),
((SELECT id FROM car_brands WHERE name='BYD'), 'Yuan'),
((SELECT id FROM car_brands WHERE name='BYD'), 'Song')
ON CONFLICT DO NOTHING;

-- Add Mercedes models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Mercedes'), 'A-Class'),
((SELECT id FROM car_brands WHERE name='Mercedes'), 'C-Class'),
((SELECT id FROM car_brands WHERE name='Mercedes'), 'E-Class'),
((SELECT id FROM car_brands WHERE name='Mercedes'), 'S-Class'),
((SELECT id FROM car_brands WHERE name='Mercedes'), 'GLA'),
((SELECT id FROM car_brands WHERE name='Mercedes'), 'GLC')
ON CONFLICT DO NOTHING;

-- Add BMW models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='BMW'), '3 Series'),
((SELECT id FROM car_brands WHERE name='BMW'), '5 Series'),
((SELECT id FROM car_brands WHERE name='BMW'), '7 Series'),
((SELECT id FROM car_brands WHERE name='BMW'), 'X1'),
((SELECT id FROM car_brands WHERE name='BMW'), 'X3'),
((SELECT id FROM car_brands WHERE name='BMW'), 'X5')
ON CONFLICT DO NOTHING;

-- Add Volkswagen models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Volkswagen'), 'Polo'),
((SELECT id FROM car_brands WHERE name='Volkswagen'), 'Golf'),
((SELECT id FROM car_brands WHERE name='Volkswagen'), 'Passat'),
((SELECT id FROM car_brands WHERE name='Volkswagen'), 'Jetta'),
((SELECT id FROM car_brands WHERE name='Volkswagen'), 'Tiguan'),
((SELECT id FROM car_brands WHERE name='Volkswagen'), 'Touareg')
ON CONFLICT DO NOTHING;

-- Add Audi models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Audi'), 'A1'),
((SELECT id FROM car_brands WHERE name='Audi'), 'A3'),
((SELECT id FROM car_brands WHERE name='Audi'), 'A4'),
((SELECT id FROM car_brands WHERE name='Audi'), 'A6'),
((SELECT id FROM car_brands WHERE name='Audi'), 'Q3'),
((SELECT id FROM car_brands WHERE name='Audi'), 'Q5')
ON CONFLICT DO NOTHING;

-- Add Opel models
INSERT INTO car_models (brand_id, name) VALUES
((SELECT id FROM car_brands WHERE name='Opel'), 'Corsa'),
((SELECT id FROM car_brands WHERE name='Opel'), 'Astra'),
((SELECT id FROM car_brands WHERE name='Opel'), 'Vectra'),
((SELECT id FROM car_brands WHERE name='Opel'), 'Insignia')
ON CONFLICT DO NOTHING;
