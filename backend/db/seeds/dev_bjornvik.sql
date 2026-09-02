-- Development-only Bjornvik seed. Run after all migrations.
BEGIN;

INSERT INTO worlds (id, name, historical_start_date, current_tick, tick_duration_seconds, next_tick_at)
VALUES ('00000000-0000-0000-0000-000000000001', 'Development World', DATE '0980-01-01', 0, 14400, now());

INSERT INTO locations (id, world_id, name, location_type) VALUES
('00000000-0000-0000-0000-000000000010','00000000-0000-0000-0000-000000000001','Bjornvik','farm'),
('00000000-0000-0000-0000-000000000011','00000000-0000-0000-0000-000000000001','Hrafnstead','farm');

INSERT INTO households (id, world_id, location_id, name, specialization, created_tick)
VALUES
('00000000-0000-0000-0000-000000000020','00000000-0000-0000-0000-000000000001','00000000-0000-0000-0000-000000000010','Bjornvik','fishing',0),
('00000000-0000-0000-0000-000000000021','00000000-0000-0000-0000-000000000001','00000000-0000-0000-0000-000000000011','Hrafnstead','agriculture',0);

INSERT INTO characters (id, household_id, name, birth_tick, labor_capacity_milli) VALUES
('00000000-0000-0000-0000-000000000101','00000000-0000-0000-0000-000000000020','Bjorn',-11680,1000),
('00000000-0000-0000-0000-000000000102','00000000-0000-0000-0000-000000000020','Astrid',-10585,1000),
('00000000-0000-0000-0000-000000000103','00000000-0000-0000-0000-000000000020','Einar',-6205,1000),
('00000000-0000-0000-0000-000000000104','00000000-0000-0000-0000-000000000020','Ragnhild',-4745,500),
('00000000-0000-0000-0000-000000000105','00000000-0000-0000-0000-000000000020','Sven',-2190,0);

INSERT INTO character_skills (character_id, skill_code, level) VALUES
('00000000-0000-0000-0000-000000000101','agriculture',1),
('00000000-0000-0000-0000-000000000102','fishing',1);

INSERT INTO resource_stocks (household_id, resource_code, quantity_milli) VALUES
('00000000-0000-0000-0000-000000000020','provisions',150000),
('00000000-0000-0000-0000-000000000020','wood',20000),
('00000000-0000-0000-0000-000000000020','trade_goods',4000),
('00000000-0000-0000-0000-000000000020','silver',30000),
-- Hrafnstead has 20 provisions left after dispatching the 30-unit demo shipment.
('00000000-0000-0000-0000-000000000021','provisions',20000);

INSERT INTO shipments (
    id, world_id, sender_household_id, receiver_household_id,
    origin_location_id, destination_location_id, resource_code,
    quantity_milli, departure_tick, expected_arrival_tick,
    transport_cost_milli, status
) VALUES (
    '00000000-0000-0000-0000-000000000301',
    '00000000-0000-0000-0000-000000000001',
    '00000000-0000-0000-0000-000000000021',
    '00000000-0000-0000-0000-000000000020',
    '00000000-0000-0000-0000-000000000011',
    '00000000-0000-0000-0000-000000000010',
    'provisions', 30000, 0, 2, 0, 'in_transit'
);

INSERT INTO market_offers (
    id, world_id, seller_household_id, origin_location_id, resource_code,
    quantity_remaining_milli, price_per_unit_milli, created_tick, expires_tick, status
) VALUES (
    '00000000-0000-0000-0000-000000000302',
    '00000000-0000-0000-0000-000000000001',
    '00000000-0000-0000-0000-000000000021',
    '00000000-0000-0000-0000-000000000011',
    'provisions', 10000, 1500, 0, 12, 'active'
);

COMMIT;
