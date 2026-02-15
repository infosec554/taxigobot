-- Down Migration - Remove all added car models and brands from previous migration

-- Delete models added for new brands
DELETE FROM car_models WHERE brand_id IN (
    SELECT id FROM car_brands WHERE name IN (
        'Gazelle', 'UAZ', 'Volga', 'Kamza', 'IVECO', 'Ford', 'Skoda', 
        'Renault', 'Mitsubishi', 'Honda', 'Nissan', 'Mazda', 'Suzuki',
        'Geely', 'Chery', 'BYD', 'Opel', 'Audi', 'Volkswagen Kombi'
    )
);

-- Delete extended models for existing brands
DELETE FROM car_models WHERE name IN (
    'Niva', '2110', '2111', '2112', 'Kalina', 'Priora', 'Xray',
    'Gazelle 2700', 'Gazelle 3221', 'Gazelle NEXT', 'Gazelle BUSINESS',
    'Patriot', 'Hunter', 'Bukhanka', '469',
    'Accent', 'i30', 'Elantra', 'Sonata', 'Tucson',
    'Picanto', 'Cerato', 'Optima', 'Sorento', 'Sportage',
    'Yaris', 'Auris', 'Avensis', 'RAV4', 'Highlander', 'Prius', 'Land Cruiser',
    'Aveo', 'Cruze', 'Malibu', 'Orlando', 'Captiva',
    'Matiz', 'Kalos', 'Evanda',
    'Fiesta', 'Focus', 'Mondeo', 'Fusion', 'Kuga',
    'Logan', 'Sandero', 'Duster', 'Megane', 'Laguna', 'Scenic',
    'Octavia', 'Fabia', 'Superb', 'Yeti',
    'Almera', 'Teana', 'Qashqai', 'X-Trail', 'Patrol',
    'Lancer', 'Outlander', 'Pajero', 'ASX',
    'Jazz', 'Civic', 'Accord', 'CR-V', 'Odyssey',
    'Mazda2', 'Mazda3', 'Mazda5', 'Mazda6', 'CX-5', 'CX-7',
    'Alto', 'Swift', 'SX4', 'Vitara', 'Jimny',
    'MK', 'MK2', 'Emgrand', 'Otaka',
    'QQ', 'Tiggo', 'Amulet', 'Jaggi',
    'Seagull', 'Yuan', 'Song',
    'A-Class', 'C-Class', 'E-Class', 'S-Class', 'GLA', 'GLC',
    '3 Series', '5 Series', '7 Series', 'X1', 'X3', 'X5',
    'Polo', 'Golf', 'Passat', 'Jetta', 'Tiguan', 'Touareg',
    'A1', 'A3', 'A4', 'A6', 'Q3', 'Q5',
    'Corsa', 'Astra', 'Vectra', 'Insignia'
);

-- Delete new brands
DELETE FROM car_brands WHERE name IN (
    'Gazelle', 'UAZ', 'Volga', 'Kamaz', 'IVECO', 'Ford', 'Skoda', 
    'Renault', 'Mitsubishi', 'Honda', 'Nissan', 'Mazda', 'Suzuki',
    'Geely', 'Chery', 'BYD', 'Opel', 'Audi', 'Volkswagen Kombi'
);
