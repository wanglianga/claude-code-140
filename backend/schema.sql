-- 居民楼屋顶光伏认购发电结算与维修平台 数据库结构
-- PostgreSQL 13+

-- ============ 楼栋 ============
CREATE TABLE IF NOT EXISTS buildings (
  id                        BIGSERIAL PRIMARY KEY,
  code                      TEXT UNIQUE NOT NULL,
  name                      TEXT NOT NULL,
  address                   TEXT NOT NULL,
  rooftop_area              NUMERIC(10,2) NOT NULL DEFAULT 0,    -- 屋顶面积 m²
  orientation               TEXT NOT NULL DEFAULT '',            -- 朝向
  load_assessment           TEXT NOT NULL DEFAULT '',            -- 承重评估
  shading                   TEXT NOT NULL DEFAULT '',            -- 遮挡情况
  property_rights           TEXT NOT NULL DEFAULT '',            -- 产权关系
  public_benefit_rule       TEXT NOT NULL DEFAULT '',            -- 公共收益规则
  grid_condition            TEXT NOT NULL DEFAULT '',            -- 并网条件
  construction_impact       TEXT NOT NULL DEFAULT '',            -- 施工影响
  total_capacity_kw         NUMERIC(10,2) NOT NULL DEFAULT 0,    -- 装机容量 kWp
  component_count           INT NOT NULL DEFAULT 0,              -- 组件总数
  component_unit_price      NUMERIC(12,2) NOT NULL DEFAULT 0,    -- 单组件价格(元)
  public_share_pct          NUMERIC(6,3) NOT NULL DEFAULT 20,    -- 公共部分比例 %
  vote_threshold_pct        NUMERIC(6,3) NOT NULL DEFAULT 67,    -- 表决通过门槛(面积/户数双过半) %
  electricity_price         NUMERIC(8,4) NOT NULL DEFAULT 0.39,  -- 上网/结算电价 元/kWh
  subsidy_rate              NUMERIC(8,4) NOT NULL DEFAULT 0.05,  -- 度电补贴 元/kWh
  maintenance_fee_pct       NUMERIC(6,3) NOT NULL DEFAULT 5,     -- 运维费占毛收益比例 %
  tax_pct                   NUMERIC(6,3) NOT NULL DEFAULT 3,     -- 税费比例 %
  repair_fund_pct           NUMERIC(6,3) NOT NULL DEFAULT 3,     -- 维修基金计提比例 %
  stage                     TEXT NOT NULL DEFAULT 'INIT',         -- INIT/VOTING/SUBSCRIBED/SCHEDULED/GRID_CONNECTED/OPERATING
  created_at                TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============ 平台管理员(全局) ============
CREATE TABLE IF NOT EXISTS admins (
  id              BIGSERIAL PRIMARY KEY,
  username        TEXT UNIQUE NOT NULL,
  password        TEXT NOT NULL,
  name            TEXT NOT NULL
);

-- ============ 住户 ============
CREATE TABLE IF NOT EXISTS residents (
  id              BIGSERIAL PRIMARY KEY,
  building_id     BIGINT NOT NULL REFERENCES buildings(id),
  unit_no         TEXT NOT NULL,                 -- 房号
  owner_name      TEXT NOT NULL,
  phone           TEXT NOT NULL DEFAULT '',
  role            TEXT NOT NULL DEFAULT 'RESIDENT', -- ADMIN(平台管理员)/PROPERTY(物业)/RESIDENT
  area_owned      NUMERIC(10,2) NOT NULL DEFAULT 0, -- 产权面积 m²
  is_top_floor    BOOLEAN NOT NULL DEFAULT FALSE,   -- 是否顶楼
  active          BOOLEAN NOT NULL DEFAULT TRUE,    -- 房屋转让后原业主停用
  password        TEXT NOT NULL DEFAULT '',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(building_id, unit_no)
);

-- ============ 认购记录 ============
-- mode: SUBSCRIBE 认购组件 / PUBLIC_ONLY 只参与公共收益分配
-- status: PENDING 待缴费 / PAID 已缴费 / REFUNDED 已退款
CREATE TABLE IF NOT EXISTS subscriptions (
  id              BIGSERIAL PRIMARY KEY,
  building_id     BIGINT NOT NULL REFERENCES buildings(id),
  resident_id     BIGINT NOT NULL REFERENCES residents(id),
  mode            TEXT NOT NULL DEFAULT 'SUBSCRIBE',
  component_qty   INT NOT NULL DEFAULT 0,
  amount_paid     NUMERIC(12,2) NOT NULL DEFAULT 0,
  status          TEXT NOT NULL DEFAULT 'PENDING',
  subscribe_pct   NUMERIC(6,3) NOT NULL DEFAULT 0,  -- 占总组件比例(冗余,结算用)
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  paid_at         TIMESTAMPTZ,
  UNIQUE(building_id, resident_id)
);

-- ============ 付款/退款流水 ============
CREATE TABLE IF NOT EXISTS payments (
  id              BIGSERIAL PRIMARY KEY,
  building_id     BIGINT NOT NULL REFERENCES buildings(id),
  resident_id     BIGINT NOT NULL REFERENCES residents(id),
  subscription_id BIGINT REFERENCES subscriptions(id),
  direction       TEXT NOT NULL,                 -- IN 认购缴款 / OUT 退款 / OUT_GAIN 收益发放
  amount          NUMERIC(12,2) NOT NULL,
  reason          TEXT NOT NULL DEFAULT '',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============ 楼栋表决票(项目引入/异议表决通用) ============
-- topic_type: PROJECT 项目立项表决 / OBJECTION 异议投票
-- vote: APPROVE / REJECT / ABSTAIN
CREATE TABLE IF NOT EXISTS votes (
  id              BIGSERIAL PRIMARY KEY,
  building_id     BIGINT NOT NULL REFERENCES buildings(id),
  resident_id     BIGINT NOT NULL REFERENCES residents(id),
  topic_type      TEXT NOT NULL DEFAULT 'PROJECT',
  topic_id        BIGINT NOT NULL DEFAULT 0,
  vote            TEXT NOT NULL,
  opinion         TEXT NOT NULL DEFAULT '',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(building_id, resident_id, topic_type, topic_id)
);

-- ============ 施工排期事件(供应延迟/并网变更留痕) ============
CREATE TABLE IF NOT EXISTS schedule_events (
  id              BIGSERIAL PRIMARY KEY,
  building_id     BIGINT NOT NULL REFERENCES buildings(id),
  kind            TEXT NOT NULL,        -- PLAN 计划 / DELAY 供应延迟 / GRID_CHANGE 并网变更 / MILESTONE 里程碑
  title           TEXT NOT NULL,
  detail          TEXT NOT NULL DEFAULT '',
  planned_on      DATE,
  actual_on       DATE,
  created_by      BIGINT REFERENCES residents(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============ 设备:逆变器/组件组 ============
CREATE TABLE IF NOT EXISTS devices (
  id              BIGSERIAL PRIMARY KEY,
  building_id     BIGINT NOT NULL REFERENCES buildings(id),
  code            TEXT NOT NULL,        -- 如 INV-01
  name            TEXT NOT NULL,
  device_type     TEXT NOT NULL DEFAULT 'INVERTER',
  component_from  INT NOT NULL,         -- 管理组件起始编号
  component_to    INT NOT NULL,         -- 管理组件结束编号
  capacity_kw     NUMERIC(10,2) NOT NULL DEFAULT 0,
  status          TEXT NOT NULL DEFAULT 'NORMAL', -- NORMAL/FAULT/REPAIRED
  installed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(building_id, code)
);

-- ============ 月度发电读数(按设备/逆变器) ============
CREATE TABLE IF NOT EXISTS meter_readings (
  id              BIGSERIAL PRIMARY KEY,
  building_id     BIGINT NOT NULL REFERENCES buildings(id),
  device_id       BIGINT NOT NULL REFERENCES devices(id),
  period          TEXT NOT NULL,        -- YYYY-MM
  reading_kwh     NUMERIC(12,2) NOT NULL,
  note            TEXT NOT NULL DEFAULT '',
  recorded_by     BIGINT REFERENCES residents(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(device_id, period)
);

-- ============ 发电异常解释 ============
CREATE TABLE IF NOT EXISTS generation_notes (
  id              BIGSERIAL PRIMARY KEY,
  building_id     BIGINT NOT NULL REFERENCES buildings(id),
  period          TEXT NOT NULL,
  title           TEXT NOT NULL,
  explanation     TEXT NOT NULL DEFAULT '',
  created_by      BIGINT REFERENCES residents(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============ 维修工单 ============
-- category: INVERTER_FAULT 逆变器故障 / LEAK 屋顶漏水 / OTHER
-- status: OPEN 处理中 / RESOLVED 已解决 / CLOSED 已关闭
-- responsibility: PROPERTY 物业 / SUPPLIER 设备供应商 / INSTALLER 施工方 / RESIDENT / PUBLIC
CREATE TABLE IF NOT EXISTS repair_orders (
  id                BIGSERIAL PRIMARY KEY,
  building_id       BIGINT NOT NULL REFERENCES buildings(id),
  device_id         BIGINT REFERENCES devices(id),
  reporter_id       BIGINT REFERENCES residents(id),
  code              TEXT NOT NULL,
  category          TEXT NOT NULL,
  title             TEXT NOT NULL,
  description       TEXT NOT NULL DEFAULT '',
  status            TEXT NOT NULL DEFAULT 'OPEN',
  responsibility    TEXT NOT NULL DEFAULT 'PUBLIC',
  cost              NUMERIC(12,2) NOT NULL DEFAULT 0,
  affect_period     TEXT,                      -- 影响的结算周期 YYYY-MM
  affected_kwh      NUMERIC(12,2) NOT NULL DEFAULT 0, -- 影响发电量(可恢复/损失)
  deductible_share  NUMERIC(6,3) NOT NULL DEFAULT 0,  -- 责任人承担收益扣减比例%
  resolution        TEXT NOT NULL DEFAULT '',
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at       TIMESTAMPTZ,
  UNIQUE(building_id, code)
);

-- ============ 月度结算单 ============
-- status: DRAFT 草稿 / CONFIRMED 已确认(入账) / ADJUSTED 已调整
CREATE TABLE IF NOT EXISTS settlements (
  id                BIGSERIAL PRIMARY KEY,
  building_id       BIGINT NOT NULL REFERENCES buildings(id),
  period            TEXT NOT NULL,           -- YYYY-MM
  total_kwh         NUMERIC(12,2) NOT NULL DEFAULT 0,
  gross_income      NUMERIC(12,2) NOT NULL DEFAULT 0,  -- 发电收入
  subsidy           NUMERIC(12,2) NOT NULL DEFAULT 0,  -- 补贴
  maintenance_fee   NUMERIC(12,2) NOT NULL DEFAULT 0,  -- 运维费
  tax               NUMERIC(12,2) NOT NULL DEFAULT 0,  -- 税费
  repair_fund       NUMERIC(12,2) NOT NULL DEFAULT 0,  -- 计提维修基金
  repair_cost       NUMERIC(12,2) NOT NULL DEFAULT 0,  -- 本期维修支出
  deduction         NUMERIC(12,2) NOT NULL DEFAULT 0,  -- 责任方赔偿/扣减
  net_income        NUMERIC(12,2) NOT NULL DEFAULT 0,  -- 可分配净额
  public_income     NUMERIC(12,2) NOT NULL DEFAULT 0,  -- 公共部分收益
  status            TEXT NOT NULL DEFAULT 'DRAFT',
  note              TEXT NOT NULL DEFAULT '',
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  confirmed_at      TIMESTAMPTZ,
  UNIQUE(building_id, period)
);

-- ============ 结算明细(住户/公共账户) ============
CREATE TABLE IF NOT EXISTS settlement_items (
  id                BIGSERIAL PRIMARY KEY,
  settlement_id     BIGINT NOT NULL REFERENCES settlements(id) ON DELETE CASCADE,
  building_id       BIGINT NOT NULL REFERENCES buildings(id),
  resident_id       BIGINT REFERENCES residents(id),  -- NULL=公共账户
  kind              TEXT NOT NULL,        -- SUBSCRIBER 认购住户 / PUBLIC 公共
  share_pct         NUMERIC(6,3) NOT NULL DEFAULT 0,
  allocated_kwh     NUMERIC(12,2) NOT NULL DEFAULT 0,
  amount            NUMERIC(12,2) NOT NULL DEFAULT 0,
  UNIQUE(settlement_id, resident_id)
);

-- ============ 公共账户流水 ============
-- type: SUBSIDY 补贴 / SETTLEMENT 结算入账 / REPAIR 维修支出 / MAINTENANCE 运维
--       REFUND 退款 / COMPENSATION 责任赔偿 / BENEFIT 公共收益发放 / TRANSFER
CREATE TABLE IF NOT EXISTS public_transactions (
  id                BIGSERIAL PRIMARY KEY,
  building_id       BIGINT NOT NULL REFERENCES buildings(id),
  period            TEXT,
  type              TEXT NOT NULL,
  direction         TEXT NOT NULL,          -- IN / OUT
  amount            NUMERIC(12,2) NOT NULL,
  balance_after     NUMERIC(12,2) NOT NULL,
  ref_type          TEXT NOT NULL DEFAULT '',
  ref_id            BIGINT,
  remark            TEXT NOT NULL DEFAULT '',
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============ 房屋/权益转让 ============
-- transfer_subscription: true=认购权益随房转移, false=不转移(原业主退出退款)
-- status: PENDING / SETTLED(历史收益已结清) / TRANSFERRED / REFUNDED
CREATE TABLE IF NOT EXISTS ownership_transfers (
  id                  BIGSERIAL PRIMARY KEY,
  building_id         BIGINT NOT NULL REFERENCES buildings(id),
  subscription_id     BIGINT REFERENCES subscriptions(id),
  from_resident_id    BIGINT NOT NULL REFERENCES residents(id),
  to_resident_id      BIGINT REFERENCES residents(id),
  new_owner_name      TEXT NOT NULL DEFAULT '',
  new_phone           TEXT NOT NULL DEFAULT '',
  transfer_subscription BOOLEAN NOT NULL DEFAULT TRUE,
  settle_history      BOOLEAN NOT NULL DEFAULT FALSE,  -- 历史收益是否结清
  settled_amount      NUMERIC(12,2) NOT NULL DEFAULT 0,
  refund_amount       NUMERIC(12,2) NOT NULL DEFAULT 0,
  status              TEXT NOT NULL DEFAULT 'PENDING',
  remark              TEXT NOT NULL DEFAULT '',
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at        TIMESTAMPTZ
);

-- ============ 楼栋公告 ============
CREATE TABLE IF NOT EXISTS announcements (
  id              BIGSERIAL PRIMARY KEY,
  building_id     BIGINT NOT NULL REFERENCES buildings(id),
  title           TEXT NOT NULL,
  content         TEXT NOT NULL DEFAULT '',
  created_by      BIGINT REFERENCES residents(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============ 异议(可发起住户投票) ============
-- status: OPEN 受理中 / VOTING 投票中 / APPROVED 异议成立 / REJECTED 异议驳回 / CLOSED
CREATE TABLE IF NOT EXISTS objections (
  id              BIGSERIAL PRIMARY KEY,
  building_id     BIGINT NOT NULL REFERENCES buildings(id),
  resident_id     BIGINT NOT NULL REFERENCES residents(id),
  title           TEXT NOT NULL,
  content         TEXT NOT NULL DEFAULT '',
  status          TEXT NOT NULL DEFAULT 'OPEN',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  closed_at       TIMESTAMPTZ
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_residents_b ON residents(building_id);
CREATE INDEX IF NOT EXISTS idx_subs_b ON subscriptions(building_id);
CREATE INDEX IF NOT EXISTS idx_meter_bp ON meter_readings(building_id, period);
CREATE INDEX IF NOT EXISTS idx_repair_bp ON repair_orders(building_id, affect_period);
CREATE INDEX IF NOT EXISTS idx_ptx_b ON public_transactions(building_id);
CREATE INDEX IF NOT EXISTS idx_settle_bp ON settlements(building_id, period);
