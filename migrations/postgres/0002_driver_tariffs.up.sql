CREATE TABLE IF NOT EXISTS driver_tariffs (
    driver_id BIGINT REFERENCES users(id),
    tariff_id BIGINT REFERENCES tariffs(id),
    PRIMARY KEY (driver_id, tariff_id)
);
