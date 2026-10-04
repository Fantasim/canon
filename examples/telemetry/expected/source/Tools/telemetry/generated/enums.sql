-- GrantKind
CREATE OR REPLACE VIEW enum_grant_kind AS
SELECT * FROM (VALUES
    (0, 'UNKNOWN'),
    (1, 'BUYBACK_DELIVERY'),
    (2, 'BUFF_REWARD'),
    (3, 'LEVEL_UP_GIFT'),
    (14, 'BOSS_REWARD_MAIL'),
    (15, 'WORLD_BOSS_REWARD_MAIL'),
    (16, 'EVENT_REWARD'),
    (22, 'MAIL_RETURN'),
    (23, 'PERIN_OVERFLOW'),
    (27, 'PLAYER_MAIL'),
    (35, 'FARM_HARVEST'),
    (47, 'STARTER_ITEM'),
    (48, 'GUILD_DISBAND_BANK'),
    (59, 'AUCTION_ESCROW_RECOVERY')
) AS t(value, name);

-- LifecycleKind
CREATE OR REPLACE VIEW enum_lifecycle_kind AS
SELECT * FROM (VALUES
    (1, 'Start'),
    (2, 'Stop')
) AS t(value, name);

-- FarmEventKind
CREATE OR REPLACE VIEW enum_farm_event_kind AS
SELECT * FROM (VALUES
    (1, 'Purchase'),
    (2, 'ClaimBatch'),
    (3, 'ClaimSlot'),
    (4, 'LevelUp'),
    (5, 'SetupModel'),
    (6, 'RemoveModel'),
    (7, 'UnlockModel'),
    (8, 'EnterOwn'),
    (9, 'VisitOther'),
    (10, 'Leave')
) AS t(value, name);

-- SovereignEventKind
CREATE OR REPLACE VIEW enum_sovereign_event_kind AS
SELECT * FROM (VALUES
    (1, 'RoleAssigned'),
    (2, 'DoctrineSet'),
    (3, 'PowerTriggered'),
    (4, 'RewardGranted')
) AS t(value, name);

-- SovereignRole
CREATE OR REPLACE VIEW enum_sovereign_role AS
SELECT * FROM (VALUES
    (0, 'None'),
    (1, 'Sovereign'),
    (2, 'MinisterMilitary'),
    (3, 'MinisterEconomy'),
    (4, 'MinisterEnvironment'),
    (5, 'MinisterCraftsmanship')
) AS t(value, name);

-- SovereignDoctrine
CREATE OR REPLACE VIEW enum_sovereign_doctrine AS
SELECT * FROM (VALUES
    (0, 'None'),
    (1, 'Mythic'),
    (2, 'Military'),
    (3, 'Merchant'),
    (4, 'Progressive'),
    (5, 'Chancellor'),
    (6, 'Craftsmanship'),
    (7, 'Familiar')
) AS t(value, name);

-- SovereignPower
CREATE OR REPLACE VIEW enum_sovereign_power AS
SELECT * FROM (VALUES
    (0, 'None'),
    (1, 'MinisterBuffPrimary'),
    (2, 'MinisterBuffSecondary'),
    (3, 'WorldBossSpawn'),
    (4, 'HiddenBonds'),
    (5, 'Heistia'),
    (6, 'AzuriaAuction'),
    (7, 'MinisterBuffTertiary'),
    (8, 'CustomDrop')
) AS t(value, name);

-- Which column of which table resolves through which view.
CREATE OR REPLACE VIEW enum_columns AS
SELECT * FROM (VALUES
    ('item_created', 'grant_kind', 'GrantKind', 'enum_grant_kind'),
    ('server_lifecycle', 'kind', 'LifecycleKind', 'enum_lifecycle_kind'),
    ('farm_events', 'kind', 'FarmEventKind', 'enum_farm_event_kind'),
    ('sovereign_events', 'kind', 'SovereignEventKind', 'enum_sovereign_event_kind'),
    ('sovereign_events', 'role', 'SovereignRole', 'enum_sovereign_role'),
    ('sovereign_events', 'doctrine', 'SovereignDoctrine', 'enum_sovereign_doctrine'),
    ('sovereign_events', 'power', 'SovereignPower', 'enum_sovereign_power')
) AS t(source_table, column_name, enum_name, lookup_view);

-- One row per live event type, in id order.
CREATE OR REPLACE VIEW event_catalog_static AS
SELECT * FROM (VALUES
    ('item_created', 11, 'WARM', true, 5),
    ('item_consumed', 31, 'READONLY', true, 3),
    ('penya_drops', 70, 'HOT', false, 2),
    ('trade_penya', 71, 'COOL', false, 2),
    ('server_lifecycle', 110, 'TINY', false, 1),
    ('farm_events', 151, 'COOL', false, 2),
    ('sovereign_events', 152, 'TINY', false, 1)
) AS t(source_table, event_type, tier, has_instance_id, event_version);
