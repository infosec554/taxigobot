CREATE TABLE IF NOT EXISTS driver_routes (
    driver_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    from_location_id BIGINT REFERENCES locations(id) ON DELETE CASCADE,
    to_location_id BIGINT REFERENCES locations(id) ON DELETE CASCADE,
    PRIMARY KEY (driver_id, from_location_id, to_location_id)
);
