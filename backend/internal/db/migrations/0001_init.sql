-- 居民楼屋顶光伏认购发电结算与维修平台 初始 schema
-- 金额单位: 元; 比例单位: % (0~100), numeric 精确存储

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT NOT NULL,
    phone         TEXT NOT NULL UNIQUE,
    unit_no       TEXT,                          -- 房号 如 1-501
    role          TEXT NOT NULL DEFAULT 'resident' CHECK (role IN ('resident','property','admin')),
    password_hash TEXT NOT NULL,
    balance       NUMERIC(12,2) NOT NULL DEFAULT 0,  -- 住户可提现/结转的现金余额
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE buildings (
    id                   BIGSERIAL PRIMARY KEY,
    name                 TEXT NOT NULL,
    address              TEXT NOT NULL DEFAULT '',
    roof_area            NUMERIC(10,2) NOT NULL DEFAULT 0,   -- 屋顶面积 m2
    orientation          TEXT NOT NULL DEFAULT '',           -- 朝向
    load_assessment      TEXT NOT NULL DEFAULT '',           -- 承重评估
    shading              TEXT NOT NULL DEFAULT '',           -- 遮挡情况
    ownership_note       TEXT NOT NULL DEFAULT '',           -- 产权关系
    public_revenue_rule  JSONB NOT NULL DEFAULT '{}'::jsonb, -- 公共收益规则
    grid_condition       TEXT NOT NULL DEFAULT '',           -- 并网条件
    construction_impact  TEXT NOT NULL DEFAULT '',           -- 施工影响
    total_capacity_kw    NUMERIC(10,2) NOT NULL DEFAULT 0,
    panel_total          INT NOT NULL DEFAULT 0,
    unit_price           NUMERIC(10,2) NOT NULL DEFAULT 0,   -- 每块组件认购单价
    subscribed_ratio     NUMERIC(7,4) NOT NULL DEFAULT 0,    -- 已认购比例% 冗余, 由认购汇总
    stage                TEXT NOT NULL DEFAULT 'draft'
        CHECK (stage IN ('draft','voting','subscribing','construction','grid_acceptance','operating')),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE building_members (
    building_id BIGINT NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    member_type TEXT NOT NULL DEFAULT 'resident' CHECK (member_type IN ('resident','property')),
    PRIMARY KEY (building_id, user_id)
);

CREATE TABLE stage_logs (
    id          BIGSERIAL PRIMARY KEY,
    building_id BIGINT NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    from_stage  TEXT,
    to_stage    TEXT NOT NULL,
    reason      TEXT NOT NULL DEFAULT '',   -- 调整原因: 漏水担忧/维护通道/收益质疑/供货延迟/并网变更
    operator_id BIGINT REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 楼栋立项表决
CREATE TABLE votes (
    id          BIGSERIAL PRIMARY KEY,
    building_id BIGINT NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','closed')),
    result      TEXT NOT NULL DEFAULT 'pending' CHECK (result IN ('pending','approved','rejected')),
    agree       INT NOT NULL DEFAULT 0,
    disagree    INT NOT NULL DEFAULT 0,
    abstain     INT NOT NULL DEFAULT 0,
    created_by  BIGINT REFERENCES users(id),
    closes_at   TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE vote_choices (
    vote_id BIGINT NOT NULL REFERENCES votes(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    choice  TEXT NOT NULL CHECK (choice IN ('agree','disagree','abstain')),
    weight  NUMERIC(6,2) NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (vote_id, user_id)
);

-- 认购
CREATE TABLE subscriptions (
    id          BIGSERIAL PRIMARY KEY,
    building_id BIGINT NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    sub_type    TEXT NOT NULL DEFAULT 'shareholder' CHECK (sub_type IN ('shareholder','public_only')),
    panel_ratio NUMERIC(7,4) NOT NULL DEFAULT 0,  -- 认购组件占比 %
    panel_count INT NOT NULL DEFAULT 0,
    amount      NUMERIC(12,2) NOT NULL DEFAULT 0,
    status      TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','paid','refunded','transferred')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at     TIMESTAMPTZ
);
CREATE INDEX idx_subs_building ON subscriptions(building_id);

-- 收款/付款流水
CREATE TABLE payments (
    id              BIGSERIAL PRIMARY KEY,
    building_id     BIGINT NOT NULL REFERENCES buildings(id),
    user_id         BIGINT REFERENCES users(id),
    amount          NUMERIC(12,2) NOT NULL,
    pay_type        TEXT NOT NULL CHECK (pay_type IN ('subscription','refund','subsidy','grid','compensation','other')),
    status          TEXT NOT NULL DEFAULT 'confirmed' CHECK (status IN ('confirmed','cancelled')),
    memo            TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 逆变器/组串
CREATE TABLE inverters (
    id             BIGSERIAL PRIMARY KEY,
    building_id    BIGINT NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    capacity_kw    NUMERIC(10,2) NOT NULL DEFAULT 0,
    capacity_ratio NUMERIC(7,4) NOT NULL DEFAULT 0,  -- 占整楼装机比例 %
    status         TEXT NOT NULL DEFAULT 'normal' CHECK (status IN ('normal','faulty','maintenance')),
    fault_note     TEXT NOT NULL DEFAULT '',
    fault_at       TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 发电计量 (inverter_id 为空表示楼总表读数)
CREATE TABLE generation_records (
    id          BIGSERIAL PRIMARY KEY,
    building_id BIGINT NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    inverter_id BIGINT REFERENCES inverters(id) ON DELETE CASCADE,
    period_month TEXT NOT NULL CHECK (period_month ~ '^\d{4}-\d{2}$'),
    kwh         NUMERIC(12,2) NOT NULL CHECK (kwh >= 0),
    note        TEXT NOT NULL DEFAULT '',
    created_by  BIGINT REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (building_id, inverter_id, period_month)
);

-- 发电异常与解释
CREATE TABLE generation_alerts (
    id           BIGSERIAL PRIMARY KEY,
    building_id  BIGINT NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    inverter_id  BIGINT REFERENCES inverters(id),
    period_month TEXT NOT NULL,
    reason       TEXT NOT NULL DEFAULT '',
    explanation  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','explained')),
    created_by   BIGINT REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    explained_at TIMESTAMPTZ
);

-- 维修工单 (漏水/逆变器故障等), 含责任判定
CREATE TABLE work_orders (
    id             BIGSERIAL PRIMARY KEY,
    building_id    BIGINT NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    inverter_id    BIGINT REFERENCES inverters(id),
    period_month   TEXT NOT NULL,
    title          TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    wo_type        TEXT NOT NULL DEFAULT 'other' CHECK (wo_type IN ('leak','inverter','channel','other')),
    responsibility TEXT NOT NULL DEFAULT 'undetermined'
        CHECK (responsibility IN ('undetermined','property','installer','owner','insurance')),
    cost           NUMERIC(12,2) NOT NULL DEFAULT 0,
    status         TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','processing','resolved')),
    reporter_id    BIGINT REFERENCES users(id),
    assignee       TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at    TIMESTAMPTZ
);

-- 月度结算
CREATE TABLE settlement_periods (
    id                  BIGSERIAL PRIMARY KEY,
    building_id         BIGINT NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    period_month        TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','finalized')),
    total_kwh           NUMERIC(12,2) NOT NULL DEFAULT 0,
    grid_price          NUMERIC(8,4) NOT NULL DEFAULT 0,  -- 上网电价 元/kWh
    subsidy_per_kwh     NUMERIC(8,4) NOT NULL DEFAULT 0,  -- 补贴 元/kWh
    gross_revenue       NUMERIC(12,2) NOT NULL DEFAULT 0,
    total_subsidy       NUMERIC(12,2) NOT NULL DEFAULT 0,
    om_fee              NUMERIC(12,2) NOT NULL DEFAULT 0,  -- 运维费
    tax                 NUMERIC(12,2) NOT NULL DEFAULT 0,  -- 税费
    work_order_cost     NUMERIC(12,2) NOT NULL DEFAULT 0,  -- 计入当期的维修净支出
    maintenance_fund_ratio NUMERIC(7,4) NOT NULL DEFAULT 20, -- 公共收益划入维修基金比例 %
    maintenance_fund_used  NUMERIC(12,2) NOT NULL DEFAULT 0,  -- 本期动用维修基金
    maintenance_fund_addition NUMERIC(12,2) NOT NULL DEFAULT 0,-- 本期计提维修基金
    compensation           NUMERIC(12,2) NOT NULL DEFAULT 0,  -- 安装商/保险赔偿
    public_ratio        NUMERIC(7,4) NOT NULL DEFAULT 0,
    distributable       NUMERIC(12,2) NOT NULL DEFAULT 0,
    created_by          BIGINT REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    finalized_at        TIMESTAMPTZ,
    UNIQUE (building_id, period_month)
);

CREATE TABLE settlement_items (
    id          BIGSERIAL PRIMARY KEY,
    period_id   BIGINT NOT NULL REFERENCES settlement_periods(id) ON DELETE CASCADE,
    user_id     BIGINT REFERENCES users(id),         -- NULL = 公共账户
    item_type   TEXT NOT NULL CHECK (item_type IN ('shareholder','public','maintenance_fund','work_order')),
    ratio       NUMERIC(7,4) NOT NULL DEFAULT 0,
    kwh         NUMERIC(12,2) NOT NULL DEFAULT 0,
    revenue     NUMERIC(12,2) NOT NULL DEFAULT 0,
    subsidy     NUMERIC(12,2) NOT NULL DEFAULT 0,
    om_share    NUMERIC(12,2) NOT NULL DEFAULT 0,
    tax_share   NUMERIC(12,2) NOT NULL DEFAULT 0,
    wo_share    NUMERIC(12,2) NOT NULL DEFAULT 0,
    amount      NUMERIC(12,2) NOT NULL DEFAULT 0,
    memo        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_items_period ON settlement_items(period_id);

-- 账户台账 (住户/公共账户统一流水)
CREATE TABLE ledger (
    id          BIGSERIAL PRIMARY KEY,
    building_id BIGINT NOT NULL REFERENCES buildings(id),
    user_id     BIGINT REFERENCES users(id),  -- NULL = 公共账户
    direction   TEXT NOT NULL CHECK (direction IN ('in','out')),
    amount      NUMERIC(12,2) NOT NULL CHECK (amount >= 0),
    category    TEXT NOT NULL, -- revenue/subsidy/public/maintenance_fund/repair/om/tax/refund/compensation/transfer_in
    period_month TEXT,
    ref_type    TEXT NOT NULL DEFAULT '',
    ref_id      BIGINT,
    memo        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ledger_building ON ledger(building_id, created_at);
CREATE INDEX idx_ledger_user ON ledger(user_id);

-- 房屋转让 / 退购
CREATE TABLE transfers (
    id              BIGSERIAL PRIMARY KEY,
    building_id     BIGINT NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    subscription_id BIGINT NOT NULL REFERENCES subscriptions(id),
    from_user_id    BIGINT NOT NULL REFERENCES users(id),
    to_user_id      BIGINT REFERENCES users(id),
    to_name         TEXT NOT NULL DEFAULT '',
    to_unit         TEXT NOT NULL DEFAULT '',
    rights_transfer BOOLEAN NOT NULL DEFAULT TRUE,   -- 认购权益是否随房转移
    refund_requested BOOLEAN NOT NULL DEFAULT FALSE,  -- 是否退购
    history_settled BOOLEAN NOT NULL DEFAULT FALSE,   -- 历史收益是否已结清
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','confirmed','rejected')),
    memo            TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at    TIMESTAMPTZ
);

-- 公告
CREATE TABLE announcements (
    id          BIGSERIAL PRIMARY KEY,
    building_id BIGINT NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    pinned      BOOLEAN NOT NULL DEFAULT FALSE,
    author_id   BIGINT REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 住户异议投票
CREATE TABLE polls (
    id          BIGSERIAL PRIMARY KEY,
    building_id BIGINT NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','closed')),
    result      TEXT NOT NULL DEFAULT 'pending' CHECK (result IN ('pending','approved','rejected')),
    agree       INT NOT NULL DEFAULT 0,
    disagree    INT NOT NULL DEFAULT 0,
    created_by  BIGINT REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE poll_votes (
    poll_id BIGINT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    support BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (poll_id, user_id)
);

-- 年度收益报告
CREATE TABLE annual_reports (
    id          BIGSERIAL PRIMARY KEY,
    building_id BIGINT NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    year        INT NOT NULL,
    summary     JSONB NOT NULL DEFAULT '{}'::jsonb,
    published   BOOLEAN NOT NULL DEFAULT FALSE,
    created_by  BIGINT REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (building_id, year)
);
